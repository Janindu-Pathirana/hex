package extract

import (
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Janindu-Pathirana/hex/internal/pdfx"
)

// Scopes of a statement column.
const (
	ScopeGroup   = "group"
	ScopeCompany = "company"
)

var (
	// Titles start the line, optionally after a number and qualifiers:
	// "Condensed Consolidated Income Statement", "1 Statement of Financial Position".
	titleLead = `(?i)^\W*(?:\d+\s+)?(?:(?:condensed|consolidated|interim|company|group|bank|separate|unaudited|summarised)\s+)*`
	titlePL   = regexp.MustCompile(titleLead + `(income statements?|statements? of (?:profit|income)|profit (?:and|&) loss (?:account|statement))`)
	titleBS   = regexp.MustCompile(titleLead + `(statements? of financial position|balance sheets?)`)
	titleCF   = regexp.MustCompile(titleLead + `(statements? of cash ?flows?|cash ?flow statement)`)
	// Titles that look like ours but are not primary statements, and
	// sentences that merely mention a statement.
	titleSkip  = regexp.MustCompile(`(?i)\b(changes in equity|notes? to|segment|comprehensive income\s*$)`)
	titleNoise = regexp.MustCompile(`(?i)\b(subsequent|reclassif|recycled|through|fvtpl|fvoci|amount|as per|recognised|included|presented|refer)`)

	scopeGroupWord   = regexp.MustCompile(`(?i)^[pvk]?(group|consolidated)[pvk]?$`)
	scopeCompanyWord = regexp.MustCompile(`(?i)^[pvk]?(company|bank)[pvk]?$`)
	// Percentages and ratios printed beside figures: 15%, (27), >100, -102%v.
	percentToken = regexp.MustCompile(`^(?:[<>]\(?\d+\)?|\(?-?\d[\d,.]*\)?%)[a-zA-Z]?$`)
	ignoreHeader = regexp.MustCompile(`(?i)^(%|change|variance|increase\s*/?|increase\s*/\s*\(decrease\)|\(decrease\)|note|notes|movement)$`)

	durQuarter = regexp.MustCompile(`(?i)\b(quarter|three months|0?3 months|3 months)\b`)
	durHalf    = regexp.MustCompile(`(?i)\b(six months|0?6 months|half[ -]year)\b`)
	durNine    = regexp.MustCompile(`(?i)\b(nine months|0?9 months)\b`)
	durYear    = regexp.MustCompile(`(?i)\b(year to|year ended|twelve months|12 months|for the year)\b`)
	durPeriod  = regexp.MustCompile(`(?i)\b(period ended|period to|for the period)\b`)

	yearToken  = regexp.MustCompile(`^(20\d{2})(?:\s*/\s*(\d{2}|20\d{2}))?[a-zA-Z]?$`)
	titleScope = regexp.MustCompile(`(?i)\b(consolidated|group)\b`)
	titleCo    = regexp.MustCompile(`(?i)\b(company|bank)\b`)
)

var titleLeadRe = regexp.MustCompile(titleLead + `statement of\s*$`)

// titleLeadOnly reports whether text is the first half of a wrapped title,
// such as "CONSOLIDATED STATEMENT OF".
func titleLeadOnly(text string) bool { return titleLeadRe.MatchString(strings.TrimSpace(text)) }

// statementKind classifies a title line.
func statementKind(text string) string {
	if titleNoise.MatchString(text) {
		return ""
	}
	if titleSkip.MatchString(text) && !titlePL.MatchString(strings.TrimSuffix(strings.ToLower(strings.TrimSpace(text)), "comprehensive income")) {
		return ""
	}
	switch {
	case titleBS.MatchString(text):
		return BalanceSheet
	case titleCF.MatchString(text):
		return CashFlow
	case titlePL.MatchString(text):
		return IncomeStatement
	}
	return ""
}

// phrase is a run of header words close enough to read as one caption.
type phrase struct {
	x0, x1 float64
	text   string
}

func (p phrase) center() float64 { return (p.x0 + p.x1) / 2 }

func phrases(l pdfx.Line, gap float64) []phrase {
	var out []phrase
	for _, w := range l.Words {
		if n := len(out); n > 0 && w.X-out[n-1].x1 <= gap {
			out[n-1].x1 = math.Max(out[n-1].x1, w.X+w.W)
			out[n-1].text += " " + w.Text
			continue
		}
		out = append(out, phrase{x0: w.X, x1: w.X + w.W, text: w.Text})
	}
	return out
}

// column is a value column found in a statement header.
type column struct {
	x0, x1   float64
	ignore   bool      // %, change, note columns
	date     time.Time // full date, if the header gives one
	year     int       // year (end year for 2026/27), 0 if unknown
	fiscal   bool      // year came from a split year such as 2026/27
	scope    string
	duration string // 3M, 6M, 9M, 12M or ""
	block    int    // index of the current/prior block the column belongs to
}

func (c column) center() float64 { return (c.x0 + c.x1) / 2 }

// dateKey orders columns by recency.
func (c column) dateKey() int {
	if !c.date.IsZero() {
		return c.date.Year()*10000 + int(c.date.Month())*100 + c.date.Day()
	}
	return c.year*10000 + 1231
}

// headerColumns finds the dated value columns (and the columns to ignore) in
// the header lines above a statement's first figure.
func headerColumns(header []pdfx.Line) []column {
	type dated struct {
		cols  []column
		count int
	}
	var best dated
	var ignores []column
	for _, l := range header {
		var cols []column
		for _, p := range phrases(l, 4) {
			t := strings.TrimSpace(p.text)
			if d, ok := parseDate(t); ok && len(t) <= 22 {
				cols = append(cols, column{x0: p.x0, x1: p.x1, date: d, year: d.Year()})
				continue
			}
		}
		// Years are separate words ("2026 2025"), so look at words, not phrases.
		if len(cols) == 0 {
			for _, w := range l.Words {
				if m := yearToken.FindStringSubmatch(strings.TrimSpace(w.Text)); m != nil {
					y, _ := strconv.Atoi(m[1])
					fiscal := m[2] != ""
					if fiscal {
						end, _ := strconv.Atoi(m[2])
						if end < 100 {
							end += y / 100 * 100
						}
						y = end
					}
					cols = append(cols, column{x0: w.X, x1: w.X + w.W, year: y, fiscal: fiscal})
				}
			}
		}
		if len(cols) >= best.count && len(cols) > 0 {
			best = dated{cols: cols, count: len(cols)}
		}
		for _, w := range l.Words {
			if ignoreHeader.MatchString(strings.TrimSpace(w.Text)) {
				ignores = append(ignores, column{x0: w.X, x1: w.X + w.W, ignore: true})
			}
		}
		for _, p := range phrases(l, 4) {
			if ignoreHeader.MatchString(strings.TrimSpace(p.text)) {
				ignores = append(ignores, column{x0: p.x0, x1: p.x1, ignore: true})
			}
		}
	}
	if best.count == 0 {
		return nil
	}
	cols := best.cols

	// Captions above the columns: scope words and durations. Columns form
	// blocks of current-then-prior dates ("2026 2025 | 2026 2025"); when there
	// is one caption per block they map in order, otherwise each column takes
	// the caption covering or nearest to it.
	sort.SliceStable(cols, func(i, j int) bool { return cols[i].x0 < cols[j].x0 })
	block := 0
	for i := range cols {
		if i > 0 && cols[i].dateKey() >= cols[i-1].dateKey() {
			block++
		}
		cols[i].block = block
	}
	blocks := block + 1

	var scopes, durs []phrase
	for _, l := range header {
		for _, w := range l.Words {
			t := strings.TrimSpace(w.Text)
			if scopeGroupWord.MatchString(t) || scopeCompanyWord.MatchString(t) {
				scopes = append(scopes, phrase{x0: w.X, x1: w.X + w.W, text: t})
			}
		}
		for _, p := range phrases(l, 12) {
			if duration(p.text) != "" {
				durs = append(durs, p)
			}
		}
	}
	sortPhrases(scopes)
	sortPhrases(durs)
	for i := range cols {
		if p, ok := captionFor(scopes, cols[i], blocks); ok {
			if scopeGroupWord.MatchString(p.text) {
				cols[i].scope = ScopeGroup
			} else {
				cols[i].scope = ScopeCompany
			}
		}
		if p, ok := captionFor(durs, cols[i], blocks); ok {
			cols[i].duration = duration(p.text)
		}
	}
	return append(cols, ignores...)
}

func sortPhrases(ps []phrase) {
	sort.SliceStable(ps, func(i, j int) bool { return ps[i].x0 < ps[j].x0 })
}

// captionFor picks the caption for a column: by block order when there is
// exactly one caption per block, else the nearest.
func captionFor(ps []phrase, c column, blocks int) (phrase, bool) {
	if len(ps) == blocks && blocks > 1 {
		return ps[c.block], true
	}
	return nearestPhrase(ps, c)
}

// nearestPhrase returns the phrase covering c's centre, else the closest one.
func nearestPhrase(ps []phrase, c column) (phrase, bool) {
	best, bestD := phrase{}, math.Inf(1)
	for _, p := range ps {
		d := 0.0
		switch cc := c.center(); {
		case cc < p.x0:
			d = p.x0 - cc
		case cc > p.x1:
			d = cc - p.x1
		}
		if d < bestD {
			best, bestD = p, d
		}
	}
	return best, !math.IsInf(bestD, 1)
}

// durYTD marks a "for the period ended" caption: year to date, resolved
// from the financial year end (see Meta.ytd).
const durYTD = "YTD"

func duration(s string) string {
	switch {
	case durQuarter.MatchString(s):
		return "3M"
	case durHalf.MatchString(s):
		return "6M"
	case durNine.MatchString(s):
		return "9M"
	case durYear.MatchString(s):
		return "12M"
	case durPeriod.MatchString(s):
		return durYTD
	}
	return ""
}

// row is one statement line: its label and its figures by column index.
type row struct {
	label  string
	values map[int]float64
	raw    map[int]string
}

// assign maps a figure to the nearest column (by centre or right edge) when
// it is close enough; figures nearest an ignored column are dropped.
func assign(cols []column, w pdfx.Word) (int, bool) {
	best, bestD := -1, math.Inf(1)
	right, cx := w.X+w.W, w.X+w.W/2
	for i, c := range cols {
		d := math.Min(math.Abs(c.center()-cx), math.Abs(c.x1-right))
		if d < bestD {
			best, bestD = i, d
		}
	}
	if best < 0 || cols[best].ignore || bestD > maxColumnDistance(cols) {
		return -1, false
	}
	return best, true
}

// maxColumnDistance is half the narrowest gap between value columns, capped.
func maxColumnDistance(cols []column) float64 {
	var cs []float64
	for _, c := range cols {
		if !c.ignore {
			cs = append(cs, c.center())
		}
	}
	sort.Float64s(cs)
	gap := 60.0
	for i := 1; i < len(cs); i++ {
		if d := cs[i] - cs[i-1]; d > 1 && d < gap {
			gap = d
		}
	}
	return math.Max(gap*0.6, 15)
}

// parseRows splits each line into a label and figures. A line without
// figures becomes a pending caption: it is prefixed to the next label when
// that label alone matches nothing, and an unlabeled total takes "total "
// plus the last caption.
func parseRows(lines []pdfx.Line, cols []column) []row {
	var out []row
	for _, l := range lines {
		r := row{values: map[int]float64{}, raw: map[int]string{}}
		var label []string
		for _, w := range l.Words {
			t := strings.TrimSpace(w.Text)
			if percentToken.MatchString(t) {
				continue
			}
			if v, ok := parseNumber(t); ok {
				if i, ok := assign(cols, w); ok {
					r.values[i] = v
					r.raw[i] = t
					continue
				}
				if !isAmount(t) && len(label) > 0 {
					continue // note number between label and figures
				}
				if isAmount(t) {
					continue // a figure in an ignored or unknown column
				}
			}
			label = append(label, t)
		}
		r.label = strings.Join(label, " ")
		if len(r.values) > 0 {
			r.label = trimStray(label)
		}
		out = append(out, r)
	}
	return out
}

// trimStray drops one-letter fragments some PDFs scatter between figures
// (e.g. "v" and "p" glyphs), keeping the label words.
func trimStray(words []string) string {
	var keep []string
	for _, w := range words {
		if lw := strings.ToLower(w); len(w) == 1 && lw >= "a" && lw <= "z" {
			continue
		}
		keep = append(keep, w)
	}
	return strings.Join(keep, " ")
}
