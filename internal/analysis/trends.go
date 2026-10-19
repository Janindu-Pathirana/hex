package analysis

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// Trend settings. Thresholds are absolute guides.
const (
	marginMoveStrong = 0.01 // net margin up or down by 1 percentage point
	accelStrong      = 0.10 // growth speeding up or slowing by 10 percentage points
	fcfYieldStrong   = 0.08 // free cash flow of 8% of market cap
)

// pointAt returns the non-balance-sheet value of field of the given period
// type whose period ends in the same month as end.
func (s *Snapshot) pointAt(field, typ string, end time.Time) (Point, bool) {
	for _, p := range s.series[field] {
		if p.Type == typ && sameMonth(p.End, end) {
			return p, true
		}
	}
	return Point{}, false
}

func (s *Snapshot) hasField(field string) bool { return len(s.series[field]) > 0 }

func trendMetric(id, name, group string, u Unit, v float64, period string, conf float64, used []string, steps ...string) Metric {
	val := v
	return Metric{ID: id, Name: name, Group: group, Unit: u, Value: &val, Display: Format(v, u), Period: period,
		Steps: steps, Confidence: conf, Used: used}
}

func trendLack(id, name, group string, u Unit, note string) Metric {
	return Metric{ID: id, Name: name, Group: group, Unit: u, Display: "n/m", Note: note}
}

func trendSkipped(id, name, group string, u Unit) Metric {
	return Metric{ID: id, Name: name, Group: group, Unit: u, Display: "n/a", Skipped: true,
		Note: "not meaningful for banks, finance and insurance companies"}
}

func minConf(cs ...float64) float64 {
	m := 1.0
	for _, c := range cs {
		if c > 0 && c < m {
			m = c
		}
	}
	return m
}

// computeTrends compares the latest period with the same period a year
// earlier, so a margin or return that is improving or slipping shows up
// without needing a benchmark. Each comparison uses periods of the same type
// and length, so interim figures are never set against full years.
func computeTrends(s *Snapshot, fin bool, profitField, revField string) []Metric {
	var out []Metric

	// Margin change: this period's margin less the same period last year.
	margins := []struct{ id, name, field string }{
		{"gross-margin-change", "Gross margin change", "gross_profit"},
		{"operating-margin-change", "Operating margin change", "operating_profit"},
		{"net-margin-change", "Net margin change", profitField},
	}
	cur, prev, hasRev := s.YoY(revField)
	for _, m := range margins {
		if fin {
			out = append(out, trendSkipped(m.id, m.name, "profitability", PP))
			continue
		}
		fc, okc := s.pointAt(m.field, cur.Type, cur.End)
		fp, okp := s.pointAt(m.field, prev.Type, prev.End)
		switch {
		case !hasRev:
			out = append(out, trendLack(m.id, m.name, "profitability", PP, "needs revenue for this period and the same period a year earlier"))
		case !okc || !okp:
			out = append(out, trendLack(m.id, m.name, "profitability", PP, "needs "+m.field+" for this period and the same period a year earlier"))
		case cur.Value == 0 || prev.Value == 0:
			out = append(out, trendLack(m.id, m.name, "profitability", PP, "revenue is zero in one of the periods"))
		default:
			now, before := fc.Value/cur.Value, fp.Value/prev.Value
			out = append(out, trendMetric(m.id, m.name, "profitability", PP, now-before,
				fmt.Sprintf("%s to %s vs %s", cur.Type, cur.End.Format(dateLayout), prev.End.Format(dateLayout)),
				minConf(cur.Confidence, prev.Confidence, fc.Confidence, fp.Confidence), []string{m.field, revField},
				fmt.Sprintf("%s / revenue now %s, a year earlier %s", m.field, Format(now, Percent), Format(before, Percent))))
		}
	}

	// Return on equity change, using the period's profit on closing equity.
	// It is not annualised, but both periods are measured the same way.
	{
		id, name := "roe-change", "ROE change (period profit on closing equity)"
		pc, pp, ok := s.YoY(profitField)
		ec, ok1 := s.BalanceNear("equity_parent", pc.End)
		ep, ok2 := s.BalanceNear("equity_parent", pp.End)
		switch {
		case !ok || !ok1 || !ok2:
			out = append(out, trendLack(id, name, "profitability", PP, "needs profit and parent equity for this period and a year earlier"))
		case ec.Value <= 0 || ep.Value <= 0:
			out = append(out, trendLack(id, name, "profitability", PP, "equity is zero or negative"))
		default:
			now, before := pc.Value/ec.Value, pp.Value/ep.Value
			out = append(out, trendMetric(id, name, "profitability", PP, now-before,
				fmt.Sprintf("%s to %s vs %s", pc.Type, pc.End.Format(dateLayout), pp.End.Format(dateLayout)),
				minConf(pc.Confidence, pp.Confidence, ec.Confidence, ep.Confidence), []string{profitField, "equity_parent"},
				fmt.Sprintf("ROE now %s, a year earlier %s", Format(now, Percent), Format(before, Percent))))
		}
	}

	// Growth acceleration: this year's growth less last year's growth.
	for _, g := range []struct{ id, name, field string }{
		{"revenue-growth-change", "Revenue growth change", revField},
		{"profit-growth-change", "Profit growth change", profitField},
	} {
		c, ok := s.growthPair(g.field)
		if !ok {
			out = append(out, trendLack(g.id, g.name, "growth", PP, "needs three comparable years of "+g.field))
			continue
		}
		out = append(out, trendMetric(g.id, g.name, "growth", PP, c.now-c.before,
			fmt.Sprintf("%s to %s", c.typ, c.end.Format(dateLayout)), c.conf, []string{g.field},
			fmt.Sprintf("growth now %s, a year earlier %s", Format(c.now, Percent), Format(c.before, Percent))))
	}

	// Valuation and efficiency from cash flow and assets.
	out = append(out, fcfYield(s), assetTurnover(s, fin, revField))
	return out
}

type growthPair struct {
	now, before float64
	typ         string
	end         time.Time
	conf        float64
}

// growthPair gives the year-on-year growth of the latest period and of the
// period a year before it.
func (s *Snapshot) growthPair(field string) (growthPair, bool) {
	p0, p1, ok := s.YoY(field)
	if !ok {
		return growthPair{}, false
	}
	p2, ok := s.pointAt(field, p1.Type, yearBefore(p1.End))
	if !ok || p1.Value == 0 || p2.Value == 0 {
		return growthPair{}, false
	}
	return growthPair{
		now:    (p0.Value - p1.Value) / math.Abs(p1.Value),
		before: (p1.Value - p2.Value) / math.Abs(p2.Value),
		typ:    p0.Type, end: p0.End,
		conf: minConf(p0.Confidence, p1.Confidence, p2.Confidence),
	}, true
}

func fcfYield(s *Snapshot) Metric {
	id, name := "fcf-yield", "Free cash flow yield"
	ocf, ok1 := s.TTM("ocf")
	capex, ok2 := s.TTM("capex")
	switch {
	case !ok1 || !ok2:
		return trendLack(id, name, "valuation", Percent, "needs operating cash flow and capital expenditure")
	case s.Price <= 0 || s.Shares <= 0:
		return trendLack(id, name, "valuation", Percent, "needs a stored price and share count")
	}
	fcf, mcap := ocf.Value-capex.Value, s.Price*s.Shares
	m := trendMetric(id, name, "valuation", Percent, fcf/mcap, ocf.Basis, minConf(ocf.Confidence, capex.Confidence),
		[]string{"ocf", "capex"}, fmt.Sprintf("(operating cash flow %s - capex %s) / market cap %s", Num(ocf.Value), Num(capex.Value), Num(mcap)))
	if ocf.Stale || capex.Stale {
		m.Note = "interim periods incomplete; an older full year was used"
	}
	return m
}

func assetTurnover(s *Snapshot, fin bool, revField string) Metric {
	id, name := "asset-turnover", "Asset turnover (revenue on closing assets)"
	if fin {
		return trendSkipped(id, name, "profitability", Times)
	}
	rev, ok1 := s.TTM(revField)
	ta, ok2 := s.Balance("total_assets")
	switch {
	case !ok1 || !ok2:
		return trendLack(id, name, "profitability", Times, "needs revenue and total assets")
	case ta.Value <= 0:
		return trendLack(id, name, "profitability", Times, "total assets are zero or negative")
	}
	return trendMetric(id, name, "profitability", Times, rev.Value/ta.Value, rev.Basis, minConf(rev.Confidence, ta.Confidence),
		[]string{revField, "total_assets"}, fmt.Sprintf("revenue %s / total assets %s", Num(rev.Value), Num(ta.Value)))
}

// ppAbs formats the size of a change in percentage points, without a sign.
func ppAbs(v float64) string { return strings.TrimPrefix(Format(math.Abs(v), PP), "+") }

func marginTrendRule(c *ctx) *Finding {
	v, ok := c.val("net-margin-change")
	switch {
	case !ok:
		return nil
	case v >= marginMoveStrong:
		return c.find("margin-trend", DimProfitability, Positive, "Net margin improving",
			fmt.Sprintf("Net margin is %s higher than the same period a year earlier.", ppAbs(v)), "net-margin-change")
	case v <= -marginMoveStrong:
		return c.find("margin-trend", DimProfitability, Watch, "Net margin slipping",
			fmt.Sprintf("Net margin is %s lower than the same period a year earlier.", ppAbs(v)), "net-margin-change")
	}
	return nil
}

func growthAccelRule(c *ctx) *Finding {
	v, ok := c.val("revenue-growth-change")
	switch {
	case !ok:
		return nil
	case v >= accelStrong:
		return c.find("growth-accel", DimGrowth, Positive, "Revenue growth speeding up",
			fmt.Sprintf("Revenue growth is %s higher than a year earlier%s", ppAbs(v), absolute+"."), "revenue-growth-change", "revenue-growth")
	case v <= -accelStrong:
		return c.find("growth-accel", DimGrowth, Watch, "Revenue growth slowing",
			fmt.Sprintf("Revenue growth is %s lower than a year earlier%s", ppAbs(v), absolute+"."), "revenue-growth-change", "revenue-growth")
	}
	return nil
}

func fcfYieldRule(c *ctx) *Finding {
	v, ok := c.val("fcf-yield")
	if !ok || v < fcfYieldStrong {
		return nil
	}
	return c.find("fcf-yield", DimCash, Positive, "High free cash flow yield",
		fmt.Sprintf("Free cash flow is %s of market capitalisation%s", Format(v, Percent), absolute+"."), "fcf-yield")
}
