package analysis

import (
	"math"
	"testing"
	"time"
)

func sameSeries(t *testing.T, name string, got, want []float64, tol float64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: length %d, want %d", name, len(got), len(want))
	}
	for i := range want {
		switch {
		case math.IsNaN(want[i]) != math.IsNaN(got[i]):
			t.Errorf("%s[%d] = %v, want %v", name, i, got[i], want[i])
		case !math.IsNaN(want[i]) && math.Abs(got[i]-want[i]) > tol:
			t.Errorf("%s[%d] = %v, want %v", name, i, got[i], want[i])
		}
	}
}

var nan = math.NaN()

func TestSMA(t *testing.T) {
	up := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	sameSeries(t, "sma3", SMA(up, 3), []float64{nan, nan, 2, 3, 4, 5, 6, 7, 8, 9}, 1e-9)
	// Warm-up edges: one value short is all NaN, exactly n has one value.
	sameSeries(t, "n-1", SMA([]float64{1, 2}, 3), []float64{nan, nan}, 0)
	sameSeries(t, "n", SMA([]float64{1, 2, 3}, 3), []float64{nan, nan, 2}, 1e-9)
	sameSeries(t, "n=0", SMA(up[:2], 0), []float64{nan, nan}, 0)
}

func TestEMA(t *testing.T) {
	up := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	// alpha 0.5, seeded with SMA(3)=2 at index 2; on a straight line EMA = close - 1.
	sameSeries(t, "ema3", EMA(up, 3), []float64{nan, nan, 2, 3, 4, 5, 6, 7, 8, 9}, 1e-9)
	sameSeries(t, "short", EMA([]float64{1, 2}, 3), []float64{nan, nan}, 0)
}

func TestRSI(t *testing.T) {
	up := []float64{1, 2, 3, 4, 5, 6}
	if v, _ := lastValid(RSI(up, 3)); v != 100 {
		t.Errorf("rising RSI = %v, want 100", v)
	}
	flat := []float64{5, 5, 5, 5, 5}
	if v, _ := lastValid(RSI(flat, 3)); v != 50 {
		t.Errorf("flat RSI = %v, want 50", v)
	}
	down := []float64{6, 5, 4, 3, 2}
	if v, _ := lastValid(RSI(down, 3)); v != 0 {
		t.Errorf("falling RSI = %v, want 0", v)
	}
	// Changes +1 -1 +1 -1: seed gain 2/3, loss 1/3 gives RS 2; then Wilder smoothing.
	got := RSI([]float64{1, 2, 1, 2, 1}, 3)
	sameSeries(t, "rsi", got, []float64{nan, nan, nan, 100 - 100/3.0, 100 - 100/1.8}, 1e-9)
	// First valid index is n, so n values are not enough.
	sameSeries(t, "n values", RSI([]float64{1, 2, 3}, 3), []float64{nan, nan, nan}, 0)
}

func TestMACD(t *testing.T) {
	up := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	// fast 2, slow 3, signal 2 on a straight line: the lag difference is a constant 0.5.
	line, sig, hist := MACD(up, 2, 3, 2)
	sameSeries(t, "line", line, []float64{nan, nan, .5, .5, .5, .5, .5, .5, .5, .5}, 1e-9)
	sameSeries(t, "signal", sig, []float64{nan, nan, nan, .5, .5, .5, .5, .5, .5, .5}, 1e-9)
	sameSeries(t, "hist", hist, []float64{nan, nan, nan, 0, 0, 0, 0, 0, 0, 0}, 1e-9)
}

func TestBollinger(t *testing.T) {
	mid, up, lo := Bollinger([]float64{5, 5, 5, 5, 5}, 3, 2)
	if mid[4] != 5 || up[4] != 5 || lo[4] != 5 {
		t.Errorf("constant series bands = %v %v %v, want all 5", lo[4], mid[4], up[4])
	}
	mid, up, lo = Bollinger([]float64{1, 2, 3}, 3, 2)
	sd := math.Sqrt(2.0 / 3) // population stdev
	if math.Abs(mid[2]-2) > 1e-9 || math.Abs(up[2]-(2+2*sd)) > 1e-9 || math.Abs(lo[2]-(2-2*sd)) > 1e-9 {
		t.Errorf("bands = %v %v %v", lo[2], mid[2], up[2])
	}
	if !math.IsNaN(mid[1]) || !math.IsNaN(up[1]) {
		t.Error("warm-up should be NaN")
	}
}

func TestTrueRangeAndATR(t *testing.T) {
	tr := TrueRange([]float64{10, 12, 13}, []float64{9, 11, 12}, []float64{9.5, 11.5, 12.5})
	sameSeries(t, "tr", tr, []float64{1, 2.5, 1.5}, 1e-9)
	// Missing high/low falls back to the close-to-close move.
	tr = TrueRange([]float64{10, nan}, []float64{9, nan}, []float64{9.5, 11.5})
	sameSeries(t, "tr nan", tr, []float64{1, 2}, 1e-9)

	atr := ATR([]float64{1, 2.5, 3}, 2)
	sameSeries(t, "atr", atr, []float64{nan, 1.75, 2.375}, 1e-9)
	sameSeries(t, "atr short", ATR([]float64{1}, 2), []float64{nan}, 0)
}

func TestStdevAndLogReturns(t *testing.T) {
	xs := []float64{2, 4, 4, 4, 5, 5, 7, 9}
	if got := stdev(xs, false); math.Abs(got-2) > 1e-9 {
		t.Errorf("population stdev = %v, want 2", got)
	}
	if got := stdev(xs, true); math.Abs(got-math.Sqrt(32.0/7)) > 1e-9 {
		t.Errorf("sample stdev = %v", got)
	}
	if got := stdev([]float64{5, 5, 5}, true); got != 0 {
		t.Errorf("constant stdev = %v, want 0", got)
	}
	if !math.IsNaN(stdev([]float64{1}, true)) {
		t.Error("sample stdev of one value should be NaN")
	}
	lr := LogReturns([]float64{1, math.E, 0})
	if !math.IsNaN(lr[0]) || math.Abs(lr[1]-1) > 1e-9 || !math.IsNaN(lr[2]) {
		t.Errorf("log returns = %v", lr)
	}
}

func TestMaxDrawdown(t *testing.T) {
	dd, p, tr, ok := MaxDrawdown([]float64{10, 8, 12, 6, 9})
	if !ok || math.Abs(dd+0.5) > 1e-9 || p != 2 || tr != 3 {
		t.Errorf("drawdown = %v peak %d trough %d, want -0.5 peak 2 trough 3", dd, p, tr)
	}
	if dd, _, _, _ := MaxDrawdown([]float64{1, 2, 3}); dd != 0 {
		t.Errorf("rising drawdown = %v, want 0", dd)
	}
	if _, _, _, ok := MaxDrawdown(nil); ok {
		t.Error("empty series should not be ok")
	}
}

func TestFormatUnits(t *testing.T) {
	for _, c := range []struct {
		v    float64
		u    Unit
		want string
	}{
		{0.0281, Change, "+2.81%"}, {-0.0011, Change, "-0.11%"}, {0, Change, "0.00%"}, {-0.00001, Change, "0.00%"},
		{61.234, Points, "61.23"}, {9, Count, "9"},
	} {
		if got := Format(c.v, c.u); got != c.want {
			t.Errorf("Format(%v, %s) = %q, want %q", c.v, c.u, got, c.want)
		}
	}
}

// bars builds n consecutive daily bars from 2025-01-01 whose close is 100+i,
// with no high or low and a volume of 1000.
func bars(n int) []Bar {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	out := make([]Bar, n)
	for i := range out {
		out[i] = Bar{Date: start.AddDate(0, 0, i), Close: 100 + float64(i), Volume: 1000}
	}
	return out
}

func techSnapshot(b, idx []Bar) *Snapshot {
	asOf := time.Date(2026, 2, 4, 0, 0, 0, 0, time.UTC)
	if len(b) > 0 {
		asOf = b[len(b)-1].Date
	}
	return NewSnapshot("T.N0000", "Test PLC", "", 60, 100, asOf, nil).WithBars(b, idx)
}

func techVal(t *testing.T, m Metrics, id string) float64 {
	t.Helper()
	v, ok := m.Val(id)
	if !ok {
		x, _ := m.Get(id)
		t.Fatalf("%s not computed (note %q)", id, x.Note)
	}
	return v
}

func TestTechnicalMetrics(t *testing.T) {
	b := bars(400) // 2025-01-01 .. 2026-02-04, closes 100..499
	b[399].Volume = 2000
	idx := bars(400)
	for i := range idx {
		idx[i].Close = 1000 + 2*float64(i) // the index rises half as fast in percent terms
	}
	m := Compute(techSnapshot(b, idx))

	near := func(id string, want float64) {
		t.Helper()
		if got := techVal(t, m, id); math.Abs(got-want) > 1e-6*math.Max(1, math.Abs(want)) {
			t.Errorf("%s = %v, want %v", id, got, want)
		}
	}
	near("sma-20", (480+499)/2.0) // mean of 480..499
	near("price-vs-sma-50", 499/474.5-1)
	near("price-vs-sma-200", 499/399.5-1)
	near("ema-12", 499-5.5) // on a straight line EMA lags by (1-a)/a = (n-1)/2 steps
	near("rsi-14", 100)
	near("max-drawdown", 0)
	near("return-1w", 499/492.0-1)
	near("return-ytd", 499/464.0-1) // close on 2025-12-31 is index 364
	near("return-1y", 499/134.0-1)  // close on 2025-02-04 is index 34
	near("from-high-52w", 0)
	near("volume-avg-20", 1000)
	near("volume-ratio", 2)
	near("active-days-14", 14)
	near("atr-14", 1) // no high/low: close-to-close moves of 1
	near("rel-strength-1y", (499/134.0-1)-(1798/1068.0-1))

	if a, _ := m.Get("atr-14"); a.Note == "" {
		t.Error("ATR without high/low should say it used close-to-close moves")
	}
	if d, _ := m.Get("max-drawdown"); d.Period == "" {
		t.Error("max drawdown should give the peak and trough dates")
	}
	if g, _ := m.Get("sma-20"); g.Period != "as of 2026-02-04" {
		t.Errorf("period = %q", g.Period)
	}
}

func TestTechnicalShortHistory(t *testing.T) {
	m := Compute(techSnapshot(bars(10), nil))
	for _, id := range []string{"sma-50", "sma-200", "price-vs-sma-50", "rsi-14", "macd", "return-1y", "rel-strength-3m", "active-days-14"} {
		x, ok := m.Get(id)
		if !ok {
			t.Errorf("%s missing from metrics", id)
			continue
		}
		if x.Value != nil || x.Note == "" {
			t.Errorf("%s = %v note %q, want no value and a note saying what it needs", id, x.Value, x.Note)
		}
	}
	// What 10 bars can support is still computed.
	if v := techVal(t, m, "return-1w"); math.Abs(v-(109/102.0-1)) > 1e-9 {
		t.Errorf("return-1w = %v", v)
	}
}

func TestTechnicalNoBars(t *testing.T) {
	m := Compute(techSnapshot(nil, nil))
	x, ok := m.Get("price-history")
	if !ok || x.Value != nil || x.Note == "" {
		t.Fatalf("price-history = %+v", x)
	}
	if _, ok := m.Get("rsi-14"); ok {
		t.Error("no per-indicator metrics expected without bars")
	}
}

func TestTechnicalDoesNotChangeCalcRegistry(t *testing.T) {
	for _, r := range Registry {
		if r.Group == groupTechnical {
			t.Errorf("%s: technical metrics must not enter the calc registry", r.ID)
		}
	}
}

func findingByID(fs []Finding, id string) (Finding, bool) {
	for _, f := range fs {
		if f.ID == id {
			return f, true
		}
	}
	return Finding{}, false
}

func TestTechnicalFindings(t *testing.T) {
	s := techSnapshot(bars(400), nil)
	fs := Evaluate(s, Compute(s))

	if f, ok := findingByID(fs, "rsi"); !ok || f.Severity != Watch || f.Dimension != DimTechnical {
		t.Errorf("rsi finding = %+v", f)
	}
	if f, ok := findingByID(fs, "trend"); !ok || f.Severity != Positive {
		t.Errorf("trend finding = %+v", f)
	}
	// A straight-line rise has a constant MACD, so the histogram is zero.
	if f, ok := findingByID(fs, "macd"); !ok || f.Severity != Neutral {
		t.Errorf("macd finding = %+v", f)
	}
	if f, ok := findingByID(fs, "range"); !ok || f.Title != "Near the 52-week high" {
		t.Errorf("range finding = %+v", f)
	}
	if _, ok := findingByID(fs, "drawdown"); ok {
		t.Error("no drawdown finding expected on a rising series")
	}
	if _, ok := findingByID(fs, "thin-trading"); ok {
		t.Error("a stock that moves every day is not thinly traded")
	}
	if _, ok := findingByID(fs, "stale-prices"); ok {
		t.Error("prices are current")
	}
	for _, f := range fs {
		if f.Detail == "" || f.Title == "" {
			t.Errorf("finding %s has no text", f.ID)
		}
	}
}

func TestThinTradingAndStalePrices(t *testing.T) {
	b := bars(100)
	for i := 80; i < 100; i++ { // the last 20 closes repeat
		b[i].Close = b[79].Close
	}
	s := techSnapshot(b, nil)
	s.AsOf = b[99].Date.AddDate(0, 0, 30)
	fs := Evaluate(s, Compute(s))
	if f, ok := findingByID(fs, "thin-trading"); !ok || f.Dimension != DimData {
		t.Errorf("thin-trading = %+v", f)
	}
	if f, ok := findingByID(fs, "stale-prices"); !ok || f.Dimension != DimData {
		t.Errorf("stale-prices = %+v", f)
	}
}

func TestDrawdownFinding(t *testing.T) {
	b := bars(100)
	for i := 50; i < 100; i++ {
		b[i].Close = 60 // a fall of about 40% from 149
	}
	s := techSnapshot(b, nil)
	if f, ok := findingByID(Evaluate(s, Compute(s)), "drawdown"); !ok || f.Severity != Watch {
		t.Errorf("drawdown finding = %+v", f)
	}
}

func TestTechnicalSection(t *testing.T) {
	s := techSnapshot(bars(60), nil)
	r := Analyze(s)
	if err := r.Filter([]string{"technical"}); err != nil {
		t.Fatal(err)
	}
	if len(r.Metrics) == 0 {
		t.Fatal("technical section is empty")
	}
	for _, m := range r.Metrics {
		if m.Group != groupTechnical {
			t.Errorf("%s in group %s after filtering to technical", m.ID, m.Group)
		}
	}
	found := false
	for _, n := range Sections() {
		found = found || n == "technical"
	}
	if !found {
		t.Errorf("Sections() = %v, want technical", Sections())
	}
}
