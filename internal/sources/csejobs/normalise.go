// Package csejobs holds the scrape-engine jobs that pull JSON data from cse.lk
// into the store, and the pure normalisers that map API structs to rows.
package csejobs

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/Janindu-Pathirana/hex/internal/cse"
	"github.com/Janindu-Pathirana/hex/internal/output"
	"github.com/Janindu-Pathirana/hex/internal/store"
)

// splitCode splits "JKH.N0000" into ticker and series.
func splitCode(code string) (ticker, series string) {
	ticker, series, _ = strings.Cut(code, ".")
	return ticker, series
}

// SecurityRows maps the symbol list to store rows.
func SecurityRows(secs []cse.Security) []store.Security {
	out := make([]store.Security, len(secs))
	for i, s := range secs {
		out[i] = store.Security{Code: s.Code, Ticker: s.Ticker(), Series: s.Series(), Name: s.Name}
	}
	return out
}

// QuoteRows maps the trade summary to quotes and security details (stockId).
func QuoteRows(rows []cse.TradeSummary) ([]store.Quote, []store.Security) {
	quotes := make([]store.Quote, 0, len(rows))
	secs := make([]store.Security, 0, len(rows))
	for _, r := range rows {
		if r.Symbol == "" {
			continue
		}
		q := store.Quote{
			Code: r.Symbol, Price: r.Price, Change: r.Change, ChangePct: r.PercentageChange,
			Open: r.Open, High: r.High, Low: r.Low, PrevClose: r.PreviousClose,
			Volume: r.ShareVolume, Trades: r.TradeVolume, Turnover: r.Turnover, MarketCap: r.MarketCap,
		}
		if r.LastTradedTime > 0 {
			q.TradedAt = time.UnixMilli(r.LastTradedTime)
		}
		quotes = append(quotes, q)
		t, s := splitCode(r.Symbol)
		secs = append(secs, store.Security{Code: r.Symbol, Ticker: t, Series: s, Name: r.Name, StockID: r.StockID})
	}
	return quotes, secs
}

// InfoRows maps companyInfoSummery to a security detail row and ranges/beta.
func InfoRows(ci *cse.CompanyInfo) (store.Security, store.QuoteStats) {
	in := ci.Info
	t, s := splitCode(in.Symbol)
	sec := store.Security{
		Code: in.Symbol, Ticker: t, Series: s, Name: in.Name, StockID: in.StockID,
		ISIN: in.ISIN, Issued: in.QuantityIssued, ParValue: in.ParValue,
	}
	st := store.QuoteStats{
		Code: in.Symbol, High52W: in.High52W, Low52W: in.Low52W, YTDHigh: in.YTDHigh, YTDLow: in.YTDLow,
		AllHigh: in.AllTimeHigh, AllLow: in.AllTimeLow, Volume52W: in.ShareVolume52W, ForeignPct: in.ForeignPercentage,
	}
	if ci.Beta != nil {
		a, sp := ci.Beta.ASPI, ci.Beta.SPSL20
		st.BetaASPI, st.BetaSPSL20, st.BetaPeriod = &a, &sp, ci.Beta.Period
		if ci.Beta.Quarter > 0 {
			st.BetaPeriod += "Q" + strconv.Itoa(ci.Beta.Quarter)
		}
	}
	return sec, st
}

// BarRows maps chart points to daily bars keyed by Colombo date. When two
// points fall on one date, the later wins.
func BarRows(bars []cse.Bar) []store.Bar {
	byDate := make(map[string]int)
	var out []store.Bar
	for _, b := range bars {
		d := output.Date(b.T)
		row := store.Bar{Date: d, Open: b.Open, High: b.High, Low: b.Low, Close: b.Close, Volume: b.Volume}
		if i, ok := byDate[d]; ok {
			out[i] = row
			continue
		}
		byDate[d] = len(out)
		out = append(out, row)
	}
	return out
}

// IndexRow maps an index snapshot (ASPI, S&P SL20) to a daily index bar.
func IndexRow(id, name string, v *cse.IndexValue) store.IndexBar {
	return store.IndexBar{IndexID: id, Date: output.Date(v.Timestamp), Name: name, Value: v.Value, Change: v.Change, ChangePct: v.Percentage}
}

// IndexChartRows maps index chart points to daily index bars. The day's change
// is the move from the previous point; one point per Colombo date, the later
// one winning.
func IndexChartRows(id, name string, pts []cse.IndexPoint) []store.IndexBar {
	byDate := map[string]store.IndexBar{}
	var order []string
	prev := 0.0
	for i, p := range pts {
		d := output.Date(p.D)
		b := store.IndexBar{IndexID: id, Date: d, Name: name, Value: p.Value, ChangePct: p.Percent}
		if i > 0 {
			b.Change = p.Value - prev
		}
		prev = p.Value
		if _, seen := byDate[d]; !seen {
			order = append(order, d)
		}
		byDate[d] = b
	}
	out := make([]store.IndexBar, 0, len(order))
	for _, d := range order {
		out = append(out, byDate[d])
	}
	return out
}

// SectorRows maps sector indices to daily index bars keyed by sector symbol.
func SectorRows(secs []cse.Sector) []store.IndexBar {
	out := make([]store.IndexBar, 0, len(secs))
	for _, s := range secs {
		out = append(out, store.IndexBar{
			IndexID: s.Symbol, Date: output.Date(s.Time), Name: s.IndexName, Value: s.Value,
			Change: s.Change, ChangePct: s.Percentage, Turnover: s.Turnover, Volume: s.Volume, Trades: s.Trades,
		})
	}
	return out
}

// MarketDayRows maps dailyMarketSummery to market days.
func MarketDayRows(days []cse.DailyMarket) []store.MarketDay {
	out := make([]store.MarketDay, 0, len(days))
	for _, d := range days {
		out = append(out, store.MarketDay{
			Date: output.Date(d.TradeDate), Turnover: d.MarketTurnover, Trades: d.Trades, ShareVolume: d.ShareVolume,
			MarketCap: d.MarketCap, PER: d.PER, PBV: d.PBV, DY: d.DY, ASI: d.ASI, SPSL20: d.SPSL20,
			DomesticBuy: d.DomesticPurchase, DomesticSell: d.DomesticSales, ForeignBuy: d.ForeignPurchase,
			ForeignSell: d.ForeignSales, Listed: d.ListedCompanies, Traded: d.TradedCompanies,
		})
	}
	return out
}

// ProfileRows maps companyProfile to a profile and its directors.
func ProfileRows(code string, p *cse.CompanyProfile) (store.Profile, []store.Director) {
	row := store.Profile{Code: code}
	if len(p.Summary) > 0 {
		s := p.Summary[0]
		row.Name, row.Sector, row.Board, row.FYEnd = s.Name, s.Sector, s.BoardType, s.FinYearEnd
		row.Auditors, row.Secretaries, row.Established = s.Auditors, s.Secretaries, s.Established
		row.Address, row.Web, row.Email = firstNonEmpty(s.RegisteredOffice, s.RegisterOffice), s.Web, s.Email1
	}
	var summary []string
	for _, b := range p.BusinessSummary {
		if body := strings.TrimSpace(b.Body); body != "" {
			summary = append(summary, body)
		}
	}
	row.BusinessSummary = strings.Join(summary, "\n\n")

	var ds []store.Director
	for _, d := range p.Directors {
		name, role := splitRole(strings.TrimSpace(d.FirstName + " " + d.LastName))
		ds = append(ds, store.Director{Name: name, Role: role})
	}
	return row, ds
}

// splitRole separates "MR. K N J BALENDRA (Chairman)" into name and role.
func splitRole(s string) (name, role string) {
	open := strings.LastIndex(s, "(")
	if open < 0 || !strings.HasSuffix(s, ")") {
		return s, ""
	}
	return strings.TrimSpace(s[:open]), strings.TrimSpace(s[open+1 : len(s)-1])
}

// ReportRows maps the financials response to report catalogue rows.
func ReportRows(code string, f *cse.Financials) []store.Report {
	var out []store.Report
	add := func(kind string, rs []cse.Report) {
		for _, r := range rs {
			if r.Path == "" {
				continue
			}
			row := store.Report{ID: r.ID, Code: code, Kind: kind, Title: strings.TrimSpace(r.FileText), Path: r.Path}
			if r.ManualDate > 0 {
				row.PeriodDate = output.Date(r.ManualDate)
			}
			if r.UploadedDate > 0 {
				row.UploadedAt = time.UnixMilli(r.UploadedDate)
			}
			if r.AuthorizedDate != nil {
				row.AuthorizedAt = time.UnixMilli(*r.AuthorizedDate)
			}
			out = append(out, row)
		}
	}
	add(store.ReportAnnual, f.Annual)
	add(store.ReportQuarterly, f.Quarterly)
	add(store.ReportOther, f.Other)
	return out
}

// ActionKind classifies an announcement category.
func ActionKind(category string) string {
	c := strings.ToUpper(category)
	switch {
	case strings.Contains(c, "SCRIP"):
		return store.ActionScrip
	case strings.Contains(c, "DIVIDEND"):
		return store.ActionDividend
	case strings.Contains(c, "RIGHTS"):
		return store.ActionRights
	case strings.Contains(c, "SUB-DIVISION"), strings.Contains(c, "SUBDIVISION"), strings.Contains(c, "SPLIT"):
		return store.ActionSplit
	case strings.Contains(c, "BONUS"), strings.Contains(c, "CAPITALIZATION"), strings.Contains(c, "CAPITALISATION"):
		return store.ActionBonus
	case strings.Contains(c, "GENERAL MEETING"):
		return store.ActionAGM
	}
	return store.ActionOther
}

// smdDate converts the DD-MM-YYYY dates of the announcement search to
// YYYY-MM-DD. Anything else, including empty, gives "".
func smdDate(s string) string {
	t, err := time.ParseInLocation("02-01-2006", strings.TrimSpace(s), output.Colombo)
	if err != nil {
		return ""
	}
	return t.Format("2006-01-02")
}

// ActionRows maps announcement-search rows to corporate actions. Rows are
// matched to a security code by ticker, preferring the voting N0000 series.
// Ratios are parsed from the published text; when the text has none the ratio
// stays empty so that a person can enter it.
func ActionRows(rows []cse.SMDAnnouncement, secs []store.Security) []store.CorporateAction {
	byTicker := make(map[string]string)
	for _, s := range secs {
		t := strings.ToUpper(s.Ticker)
		if cur, ok := byTicker[t]; !ok || (s.Series == "N0000" && !strings.HasSuffix(cur, ".N0000")) {
			byTicker[t] = s.Code
		}
	}
	out := make([]store.CorporateAction, 0, len(rows))
	for _, r := range rows {
		raw, _ := json.Marshal(r)
		a := store.CorporateAction{
			ID: r.ID, Code: byTicker[strings.ToUpper(r.Symbol)], Ticker: strings.ToUpper(r.Symbol), SecID: r.SecID,
			Company: r.Company, Category: r.Category, Kind: ActionKind(r.Category), Announced: smdDate(r.Date),
			Remarks: strings.TrimSpace(r.Remarks), Raw: string(raw),
			XD: smdDate(r.XD), XR: smdDate(r.XR), XC: smdDate(r.XC), RecordDate: smdDate(r.RecordDate),
			PaymentDate: smdDate(r.Payment), TradingCommencement: smdDate(r.TradingCommence),
			TradingSuspended: smdDate(r.TradingSuspended), AllotmentDate: smdDate(firstNonEmpty(r.Allotment, r.DateAllotment)),
			EntitlementDate: smdDate(r.EntitlementAsAt), AGMDate: smdDate(r.AGM), ChildID: r.ChildAnnouncement,
		}
		if v, ok := r.VotingDivPerShare.Float(); ok {
			a.DPS = &v
		}
		var proportion cse.Text
		switch a.Kind {
		case store.ActionDividend:
			a.ExDate = a.XD
		case store.ActionScrip:
			a.ExDate, proportion = a.XD, r.VotingProportion
			if v, ok := r.VotingConsider.Float(); ok {
				a.Price = &v
			}
		case store.ActionBonus:
			a.ExDate, proportion = a.XC, r.VotingProportion
		case store.ActionRights:
			a.ExDate, proportion = a.XR, r.RightsProportion
			if v, ok := r.RightsPrice.Float(); ok {
				a.Price = &v
			}
		case store.ActionSplit:
			a.ExDate = a.TradingCommencement
			proportion = cse.Text(r.Remarks)
		}
		a.Proportion = string(proportion)
		if num, den, ok := ParseRatio(a.Kind, a.Proportion); ok {
			a.RatioNum, a.RatioDen = &num, &den
		}
		out = append(out, a)
	}
	return out
}

func normName(s string) string { return strings.Join(strings.Fields(strings.ToUpper(s)), " ") }

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
