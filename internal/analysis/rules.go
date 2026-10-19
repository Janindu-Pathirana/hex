package analysis

import (
	"fmt"
	"math"
	"strings"
)

// Severity says how a finding should read: a strength, plain information, a
// point to check, or a risk.
type Severity string

const (
	Positive Severity = "positive"
	Neutral  Severity = "neutral"
	Watch    Severity = "watch"
	Risk     Severity = "risk"
)

// Dimensions group findings in the report.
const (
	DimValuation     = "valuation"
	DimProfitability = "profitability"
	DimGrowth        = "growth"
	DimHealth        = "financial health"
	DimCash          = "cash flow"
	DimDividends     = "dividends"
	DimTechnical     = "technical"
	DimData          = "data quality"
)

// Thresholds. They are absolute until sector and market benchmarks arrive,
// so every finding that uses one says so in its detail.
const (
	peLow         = 8.0
	peHigh        = 20.0
	pbHigh        = 3.0
	roeStrong     = 0.15
	roeWeak       = 0.08
	growthStrong  = 0.10
	growthSevere  = -0.20
	currentWeak   = 1.0
	currentStrong = 1.5
	coverRisk     = 1.5
	coverWatch    = 3.0
	cashWeak      = 0.5
	payoutHigh    = 1.0
	staleDays     = 200
	lowConfidence = 0.7 // matches reportjobs.LowConfidence

	rsiHigh      = 70.0
	rsiLow       = 30.0
	nearRange    = 0.05  // within 5% of the 52-week high or low
	volHigh      = 0.50  // annualised volatility
	drawdownDeep = -0.30 // fall from the peak
)

// Evidence is one figure a finding rests on.
type Evidence struct {
	Metric  string `json:"metric"`
	Display string `json:"display"`
	Period  string `json:"period,omitempty"`
}

// Finding is one rule firing on the numbers.
type Finding struct {
	ID        string     `json:"id"`
	Dimension string     `json:"dimension"`
	Severity  Severity   `json:"severity"`
	Title     string     `json:"title"`
	Detail    string     `json:"detail"`
	Evidence  []Evidence `json:"evidence"`
}

type rule struct {
	id   string
	dim  string
	eval func(c *ctx) *Finding
}

// ctx gives rules the metrics and snapshot, and builds findings.
type ctx struct {
	m Metrics
	s *Snapshot
}

func (c *ctx) val(id string) (float64, bool) { return c.m.Val(id) }

func (c *ctx) ev(ids ...string) []Evidence {
	var out []Evidence
	for _, id := range ids {
		if m, ok := c.m.Get(id); ok && m.Value != nil {
			out = append(out, Evidence{Metric: m.Name, Display: m.Display, Period: m.Period})
		}
	}
	return out
}

func (c *ctx) find(id, dim string, sev Severity, title, detail string, ids ...string) *Finding {
	return &Finding{ID: id, Dimension: dim, Severity: sev, Title: title, Detail: detail, Evidence: c.ev(ids...)}
}

const absolute = " (absolute threshold; no sector benchmark yet)"

var rules = []rule{
	{"pe", DimValuation, func(c *ctx) *Finding {
		if m, ok := c.m.Get("pe"); ok && m.Value == nil && m.Note != "" && !m.Skipped && len(m.Missing) == 0 {
			return c.find("pe", DimValuation, Watch, "Loss-making over the last twelve months",
				"EPS is zero or negative, so P/E is not meaningful.", "eps")
		}
		pe, ok := c.val("pe")
		switch {
		case !ok:
			return nil
		case pe < peLow:
			return c.find("pe", DimValuation, Positive, "Low earnings multiple",
				fmt.Sprintf("P/E of %.1fx is below %.0fx%s.", pe, peLow, absolute), "pe", "eps")
		case pe > peHigh:
			return c.find("pe", DimValuation, Watch, "High earnings multiple",
				fmt.Sprintf("P/E of %.1fx is above %.0fx%s.", pe, peHigh, absolute), "pe", "eps")
		}
		return c.find("pe", DimValuation, Neutral, "Earnings multiple in a middle range",
			fmt.Sprintf("P/E of %.1fx is between %.0fx and %.0fx%s.", pe, peLow, peHigh, absolute), "pe", "eps")
	}},
	{"pb", DimValuation, func(c *ctx) *Finding {
		pb, ok := c.val("pb")
		if !ok {
			return nil
		}
		roe, hasROE := c.val("roe")
		switch {
		case pb < 1 && hasROE && roe >= roeStrong:
			return c.find("pb", DimValuation, Positive, "Below book value with strong returns",
				fmt.Sprintf("P/B of %.2fx while ROE is %.1f%%.", pb, roe*100), "pb", "roe")
		case pb < 1:
			return c.find("pb", DimValuation, Neutral, "Trades below book value",
				fmt.Sprintf("P/B of %.2fx; check whether returns justify the discount.", pb), "pb", "roe")
		case pb > pbHigh:
			return c.find("pb", DimValuation, Watch, "High price to book",
				fmt.Sprintf("P/B of %.2fx is above %.0fx%s.", pb, pbHigh, absolute), "pb", "roe")
		}
		return nil
	}},
	{"graham", DimValuation, func(c *ctx) *Finding {
		g, ok := c.val("graham")
		if !ok || c.s.Price <= 0 {
			return nil
		}
		if c.s.Price > g {
			return c.find("graham", DimValuation, Watch, "Price above the Graham number",
				fmt.Sprintf("Price %s is above the Graham number %s.", Num(c.s.Price), Num(g)), "graham", "eps", "bvps")
		}
		return c.find("graham", DimValuation, Positive, "Price below the Graham number",
			fmt.Sprintf("Price %s is below the Graham number %s.", Num(c.s.Price), Num(g)), "graham", "eps", "bvps")
	}},
	{"roe", DimProfitability, func(c *ctx) *Finding {
		roe, ok := c.val("roe")
		switch {
		case !ok:
			return nil
		case roe < 0:
			return c.find("roe", DimProfitability, Risk, "Negative return on equity",
				fmt.Sprintf("ROE is %.1f%%.", roe*100), "roe")
		case roe >= roeStrong:
			return c.find("roe", DimProfitability, Positive, "Strong return on equity",
				fmt.Sprintf("ROE of %.1f%% is at or above %.0f%%%s.", roe*100, roeStrong*100, absolute), "roe")
		case roe < roeWeak:
			return c.find("roe", DimProfitability, Watch, "Weak return on equity",
				fmt.Sprintf("ROE of %.1f%% is below %.0f%%%s.", roe*100, roeWeak*100, absolute), "roe")
		}
		return c.find("roe", DimProfitability, Neutral, "Moderate return on equity",
			fmt.Sprintf("ROE of %.1f%%.", roe*100), "roe")
	}},
	{"margins", DimProfitability, func(c *ctx) *Finding {
		pg, ok1 := c.val("profit-growth")
		rg, ok2 := c.val("revenue-growth")
		nm, ok3 := c.val("net-margin")
		if !ok1 || !ok2 || !ok3 {
			return nil
		}
		switch {
		case pg > rg && rg > 0:
			return c.find("margins", DimProfitability, Positive, "Margins widening",
				fmt.Sprintf("Profit grew %.1f%% against revenue growth of %.1f%%.", pg*100, rg*100), "net-margin", "profit-growth", "revenue-growth")
		case pg < rg && pg < 0:
			return c.find("margins", DimProfitability, Watch, "Margins narrowing",
				fmt.Sprintf("Profit fell %.1f%% while revenue moved %.1f%% (net margin %.1f%%).", -pg*100, rg*100, nm*100), "net-margin", "profit-growth", "revenue-growth")
		}
		return nil
	}},
	{"revenue-growth", DimGrowth, func(c *ctx) *Finding { return growthFinding(c, "revenue-growth", "Revenue") }},
	{"profit-growth", DimGrowth, func(c *ctx) *Finding { return growthFinding(c, "profit-growth", "Profit") }},
	{"current-ratio", DimHealth, func(c *ctx) *Finding {
		cr, ok := c.val("current-ratio")
		switch {
		case !ok:
			return nil
		case cr < currentWeak:
			return c.find("current-ratio", DimHealth, Watch, "Current liabilities exceed current assets",
				fmt.Sprintf("Current ratio of %.2fx is below %.1fx.", cr, currentWeak), "current-ratio")
		case cr >= currentStrong:
			return c.find("current-ratio", DimHealth, Positive, "Comfortable short-term liquidity",
				fmt.Sprintf("Current ratio of %.2fx is at or above %.1fx.", cr, currentStrong), "current-ratio")
		}
		return nil
	}},
	{"interest-cover", DimHealth, func(c *ctx) *Finding {
		ic, ok := c.val("interest-cover")
		switch {
		case !ok:
			return nil
		case ic < coverRisk:
			return c.find("interest-cover", DimHealth, Risk, "Thin interest cover",
				fmt.Sprintf("Operating profit covers finance cost %.2fx.", ic), "interest-cover")
		case ic < coverWatch:
			return c.find("interest-cover", DimHealth, Watch, "Modest interest cover",
				fmt.Sprintf("Operating profit covers finance cost %.2fx.", ic), "interest-cover")
		}
		return c.find("interest-cover", DimHealth, Positive, "Healthy interest cover",
			fmt.Sprintf("Operating profit covers finance cost %.2fx.", ic), "interest-cover")
	}},
	{"cash-conversion", DimCash, func(c *ctx) *Finding {
		cc, ok := c.val("cash-conversion")
		if !ok {
			if m, found := c.m.Get("cash-conversion"); found && m.Value == nil && len(m.Missing) == 0 && !m.Skipped {
				return c.find("cash-conversion", DimCash, Watch, "Profit is not backed by a positive base",
					"Net profit is zero or negative, so cash conversion is not meaningful.", "ocf")
			}
			return nil
		}
		switch {
		case cc < 0:
			return c.find("cash-conversion", DimCash, Risk, "Profitable on paper, cash outflow from operations",
				fmt.Sprintf("Operating cash flow is %.2fx net profit.", cc), "cash-conversion", "ocf")
		case cc < cashWeak:
			return c.find("cash-conversion", DimCash, Watch, "Weak cash conversion",
				fmt.Sprintf("Operating cash flow is only %.2fx net profit.", cc), "cash-conversion")
		case cc >= 1:
			return c.find("cash-conversion", DimCash, Positive, "Profit is backed by cash",
				fmt.Sprintf("Operating cash flow is %.2fx net profit.", cc), "cash-conversion")
		}
		return nil
	}},
	{"fcf", DimCash, func(c *ctx) *Finding {
		fcf, ok := c.val("fcf")
		if !ok {
			return nil
		}
		if fcf < 0 {
			return c.find("fcf", DimCash, Watch, "Negative free cash flow",
				"Capital expenditure exceeds operating cash flow; check how it is funded.", "fcf", "ocf")
		}
		return c.find("fcf", DimCash, Positive, "Positive free cash flow",
			"Operating cash flow covers capital expenditure.", "fcf", "ocf")
	}},
	{"payout", DimDividends, func(c *ctx) *Finding {
		po, ok := c.val("payout")
		if !ok {
			return nil
		}
		if po > payoutHigh {
			return c.find("payout", DimDividends, Risk, "Dividend exceeds earnings",
				fmt.Sprintf("Payout of %.0f%% of EPS is not covered by profit.", po*100), "payout", "dividend-yield")
		}
		return c.find("payout", DimDividends, Neutral, "Dividend covered by earnings",
			fmt.Sprintf("Payout of %.0f%% of EPS.", po*100), "payout", "dividend-yield")
	}},
	{"rsi", DimTechnical, func(c *ctx) *Finding {
		v, ok := c.val("rsi-14")
		switch {
		case !ok:
			return nil
		case v > rsiHigh:
			return c.find("rsi", DimTechnical, Watch, "RSI in the overbought zone",
				fmt.Sprintf("RSI of %.0f is above %.0f; the price has risen fast over recent days.", v, rsiHigh), "rsi-14")
		case v < rsiLow:
			return c.find("rsi", DimTechnical, Watch, "RSI in the oversold zone",
				fmt.Sprintf("RSI of %.0f is below %.0f; the price has fallen fast over recent days.", v, rsiLow), "rsi-14")
		}
		return c.find("rsi", DimTechnical, Neutral, "RSI in a middle range",
			fmt.Sprintf("RSI of %.0f is between %.0f and %.0f.", v, rsiLow, rsiHigh), "rsi-14")
	}},
	{"trend", DimTechnical, func(c *ctx) *Finding {
		v50, ok50 := c.val("price-vs-sma-50")
		v200, ok200 := c.val("price-vs-sma-200")
		switch {
		case ok50 && ok200:
			s50, _ := c.val("sma-50")
			s200, _ := c.val("sma-200")
			cross := "SMA 50 is below SMA 200"
			if s50 > s200 {
				cross = "SMA 50 is above SMA 200"
			}
			sev := Neutral
			if v50 > 0 && v200 > 0 {
				sev = Positive
			} else if v50 < 0 && v200 < 0 {
				sev = Watch
			}
			return c.find("trend", DimTechnical, sev, "Price against its moving averages",
				fmt.Sprintf("Price is %s against SMA 50 and %s against SMA 200; %s.", Format(v50, Change), Format(v200, Change), cross),
				"price-vs-sma-50", "price-vs-sma-200")
		case ok50:
			return c.find("trend", DimTechnical, Neutral, "Price against its 50-day average",
				fmt.Sprintf("Price is %s against SMA 50 (not enough history for SMA 200).", Format(v50, Change)), "price-vs-sma-50")
		}
		return nil
	}},
	{"macd", DimTechnical, func(c *ctx) *Finding {
		h, ok := c.val("macd-hist")
		if !ok {
			return nil
		}
		if math.Abs(h) < 1e-9 {
			return c.find("macd", DimTechnical, Neutral, "MACD level with its signal line",
				"Short-term momentum matches the 9-day average of itself.", "macd", "macd-signal", "macd-hist")
		}
		if h > 0 {
			return c.find("macd", DimTechnical, Positive, "MACD above its signal line",
				"Short-term momentum is stronger than the 9-day average of itself.", "macd", "macd-signal", "macd-hist")
		}
		return c.find("macd", DimTechnical, Watch, "MACD below its signal line",
			"Short-term momentum is weaker than the 9-day average of itself.", "macd", "macd-signal", "macd-hist")
	}},
	{"range", DimTechnical, func(c *ctx) *Finding {
		fh, okh := c.val("from-high-52w")
		fl, okl := c.val("from-low-52w")
		switch {
		case okh && fh >= -nearRange:
			return c.find("range", DimTechnical, Neutral, "Near the 52-week high",
				fmt.Sprintf("Close is within %.0f%% of the highest close in a year.", nearRange*100), "from-high-52w")
		case okl && fl <= nearRange:
			return c.find("range", DimTechnical, Watch, "Near the 52-week low",
				fmt.Sprintf("Close is within %.0f%% of the lowest close in a year.", nearRange*100), "from-low-52w")
		}
		return nil
	}},
	{"volatility", DimTechnical, func(c *ctx) *Finding {
		v, ok := c.val("volatility-1y")
		if !ok || v < volHigh {
			return nil
		}
		return c.find("volatility", DimTechnical, Watch, "High volatility",
			fmt.Sprintf("Annualised volatility of %.0f%% is above %.0f%%.", v*100, volHigh*100), "volatility-1y")
	}},
	{"drawdown", DimTechnical, func(c *ctx) *Finding {
		v, ok := c.val("max-drawdown")
		if !ok || v > drawdownDeep {
			return nil
		}
		return c.find("drawdown", DimTechnical, Watch, "Deep fall from a past peak",
			fmt.Sprintf("The price fell %.0f%% from its peak within the stored history. Stored prices are not adjusted for splits, bonus issues or rights, so check the corporate actions.", -v*100), "max-drawdown")
	}},
	{"thin-trading", DimData, func(c *ctx) *Finding {
		n, ok := c.val("active-days-14")
		if !ok || n >= activeMin {
			return nil
		}
		return &Finding{ID: "thin-trading", Dimension: DimData, Severity: Watch, Title: "Thinly traded",
			Detail:   fmt.Sprintf("The close changed on only %.0f of the last %d days, so indicators may rest on stale prices.", n, activeWindow),
			Evidence: c.ev("active-days-14")}
	}},
	{"stale-prices", DimData, func(c *ctx) *Finding {
		if len(c.s.bars) == 0 {
			return nil
		}
		last := c.s.bars[len(c.s.bars)-1].Date
		days := int(c.s.AsOf.Sub(last).Hours() / 24)
		if days <= staleBarsDays {
			return nil
		}
		return &Finding{ID: "stale-prices", Dimension: DimData, Severity: Watch, Title: "Price history is out of date",
			Detail: fmt.Sprintf("The newest stored bar is %s, %d days ago. Run 'hex sync history'.", last.Format(dateLayout), days)}
	}},
	{"low-confidence", DimData, func(c *ctx) *Finding {
		var low []string
		for _, m := range c.m.List {
			if m.Value != nil && m.Confidence > 0 && m.Confidence < lowConfidence {
				low = append(low, fmt.Sprintf("%s (%.2f)", m.Name, m.Confidence))
			}
		}
		if len(low) == 0 {
			return nil
		}
		return &Finding{ID: "low-confidence", Dimension: DimData, Severity: Watch,
			Title:  "Some inputs were read with low confidence",
			Detail: "Check these against the report PDF: " + strings.Join(low, ", ") + ".", Evidence: nil}
	}},
	{"stale", DimData, func(c *ctx) *Finding {
		newest, ok := c.s.Newest()
		if !ok {
			return nil
		}
		days := int(c.s.AsOf.Sub(newest).Hours() / 24)
		if days <= staleDays {
			return nil
		}
		return &Finding{ID: "stale", Dimension: DimData, Severity: Watch, Title: "Financial statements are out of date",
			Detail: fmt.Sprintf("The newest stored period ended %s, %d days ago. Run 'hex sync fundamentals'.", newest.Format(dateLayout), days)}
	}},
	{"no-price", DimData, func(c *ctx) *Finding {
		if c.s.Price > 0 {
			return nil
		}
		return &Finding{ID: "no-price", Dimension: DimData, Severity: Watch, Title: "No stored price",
			Detail: "Valuation and yield metrics need a quote. Run 'hex sync'."}
	}},
}

func growthFinding(c *ctx, id, label string) *Finding {
	g, ok := c.val(id)
	if !ok {
		if m, found := c.m.Get(id); found && m.Value == nil && len(m.Missing) == 0 && m.Note != "" {
			return c.find(id, DimGrowth, Neutral, label+" growth not meaningful", m.Note)
		}
		return nil
	}
	switch {
	case g >= growthStrong:
		return c.find(id, DimGrowth, Positive, label+" growing",
			fmt.Sprintf("%s grew %.1f%% year on year.", label, g*100), id)
	case g <= growthSevere:
		return c.find(id, DimGrowth, Risk, label+" falling sharply",
			fmt.Sprintf("%s fell %.1f%% year on year.", label, -g*100), id)
	case g < 0:
		return c.find(id, DimGrowth, Watch, label+" falling",
			fmt.Sprintf("%s fell %.1f%% year on year.", label, -g*100), id)
	}
	return c.find(id, DimGrowth, Neutral, label+" broadly flat",
		fmt.Sprintf("%s changed %.1f%% year on year.", label, g*100), id)
}

// Evaluate runs every rule and returns the findings in rule order.
func Evaluate(s *Snapshot, m Metrics) []Finding {
	c := &ctx{m: m, s: s}
	var out []Finding
	for _, r := range rules {
		if f := r.eval(c); f != nil {
			out = append(out, *f)
		}
	}
	return out
}
