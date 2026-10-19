package analysis

import (
	"fmt"
	"math"
	"sort"
	"time"
)

const groupMarket = "market"

// Market context settings. Thresholds are absolute guides.
const (
	marketWindow     = 20   // days for flow, turnover and volatility
	flowStrong       = 0.05 // net foreign flow beyond +/-5% of turnover
	relStrengthPoint = 0.10 // 10 percentage points against the index
	marketStaleDays  = 10   // newest market day older than this is stale
)

// MarketDay is one day of market-wide statistics.
type MarketDay struct {
	Date                    time.Time
	ASI, SPSL20             float64 // index levels, 0 when unknown
	Turnover                float64
	ForeignBuy, ForeignSell float64
	Listed, Traded          float64
	PER, DY                 float64
}

// WithMarket adds the market-wide daily statistics.
func (s *Snapshot) WithMarket(days []MarketDay) *Snapshot {
	s.market = append([]MarketDay(nil), days...)
	sort.SliceStable(s.market, func(i, j int) bool { return s.market[i].Date.Before(s.market[j].Date) })
	return s
}

// indexSeries turns one index level of the market days into bars.
func indexSeries(days []MarketDay, level func(MarketDay) float64) []Bar {
	var out []Bar
	for _, d := range days {
		if v := level(d); v > 0 {
			out = append(out, Bar{Date: d.Date, Close: v})
		}
	}
	return out
}

// computeMarket describes the market the stock trades in: how the indices
// moved, how heavily the market traded, and what foreign investors did. All
// figures come from the daily market summary, not from the stock itself.
func computeMarket(s *Snapshot) []Metric {
	days := s.market
	if len(days) == 0 {
		return []Metric{{ID: "market-history", Name: "Market history", Group: groupMarket, Display: "n/m",
			Note: "no stored market days; run 'hex sync market'"}}
	}
	last := days[len(days)-1]
	period := "as of " + last.Date.Format(dateLayout)
	var out []Metric
	add := func(id, name string, u Unit, v float64, per string, steps ...string) {
		val := v
		out = append(out, Metric{ID: id, Name: name, Group: groupMarket, Unit: u, Value: &val,
			Display: Format(v, u), Period: per, Steps: steps})
	}
	lack := func(id, name string, u Unit, need string) {
		out = append(out, Metric{ID: id, Name: name, Group: groupMarket, Unit: u, Display: "n/m",
			Note: fmt.Sprintf("needs %s; %d market days stored", need, len(days))})
	}

	asi := indexSeries(days, func(d MarketDay) float64 { return d.ASI })
	sl20 := indexSeries(days, func(d MarketDay) float64 { return d.SPSL20 })

	for _, w := range []struct {
		id, name string
		months   int
	}{{"aspi-return-1m", "ASPI return 1 month", 1}, {"aspi-return-3m", "ASPI return 3 months", 3}} {
		if len(asi) == 0 {
			lack(w.id, w.name, Change, "ASPI levels")
			continue
		}
		start := asi[len(asi)-1].Date.AddDate(0, -w.months, 0)
		if r, ok := periodReturn(asi, start); ok {
			add(w.id, w.name, Change, r, period, "last ASPI / ASPI on or before "+start.Format(dateLayout)+" - 1")
		} else {
			lack(w.id, w.name, Change, "ASPI history from "+start.Format(dateLayout))
		}
	}
	if v, ok := lastValid(SMA(closes(asi), 50)); ok {
		add("aspi-vs-sma-50", "ASPI vs its 50-day average", Change, pctOf(asi[len(asi)-1].Close, v), period,
			"ASPI / SMA 50 of ASPI - 1")
	} else {
		lack("aspi-vs-sma-50", "ASPI vs its 50-day average", Change, "50 ASPI levels")
	}
	if lr := LogReturns(closes(asi)); len(lr) > marketWindow {
		if sd := stdev(lr[len(lr)-marketWindow:], true); !math.IsNaN(sd) {
			add("aspi-volatility-20", "ASPI volatility 20 days (annualised)", Percent, sd*math.Sqrt(tradingDays), period,
				"sample stdev of 20 daily log returns x sqrt(240)")
		}
	} else {
		lack("aspi-volatility-20", "ASPI volatility 20 days (annualised)", Percent, "21 ASPI levels")
	}

	// The stock against the S&P SL20, both ending on the stock's last bar.
	if len(s.bars) > 0 && len(sl20) > 0 {
		end := s.bars[len(s.bars)-1].Date
		start := end.AddDate(0, -3, 0)
		sr, ok1 := periodReturn(s.bars, start)
		ir, ok2 := periodReturn(truncate(sl20, end), start)
		if ok1 && ok2 {
			add("rel-strength-spsl20-3m", "Return vs S&P SL20, 3 months", Change, sr-ir, "as of "+end.Format(dateLayout),
				fmt.Sprintf("stock return %s - S&P SL20 return %s", Format(sr, Change), Format(ir, Change)))
		} else {
			lack("rel-strength-spsl20-3m", "Return vs S&P SL20, 3 months", Change, "stock and S&P SL20 history from "+start.Format(dateLayout))
		}
	}

	// Turnover and foreign flow over the last window.
	if len(days) > marketWindow {
		prev := days[len(days)-1-marketWindow : len(days)-1]
		var sum float64
		for _, d := range prev {
			sum += d.Turnover
		}
		if avg := sum / marketWindow; avg > 0 {
			add("market-turnover-ratio", "Market turnover vs 20-day average", Times, last.Turnover/avg, period,
				"latest turnover / mean of the 20 days before")
		}
		win := days[len(days)-marketWindow:]
		var net, turn float64
		for _, d := range win {
			net += d.ForeignBuy - d.ForeignSell
			turn += d.Turnover
		}
		if turn > 0 {
			add("foreign-net-flow-20", "Foreign net flow, 20 days (% of turnover)", Change, net/turn,
				fmt.Sprintf("last 20 days to %s", last.Date.Format(dateLayout)),
				fmt.Sprintf("(foreign buy - foreign sell) LKR %s / market turnover LKR %s", Num(net), Num(turn)))
		}
	} else {
		lack("market-turnover-ratio", "Market turnover vs 20-day average", Times, "21 market days")
		lack("foreign-net-flow-20", "Foreign net flow, 20 days (% of turnover)", Change, "20 market days")
	}
	if last.Listed > 0 {
		add("market-traded-share", "Share of listed securities traded", Percent, last.Traded/last.Listed, period,
			fmt.Sprintf("%.0f traded of %.0f listed", last.Traded, last.Listed))
	}
	if last.PER > 0 {
		add("market-per", "Market P/E (CSE)", Times, last.PER, period, "as reported by the CSE")
	}
	if last.DY > 0 {
		add("market-dy", "Market dividend yield (CSE, %)", Points, last.DY, period, "as reported by the CSE")
	}
	return out
}

// marketRules are the market-context findings.
func marketTrendRule(c *ctx) *Finding {
	r3, ok1 := c.val("aspi-return-3m")
	vs, ok2 := c.val("aspi-vs-sma-50")
	if !ok1 || !ok2 {
		return nil
	}
	ids := []string{"aspi-return-3m", "aspi-vs-sma-50"}
	detail := fmt.Sprintf("ASPI is %s against its 50-day average and %s over 3 months.", Format(vs, Change), Format(r3, Change))
	switch {
	case vs > 0 && r3 > 0:
		return c.find("market-trend", DimMarket, Positive, "Market trending up", detail, ids...)
	case vs < 0 && r3 < 0:
		return c.find("market-trend", DimMarket, Watch, "Market trending down", detail, ids...)
	}
	return c.find("market-trend", DimMarket, Neutral, "Market trend mixed", detail, ids...)
}

func relStrengthRule(c *ctx) *Finding {
	v, ok := c.val("rel-strength-spsl20-3m")
	if !ok {
		return nil
	}
	pts := fmt.Sprintf("%.1f percentage points", math.Abs(v)*100)
	switch {
	case v >= relStrengthPoint:
		return c.find("rel-strength", DimMarket, Positive, "Ahead of the S&P SL20",
			fmt.Sprintf("Over 3 months the stock returned %s more than the S&P SL20%s.", pts, absolute), "rel-strength-spsl20-3m")
	case v <= -relStrengthPoint:
		return c.find("rel-strength", DimMarket, Watch, "Behind the S&P SL20",
			fmt.Sprintf("Over 3 months the stock returned %s less than the S&P SL20%s.", pts, absolute), "rel-strength-spsl20-3m")
	}
	return nil
}

func foreignFlowRule(c *ctx) *Finding {
	v, ok := c.val("foreign-net-flow-20")
	if !ok {
		return nil
	}
	switch {
	case v >= flowStrong:
		return c.find("foreign-flow", DimMarket, Positive, "Foreign investors net buying",
			fmt.Sprintf("Foreign net buying was %s of market turnover over 20 days. This is market-wide, not this stock.", Format(v, Change)),
			"foreign-net-flow-20")
	case v <= -flowStrong:
		return c.find("foreign-flow", DimMarket, Watch, "Foreign investors net selling",
			fmt.Sprintf("Foreign net selling was %s of market turnover over 20 days. This is market-wide, not this stock.", Format(-v, Change)),
			"foreign-net-flow-20")
	}
	return nil
}

func staleMarketRule(c *ctx) *Finding {
	if len(c.s.market) == 0 {
		return nil
	}
	last := c.s.market[len(c.s.market)-1].Date
	days := int(c.s.AsOf.Sub(last).Hours() / 24)
	if days <= marketStaleDays {
		return nil
	}
	return &Finding{ID: "stale-market", Dimension: DimData, Severity: Watch, Title: "Market data is out of date",
		Detail: fmt.Sprintf("The newest stored market day is %s, %d days ago. Run 'hex sync market'.", last.Format(dateLayout), days)}
}
