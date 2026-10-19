package analysis

import (
	"math"
	"testing"
	"time"
)

// marketDays builds n consecutive days from 2025-01-01: ASPI 10000+10i,
// S&P SL20 3000+3i, turnover 100m with foreign buying 60m and selling 50m.
func marketDays(n int) []MarketDay {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	out := make([]MarketDay, n)
	for i := range out {
		out[i] = MarketDay{Date: start.AddDate(0, 0, i), ASI: 10000 + 10*float64(i), SPSL20: 3000 + 3*float64(i),
			Turnover: 100e6, ForeignBuy: 60e6, ForeignSell: 50e6, Listed: 290, Traded: 100, PER: 9.5, DY: 3.2}
	}
	return out
}

func marketSnapshot(days []MarketDay) *Snapshot {
	return techSnapshot(bars(100), nil).WithMarket(days)
}

func TestMarketMetrics(t *testing.T) {
	s := marketSnapshot(marketDays(100))
	m := Compute(s)
	near := func(id string, want float64) {
		t.Helper()
		if got := techVal(t, m, id); math.Abs(got-want) > 1e-9*math.Max(1, math.Abs(want)) {
			t.Errorf("%s = %v, want %v", id, got, want)
		}
	}
	near("aspi-return-3m", 10990.0/10090-1) // start 2025-01-10 is index 9
	near("aspi-return-1m", 10990.0/10680-1) // start 2025-03-10 is index 68
	near("aspi-vs-sma-50", 10990/10745.0-1) // mean of ASPI over the last 50 days
	near("rel-strength-spsl20-3m", (199.0/109-1)-(3297.0/3027-1))
	near("market-turnover-ratio", 1)
	near("foreign-net-flow-20", 0.1)
	near("market-traded-share", 100.0/290)
	near("market-per", 9.5)
	near("market-dy", 3.2)
	if v := techVal(t, m, "aspi-volatility-20"); v <= 0 {
		t.Errorf("aspi-volatility-20 = %v, want positive", v)
	}
	if x, _ := m.Get("foreign-net-flow-20"); x.Period != "last 20 days to 2025-04-10" {
		t.Errorf("period = %q", x.Period)
	}

	fs := Evaluate(s, m)
	for id, want := range map[string]Severity{"market-trend": Positive, "rel-strength": Positive, "foreign-flow": Positive} {
		if f, ok := findingByID(fs, id); !ok || f.Severity != want || f.Dimension != DimMarket {
			t.Errorf("%s finding = %+v, want %s", id, f, want)
		}
	}
	if _, ok := findingByID(fs, "stale-market"); ok {
		t.Error("market data is current")
	}
}

func TestMarketDownturnAndForeignSelling(t *testing.T) {
	days := marketDays(100)
	for i := range days {
		days[i].ASI = 12000 - 10*float64(i)
		days[i].SPSL20 = 4000 - 3*float64(i)
		days[i].ForeignBuy, days[i].ForeignSell = 40e6, 50e6
	}
	s := marketSnapshot(days)
	fs := Evaluate(s, Compute(s))
	if f, ok := findingByID(fs, "market-trend"); !ok || f.Severity != Watch {
		t.Errorf("market-trend = %+v", f)
	}
	if f, ok := findingByID(fs, "foreign-flow"); !ok || f.Severity != Watch {
		t.Errorf("foreign-flow = %+v", f)
	}
}

func TestMarketShortHistory(t *testing.T) {
	m := Compute(marketSnapshot(marketDays(10)))
	for _, id := range []string{"aspi-return-3m", "aspi-vs-sma-50", "aspi-volatility-20", "market-turnover-ratio", "foreign-net-flow-20"} {
		x, ok := m.Get(id)
		if !ok || x.Value != nil || x.Note == "" {
			t.Errorf("%s = %+v, want a note and no value", id, x)
		}
	}
	if v := techVal(t, m, "market-traded-share"); math.Abs(v-100.0/290) > 1e-9 {
		t.Errorf("market-traded-share = %v", v)
	}
}

func TestMarketEmptyAndStale(t *testing.T) {
	m := Compute(marketSnapshot(nil))
	if x, ok := m.Get("market-history"); !ok || x.Value != nil || x.Note == "" {
		t.Errorf("market-history = %+v", x)
	}
	days := marketDays(100)
	s := marketSnapshot(days)
	s.AsOf = days[99].Date.AddDate(0, 0, 30)
	if f, ok := findingByID(Evaluate(s, Compute(s)), "stale-market"); !ok || f.Dimension != DimData {
		t.Errorf("stale-market = %+v", f)
	}
}

func TestMarketUnsortedInputAndUnknownIndex(t *testing.T) {
	days := marketDays(100)
	days[0], days[99] = days[99], days[0] // out of order
	for i := range days {
		days[i].SPSL20 = 0 // index level not stored
	}
	m := Compute(marketSnapshot(days))
	if v := techVal(t, m, "aspi-return-3m"); math.Abs(v-(10990.0/10090-1)) > 1e-9 {
		t.Errorf("aspi-return-3m = %v after sorting", v)
	}
	if _, ok := m.Get("rel-strength-spsl20-3m"); ok {
		t.Error("no S&P SL20 comparison expected without index levels")
	}
}

func TestMarketSectionAndRegistry(t *testing.T) {
	r := Analyze(marketSnapshot(marketDays(60)))
	if err := r.Filter([]string{"market"}); err != nil {
		t.Fatal(err)
	}
	if len(r.Metrics) == 0 {
		t.Fatal("market section is empty")
	}
	for _, m := range r.Metrics {
		if m.Group != groupMarket {
			t.Errorf("%s in group %s", m.ID, m.Group)
		}
	}
	for _, rt := range Registry {
		if rt.Group == groupMarket {
			t.Errorf("%s: market metrics must not enter the calc registry", rt.ID)
		}
	}
}
