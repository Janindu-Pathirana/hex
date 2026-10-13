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

// ActionRows maps announcements to corporate actions. Companies are matched
// to a security code by exact name (preferring the N0000 series), because
// the announcement list carries no symbol.
func ActionRows(anns []cse.Announcement, secs []store.Security) []store.CorporateAction {
	byName := make(map[string]string)
	for _, s := range secs {
		key := normName(s.Name)
		if cur, ok := byName[key]; !ok || (s.Series == "N0000" && !strings.HasSuffix(cur, ".N0000")) {
			byName[key] = s.Code
		}
	}
	out := make([]store.CorporateAction, 0, len(anns))
	for _, a := range anns {
		raw, _ := json.Marshal(a)
		row := store.CorporateAction{
			ID: a.ID, Company: a.Company, Category: a.Category, Kind: ActionKind(a.Category),
			Announced: announcedDate(a), XD: deref(a.XD), RecordDate: deref(a.RecordDate),
			PaymentDate: deref(a.PaymentDate), Remarks: strings.TrimSpace(deref(a.Remarks)), Raw: string(raw),
		}
		if a.Symbol != nil && *a.Symbol != "" {
			row.Code = *a.Symbol
		} else {
			row.Code = byName[normName(a.Company)]
		}
		out = append(out, row)
	}
	return out
}

func announcedDate(a cse.Announcement) string {
	if t, err := time.ParseInLocation("02 Jan 2006", strings.TrimSpace(a.Date), output.Colombo); err == nil {
		return t.Format("2006-01-02")
	}
	return output.Date(a.CreatedDate)
}

func normName(s string) string { return strings.Join(strings.Fields(strings.ToUpper(s)), " ") }

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
