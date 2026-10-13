package analysis

import (
	"bytes"
	"math"
	"strings"
	"testing"
	"time"
)

// fixture is a March-year-end manufacturer. Hand-computed TTM to 2025-12-31:
// revenue 1000 − 700 + 800 = 1100, profit 100 − 70 + 90 = 120, EPS 10 − 7 + 9 = 12.
func fixture(sector string) *Snapshot {
	row := func(end, typ, field string, v float64) Row {
		return Row{PeriodEnd: end, PeriodType: typ, Scope: "group", Field: field, Source: "pdf", Value: v, Confidence: 0.9}
	}
	rows := []Row{
		row("2025-03-31", "12M", "revenue", 1000), row("2025-12-31", "9M", "revenue", 800), row("2024-12-31", "9M", "revenue", 700),
		row("2025-03-31", "12M", "pat_parent", 100), row("2025-12-31", "9M", "pat_parent", 90), row("2024-12-31", "9M", "pat_parent", 70),
		row("2025-03-31", "12M", "eps_basic", 10), row("2025-12-31", "9M", "eps_basic", 9), row("2024-12-31", "9M", "eps_basic", 7),
		row("2025-03-31", "12M", "operating_profit", 200), row("2025-03-31", "12M", "finance_cost", 50),
		row("2025-03-31", "12M", "ocf", 120), row("2025-12-31", "9M", "ocf", 100), row("2024-12-31", "9M", "ocf", 60),
		row("2025-03-31", "12M", "capex", 30), row("2025-12-31", "9M", "capex", 20), row("2024-12-31", "9M", "capex", 15),
		row("2025-03-31", "12M", "dps", 4), row("2025-12-31", "9M", "dps", 2), row("2024-12-31", "9M", "dps", 1),
		row("2025-12-31", "PIT", "equity_parent", 1200), row("2024-12-31", "PIT", "equity_parent", 1000),
		row("2025-12-31", "PIT", "total_assets", 3000), row("2024-12-31", "PIT", "total_assets", 2500),
		row("2025-12-31", "PIT", "current_assets", 600), row("2025-12-31", "PIT", "current_liabilities", 300),
		row("2025-12-31", "PIT", "total_equity", 1250),
		// A company-scope value must be ignored when group values exist.
		{PeriodEnd: "2025-12-31", PeriodType: "9M", Scope: "company", Field: "revenue", Source: "pdf", Value: 1, Confidence: 0.9},
	}
	return NewSnapshot("TEST.N0000", "Test PLC", sector, 60, 100, time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC), rows)
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9*math.Max(1, math.Abs(b)) }

func TestTTM(t *testing.T) {
	s := fixture("Manufacturing")
	f, ok := s.TTM("revenue")
	if !ok || f.Value != 1100 || f.Basis != "TTM to 2025-12-31" {
		t.Fatalf("revenue TTM = %+v, %v", f, ok)
	}
	// operating_profit has a full year only, so TTM is that year.
	f, ok = s.TTM("operating_profit")
	if !ok || f.Value != 200 || f.Basis != "FY to 2025-03-31" {
		t.Fatalf("operating_profit TTM = %+v, %v", f, ok)
	}
}

func TestTTMFallsBackWhenPartsMissing(t *testing.T) {
	rows := []Row{
		{PeriodEnd: "2025-03-31", PeriodType: "12M", Scope: "group", Field: "revenue", Value: 1000, Confidence: 0.9},
		{PeriodEnd: "2025-12-31", PeriodType: "9M", Scope: "group", Field: "revenue", Value: 800, Confidence: 0.9},
	}
	s := NewSnapshot("X", "", "", 0, 0, time.Now(), rows)
	f, ok := s.TTM("revenue")
	if !ok || f.Value != 1000 || !f.Stale {
		t.Fatalf("got %+v, %v; want stale full year", f, ok)
	}
}

func TestYoY(t *testing.T) {
	cur, prev, ok := fixture("").YoY("revenue")
	if !ok || cur.Value != 800 || prev.Value != 700 {
		t.Fatalf("got %v %v %v", cur, prev, ok)
	}
}

func TestYearBefore(t *testing.T) {
	got := yearBefore(time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC))
	if got.Format(dateLayout) != "2023-02-28" {
		t.Fatalf("got %s", got.Format(dateLayout))
	}
}

func TestMetrics(t *testing.T) {
	m := Compute(fixture("Manufacturing"))
	want := map[string]float64{
		"eps": 12, "pe": 5, "earnings-yield": 0.2, "bvps": 12, "pb": 5, "market-cap": 6000,
		"graham":          math.Sqrt(22.5 * 12 * 12),
		"roe":             120.0 / 1100, // average of 1000 and 1200
		"roa":             120.0 / 2750,
		"net-margin":      120.0 / 1100,
		"current-ratio":   2,
		"interest-cover":  4,
		"cash-conversion": 160.0 / 120, // OCF TTM 120 − 60 + 100
		"fcf":             160 - 35,
		"dividend-yield":  5.0 / 60,
		"payout":          5.0 / 12,
		"revenue-growth":  100.0 / 700,
		"profit-growth":   20.0 / 70,
		"eps-growth":      2.0 / 7,
		"peg":             5 / (200.0 / 7),
	}
	for id, w := range want {
		got, ok := m.Val(id)
		if !ok || !near(got, w) {
			x, _ := m.Get(id)
			t.Errorf("%s = %v (ok %v), want %v; note %q", id, got, ok, w, x.Note)
		}
	}
}

func TestMetricMissingAndPeriodMismatch(t *testing.T) {
	m := Compute(fixture("Manufacturing"))
	de, _ := m.Get("debt-equity")
	if de.Value != nil || !strings.Contains(de.Note, "Interest-bearing debt") {
		t.Errorf("debt-equity = %+v, want missing debt", de)
	}
	// Operating profit is a full year, revenue is TTM to a later date.
	om, _ := m.Get("operating-margin")
	if om.Value != nil || !strings.Contains(om.Note, "periods differ") {
		t.Errorf("operating-margin = %+v, want periods differ", om)
	}
}

func TestFinancialsSkipMargins(t *testing.T) {
	m := Compute(fixture("Banks"))
	for _, id := range []string{"net-margin", "current-ratio", "debt-equity", "interest-cover"} {
		x, _ := m.Get(id)
		if !x.Skipped || x.Value != nil {
			t.Errorf("%s should be skipped for a bank: %+v", id, x)
		}
	}
	if _, ok := m.Val("roe"); !ok {
		t.Error("roe should still be computed for a bank")
	}
}

func TestFindings(t *testing.T) {
	r := Analyze(fixture("Manufacturing"))
	got := map[string]Severity{}
	for _, f := range r.Findings {
		got[f.ID] = f.Severity
	}
	want := map[string]Severity{
		"pe": Positive, "pb": Watch, "graham": Watch, "roe": Neutral, "margins": Positive,
		"revenue-growth": Positive, "profit-growth": Positive, "current-ratio": Positive,
		"interest-cover": Positive, "cash-conversion": Positive, "fcf": Positive, "payout": Neutral,
	}
	for id, sev := range want {
		if got[id] != sev {
			t.Errorf("finding %s = %q, want %q", id, got[id], sev)
		}
	}
	for _, id := range []string{"stale", "low-confidence", "no-price"} {
		if _, ok := got[id]; ok {
			t.Errorf("unexpected data-quality finding %s", id)
		}
	}
}

func TestDataQualityFindings(t *testing.T) {
	s := NewSnapshot("X", "", "", 0, 100, time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC), []Row{
		{PeriodEnd: "2025-03-31", PeriodType: "12M", Scope: "group", Field: "pat_parent", Value: 100, Confidence: 0.5},
	})
	got := map[string]bool{}
	for _, f := range Analyze(s).Findings {
		got[f.ID] = true
	}
	for _, id := range []string{"stale", "low-confidence", "no-price"} {
		if !got[id] {
			t.Errorf("missing finding %s", id)
		}
	}
}

func TestLossMakingPE(t *testing.T) {
	s := NewSnapshot("X", "", "Manufacturing", 10, 100, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), []Row{
		{PeriodEnd: "2025-12-31", PeriodType: "12M", Scope: "group", Field: "eps_basic", Value: -2, Confidence: 0.9},
	})
	for _, f := range Analyze(s).Findings {
		if f.ID == "pe" && f.Severity == Watch {
			return
		}
	}
	t.Fatal("want a loss-making finding")
}

func TestCoverage(t *testing.T) {
	r := Analyze(fixture("Manufacturing"))
	if len(r.Coverage.Used) == 0 {
		t.Fatal("no fields marked used")
	}
	for _, f := range r.Coverage.Used {
		if f == "total_equity" {
			t.Error("total_equity only feeds debt-equity, which is missing, so it must not count as used")
		}
	}
}

func TestRenderIsDeterministic(t *testing.T) {
	var a, b bytes.Buffer
	if err := Analyze(fixture("Manufacturing")).Render(&a); err != nil {
		t.Fatal(err)
	}
	_ = Analyze(fixture("Manufacturing")).Render(&b)
	if a.String() != b.String() {
		t.Fatal("render differs between runs")
	}
	if !strings.Contains(a.String(), "not investment advice") {
		t.Error("missing disclaimer")
	}
}
