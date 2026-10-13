package csejobs

import (
	"context"
	"errors"
	"fmt"
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

// Actions stores the recent corporate announcements, matched to securities.
func Actions() engine.Job {
	return engine.FuncJob{K: KindActions, Kd: KindActions, T: 24 * time.Hour, F: func(ctx context.Context, env *engine.Env) error {
		anns, err := env.CSE.Announcements(ctx)
		if err != nil {
			return fmt.Errorf("fetch announcements: %w", err)
		}
		secs, err := env.Store.Securities(ctx)
		if err != nil {
			return err
		}
		return env.Store.Update(ctx, func(tx *store.Tx) error { return tx.UpsertActions(ctx, ActionRows(anns, secs)) })
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
