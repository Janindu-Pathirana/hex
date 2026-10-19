package analysis

import (
	"strings"
	"testing"
	"time"
)

// pioRows returns two full years to March 2025 and March 2024 in which every
// Piotroski test passes. over replaces values by "field/2025" or "field/2024";
// a value of -1e18 removes the row.
func pioRows(over map[string]float64) []Row {
	base := map[string][2]float64{ // field: {2025, 2024}
		"revenue": {1100, 900}, "gross_profit": {440, 342}, "pat_parent": {150, 90}, "ocf": {200, 100},
		"total_assets": {1000, 1000}, "total_liabilities": {400, 500},
		"current_assets": {500, 400}, "current_liabilities": {250, 250},
	}
	balance := map[string]bool{"total_assets": true, "total_liabilities": true, "current_assets": true, "current_liabilities": true}
	var rows []Row
	for f, v := range base {
		for i, end := range []string{"2025-03-31", "2024-03-31"} {
			val := v[i]
			key := f + "/" + end[:4]
			if o, ok := over[key]; ok {
				if o == -1e18 {
					continue
				}
				val = o
			}
			typ := "12M"
			if balance[f] {
				typ = "PIT"
			}
			rows = append(rows, Row{PeriodEnd: end, PeriodType: typ, Scope: "group", Field: f, Source: "pdf", Value: val, Confidence: 0.9})
		}
	}
	return rows
}

func pioSnapshot(sector string, over map[string]float64) *Snapshot {
	return NewSnapshot("T.N0000", "Test PLC", sector, 60, 100, time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC), pioRows(over))
}

func TestPiotroskiAllPass(t *testing.T) {
	s := pioSnapshot("Manufacturing", nil)
	m := Compute(s)
	x, _ := m.Get("piotroski")
	if x.Value == nil || *x.Value != 8 || x.Display != "8 of 8 tests" || x.Group != "health" {
		t.Fatalf("piotroski = %+v", x)
	}
	if x.Note != "" {
		t.Errorf("note = %q, want none when all tests could be computed", x.Note)
	}
	passes := 0
	for _, s := range x.Steps {
		if strings.HasPrefix(s, "pass ") {
			passes++
		}
	}
	if passes != 8 || !strings.Contains(strings.Join(x.Steps, "\n"), "No new shares issued") {
		t.Errorf("steps = %v", x.Steps)
	}
	if x.Confidence != 0.9 || len(x.Used) == 0 {
		t.Errorf("confidence %v used %v", x.Confidence, x.Used)
	}
	if f, ok := findingByID(Evaluate(s, m), "piotroski"); !ok || f.Severity != Positive || f.Dimension != DimHealth {
		t.Errorf("finding = %+v", f)
	}
}

func TestPiotroskiNonePass(t *testing.T) {
	s := pioSnapshot("Manufacturing", map[string]float64{
		"pat_parent/2025": -50, "ocf/2025": -60, "total_liabilities/2025": 700, "current_assets/2025": 300,
		"gross_profit/2025": 300, "revenue/2025": 800,
	})
	m := Compute(s)
	x, _ := m.Get("piotroski")
	if x.Value == nil || *x.Value != 0 || x.Display != "0 of 8 tests" {
		t.Fatalf("piotroski = %+v", x)
	}
	if f, ok := findingByID(Evaluate(s, m), "piotroski"); !ok || f.Severity != Watch {
		t.Errorf("finding = %+v", f)
	}
}

func TestPiotroskiMixed(t *testing.T) {
	// Four tests fail: profit positive and cash flow positive still pass.
	s := pioSnapshot("Manufacturing", map[string]float64{
		"total_liabilities/2025": 700, "current_assets/2025": 300, "gross_profit/2025": 300, "pat_parent/2025": 80,
	})
	x, _ := Compute(s).Get("piotroski")
	if x.Value == nil {
		t.Fatalf("piotroski = %+v", x)
	}
	// Profit 80 vs 90 so ROA falls; OCF 200 > 80 passes; leverage, current ratio, gross margin fail; turnover passes.
	if *x.Value != 4 {
		t.Errorf("points = %v, want 4 (steps %v)", *x.Value, x.Steps)
	}
	if f, ok := findingByID(Evaluate(s, Compute(s)), "piotroski"); !ok || f.Severity != Neutral {
		t.Errorf("finding = %+v", f)
	}
}

func TestPiotroskiEqualDoesNotCount(t *testing.T) {
	// Leverage unchanged at 0.5 is not "falling"; the other seven pass.
	s := pioSnapshot("Manufacturing", map[string]float64{"total_liabilities/2025": 500})
	x, _ := Compute(s).Get("piotroski")
	if x.Value == nil || *x.Value != 7 || x.Display != "7 of 8 tests" {
		t.Errorf("piotroski = %+v", x)
	}
}

func TestPiotroskiPartialInputs(t *testing.T) {
	// No gross profit and no liabilities: six tests remain, and the note says so.
	s := pioSnapshot("Manufacturing", map[string]float64{
		"gross_profit/2025": -1e18, "gross_profit/2024": -1e18, "total_liabilities/2025": -1e18, "total_liabilities/2024": -1e18,
	})
	x, _ := Compute(s).Get("piotroski")
	if x.Value == nil || x.Display != "6 of 6 tests" || !strings.Contains(x.Note, "6 of 8") {
		t.Errorf("piotroski = %+v", x)
	}
}

func TestPiotroskiTooFewTests(t *testing.T) {
	rows := []Row{
		{PeriodEnd: "2025-03-31", PeriodType: "12M", Scope: "group", Field: "pat_parent", Source: "pdf", Value: 100, Confidence: 0.9},
		{PeriodEnd: "2024-03-31", PeriodType: "12M", Scope: "group", Field: "pat_parent", Source: "pdf", Value: 80, Confidence: 0.9},
	}
	s := NewSnapshot("T.N0000", "T", "Manufacturing", 60, 100, time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC), rows)
	m := Compute(s)
	x, _ := m.Get("piotroski")
	if x.Value != nil || !strings.Contains(x.Note, "only 1 of 8") {
		t.Errorf("piotroski = %+v", x)
	}
	if _, ok := findingByID(Evaluate(s, m), "piotroski"); ok {
		t.Error("no finding expected without a score")
	}
}

func TestPiotroskiSkippedForFinancials(t *testing.T) {
	x, ok := Compute(pioSnapshot("Banks Finance and Insurance", nil)).Get("piotroski")
	if !ok || !x.Skipped || x.Value != nil {
		t.Errorf("piotroski = %+v, want skipped for a bank", x)
	}
}

func TestPiotroskiNotInRegistry(t *testing.T) {
	if _, ok := Lookup("piotroski"); ok {
		t.Error("piotroski must not be in the calc registry")
	}
}
