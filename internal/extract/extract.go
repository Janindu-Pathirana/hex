package extract

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/Janindu-Pathirana/hex/internal/pdfx"
)

// Version identifies the rules; it is stored with every extraction run.
const Version = "rules-1"

// PointInTime is the period type of balance-sheet values.
const PointInTime = "PIT"

// Meta is what is known about a report before reading it.
type Meta struct {
	Annual    bool       // annual report (else interim)
	PeriodEnd time.Time  // from the report title
	Shares    float64    // shares in issue, for EPS/NAV cross-checks (0 = unknown)
	FYEnd     time.Month // financial year end month (0 = unknown)
}

// ytd returns the year-to-date period type at PeriodEnd ("6M" for June in a
// December year), or "" when the year end is unknown.
func (m Meta) ytd() string {
	if m.FYEnd == 0 || m.PeriodEnd.IsZero() {
		return ""
	}
	n := (int(m.PeriodEnd.Month()) - int(m.FYEnd) + 12) % 12
	if n == 0 {
		n = 12
	}
	return fmt.Sprintf("%dM", n)
}

// FYEndMonth maps the CSE profile code for the financial year end to a
// month: "1" is March, "2" is December (observed 2026-10).
func FYEndMonth(code string) time.Month {
	switch strings.TrimSpace(code) {
	case "1":
		return time.March
	case "2":
		return time.December
	}
	return 0
}

// Value is one extracted figure, in rupees (per-share fields in rupees per share).
type Value struct {
	Field      string
	Statement  string
	Scope      string
	PeriodEnd  time.Time
	PeriodType string // 3M, 6M, 9M, 12M, or PIT for balance sheets
	Value      float64
	Page       int
	Label      string // the label as printed
	Raw        string // the figure as printed
	Confidence float64
}

// Result is the outcome of extracting one report.
type Result struct {
	Values     []Value
	Pages      map[int]string // statement pages found: page → pl/bs/cf
	Confidence float64        // mean value confidence, 0 when nothing was found
	Issues     []string
}

type key struct {
	field, scope, ptype string
	end                 time.Time
}

type candidate struct {
	Value
	priority int
}

// Extract reads the statements in pages.
func Extract(pages []pdfx.Page, meta Meta) Result {
	res := Result{Pages: map[int]string{}}
	found := map[key]candidate{}
	var order []key
	docScale := 0.0
	for _, p := range pages {
		if s := unitScale(p.Text()); s > 0 && docScale == 0 && strings.Contains(strings.ToLower(p.Text()), "statement") {
			docScale = s
		}
	}

	for _, p := range pages {
		for _, sec := range sections(p) {
			vals, issues := sec.extract(meta, docScale)
			res.Issues = append(res.Issues, issues...)
			if len(vals) > 0 {
				res.Pages[p.N] = sec.kind
			}
			for _, v := range vals {
				k := key{v.Field, v.Scope, v.PeriodType, v.PeriodEnd}
				cur, seen := found[k]
				if !seen {
					order = append(order, k)
				}
				if !seen || v.priority < cur.priority {
					found[k] = v
				}
			}
		}
	}

	for _, k := range order {
		res.Values = append(res.Values, found[k].Value)
	}
	res.Issues = append(res.Issues, score(res.Values, meta)...)
	if len(res.Values) > 0 {
		sum := 0.0
		for _, v := range res.Values {
			sum += v.Confidence
		}
		res.Confidence = sum / float64(len(res.Values))
	} else {
		res.Issues = append(res.Issues, "no statement values found")
	}
	return res
}

// section is one statement on a page: its title line and the lines under it.
type section struct {
	page  int
	kind  string
	title string
	lines []pdfx.Line
}

// sections splits a page at statement title lines (a title may wrap onto a
// second line). Lines before the first title, and statements of other kinds,
// are dropped.
func sections(p pdfx.Page) []section {
	var out []section
	skip := false
	for i, l := range p.Lines {
		if skip {
			skip = false
			continue
		}
		text := l.Text()
		kind := ""
		if len(text) < 140 && !hasAmount(l) {
			kind = statementKind(text)
			if kind == "" && len(text) < 60 && i+1 < len(p.Lines) && !hasAmount(p.Lines[i+1]) {
				joined := text + " " + p.Lines[i+1].Text()
				if titleSkip.MatchString(joined) && statementKind(joined) == "" && titleLeadOnly(text) {
					// A wrapped title of a statement we skip (e.g. comprehensive
					// income): end the current section here.
					out = append(out, section{page: p.N, title: joined})
					skip = true
					continue
				}
				if kind = statementKind(joined); kind != "" {
					text, skip = joined, true
				}
			}
		}
		if kind != "" {
			out = append(out, section{page: p.N, kind: kind, title: text})
			continue
		}
		if n := len(out); n > 0 {
			out[n-1].lines = append(out[n-1].lines, l)
		}
	}
	kept := out[:0]
	for _, s := range out {
		if s.kind != "" {
			kept = append(kept, s)
		}
	}
	return kept
}

func hasAmount(l pdfx.Line) bool {
	for _, w := range l.Words {
		if isAmount(w.Text) {
			return true
		}
	}
	return false
}

func (s section) extract(meta Meta, docScale float64) ([]candidate, []string) {
	first := -1
	for i, l := range s.lines {
		if hasAmount(l) {
			first = i
			break
		}
	}
	if first < 0 {
		return nil, nil // a contents page or an empty heading
	}
	header := s.lines[:first]
	cols := headerColumns(header)
	if len(cols) == 0 {
		return nil, []string{fmt.Sprintf("page %d: %s %q has no dated columns", s.page, s.kind, s.title)}
	}

	defScope := ScopeGroup
	if titleCo.MatchString(s.title) && !titleScope.MatchString(s.title) {
		defScope = ScopeCompany
	}
	defDur := "3M"
	if meta.Annual {
		defDur = "12M"
	}
	var headText strings.Builder
	headText.WriteString(s.title + "\n")
	for i := range cols {
		if cols[i].scope == "" {
			cols[i].scope = defScope
		}
		switch {
		case s.kind == BalanceSheet:
			cols[i].duration = PointInTime
		case cols[i].duration == durYTD:
			cols[i].duration = meta.ytd()
			if cols[i].duration == "" {
				cols[i].duration = defDur
			}
		case cols[i].duration == "":
			cols[i].duration = defDur
		}
	}
	for _, l := range header {
		headText.WriteString(l.Text() + "\n")
	}

	current := currentColumns(cols, meta)
	if len(current) == 0 {
		return nil, []string{fmt.Sprintf("page %d: %s has no column for the report period", s.page, s.kind)}
	}

	var issues []string
	scale := unitScale(headText.String())
	if scale == 0 {
		for _, l := range s.lines {
			if scale = unitScale(l.Text()); scale > 0 {
				break
			}
		}
	}
	if scale == 0 {
		scale = docScale
	}
	if scale == 0 {
		scale = 1e3
		issues = append(issues, fmt.Sprintf("page %d: amount unit not stated, assumed Rs.'000", s.page))
	}

	var out []candidate
	caption := ""
	for _, r := range parseRows(s.lines[first:], cols) {
		if len(r.values) == 0 {
			if r.label != "" {
				caption = r.label
			}
			continue
		}
		fd, prio := matchField(s.kind, r.label)
		label := r.label
		if fd == nil && caption != "" {
			if r.label == "" {
				fd, prio = matchField(s.kind, "total "+caption)
			} else {
				fd, prio = matchField(s.kind, caption+" "+r.label)
			}
			label = strings.TrimSpace(caption + " " + r.label)
		}
		if fd == nil {
			continue
		}
		for ci, end := range current {
			v, ok := r.values[ci]
			if !ok {
				continue
			}
			if !fd.PerShare {
				v *= scale
			}
			if fd.Magnitude {
				v = math.Abs(v)
			}
			out = append(out, candidate{priority: prio, Value: Value{
				Field: fd.Name, Statement: s.kind, Scope: cols[ci].scope, PeriodEnd: end,
				PeriodType: cols[ci].duration, Value: v, Page: s.page, Label: label, Raw: r.raw[ci],
			}})
		}
	}
	return out, issues
}

// currentColumns picks the current-period column of each header block (its
// most recent date) that belongs to the report period, and returns its
// index → period end. When two blocks give the same scope and duration, the
// more recent wins (then the leftmost).
func currentColumns(cols []column, meta Meta) map[int]time.Time {
	type group struct{ scope, dur string }
	byBlock := map[int]int{}
	for i, c := range cols {
		if c.ignore {
			continue
		}
		if j, ok := byBlock[c.block]; !ok || c.dateKey() > cols[j].dateKey() {
			byBlock[c.block] = i
		}
	}
	var idx []int
	for _, i := range byBlock {
		idx = append(idx, i)
	}
	sort.Slice(idx, func(a, b int) bool {
		ka, kb := cols[idx[a]].dateKey(), cols[idx[b]].dateKey()
		if ka != kb {
			return ka > kb
		}
		return idx[a] < idx[b]
	})
	seen := map[group]bool{}
	out := map[int]time.Time{}
	for _, i := range idx {
		c := cols[i]
		g := group{c.scope, c.duration}
		if seen[g] {
			continue
		}
		var end time.Time
		switch {
		case !c.date.IsZero():
			if meta.PeriodEnd.IsZero() || !c.date.After(meta.PeriodEnd.AddDate(0, 0, 1)) {
				end = c.date
			}
		case meta.PeriodEnd.IsZero():
		case c.year == meta.PeriodEnd.Year(), c.fiscal && (c.year == meta.PeriodEnd.Year()+1):
			end = meta.PeriodEnd
		}
		if !end.IsZero() {
			seen[g] = true
			out[i] = end
		}
	}
	return out
}

// sortValues orders values for display: statement, scope, period, field.
func sortValues(vs []Value) {
	sort.SliceStable(vs, func(i, j int) bool {
		a, b := vs[i], vs[j]
		if a.Statement != b.Statement {
			return a.Statement > b.Statement // pl, cf, bs
		}
		if a.Scope != b.Scope {
			return a.Scope > b.Scope
		}
		return a.PeriodType < b.PeriodType
	})
}

// FieldByName returns the canonical field with that name, or nil.
func FieldByName(name string) *Field {
	for i := range Fields {
		if Fields[i].Name == name {
			return &Fields[i]
		}
	}
	return nil
}

// Finish scores values from any source with the accounting identities and
// fills Confidence and Issues, as Extract does for its own values.
func Finish(vals []Value, meta Meta, issues []string) Result {
	res := Result{Values: vals, Pages: map[int]string{}, Issues: issues}
	res.Issues = append(res.Issues, score(res.Values, meta)...)
	for _, v := range vals {
		res.Pages[v.Page] = v.Statement
	}
	if len(vals) > 0 {
		sum := 0.0
		for _, v := range res.Values {
			sum += v.Confidence
		}
		res.Confidence = sum / float64(len(vals))
	}
	return res
}

// CandidatePages returns the pages that carry a statement title, even when
// their columns could not be read: what a fallback reader should look at.
func CandidatePages(pages []pdfx.Page) []int {
	var out []int
	for _, p := range pages {
		if len(sections(p)) > 0 {
			out = append(out, p.N)
		}
	}
	return out
}

// ParseAmount parses a figure as printed in a statement ("(1,234)" is -1234).
func ParseAmount(s string) (float64, bool) { return parseNumber(s) }
