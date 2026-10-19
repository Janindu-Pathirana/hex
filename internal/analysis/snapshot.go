package analysis

import (
	"sort"
	"strings"
	"time"
)

// Period types stored with fundamentals. Balance-sheet values are PIT; income
// and cash-flow values are cumulative from the start of the financial year.
const (
	PIT        = "PIT"
	FY         = "12M"
	dateLayout = "2006-01-02"
	scopeGroup = "group"
)

// Row is one stored statement value, already the winning source for its key.
type Row struct {
	PeriodEnd  string // YYYY-MM-DD
	PeriodType string // 3M, 6M, 9M, 12M, PIT
	Scope      string // group, company
	Field      string
	Source     string
	Value      float64
	Confidence float64
}

// Point is one value of a field at a period end.
type Point struct {
	End        time.Time
	Type       string
	Value      float64
	Source     string
	Confidence float64
}

// Snapshot is everything the analysis knows about one company. It is built
// from plain data, so the engine does no I/O.
type Snapshot struct {
	Code   string
	Name   string
	Sector string
	Price  float64 // 0 when no quote is stored
	Shares float64 // 0 when unknown
	AsOf   time.Time

	series map[string][]Point
	bars   []Bar // daily history of the stock, oldest first (see WithBars)
	index  []Bar // daily history of the benchmark index
}

// NewSnapshot groups rows into a series per field, newest period first. When
// a field has group values it uses those only, otherwise the company values.
func NewSnapshot(code, name, sector string, price, shares float64, asOf time.Time, rows []Row) *Snapshot {
	s := &Snapshot{Code: code, Name: name, Sector: sector, Price: price, Shares: shares, AsOf: asOf,
		series: map[string][]Point{}}
	hasGroup := map[string]bool{}
	for _, r := range rows {
		if r.Scope == scopeGroup {
			hasGroup[r.Field] = true
		}
	}
	for _, r := range rows {
		if hasGroup[r.Field] && r.Scope != scopeGroup {
			continue
		}
		end, err := time.Parse(dateLayout, r.PeriodEnd)
		if err != nil {
			continue
		}
		s.series[r.Field] = append(s.series[r.Field], Point{End: end, Type: r.PeriodType, Value: r.Value,
			Source: r.Source, Confidence: r.Confidence})
	}
	for _, pts := range s.series {
		sort.SliceStable(pts, func(i, j int) bool {
			if !pts[i].End.Equal(pts[j].End) {
				return pts[i].End.After(pts[j].End)
			}
			return pts[i].Type < pts[j].Type
		})
	}
	return s
}

// Fields returns the stored field names, sorted.
func (s *Snapshot) Fields() []string {
	out := make([]string, 0, len(s.series))
	for f := range s.series {
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}

// Series returns a field's points, newest first.
func (s *Snapshot) Series(field string) []Point { return s.series[field] }

// Balance returns the newest balance-sheet value of field.
func (s *Snapshot) Balance(field string) (Point, bool) {
	for _, p := range s.series[field] {
		if p.Type == PIT {
			return p, true
		}
	}
	return Point{}, false
}

// BalanceNear returns the balance-sheet value whose period end falls in the
// same month as end.
func (s *Snapshot) BalanceNear(field string, end time.Time) (Point, bool) {
	for _, p := range s.series[field] {
		if p.Type == PIT && sameMonth(p.End, end) {
			return p, true
		}
	}
	return Point{}, false
}

// Flow is an income or cash-flow amount over a trailing twelve months, with
// where it came from.
type Flow struct {
	Value      float64
	End        time.Time // end of the latest period used
	Basis      string    // "FY to 2026-03-31" or "TTM to 2026-06-30"
	Stale      bool      // TTM could not be built, so an older full year was used
	Confidence float64   // lowest confidence among the points used
	Sources    []string
}

// TTM returns the trailing-twelve-month value of an income or cash-flow
// field. The latest period is a full year, or year-to-date; in that case TTM
// is YTD + last full year − the same YTD a year earlier. When the parts are
// missing it falls back to the newest full year and marks the result Stale.
func (s *Snapshot) TTM(field string) (Flow, bool) {
	var latest *Point
	for i := range s.series[field] {
		if s.series[field][i].Type != PIT {
			latest = &s.series[field][i]
			break
		}
	}
	if latest == nil {
		return Flow{}, false
	}
	if latest.Type == FY {
		return flow(latest.Value, latest.End, "FY to "+latest.End.Format(dateLayout), false, *latest), true
	}
	yearAgo := yearBefore(latest.End)
	var fy, prior *Point
	for i := range s.series[field] {
		p := &s.series[field][i]
		switch {
		case fy == nil && p.Type == FY && p.End.Before(latest.End) && p.End.After(yearAgo):
			fy = p
		case prior == nil && p.Type == latest.Type && sameMonth(p.End, yearAgo):
			prior = p
		}
	}
	if fy != nil && prior != nil {
		v := latest.Value + fy.Value - prior.Value
		return flow(v, latest.End, "TTM to "+latest.End.Format(dateLayout), false, *latest, *fy, *prior), true
	}
	for _, p := range s.series[field] {
		if p.Type == FY {
			return flow(p.Value, p.End, "FY to "+p.End.Format(dateLayout), true, p), true
		}
	}
	return Flow{}, false
}

// YoY returns the latest period's value and the same period a year earlier:
// full year against full year, or year-to-date against year-to-date.
func (s *Snapshot) YoY(field string) (cur, prev Point, ok bool) {
	for _, p := range s.series[field] {
		if p.Type == PIT {
			continue
		}
		cur = p
		break
	}
	if cur.Type == "" {
		return Point{}, Point{}, false
	}
	yearAgo := yearBefore(cur.End)
	for _, p := range s.series[field] {
		if p.Type == cur.Type && sameMonth(p.End, yearAgo) {
			return cur, p, true
		}
	}
	return Point{}, Point{}, false
}

// Newest returns the period end of the most recent value of any field.
func (s *Snapshot) Newest() (time.Time, bool) {
	var newest time.Time
	for _, pts := range s.series {
		if len(pts) > 0 && pts[0].End.After(newest) {
			newest = pts[0].End
		}
	}
	return newest, !newest.IsZero()
}

func flow(v float64, end time.Time, basis string, stale bool, pts ...Point) Flow {
	f := Flow{Value: v, End: end, Basis: basis, Stale: stale, Confidence: 1}
	seen := map[string]bool{}
	for _, p := range pts {
		if p.Confidence < f.Confidence {
			f.Confidence = p.Confidence
		}
		if !seen[p.Source] {
			seen[p.Source] = true
			f.Sources = append(f.Sources, p.Source)
		}
	}
	return f
}

// yearBefore is t minus one year, clamped to the last day of the month so
// 29 February maps to 28 February.
func yearBefore(t time.Time) time.Time {
	y := t.AddDate(-1, 0, 0)
	if y.Month() != t.Month() {
		return time.Date(t.Year()-1, t.Month()+1, 0, 0, 0, 0, 0, time.UTC)
	}
	return y
}

func sameMonth(a, b time.Time) bool { return a.Year() == b.Year() && a.Month() == b.Month() }

// IsFinancial reports whether the sector is a bank, finance or insurance
// business, where margins, debt/equity and current ratio do not apply.
func IsFinancial(sector string) bool {
	s := strings.ToLower(sector)
	return strings.Contains(s, "bank") || strings.Contains(s, "financ") || strings.Contains(s, "insur")
}
