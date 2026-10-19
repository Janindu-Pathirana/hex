package analysis

import (
	"math"
	"testing"
)

// liquidBars: closes 100..(100+n-1), volume 5000 every day.
func liquidBars(n int) []Bar {
	b := bars(n)
	for i := range b {
		b[i].Volume = 5000
	}
	return b
}

func TestLiquidityLiquidStock(t *testing.T) {
	b := liquidBars(100)
	s := techSnapshot(b, nil)
	m := Compute(s)

	if v := techVal(t, m, "traded-days"); v != 1 {
		t.Errorf("traded-days = %v, want 1", v)
	}
	if v := techVal(t, m, "zero-return-days"); v != 0 {
		t.Errorf("zero-return-days = %v, want 0", v)
	}
	// Last 20 closes are 180..199, mean 189.5, volume 5000.
	if v := techVal(t, m, "avg-turnover"); math.Abs(v-189.5*5000) > 1e-6 {
		t.Errorf("avg-turnover = %v, want %v", v, 189.5*5000)
	}
	// Amihud over the last 60 returns, worked out directly.
	var sum float64
	for i := 40; i < 100; i++ {
		ret := math.Abs(b[i].Close/b[i-1].Close - 1)
		sum += ret / (b[i].Close * b[i].Volume / 1e6)
	}
	if v := techVal(t, m, "amihud"); math.Abs(v-sum/60) > 1e-12 {
		t.Errorf("amihud = %v, want %v", v, sum/60)
	}
	if x, _ := m.Get("traded-days"); x.Period != "last 60 days to 2025-04-10" {
		t.Errorf("period = %q", x.Period)
	}

	f, ok := findingByID(Evaluate(s, m), "liquidity")
	if !ok || f.Severity != Positive || f.Dimension != DimLiquidity {
		t.Errorf("liquidity finding = %+v", f)
	}
}

func TestLiquidityThinStock(t *testing.T) {
	b := bars(100) // volume 1000, turnover about 140k, below the guide
	for i := range b {
		if i%2 == 0 {
			b[i].Volume = 0
		}
		b[i].Close = 50 // and the close never moves
	}
	s := techSnapshot(b, nil)
	m := Compute(s)
	if v := techVal(t, m, "traded-days"); math.Abs(v-0.5) > 1e-9 {
		t.Errorf("traded-days = %v, want 0.5", v)
	}
	if v := techVal(t, m, "zero-return-days"); v != 1 {
		t.Errorf("zero-return-days = %v, want 1", v)
	}
	if v := techVal(t, m, "amihud"); v != 0 {
		t.Errorf("amihud = %v, want 0 when the price never moves", v)
	}
	if f, ok := findingByID(Evaluate(s, m), "liquidity"); !ok || f.Severity != Watch {
		t.Errorf("liquidity finding = %+v", f)
	}
}

func TestLiquidityMiddle(t *testing.T) {
	b := liquidBars(100)
	for i := 0; i < 100; i += 4 { // every fourth day has no volume: 75% traded
		b[i].Volume = 0
	}
	s := techSnapshot(b, nil)
	if f, ok := findingByID(Evaluate(s, Compute(s)), "liquidity"); !ok || f.Severity != Neutral {
		t.Errorf("liquidity finding = %+v", f)
	}
}

func TestLiquidityShortAndEmpty(t *testing.T) {
	m := Compute(techSnapshot(liquidBars(10), nil))
	for _, id := range []string{"traded-days", "zero-return-days", "avg-turnover", "amihud"} {
		x, ok := m.Get(id)
		if !ok || x.Value != nil || x.Note == "" {
			t.Errorf("%s = %+v, want a note and no value", id, x)
		}
	}
	if _, ok := findingByID(Evaluate(techSnapshot(liquidBars(10), nil), m), "liquidity"); ok {
		t.Error("no liquidity finding expected without enough bars")
	}
	if _, ok := Compute(techSnapshot(nil, nil)).Get("traded-days"); ok {
		t.Error("no liquidity metrics expected without bars")
	}
}

func TestLiquidityNoVolumeAtAll(t *testing.T) {
	b := bars(40)
	for i := range b {
		b[i].Volume = 0
	}
	m := Compute(techSnapshot(b, nil))
	if x, _ := m.Get("amihud"); x.Value != nil || x.Note == "" {
		t.Errorf("amihud = %+v, want n/m with a note", x)
	}
	if v := techVal(t, m, "traded-days"); v != 0 {
		t.Errorf("traded-days = %v, want 0", v)
	}
}

func TestLiquiditySection(t *testing.T) {
	r := Analyze(techSnapshot(liquidBars(60), nil))
	if err := r.Filter([]string{"liquidity"}); err != nil {
		t.Fatal(err)
	}
	if len(r.Metrics) != 4 {
		t.Errorf("liquidity section has %d metrics, want 4", len(r.Metrics))
	}
	for _, m := range r.Metrics {
		if m.Group != groupLiquidity {
			t.Errorf("%s in group %s", m.ID, m.Group)
		}
	}
}
