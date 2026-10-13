package analysis

import (
	"fmt"
	"math"
)

// Shared inputs. Keys match the fields of stored fundamentals.
var (
	inPrice           = Input{"price", "price", "Share price (LKR)"}
	inShares          = Input{"shares", "shares", "Shares in issue"}
	inNetProfit       = Input{"net_profit", "net-profit", "Net profit attributable to parent (LKR)"}
	inRevenue         = Input{"revenue", "revenue", "Revenue (LKR)"}
	inGrossProfit     = Input{"gross_profit", "gross-profit", "Gross profit (LKR)"}
	inOperatingProfit = Input{"operating_profit", "operating-profit", "Operating profit (LKR)"}
	inEquityOpen      = Input{"equity_open", "equity-open", "Equity attributable to parent, start of period (LKR)"}
	inEquityClose     = Input{"equity_close", "equity-close", "Equity attributable to parent, end of period (LKR)"}
	inAssetsOpen      = Input{"assets_open", "assets-open", "Total assets, start of period (LKR)"}
	inAssetsClose     = Input{"assets_close", "assets-close", "Total assets, end of period (LKR)"}
	inTotalEquity     = Input{"total_equity", "total-equity", "Total equity (LKR)"}
	inDebt            = Input{"debt", "debt", "Interest-bearing debt (LKR)"}
	inCurAssets       = Input{"current_assets", "current-assets", "Current assets (LKR)"}
	inCurLiabilities  = Input{"current_liabilities", "current-liabilities", "Current liabilities (LKR)"}
	inEPS             = Input{"eps", "eps", "Earnings per share (LKR)"}
	inDPS             = Input{"dps", "dps", "Dividends per share, last 12 months (LKR)"}
	inNAV             = Input{"nav_per_share", "nav", "Net assets (book value) per share (LKR)"}
	inEPSGrowth       = Input{"eps_growth", "eps-growth", "EPS growth, % per year (e.g. 15 for 15%)"}
	inOCF             = Input{"ocf", "ocf", "Net cash from operating activities (LKR)"}
	inCapex           = Input{"capex", "capex", "Capital expenditure, as a positive amount (LKR)"}
	inFinanceCost     = Input{"finance_cost", "finance-cost", "Finance cost, as a positive amount (LKR)"}
	inCurrent         = Input{"current", "current", "Value this period"}
	inPrevious        = Input{"previous", "previous", "Value same period last year"}
)

// Registry lists every ratio. Each entry becomes a "hex calc" subcommand.
var Registry = []Ratio{
	// Profitability
	margin("net-margin", "Net profit margin", "net profit ÷ revenue", inNetProfit),
	margin("gross-margin", "Gross profit margin", "gross profit ÷ revenue", inGrossProfit),
	margin("operating-margin", "Operating profit margin", "operating profit ÷ revenue", inOperatingProfit),
	{
		ID: "roe", Name: "Return on equity (ROE)", Group: "profitability", Unit: Percent,
		Formula:  "net profit ÷ average equity attributable to parent",
		Inputs:   []Input{inNetProfit, inEquityClose},
		Optional: []Input{inEquityOpen},
		Compute: func(in Values) Result {
			return returnOn("ROE", "equity", in["net_profit"], in, "equity_open", "equity_close")
		},
	},
	{
		ID: "roa", Name: "Return on assets (ROA)", Group: "profitability", Unit: Percent,
		Formula:  "net profit ÷ average total assets",
		Inputs:   []Input{inNetProfit, inAssetsClose},
		Optional: []Input{inAssetsOpen},
		Compute: func(in Values) Result {
			return returnOn("ROA", "assets", in["net_profit"], in, "assets_open", "assets_close")
		},
	},

	// Valuation
	{
		ID: "market-cap", Name: "Market capitalisation", Group: "valuation", Unit: LKR,
		Formula: "share price × shares in issue",
		Inputs:  []Input{inPrice, inShares},
		Compute: func(in Values) Result {
			v := in["price"] * in["shares"]
			return ok(v, LKR, fmt.Sprintf("market cap = %s × %s = %s", Num(in["price"]), Num(in["shares"]), Num(v)))
		},
	},
	{
		ID: "eps", Name: "Earnings per share (EPS)", Group: "valuation", Unit: LKR,
		Formula: "net profit attributable to parent ÷ shares in issue",
		Inputs:  []Input{inNetProfit, inShares},
		Compute: func(in Values) Result {
			if in["shares"] <= 0 {
				return nm("shares must be positive")
			}
			v := in["net_profit"] / in["shares"]
			return ok(v, LKR, fmt.Sprintf("EPS = %s ÷ %s = %s", Num(in["net_profit"]), Num(in["shares"]), Num(v)))
		},
	},
	{
		ID: "pe", Name: "Price to earnings (P/E)", Group: "valuation", Unit: Times,
		Formula: "share price ÷ EPS",
		Inputs:  []Input{inPrice, inEPS},
		Compute: func(in Values) Result {
			if in["eps"] <= 0 {
				return nm("EPS is zero or negative, so P/E is not meaningful")
			}
			v := in["price"] / in["eps"]
			return ok(v, Times, fmt.Sprintf("P/E = %s ÷ %s = %s", Num(in["price"]), Num(in["eps"]), Format(v, Times)))
		},
	},
	{
		ID: "earnings-yield", Name: "Earnings yield", Group: "valuation", Unit: Percent,
		Formula: "EPS ÷ share price",
		Inputs:  []Input{inEPS, inPrice},
		Compute: func(in Values) Result {
			return divide("earnings yield", in["eps"], in["price"], Percent, "share price must be positive")
		},
	},
	{
		ID: "bvps", Name: "Book value (NAV) per share", Group: "valuation", Unit: LKR,
		Formula: "equity attributable to parent ÷ shares in issue",
		Inputs:  []Input{inEquityClose, inShares},
		Compute: func(in Values) Result {
			return divide("BVPS", in["equity_close"], in["shares"], LKR, "shares must be positive")
		},
	},
	{
		ID: "pb", Name: "Price to book (P/B)", Group: "valuation", Unit: Times,
		Formula: "share price ÷ book value per share",
		Inputs:  []Input{inPrice, inNAV},
		Compute: func(in Values) Result {
			if in["nav_per_share"] <= 0 {
				return nm("book value is zero or negative, so P/B is not meaningful")
			}
			return divide("P/B", in["price"], in["nav_per_share"], Times, "")
		},
	},
	{
		ID: "peg", Name: "PEG ratio", Group: "valuation", Unit: Times,
		Formula: "P/E ÷ EPS growth (%)",
		Inputs:  []Input{inPrice, inEPS, inEPSGrowth},
		Compute: func(in Values) Result {
			if in["eps"] <= 0 {
				return nm("EPS is zero or negative, so P/E is not meaningful")
			}
			if in["eps_growth"] <= 0 {
				return nm("EPS growth is zero or negative, so PEG is not meaningful")
			}
			pe := in["price"] / in["eps"]
			v := pe / in["eps_growth"]
			return ok(v, Times,
				fmt.Sprintf("P/E = %s ÷ %s = %s", Num(in["price"]), Num(in["eps"]), Num(pe)),
				fmt.Sprintf("PEG = %s ÷ %s = %s", Num(pe), Num(in["eps_growth"]), Format(v, Times)))
		},
	},
	{
		ID: "graham", Name: "Graham number", Group: "valuation", Unit: LKR,
		Formula: "√(22.5 × EPS × book value per share)",
		Inputs:  []Input{inEPS, inNAV},
		Compute: func(in Values) Result {
			if in["eps"] <= 0 || in["nav_per_share"] <= 0 {
				return nm("EPS and book value must both be positive")
			}
			v := math.Sqrt(22.5 * in["eps"] * in["nav_per_share"])
			return ok(v, LKR, fmt.Sprintf("Graham number = √(22.5 × %s × %s) = %s", Num(in["eps"]), Num(in["nav_per_share"]), Num(v)))
		},
	},

	// Income
	{
		ID: "dividend-yield", Name: "Dividend yield", Group: "income", Unit: Percent,
		Formula: "dividends per share (last 12 months) ÷ share price",
		Inputs:  []Input{inDPS, inPrice},
		Compute: func(in Values) Result {
			return divide("dividend yield", in["dps"], in["price"], Percent, "share price must be positive")
		},
	},
	{
		ID: "payout", Name: "Dividend payout ratio", Group: "income", Unit: Percent,
		Formula: "dividends per share ÷ EPS",
		Inputs:  []Input{inDPS, inEPS},
		Compute: func(in Values) Result {
			if in["eps"] <= 0 {
				return nm("EPS is zero or negative, so payout is not meaningful")
			}
			return divide("payout", in["dps"], in["eps"], Percent, "")
		},
	},

	// Financial health
	{
		ID: "debt-equity", Name: "Debt to equity", Group: "health", Unit: Times, SkipFinancials: true,
		Formula: "interest-bearing debt ÷ total equity",
		Inputs:  []Input{inDebt, inTotalEquity},
		Compute: func(in Values) Result {
			if in["total_equity"] <= 0 {
				return nm("equity is zero or negative, so debt/equity is not meaningful")
			}
			return divide("debt/equity", in["debt"], in["total_equity"], Times, "")
		},
	},
	{
		ID: "current-ratio", Name: "Current ratio", Group: "health", Unit: Times, SkipFinancials: true,
		Formula: "current assets ÷ current liabilities",
		Inputs:  []Input{inCurAssets, inCurLiabilities},
		Compute: func(in Values) Result {
			return divide("current ratio", in["current_assets"], in["current_liabilities"], Times, "current liabilities must be positive")
		},
	},

	{
		ID: "interest-cover", Name: "Interest cover", Group: "health", Unit: Times, SkipFinancials: true,
		Formula: "operating profit ÷ finance cost",
		Inputs:  []Input{inOperatingProfit, inFinanceCost},
		Compute: func(in Values) Result {
			return divide("interest cover", in["operating_profit"], in["finance_cost"], Times, "finance cost must be positive")
		},
	},

	// Cash flow
	{
		ID: "cash-conversion", Name: "Cash conversion", Group: "cashflow", Unit: Times,
		Formula: "operating cash flow ÷ net profit attributable to parent",
		Inputs:  []Input{inOCF, inNetProfit},
		Compute: func(in Values) Result {
			return divide("cash conversion", in["ocf"], in["net_profit"], Times, "net profit is zero or negative, so cash conversion is not meaningful")
		},
	},
	{
		ID: "fcf", Name: "Free cash flow", Group: "cashflow", Unit: LKR,
		Formula: "operating cash flow − capital expenditure",
		Inputs:  []Input{inOCF, inCapex},
		Compute: func(in Values) Result {
			v := in["ocf"] - in["capex"]
			return ok(v, LKR, fmt.Sprintf("free cash flow = %s − %s = %s", Num(in["ocf"]), Num(in["capex"]), Num(v)))
		},
	},

	// Growth
	{
		ID: "growth", Name: "Year-on-year growth", Group: "growth", Unit: Percent,
		Formula: "(this period − same period last year) ÷ |same period last year|",
		Inputs:  []Input{inCurrent, inPrevious},
		Compute: func(in Values) Result {
			cur, prev := in["current"], in["previous"]
			if prev <= 0 {
				if cur > 0 {
					return nm("turnaround: last year was zero or negative")
				}
				return nm("last year was zero or negative, so growth is not meaningful")
			}
			v := (cur - prev) / math.Abs(prev)
			return ok(v, Percent, fmt.Sprintf("growth = (%s − %s) ÷ %s = %s", Num(cur), Num(prev), Num(math.Abs(prev)), Format(v, Percent)))
		},
	},
}

// margin builds a "<profit> ÷ revenue" ratio.
func margin(id, name, formula string, profit Input) Ratio {
	return Ratio{
		ID: id, Name: name, Group: "profitability", Unit: Percent, SkipFinancials: true,
		Formula: formula,
		Inputs:  []Input{profit, inRevenue},
		Compute: func(in Values) Result {
			return divide(name, in[profit.Key], in["revenue"], Percent, "revenue must be positive")
		},
	}
}

// divide computes num ÷ den, returning n/m with zeroNote when den ≤ 0.
func divide(label string, num, den float64, u Unit, zeroNote string) Result {
	if den <= 0 {
		if zeroNote == "" {
			zeroNote = "divisor must be positive"
		}
		return nm(zeroNote)
	}
	v := num / den
	return ok(v, u, fmt.Sprintf("%s = %s ÷ %s = %s", label, Num(num), Num(den), Format(v, u)))
}

// returnOn computes profit ÷ average balance, using the closing balance
// alone when the opening one is missing.
func returnOn(label, what string, profit float64, in Values, openKey, closeKey string) Result {
	closing := in[closeKey]
	avg := closing
	var steps []string
	note := ""
	if in.Has(openKey) {
		avg = (in[openKey] + closing) / 2
		steps = append(steps, fmt.Sprintf("average %s = (%s + %s) ÷ 2 = %s", what, Num(in[openKey]), Num(closing), Num(avg)))
	} else {
		note = "used closing " + what + " only; pass the opening value for an average"
	}
	if avg <= 0 {
		r := nm(what+" is zero or negative, so "+label+" is not meaningful", steps...)
		return r
	}
	v := profit / avg
	r := ok(v, Percent, append(steps, fmt.Sprintf("%s = %s ÷ %s = %s", label, Num(profit), Num(avg), Format(v, Percent)))...)
	r.Note = note
	return r
}
