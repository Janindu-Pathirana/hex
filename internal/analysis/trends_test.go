package analysis

import (
	"math"
	"testing"
	"time"
)

// threeYears has full years to March 2023, 2024 and 2025.
func threeYears(sector string) *Snapshot {
	row := func(end, field string, v float64) Row {
		return Row{PeriodEnd: end, PeriodType: "12M", Scope: "group", Field: field, Source: "pdf", Value: v, Confidence: 0.9}
	}
	rows := []Row{
		row("2023-03-31", "revenue", 800), row("2024-03-31", "revenue", 900), row("2025-03-31", "revenue", 1100),
		row("2023-03-31", "pat_parent", 80), row("2024-03-31", "pat_parent", 90), row("2025-03-31", "pat_parent", 150),
		row("2024-03-31", "gross_profit", 360), row("2025-03-31", "gross_profit", 400),
		{PeriodEnd: "2025-03-31", PeriodType: "PIT", Scope: "group", Field: "equity_parent", Source: "pdf", Value: 1000, Confidence: 0.8},
		{PeriodEnd: "2024-03-31", PeriodType: "PIT", Scope: "group", Field: "equity_parent", Source: "pdf", Value: 900, Confidence: 0.9},
	}
	return NewSnapshot("T.N0000", "Test PLC", sector, 60, 100, time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC), rows)
}

func TestTrendsOnInterimPeriods(t *testing.T) {
	// fixture(): latest period is 9M to 2025-12-31 against 9M to 2024-12-31.
	m := Compute(fixture("Manufacturing"))
	want := map[string]float64{
		"net-margin-change": 90.0/800 - 70.0/700, // +1.25pp
		"roe-change":        90.0/1200 - 70.0/1000,
		"fcf-yield":         (160.0 - 35) / 6000, // TTM ocf 120+100-60, capex 30+20-15, market cap 60 x 100
		"asset-turnover":    1100.0 / 3000,
	}
	for id, w := range want {
		if got := techVal(t, m, id); math.Abs(got-w) > 1e-9 {
			t.Errorf("%s = %v, want %v", id, got, w)
		}
	}
	if x, _ := m.Get("net-margin-change"); x.Period != "9M to 2025-12-31 vs 2024-12-31" || x.Display != "+1.25pp" {
		t.Errorf("net-margin-change = %q %q", x.Display, x.Period)
	}
	// Operating profit exists only for the full year, so there is nothing to compare with.
	if x, _ := m.Get("operating-margin-change"); x.Value != nil || x.Note == "" {
		t.Errorf("operating-margin-change = %+v, want a note", x)
	}
	// Only one year of 9M revenue is stored beyond the prior year.
	if x, _ := m.Get("revenue-growth-change"); x.Value != nil || x.Note == "" {
		t.Errorf("revenue-growth-change = %+v, want a note", x)
	}
}

func TestTrendsOverThreeYears(t *testing.T) {
	m := Compute(threeYears("Manufacturing"))
	near := func(id string, want float64) {
		t.Helper()
		if got := techVal(t, m, id); math.Abs(got-want) > 1e-9 {
			t.Errorf("%s = %v, want %v", id, got, want)
		}
	}
	near("revenue-growth-change", (1100.0/900-1)-(900.0/800-1))
	near("profit-growth-change", (150.0/90-1)-(90.0/80-1))
	near("gross-margin-change", 400.0/1100-360.0/900)
	near("net-margin-change", 150.0/1100-90.0/900)
	near("roe-change", 150.0/1000-90.0/900)
	// Confidence is the lowest of the inputs (the 2025 equity is 0.8).
	if x, _ := m.Get("roe-change"); x.Confidence != 0.8 {
		t.Errorf("roe-change confidence = %v, want 0.8", x.Confidence)
	}
	if x, _ := m.Get("roe-change"); len(x.Used) != 2 {
		t.Errorf("roe-change used = %v", x.Used)
	}
}

func TestTrendsFinancialsSkipMargins(t *testing.T) {
	m := Compute(threeYears("Banks Finance and Insurance"))
	for _, id := range []string{"gross-margin-change", "operating-margin-change", "net-margin-change", "asset-turnover"} {
		if x, ok := m.Get(id); !ok || !x.Skipped {
			t.Errorf("%s = %+v, want skipped for a bank", id, x)
		}
	}
	// Growth and ROE trends still apply.
	if _, ok := m.Val("revenue-growth-change"); !ok {
		t.Error("revenue growth change should still be computed")
	}
}

func TestTrendFindings(t *testing.T) {
	s := threeYears("Manufacturing")
	fs := Evaluate(s, Compute(s))
	if f, ok := findingByID(fs, "margin-trend"); !ok || f.Severity != Positive || f.Dimension != DimProfitability {
		t.Errorf("margin-trend = %+v", f)
	} else if f.Detail != "Net margin is 3.64pp higher than the same period a year earlier." {
		t.Errorf("margin-trend detail = %q", f.Detail)
	}
	// +9.7pp is under the 10pp guide, so no growth finding yet.
	if _, ok := findingByID(fs, "growth-accel"); ok {
		t.Error("9.7pp should not trigger growth-accel")
	}
	// Slowing: growth falls from 50% to 0%.
	rows := []Row{
		{PeriodEnd: "2023-03-31", PeriodType: "12M", Scope: "group", Field: "revenue", Source: "pdf", Value: 600, Confidence: 0.9},
		{PeriodEnd: "2024-03-31", PeriodType: "12M", Scope: "group", Field: "revenue", Source: "pdf", Value: 900, Confidence: 0.9},
		{PeriodEnd: "2025-03-31", PeriodType: "12M", Scope: "group", Field: "revenue", Source: "pdf", Value: 900, Confidence: 0.9},
	}
	s2 := NewSnapshot("T.N0000", "T", "Manufacturing", 60, 100, time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC), rows)
	if f, ok := findingByID(Evaluate(s2, Compute(s2)), "growth-accel"); !ok || f.Severity != Watch {
		t.Errorf("growth-accel = %+v", f)
	}
}

func TestFCFYieldFinding(t *testing.T) {
	rows := []Row{
		{PeriodEnd: "2025-03-31", PeriodType: "12M", Scope: "group", Field: "ocf", Source: "pdf", Value: 700, Confidence: 0.9},
		{PeriodEnd: "2025-03-31", PeriodType: "12M", Scope: "group", Field: "capex", Source: "pdf", Value: 100, Confidence: 0.9},
	}
	s := NewSnapshot("T.N0000", "T", "Manufacturing", 60, 100, time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC), rows)
	m := Compute(s)
	if v := techVal(t, m, "fcf-yield"); math.Abs(v-0.1) > 1e-9 { // 600 / 6000
		t.Errorf("fcf-yield = %v, want 0.1", v)
	}
	if f, ok := findingByID(Evaluate(s, m), "fcf-yield"); !ok || f.Severity != Positive {
		t.Errorf("fcf-yield finding = %+v", f)
	}
	// Without a price there is no market cap.
	s.Price = 0
	if x, _ := Compute(s).Get("fcf-yield"); x.Value != nil || x.Note == "" {
		t.Errorf("fcf-yield without price = %+v", x)
	}
}

func TestPPFormat(t *testing.T) {
	for _, c := range []struct {
		v    float64
		want string
	}{{0.0125, "+1.25pp"}, {-0.0364, "-3.64pp"}, {0, "0.00pp"}, {-0.000001, "0.00pp"}} {
		if got := Format(c.v, PP); got != c.want {
			t.Errorf("Format(%v, PP) = %q, want %q", c.v, got, c.want)
		}
	}
}

func TestTrendMetricsStayOutOfRegistry(t *testing.T) {
	for _, id := range []string{"net-margin-change", "fcf-yield", "asset-turnover", "roe-change", "revenue-growth-change"} {
		if _, ok := Lookup(id); ok {
			t.Errorf("%s must not be in the calc registry", id)
		}
	}
}
