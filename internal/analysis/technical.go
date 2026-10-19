package analysis

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// tradingDays is the number of daily bars in a CSE year (what the one-year
// chart returns), used to annualise volatility.
const tradingDays = 240

// Indicator functions. Each takes a series ordered oldest to newest and
// returns a slice of the same length, with NaN during warm-up. They never
// panic on short input: they return all NaN.

func nans(n int) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = math.NaN()
	}
	return out
}

// SMA is the simple moving average of the last n values at each index. The
// first valid index is n-1.
func SMA(xs []float64, n int) []float64 {
	out := nans(len(xs))
	if n <= 0 || len(xs) < n {
		return out
	}
	sum := 0.0
	for i, x := range xs {
		sum += x
		if i >= n {
			sum -= xs[i-n]
		}
		if i >= n-1 {
			out[i] = sum / float64(n)
		}
	}
	return out
}

// EMA is the exponential moving average with alpha = 2/(n+1), seeded with the
// SMA of the first n values (so the first valid index is n-1). Other tools
// seed differently, so early values can differ from theirs.
func EMA(xs []float64, n int) []float64 {
	out := nans(len(xs))
	if n <= 0 || len(xs) < n {
		return out
	}
	seed := 0.0
	for _, x := range xs[:n] {
		seed += x
	}
	out[n-1] = seed / float64(n)
	alpha := 2 / float64(n+1)
	for i := n; i < len(xs); i++ {
		out[i] = alpha*xs[i] + (1-alpha)*out[i-1]
	}
	return out
}

// RSI is Wilder's relative strength index. The averages are seeded with the
// simple mean of the first n changes (first valid index n), then smoothed with
// alpha = 1/n. With no losses RSI is 100; with no movement at all it is 50.
func RSI(xs []float64, n int) []float64 {
	out := nans(len(xs))
	if n <= 0 || len(xs) <= n {
		return out
	}
	var gain, loss float64
	for i := 1; i <= n; i++ {
		d := xs[i] - xs[i-1]
		if d > 0 {
			gain += d
		} else {
			loss -= d
		}
	}
	gain /= float64(n)
	loss /= float64(n)
	out[n] = rsiValue(gain, loss)
	for i := n + 1; i < len(xs); i++ {
		d := xs[i] - xs[i-1]
		g, l := 0.0, 0.0
		if d > 0 {
			g = d
		} else {
			l = -d
		}
		gain = (gain*float64(n-1) + g) / float64(n)
		loss = (loss*float64(n-1) + l) / float64(n)
		out[i] = rsiValue(gain, loss)
	}
	return out
}

func rsiValue(gain, loss float64) float64 {
	switch {
	case loss == 0 && gain == 0:
		return 50
	case loss == 0:
		return 100
	}
	return 100 - 100/(1+gain/loss)
}

// MACD returns the MACD line (EMA fast minus EMA slow), the signal line (an
// EMA of the valid MACD values) and the histogram.
func MACD(xs []float64, fast, slow, signal int) (line, sig, hist []float64) {
	line, sig, hist = nans(len(xs)), nans(len(xs)), nans(len(xs))
	f, s := EMA(xs, fast), EMA(xs, slow)
	first := -1
	for i := range xs {
		if !math.IsNaN(f[i]) && !math.IsNaN(s[i]) {
			line[i] = f[i] - s[i]
			if first < 0 {
				first = i
			}
		}
	}
	if first < 0 {
		return
	}
	for i, v := range EMA(line[first:], signal) {
		if !math.IsNaN(v) {
			sig[first+i] = v
			hist[first+i] = line[first+i] - v
		}
	}
	return
}

// Bollinger returns the middle band (SMA n) and the bands k population
// standard deviations either side.
func Bollinger(xs []float64, n int, k float64) (mid, upper, lower []float64) {
	mid, upper, lower = SMA(xs, n), nans(len(xs)), nans(len(xs))
	for i := n - 1; n > 0 && i < len(xs); i++ {
		if math.IsNaN(mid[i]) {
			continue
		}
		sd := stdev(xs[i-n+1:i+1], false)
		upper[i], lower[i] = mid[i]+k*sd, mid[i]-k*sd
	}
	return
}

// TrueRange is max(H-L, |H-prevC|, |L-prevC|). The first bar has no previous
// close, so it is H-L. High and low may be NaN (the CSE leaves them out
// sometimes); then the range is the move from the previous close.
func TrueRange(high, low, closes []float64) []float64 {
	out := make([]float64, len(closes))
	for i := range closes {
		h, l := high[i], low[i]
		if i == 0 {
			if math.IsNaN(h) || math.IsNaN(l) {
				out[i] = 0
			} else {
				out[i] = h - l
			}
			continue
		}
		pc := closes[i-1]
		if math.IsNaN(h) || math.IsNaN(l) {
			out[i] = math.Abs(closes[i] - pc)
			continue
		}
		out[i] = math.Max(h-l, math.Max(math.Abs(h-pc), math.Abs(l-pc)))
	}
	return out
}

// ATR is Wilder's average true range: the mean of the first n true ranges,
// then smoothed with alpha = 1/n. The first valid index is n-1.
func ATR(tr []float64, n int) []float64 {
	out := nans(len(tr))
	if n <= 0 || len(tr) < n {
		return out
	}
	sum := 0.0
	for _, v := range tr[:n] {
		sum += v
	}
	out[n-1] = sum / float64(n)
	for i := n; i < len(tr); i++ {
		out[i] = (out[i-1]*float64(n-1) + tr[i]) / float64(n)
	}
	return out
}

// LogReturns returns ln(C_t/C_{t-1}); index 0 is NaN. Non-positive prices
// give NaN.
func LogReturns(xs []float64) []float64 {
	out := nans(len(xs))
	for i := 1; i < len(xs); i++ {
		if xs[i] > 0 && xs[i-1] > 0 {
			out[i] = math.Log(xs[i] / xs[i-1])
		}
	}
	return out
}

// stdev ignores NaN. sample uses n-1, otherwise n. It returns NaN when there
// are too few values.
func stdev(xs []float64, sample bool) float64 {
	var vals []float64
	for _, x := range xs {
		if !math.IsNaN(x) {
			vals = append(vals, x)
		}
	}
	n := float64(len(vals))
	div := n
	if sample {
		div = n - 1
	}
	if n < 1 || div < 1 {
		return math.NaN()
	}
	mean := 0.0
	for _, v := range vals {
		mean += v
	}
	mean /= n
	ss := 0.0
	for _, v := range vals {
		ss += (v - mean) * (v - mean)
	}
	return math.Sqrt(ss / div)
}

// MaxDrawdown returns the largest fall from a running peak as a negative
// fraction (0 when the series never falls) and the indexes of that peak and
// trough. ok is false for an empty series.
func MaxDrawdown(xs []float64) (dd float64, peak, trough int, ok bool) {
	if len(xs) == 0 {
		return 0, 0, 0, false
	}
	runPeak := 0
	for i, x := range xs {
		if x > xs[runPeak] {
			runPeak = i
		}
		if xs[runPeak] > 0 {
			if d := x/xs[runPeak] - 1; d < dd {
				dd, peak, trough = d, runPeak, i
			}
		}
	}
	return dd, peak, trough, true
}

// lastValid returns the final non-NaN value.
func lastValid(xs []float64) (float64, bool) {
	for i := len(xs) - 1; i >= 0; i-- {
		if !math.IsNaN(xs[i]) {
			return xs[i], true
		}
	}
	return 0, false
}

// Bar is one daily price bar. High and Low are nil when the CSE gave none.
type Bar struct {
	Date      time.Time
	High, Low *float64
	Close     float64
	Volume    float64
}

// WithBars adds daily price history for the stock and for the index used as a
// benchmark (index.Close holds the index value). Either may be empty.
func (s *Snapshot) WithBars(bars, index []Bar) *Snapshot {
	s.bars, s.index = sortedBars(bars), sortedBars(index)
	return s
}

func sortedBars(in []Bar) []Bar {
	out := append([]Bar(nil), in...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Date.Before(out[j].Date) })
	return out
}

// Technical metric thresholds used by the metrics and rules.
const (
	activeWindow  = 14 // bars checked for real trading
	activeMin     = 10 // fewer price changes than this in the window means thin trading
	rangeDays     = 365
	staleBarsDays = 10
)

func closes(bars []Bar) []float64 {
	out := make([]float64, len(bars))
	for i, b := range bars {
		out[i] = b.Close
	}
	return out
}

// closeOnOrBefore returns the last close dated on or before d. ok is false
// when the history starts after d.
func closeOnOrBefore(bars []Bar, d time.Time) (float64, bool) {
	for i := len(bars) - 1; i >= 0; i-- {
		if !bars[i].Date.After(d) {
			return bars[i].Close, true
		}
	}
	return 0, false
}

// periodReturn is the change from the last close on or before start to the
// last bar's close.
func periodReturn(bars []Bar, start time.Time) (float64, bool) {
	if len(bars) == 0 {
		return 0, false
	}
	c0, ok := closeOnOrBefore(bars, start)
	if !ok || c0 <= 0 {
		return 0, false
	}
	return bars[len(bars)-1].Close/c0 - 1, true
}

type techBuilder struct {
	out    []Metric
	asOf   string
	nBars  int
	closes []float64
}

func (t *techBuilder) add(id, name string, u Unit, v float64, steps ...string) {
	val := v
	t.out = append(t.out, Metric{ID: id, Name: name, Group: groupTechnical, Unit: u, Value: &val,
		Display: Format(v, u), Period: t.asOf, Steps: steps})
}

func (t *techBuilder) lack(id, name string, u Unit, need string) {
	t.out = append(t.out, Metric{ID: id, Name: name, Group: groupTechnical, Unit: u, Display: "n/m",
		Note: fmt.Sprintf("needs %s; %d bars stored", need, t.nBars)})
}

func (t *techBuilder) note(id, note string) {
	for i := range t.out {
		if t.out[i].ID == id {
			t.out[i].Note = note
		}
	}
}

func pctOf(v, base float64) float64 { return v/base - 1 }

// computeTechnical derives the price and volume indicators from the stored
// daily bars. History is as stored by the CSE: it is not adjusted for splits,
// bonus issues or rights, so a corporate action shows up as a price move.
func computeTechnical(s *Snapshot) []Metric {
	bars := s.bars
	if len(bars) == 0 {
		return []Metric{{ID: "price-history", Name: "Price history", Group: groupTechnical, Display: "n/m",
			Note: "no stored price history; run 'hex sync history'"}}
	}
	last := bars[len(bars)-1]
	t := &techBuilder{asOf: "as of " + last.Date.Format(dateLayout), nBars: len(bars), closes: closes(bars)}
	cl := t.closes
	price := last.Close

	// Returns over calendar windows.
	for _, w := range []struct {
		id, name string
		start    time.Time
	}{
		{"return-1w", "Return 1 week", last.Date.AddDate(0, 0, -7)},
		{"return-1m", "Return 1 month", last.Date.AddDate(0, -1, 0)},
		{"return-3m", "Return 3 months", last.Date.AddDate(0, -3, 0)},
		{"return-ytd", "Return year to date", time.Date(last.Date.Year()-1, 12, 31, 0, 0, 0, 0, last.Date.Location())},
		{"return-1y", "Return 1 year", last.Date.AddDate(-1, 0, 0)},
	} {
		if r, ok := periodReturn(bars, w.start); ok {
			t.add(w.id, w.name, Change, r, fmt.Sprintf("last close / close on or before %s - 1", w.start.Format(dateLayout)))
		} else {
			t.lack(w.id, w.name, Change, "history from "+w.start.Format(dateLayout))
		}
	}

	// Trend.
	for _, n := range []int{20, 50, 200} {
		id, name := fmt.Sprintf("sma-%d", n), fmt.Sprintf("SMA %d", n)
		if v, ok := lastValid(SMA(cl, n)); ok {
			t.add(id, name, LKR, v, fmt.Sprintf("mean of the last %d closes", n))
			if n > 20 {
				t.add(fmt.Sprintf("price-vs-sma-%d", n), fmt.Sprintf("Price vs SMA %d", n), Change, pctOf(price, v),
					fmt.Sprintf("close %s / SMA %d - 1", Num(price), n))
			}
		} else {
			t.lack(id, name, LKR, fmt.Sprintf("%d bars", n))
			if n > 20 {
				t.lack(fmt.Sprintf("price-vs-sma-%d", n), fmt.Sprintf("Price vs SMA %d", n), Change, fmt.Sprintf("%d bars", n))
			}
		}
	}
	for _, n := range []int{12, 26} {
		id, name := fmt.Sprintf("ema-%d", n), fmt.Sprintf("EMA %d", n)
		if v, ok := lastValid(EMA(cl, n)); ok {
			t.add(id, name, LKR, v, fmt.Sprintf("alpha 2/%d, seeded with the first %d-bar mean", n+1, n))
		} else {
			t.lack(id, name, LKR, fmt.Sprintf("%d bars", n))
		}
	}

	// Momentum.
	if v, ok := lastValid(RSI(cl, 14)); ok {
		t.add("rsi-14", "RSI 14", Points, v, "Wilder smoothing, 14 days")
	} else {
		t.lack("rsi-14", "RSI 14", Points, "15 bars")
	}
	line, sig, hist := MACD(cl, 12, 26, 9)
	if m, ok := lastValid(line); ok {
		t.add("macd", "MACD 12/26", Points, m, "EMA 12 - EMA 26")
	} else {
		t.lack("macd", "MACD 12/26", Points, "26 bars")
	}
	if v, ok := lastValid(sig); ok {
		t.add("macd-signal", "MACD signal 9", Points, v, "EMA 9 of the MACD line")
		h, _ := lastValid(hist)
		t.add("macd-hist", "MACD histogram", Points, h, "MACD - signal")
	} else {
		t.lack("macd-signal", "MACD signal 9", Points, "34 bars")
		t.lack("macd-hist", "MACD histogram", Points, "34 bars")
	}

	// Volatility and range.
	if mid, up, lo := Bollinger(cl, 20, 2); !math.IsNaN(mid[len(mid)-1]) {
		u, l := up[len(up)-1], lo[len(lo)-1]
		if u > l {
			t.add("bollinger-b", "Bollinger %B (20, 2σ)", Points, (price-l)/(u-l), "(close - lower band) / (upper - lower)")
		} else {
			t.lack("bollinger-b", "Bollinger %B (20, 2σ)", Points, "a price that moved in the last 20 days")
		}
	} else {
		t.lack("bollinger-b", "Bollinger %B (20, 2σ)", Points, "20 bars")
	}
	t.atr(bars)
	t.volatility(bars)
	if dd, p, tr, ok := MaxDrawdown(cl); ok {
		t.add("max-drawdown", "Max drawdown", Percent, dd, "largest fall from a running peak over all stored bars")
		t.out[len(t.out)-1].Period = fmt.Sprintf("peak %s, trough %s", bars[p].Date.Format(dateLayout), bars[tr].Date.Format(dateLayout))
	}
	t.rangeAndVolume(bars)
	t.relative(bars, s.index)
	t.activity(bars)
	return t.out
}

func (t *techBuilder) atr(bars []Bar) {
	hi, lo := make([]float64, len(bars)), make([]float64, len(bars))
	missing := false
	for i, b := range bars {
		hi[i], lo[i] = math.NaN(), math.NaN()
		if b.High != nil && b.Low != nil {
			hi[i], lo[i] = *b.High, *b.Low
		} else {
			missing = true
		}
	}
	v, ok := lastValid(ATR(TrueRange(hi, lo, t.closes), 14))
	if !ok {
		t.lack("atr-14", "ATR 14", LKR, "14 bars")
		t.lack("atr-pct", "ATR 14 % of price", Percent, "14 bars")
		return
	}
	t.add("atr-14", "ATR 14", LKR, v, "Wilder average of true range, 14 days")
	t.add("atr-pct", "ATR 14 % of price", Percent, v/t.closes[len(t.closes)-1], "ATR / close")
	if missing {
		t.note("atr-14", "some bars have no high/low; those days use the close-to-close move")
	}
}

func (t *techBuilder) volatility(bars []Bar) {
	lr := LogReturns(t.closes)
	ann := math.Sqrt(tradingDays)
	if len(lr) > 20 {
		if sd := stdev(lr[len(lr)-20:], true); !math.IsNaN(sd) {
			t.add("volatility-20", "Volatility 20 days (annualised)", Percent, sd*ann, "sample stdev of 20 daily log returns x sqrt(240)")
		}
	} else {
		t.lack("volatility-20", "Volatility 20 days (annualised)", Percent, "21 bars")
	}
	last := bars[len(bars)-1].Date
	start := 0
	for start < len(bars) && bars[start].Date.Before(last.AddDate(-1, 0, 0)) {
		start++
	}
	if len(lr)-start >= 60 {
		if sd := stdev(lr[start+1:], true); !math.IsNaN(sd) {
			t.add("volatility-1y", "Volatility 1 year (annualised)", Percent, sd*ann, "sample stdev of daily log returns x sqrt(240)")
			return
		}
	}
	t.lack("volatility-1y", "Volatility 1 year (annualised)", Percent, "60 daily returns in the last year")
}

func (t *techBuilder) rangeAndVolume(bars []Bar) {
	last := bars[len(bars)-1]
	hi, lo := last.Close, last.Close
	cutoff := last.Date.AddDate(0, 0, -rangeDays)
	for _, b := range bars {
		if b.Date.Before(cutoff) {
			continue
		}
		hi, lo = math.Max(hi, b.Close), math.Min(lo, b.Close)
	}
	if bars[0].Date.After(cutoff.AddDate(0, 0, 14)) {
		t.lack("from-high-52w", "Close vs 52-week high", Change, "a year of history")
		t.lack("from-low-52w", "Close vs 52-week low", Change, "a year of history")
	} else {
		t.add("from-high-52w", "Close vs 52-week high", Change, pctOf(last.Close, hi), "close / highest close in 365 days - 1")
		t.add("from-low-52w", "Close vs 52-week low", Change, pctOf(last.Close, lo), "close / lowest close in 365 days - 1")
	}
	if len(bars) > 20 {
		sum := 0.0
		for _, b := range bars[len(bars)-21 : len(bars)-1] {
			sum += b.Volume
		}
		avg := sum / 20
		t.add("volume-avg-20", "Volume, 20-day average", LKR, avg, "mean volume of the 20 bars before the latest")
		if avg > 0 {
			t.add("volume-ratio", "Latest volume vs 20-day average", Times, last.Volume/avg, "latest volume / average")
		}
	} else {
		t.lack("volume-avg-20", "Volume, 20-day average", LKR, "21 bars")
	}
}

func (t *techBuilder) relative(bars, index []Bar) {
	last := bars[len(bars)-1].Date
	for _, w := range []struct {
		id, name string
		start    time.Time
	}{
		{"rel-strength-3m", "Return vs ASPI, 3 months", last.AddDate(0, -3, 0)},
		{"rel-strength-1y", "Return vs ASPI, 1 year", last.AddDate(-1, 0, 0)},
	} {
		sr, ok1 := periodReturn(bars, w.start)
		ir, ok2 := periodReturn(truncate(index, last), w.start)
		if ok1 && ok2 {
			t.add(w.id, w.name, Change, sr-ir, fmt.Sprintf("stock return %s - ASPI return %s", Format(sr, Change), Format(ir, Change)))
		} else {
			t.lack(w.id, w.name, Change, "stock and ASPI history from "+w.start.Format(dateLayout))
		}
	}
}

// truncate drops bars after d so both returns end on the same date.
func truncate(bars []Bar, d time.Time) []Bar {
	i := len(bars)
	for i > 0 && bars[i-1].Date.After(d) {
		i--
	}
	return bars[:i]
}

// activity counts the days in the last window on which the close changed.
// Many CSE stocks trade rarely and repeat the previous close on quiet days.
func (t *techBuilder) activity(bars []Bar) {
	if len(bars) <= activeWindow {
		t.lack("active-days-14", "Days with a price change, last 14", Count, "15 bars")
		return
	}
	n := 0
	for i := len(bars) - activeWindow; i < len(bars); i++ {
		if bars[i].Close != bars[i-1].Close {
			n++
		}
	}
	t.add("active-days-14", "Days with a price change, last 14", Count, float64(n), "closes that differ from the previous close")
}
