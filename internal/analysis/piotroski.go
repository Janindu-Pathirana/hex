package analysis

import (
	"fmt"
	"strings"
)

// Piotroski bands: the share of computable tests passed.
const (
	piotroskiMinTests = 5     // fewer computable tests than this and no score is given
	piotroskiStrong   = 0.75  // passed at least this share
	piotroskiWeak     = 0.375 // passed at most this share
)

// pTest is one Piotroski test. ok is false when its inputs are not stored.
type pTest struct {
	name string
	pass bool
	ok   bool
	need string // inputs missing when !ok
	conf float64
	used []string
}

// piotroskiTests runs the eight tests hex can compute. The original ninth
// (no new shares issued) needs a share-count history that is not stored.
// Each test compares the latest period with the same period a year earlier;
// balance-sheet values are taken at the same period ends.
func piotroskiTests(s *Snapshot, profitField, revField string) []pTest {
	pc, pp, okP := s.YoY(profitField)
	rc, rp, okR := s.YoY(revField)
	assets := func(p Point) (Point, bool) { return s.BalanceNear("total_assets", p.End) }

	lack := func(name, need string) pTest { return pTest{name: name, need: need} }
	done := func(name string, pass bool, conf []float64, used ...string) pTest {
		return pTest{name: name, pass: pass, ok: true, conf: minConf(conf...), used: used}
	}

	var out []pTest

	// 1. Profit is positive.
	if okP {
		out = append(out, done("Profit is positive", pc.Value > 0, []float64{pc.Confidence}, profitField))
	} else {
		out = append(out, lack("Profit is positive", profitField))
	}

	// 2. Operating cash flow is positive. 4. Cash flow is above profit.
	ocf, okO := s.TTM("ocf")
	profit, okT := s.TTM(profitField)
	if okO {
		out = append(out, done("Operating cash flow is positive", ocf.Value > 0, []float64{ocf.Confidence}, "ocf"))
	} else {
		out = append(out, lack("Operating cash flow is positive", "ocf"))
	}

	// 3. Return on assets is rising.
	ac, okc := assets(pc)
	ap, okp := assets(pp)
	if okP && okc && okp && ac.Value > 0 && ap.Value > 0 {
		out = append(out, done("Return on assets is rising", pc.Value/ac.Value > pp.Value/ap.Value,
			[]float64{pc.Confidence, pp.Confidence, ac.Confidence, ap.Confidence}, profitField, "total_assets"))
	} else {
		out = append(out, lack("Return on assets is rising", profitField+" and total_assets for this period and a year earlier"))
	}

	if okO && okT {
		out = append(out, done("Cash flow is above profit", ocf.Value > profit.Value,
			[]float64{ocf.Confidence, profit.Confidence}, "ocf", profitField))
	} else {
		out = append(out, lack("Cash flow is above profit", "ocf and "+profitField))
	}

	// 5. Leverage is falling (liabilities over assets).
	lc, okLc := s.BalanceNear("total_liabilities", pc.End)
	lp, okLp := s.BalanceNear("total_liabilities", pp.End)
	if okP && okc && okp && okLc && okLp && ac.Value > 0 && ap.Value > 0 {
		out = append(out, done("Leverage is falling", lc.Value/ac.Value < lp.Value/ap.Value,
			[]float64{lc.Confidence, lp.Confidence, ac.Confidence, ap.Confidence}, "total_liabilities", "total_assets"))
	} else {
		out = append(out, lack("Leverage is falling", "total_liabilities and total_assets for this period and a year earlier"))
	}

	// 6. Current ratio is rising.
	cac, ok1 := s.BalanceNear("current_assets", pc.End)
	clc, ok2 := s.BalanceNear("current_liabilities", pc.End)
	cap_, ok3 := s.BalanceNear("current_assets", pp.End)
	clp, ok4 := s.BalanceNear("current_liabilities", pp.End)
	if okP && ok1 && ok2 && ok3 && ok4 && clc.Value > 0 && clp.Value > 0 {
		out = append(out, done("Current ratio is rising", cac.Value/clc.Value > cap_.Value/clp.Value,
			[]float64{cac.Confidence, clc.Confidence, cap_.Confidence, clp.Confidence}, "current_assets", "current_liabilities"))
	} else {
		out = append(out, lack("Current ratio is rising", "current_assets and current_liabilities for this period and a year earlier"))
	}

	// 7. Gross margin is rising.
	gc, okg1 := s.pointAt("gross_profit", rc.Type, rc.End)
	gp, okg2 := s.pointAt("gross_profit", rp.Type, rp.End)
	if okR && okg1 && okg2 && rc.Value != 0 && rp.Value != 0 {
		out = append(out, done("Gross margin is rising", gc.Value/rc.Value > gp.Value/rp.Value,
			[]float64{gc.Confidence, gp.Confidence, rc.Confidence, rp.Confidence}, "gross_profit", revField))
	} else {
		out = append(out, lack("Gross margin is rising", "gross_profit and "+revField+" for this period and a year earlier"))
	}

	// 8. Asset turnover is rising.
	arc, okA1 := assets(rc)
	arp, okA2 := assets(rp)
	if okR && okA1 && okA2 && arc.Value > 0 && arp.Value > 0 {
		out = append(out, done("Asset turnover is rising", rc.Value/arc.Value > rp.Value/arp.Value,
			[]float64{rc.Confidence, rp.Confidence, arc.Confidence, arp.Confidence}, revField, "total_assets"))
	} else {
		out = append(out, lack("Asset turnover is rising", revField+" and total_assets for this period and a year earlier"))
	}
	return out
}

// computePiotroski scores the company with the Piotroski F-score tests that
// hex can compute. It is a screen for financial strength, not a forecast.
func computePiotroski(s *Snapshot, fin bool, profitField, revField string) []Metric {
	id, name := "piotroski", "Piotroski F-score"
	if fin {
		return []Metric{trendSkipped(id, name, "health", Count)}
	}
	tests := piotroskiTests(s, profitField, revField)
	var points, computable int
	var steps, missing, used []string
	conf := 1.0
	for _, t := range tests {
		switch {
		case !t.ok:
			steps = append(steps, fmt.Sprintf("n/a   %s (needs %s)", t.name, t.need))
			missing = append(missing, t.name)
		case t.pass:
			computable++
			points++
			steps = append(steps, "pass  "+t.name)
		default:
			computable++
			steps = append(steps, "fail  "+t.name)
		}
		if t.ok {
			conf = min(conf, t.conf)
			used = append(used, t.used...)
		}
	}
	steps = append(steps, "n/a   No new shares issued (no share-count history is stored)")
	if computable < piotroskiMinTests {
		m := trendLack(id, name, "health", Count, fmt.Sprintf("only %d of 8 tests could be computed (need %d): %s",
			computable, piotroskiMinTests, strings.Join(missing, "; ")))
		m.Steps = steps
		return []Metric{m}
	}
	v := float64(points)
	m := trendMetric(id, name, "health", Count, v, "", conf, dedupe(used), steps...)
	m.Display = fmt.Sprintf("%d of %d tests", points, computable)
	if computable < 8 {
		m.Note = fmt.Sprintf("%d of 8 tests could be computed; the rest lack stored inputs", computable)
	}
	return []Metric{m}
}

func piotroskiRule(c *ctx) *Finding {
	m, ok := c.m.Get("piotroski")
	if !ok || m.Value == nil {
		return nil
	}
	tests := piotroskiTests(c.s, firstField(c.s, "pat_parent", "pat"), firstField(c.s, "revenue", "gross_income", "total_operating_income"))
	var computable int
	for _, t := range tests {
		if t.ok {
			computable++
		}
	}
	if computable == 0 {
		return nil
	}
	share := *m.Value / float64(computable)
	detail := fmt.Sprintf("%s (%d of 8 tests could be computed)%s.", m.Display, computable, absolute)
	switch {
	case share >= piotroskiStrong:
		return c.find("piotroski", DimHealth, Positive, "Passes most financial strength tests", detail, "piotroski")
	case share <= piotroskiWeak:
		return c.find("piotroski", DimHealth, Watch, "Fails most financial strength tests", detail, "piotroski")
	}
	return c.find("piotroski", DimHealth, Neutral, "Mixed financial strength tests", detail, "piotroski")
}
