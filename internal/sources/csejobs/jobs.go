package csejobs

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Janindu-Pathirana/hex/internal/cse"
	"github.com/Janindu-Pathirana/hex/internal/engine"
	"github.com/Janindu-Pathirana/hex/internal/output"
	"github.com/Janindu-Pathirana/hex/internal/store"
)

// Job kinds, as shown by "hex sync status".
const (
	KindSecurities = "securities"
	KindMarket     = "market"
	KindIndex      = "index"
	KindQuotes     = "quotes"
	KindInfo       = "info"
	KindHistory    = "history"
	KindProfile    = "profile"
	KindReports    = "reports"
	KindActions    = "actions"
)

// ErrNoStockID is returned by a history job when the security's numeric
// stockId is unknown (the info or quotes job stores it).
var ErrNoStockID = errors.New("stockId unknown; sync info first")

// Securities refreshes the symbol list.
func Securities() engine.Job {
	return engine.FuncJob{K: KindSecurities, Kd: KindSecurities, T: 24 * time.Hour, F: func(ctx context.Context, env *engine.Env) error {
		secs, err := env.CSE.Securities(ctx)
		if err != nil {
			return fmt.Errorf("fetch securities: %w", err)
		}
		return env.Store.Update(ctx, func(tx *store.Tx) error { return tx.UpsertSecurities(ctx, SecurityRows(secs)) })
	}}
}

// Quotes stores today's trade summary for every traded security.
func Quotes(ttl time.Duration) engine.Job {
	return engine.FuncJob{K: KindQuotes, Kd: KindQuotes, T: ttl, F: func(ctx context.Context, env *engine.Env) error {
		rows, err := env.CSE.TradeSummary(ctx)
		if err != nil {
			return fmt.Errorf("fetch trade summary: %w", err)
		}
		quotes, secs := QuoteRows(rows)
		return env.Store.Update(ctx, func(tx *store.Tx) error {
			for _, s := range secs {
				if err := tx.SetSecurityDetail(ctx, s); err != nil {
					return err
				}
			}
			return tx.UpsertQuotes(ctx, quotes)
		})
	}}
}

// Market stores market status, today's summary, ASPI, S&P SL20, sector
// indices and the daily market statistics.
func Market(ttl time.Duration) engine.Job {
	return engine.FuncJob{K: KindMarket, Kd: KindMarket, T: ttl, F: func(ctx context.Context, env *engine.Env) error {
		c := env.CSE
		status, err := c.MarketStatus(ctx)
		if err != nil {
			return fmt.Errorf("fetch market status: %w", err)
		}
		aspi, err := c.ASPI(ctx)
		if err != nil {
			return fmt.Errorf("fetch aspi: %w", err)
		}
		snp, err := c.SNP(ctx)
		if err != nil {
			return fmt.Errorf("fetch s&p sl20: %w", err)
		}
		sectors, err := c.Sectors(ctx)
		if err != nil {
			return fmt.Errorf("fetch sectors: %w", err)
		}
		days, err := c.DailyMarketSummary(ctx)
		if err != nil {
			return fmt.Errorf("fetch daily market summary: %w", err)
		}
		bars := append([]store.IndexBar{IndexRow("ASPI", "All Share Price Index", aspi), IndexRow("SPSL20", "S&P Sri Lanka 20", snp)}, SectorRows(sectors)...)
		return env.Store.Update(ctx, func(tx *store.Tx) error {
			if err := tx.SetMeta(ctx, "market_status", status); err != nil {
				return err
			}
			if err := tx.UpsertIndexBars(ctx, bars); err != nil {
				return err
			}
			return tx.UpsertMarketDays(ctx, MarketDayRows(days))
		})
	}}
}

// IndexHistory stores a year of daily ASPI and S&P SL20 levels. The market job
// only records today's value, so without this the index series stays empty
// and every comparison with the market is unavailable. History beyond a year
// builds up as the job keeps running.
func IndexHistory() engine.Job {
	return engine.FuncJob{K: KindIndex, Kd: KindIndex, T: 24 * time.Hour, F: func(ctx context.Context, env *engine.Env) error {
		var bars []store.IndexBar
		for _, ix := range []struct {
			chart    int
			id, name string
		}{{cse.ChartASPI, "ASPI", "All Share Price Index"}, {cse.ChartSPSL20, "SPSL20", "S&P Sri Lanka 20"}} {
			pts, err := env.CSE.IndexChart(ctx, ix.chart, cse.PeriodYear)
			if err != nil {
				return fmt.Errorf("fetch %s history: %w", ix.id, err)
			}
			bars = append(bars, IndexChartRows(ix.id, ix.name, pts)...)
		}
		return env.Store.Update(ctx, func(tx *store.Tx) error { return tx.UpsertIndexBars(ctx, bars) })
	}}
}

// Info stores a security's identifiers, 52-week and other ranges, and beta.
func Info(code string) engine.Job {
	return engine.FuncJob{K: KindInfo + ":" + code, Kd: KindInfo, T: 24 * time.Hour, F: func(ctx context.Context, env *engine.Env) error {
		ci, err := env.CSE.CompanyInfoSummary(ctx, code)
		if err != nil {
			return fmt.Errorf("fetch company info for %s: %w", code, err)
		}
		sec, stats := InfoRows(ci)
		return env.Store.Update(ctx, func(tx *store.Tx) error {
			if err := tx.SetSecurityDetail(ctx, sec); err != nil {
				return err
			}
			return tx.UpsertQuoteStats(ctx, stats)
		})
	}}
}

// History merges the last year of daily bars into the stored history.
func History(code string) engine.Job {
	return engine.FuncJob{K: KindHistory + ":" + code, Kd: KindHistory, T: 24 * time.Hour, F: func(ctx context.Context, env *engine.Env) error {
		sec, err := env.Store.Security(ctx, code)
		if err != nil || sec.StockID == 0 {
			return fmt.Errorf("fetch history for %s: %w", code, ErrNoStockID)
		}
		bars, err := env.CSE.ChartByStock(ctx, sec.StockID, cse.PeriodYear)
		if errors.Is(err, cse.ErrNoData) {
			return nil // never traded in the period
		}
		if err != nil {
			return fmt.Errorf("fetch history for %s: %w", code, err)
		}
		return env.Store.Update(ctx, func(tx *store.Tx) error { return tx.MergeBars(ctx, code, BarRows(bars)) })
	}}
}

// Profile stores a company's profile and board.
func Profile(code string) engine.Job {
	return engine.FuncJob{K: KindProfile + ":" + code, Kd: KindProfile, T: 7 * 24 * time.Hour, F: func(ctx context.Context, env *engine.Env) error {
		p, err := env.CSE.CompanyProfile(ctx, code)
		if err != nil {
			return fmt.Errorf("fetch company profile for %s: %w", code, err)
		}
		row, directors := ProfileRows(code, p)
		return env.Store.Update(ctx, func(tx *store.Tx) error { return tx.UpsertProfile(ctx, row, directors) })
	}}
}

// Reports stores the catalogue of a company's financial report PDFs.
func Reports(code string) engine.Job {
	return engine.FuncJob{K: KindReports + ":" + code, Kd: KindReports, T: 24 * time.Hour, F: func(ctx context.Context, env *engine.Env) error {
		f, err := env.CSE.Financials(ctx, code)
		if errors.Is(err, cse.ErrNoData) {
			return nil // no reports uploaded
		}
		if err != nil {
			return fmt.Errorf("fetch financials for %s: %w", code, err)
		}
		return env.Store.Update(ctx, func(tx *store.Tx) error { return tx.UpsertReports(ctx, ReportRows(code, f)) })
	}}
}

// DefaultActionsWindow is how far back the actions job looks when no start
// date is given: long enough to hold the last ex-date of an annual dividend.
const DefaultActionsWindow = 400 * 24 * time.Hour

// Actions stores corporate announcements from since to today (dividends,
// scrip dividends, sub-divisions, capitalisations, rights issues), with their
// dates and proportions, matched to securities by ticker. It searches the
// website's announcement search 20 companies at a time. Pass an earlier
// since, with --force, to backfill.
func Actions(since time.Time) engine.Job {
	return engine.FuncJob{K: KindActions, Kd: KindActions, T: 24 * time.Hour, F: func(ctx context.Context, env *engine.Env) error {
		secs, err := env.Store.Securities(ctx)
		if err != nil {
			return err
		}
		ids, err := env.CSE.SecurityIDs(ctx)
		if err != nil {
			return fmt.Errorf("fetch security ids: %w", err)
		}
		tickers := map[string]bool{}
		for _, s := range secs {
			tickers[strings.ToUpper(s.Ticker)] = true
		}
		var companyIDs []int64
		for t, id := range ids {
			if tickers[t] {
				companyIDs = append(companyIDs, id)
			}
		}
		sort.Slice(companyIDs, func(i, j int) bool { return companyIDs[i] < companyIDs[j] })

		q := cse.SMDQuery{Categories: cse.SMDCategories, FromDate: since.In(output.Colombo).Format("2006-01-02"),
			ToDate: time.Now().In(output.Colombo).Format("2006-01-02")}
		var rows []cse.SMDAnnouncement
		for i := 0; i < len(companyIDs); i += cse.SMDMaxCompanies {
			q.CompanyIDs = companyIDs[i:min(i+cse.SMDMaxCompanies, len(companyIDs))]
			got, err := env.CSE.SearchAnnouncements(ctx, q)
			if errors.Is(err, cse.ErrNoData) {
				continue
			}
			if err != nil {
				return fmt.Errorf("search announcements for companies %d to %d: %w", i+1, i+len(q.CompanyIDs), err)
			}
			rows = append(rows, got...)
		}
		return env.Store.Update(ctx, func(tx *store.Tx) error { return tx.UpsertActions(ctx, ActionRows(rows, secs)) })
	}}
}

// MarketOpen reports whether t falls in CSE regular trading hours
// (weekdays 09:30–14:30 Asia/Colombo; holidays are not known here).
func MarketOpen(t time.Time) bool {
	c := t.In(output.Colombo)
	if c.Weekday() == time.Saturday || c.Weekday() == time.Sunday {
		return false
	}
	mins := c.Hour()*60 + c.Minute()
	return mins >= 9*60+30 && mins < 14*60+30
}
