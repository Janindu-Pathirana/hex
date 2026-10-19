package analysis

import (
	"fmt"
	"math"
	"strings"
)

const groupLiquidity = "liquidity"

// Liquidity settings. Thresholds are absolute: the CSE has no tick-by-tick
// depth in the data hex stores, so liquidity is judged from daily bars only.
const (
	liqWindow      = 60        // bars in the main window
	liqMinBars     = 20        // fewer bars than this and nothing is computed
	liqTurnoverWin = 20        // bars for the average turnover
	liqTradedMin   = 0.5       // share of days with volume below this is thin trading
	liqTurnoverMin = 250_000.0 // average daily turnover (LKR) below this is thin trading
	liqGood        = 0.9       // share of days with volume at or above this, with turnover above the minimum, is liquid
)

// computeLiquidity measures how often and how heavily the stock trades, from
// the stored daily bars. A stored volume of 0 means none was recorded, which
// is treated as no trade. Turnover is close x volume, an estimate of the
// value traded that ignores intraday price moves.
func computeLiquidity(s *Snapshot) []Metric {
	bars := s.bars
	if len(bars) == 0 {
		return nil
	}
	m := func(id, name string, u Unit, v float64, period string, steps ...string) Metric {
		val := v
		return Metric{ID: id, Name: name, Group: groupLiquidity, Unit: u, Value: &val, Display: Format(v, u),
			Period: period, Steps: steps}
	}
	if len(bars) < liqMinBars {
		var out []Metric
		for _, x := range [][2]string{{"traded-days", "Share of days traded"}, {"zero-return-days", "Share of days with no price change"},
			{"avg-turnover", "Average daily turnover (LKR)"}, {"amihud", "Amihud illiquidity"}} {
			out = append(out, Metric{ID: x[0], Name: x[1], Group: groupLiquidity, Display: "n/m",
				Note: fmt.Sprintf("needs %d bars; %d bars stored", liqMinBars, len(bars))})
		}
		return out
	}

	w := liqWindow
	if w > len(bars)-1 {
		w = len(bars) - 1
	}
	win := bars[len(bars)-w-1:] // one extra bar so the first day has a return
	period := fmt.Sprintf("last %d days to %s", w, bars[len(bars)-1].Date.Format(dateLayout))

	var traded, zero int
	var amihud float64
	var amihudDays int
	for i := 1; i < len(win); i++ {
		b, prev := win[i], win[i-1]
		if b.Volume > 0 {
			traded++
		}
		if b.Close == prev.Close {
			zero++
		}
		if b.Volume > 0 && prev.Close > 0 {
			ret := math.Abs(b.Close/prev.Close - 1)
			amihud += ret / (b.Close * b.Volume / 1e6)
			amihudDays++
		}
	}
	tw := liqTurnoverWin
	if tw > len(bars) {
		tw = len(bars)
	}
	var turnover float64
	for _, b := range bars[len(bars)-tw:] {
		turnover += b.Close * b.Volume
	}
	turnover /= float64(tw)

	out := []Metric{
		m("traded-days", "Share of days traded", Percent, float64(traded)/float64(w), period,
			fmt.Sprintf("%d of %d days had volume", traded, w)),
		m("zero-return-days", "Share of days with no price change", Percent, float64(zero)/float64(w), period,
			fmt.Sprintf("%d of %d closes equal the previous close", zero, w)),
		m("avg-turnover", "Average daily turnover (LKR)", LKR, turnover,
			fmt.Sprintf("last %d days to %s", tw, bars[len(bars)-1].Date.Format(dateLayout)),
			"mean of close x volume"),
	}
	if amihudDays > 0 {
		out = append(out, m("amihud", "Amihud illiquidity (return per LKR 1m traded)", LKR, amihud/float64(amihudDays), period,
			"mean of |daily return| / (turnover in LKR millions) over days with volume; higher means a trade moves the price more"))
	} else {
		out = append(out, Metric{ID: "amihud", Name: "Amihud illiquidity (return per LKR 1m traded)", Group: groupLiquidity,
			Unit: LKR, Display: "n/m", Note: "no days with volume in the window"})
	}
	return out
}

// liquidityRule is the liquidity gate: it says whether prices and indicators
// for this stock can be trusted.
func liquidityRule(c *ctx) *Finding {
	traded, ok1 := c.val("traded-days")
	turnover, ok2 := c.val("avg-turnover")
	if !ok1 || !ok2 {
		return nil
	}
	ids := []string{"traded-days", "avg-turnover", "zero-return-days"}
	switch {
	case traded < liqTradedMin || turnover < liqTurnoverMin:
		var why []string
		if traded < liqTradedMin {
			why = append(why, fmt.Sprintf("it traded on %.0f%% of recent days (guide: at least %.0f%%)", traded*100, liqTradedMin*100))
		}
		if turnover < liqTurnoverMin {
			why = append(why, fmt.Sprintf("average daily turnover is LKR %s (guide: at least LKR %s)", Num(turnover), Num(liqTurnoverMin)))
		}
		return c.find("liquidity", DimLiquidity, Watch, "Thinly traded: treat prices and indicators with care",
			"Because "+strings.Join(why, " and ")+absolute+".", ids...)
	case traded >= liqGood:
		return c.find("liquidity", DimLiquidity, Positive, "Trades most days",
			fmt.Sprintf("Traded on %.0f%% of recent days with average turnover of LKR %s.", traded*100, Num(turnover)), ids...)
	}
	return c.find("liquidity", DimLiquidity, Neutral, "Trades on most days but not all",
		fmt.Sprintf("Traded on %.0f%% of recent days with average turnover of LKR %s.", traded*100, Num(turnover)), ids...)
}
