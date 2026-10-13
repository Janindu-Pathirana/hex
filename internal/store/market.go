package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Quote is the latest trade-summary snapshot for a security.
type Quote struct {
	Code      string
	TradedAt  time.Time
	Price     float64
	Change    float64
	ChangePct float64
	Open      float64
	High      float64
	Low       float64
	PrevClose float64
	Volume    float64
	Trades    float64
	Turnover  float64
	MarketCap float64
	FetchedAt time.Time
}

// UpsertQuotes replaces the latest quote for each security.
func (t *Tx) UpsertQuotes(ctx context.Context, qs []Quote) error {
	for _, q := range qs {
		err := t.exec(ctx, `
			INSERT INTO quotes (code, traded_at, price, change, change_pct, open, high, low, prev_close,
				volume, trades, turnover, market_cap, fetched_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (code) DO UPDATE SET traded_at = excluded.traded_at, price = excluded.price,
				change = excluded.change, change_pct = excluded.change_pct, open = excluded.open,
				high = excluded.high, low = excluded.low, prev_close = excluded.prev_close,
				volume = excluded.volume, trades = excluded.trades, turnover = excluded.turnover,
				market_cap = excluded.market_cap, fetched_at = excluded.fetched_at`,
			q.Code, nullTime(q.TradedAt), q.Price, q.Change, q.ChangePct, q.Open, q.High, q.Low, q.PrevClose,
			q.Volume, q.Trades, q.Turnover, q.MarketCap, ms(t.now))
		if err != nil {
			return fmt.Errorf("upsert quote %s: %w", q.Code, err)
		}
	}
	return nil
}

// Quote returns the latest quote for code, or sql.ErrNoRows.
func (db *DB) Quote(ctx context.Context, code string) (Quote, error) {
	var q Quote
	var traded sql.NullInt64
	var fetched int64
	err := db.sql.QueryRowContext(ctx, `
		SELECT code, traded_at, price, change, change_pct, open, high, low, prev_close, volume, trades,
			turnover, market_cap, fetched_at FROM quotes WHERE code = ?`, code).
		Scan(&q.Code, &traded, &q.Price, &q.Change, &q.ChangePct, &q.Open, &q.High, &q.Low, &q.PrevClose,
			&q.Volume, &q.Trades, &q.Turnover, &q.MarketCap, &fetched)
	if err != nil {
		return Quote{}, err
	}
	q.TradedAt, q.FetchedAt = fromMS(traded), time.UnixMilli(fetched)
	return q, nil
}

// QuoteStats holds the ranges and beta from companyInfoSummery.
type QuoteStats struct {
	Code       string
	High52W    float64
	Low52W     float64
	YTDHigh    float64
	YTDLow     float64
	AllHigh    float64
	AllLow     float64
	Volume52W  float64
	BetaASPI   *float64
	BetaSPSL20 *float64
	BetaPeriod string
	ForeignPct *float64
	FetchedAt  time.Time
}

// UpsertQuoteStats replaces the stored ranges and beta for a security.
func (t *Tx) UpsertQuoteStats(ctx context.Context, s QuoteStats) error {
	err := t.exec(ctx, `
		INSERT INTO quote_stats (code, high_52w, low_52w, ytd_high, ytd_low, all_high, all_low, volume_52w,
			beta_aspi, beta_spsl20, beta_period, foreign_pct, fetched_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (code) DO UPDATE SET high_52w = excluded.high_52w, low_52w = excluded.low_52w,
			ytd_high = excluded.ytd_high, ytd_low = excluded.ytd_low, all_high = excluded.all_high,
			all_low = excluded.all_low, volume_52w = excluded.volume_52w, beta_aspi = excluded.beta_aspi,
			beta_spsl20 = excluded.beta_spsl20, beta_period = excluded.beta_period,
			foreign_pct = excluded.foreign_pct, fetched_at = excluded.fetched_at`,
		s.Code, s.High52W, s.Low52W, s.YTDHigh, s.YTDLow, s.AllHigh, s.AllLow, s.Volume52W,
		s.BetaASPI, s.BetaSPSL20, nullStr(s.BetaPeriod), s.ForeignPct, ms(t.now))
	if err != nil {
		return fmt.Errorf("upsert quote stats %s: %w", s.Code, err)
	}
	return nil
}

// QuoteStats returns the stored ranges and beta for code, or sql.ErrNoRows.
func (db *DB) QuoteStats(ctx context.Context, code string) (QuoteStats, error) {
	var s QuoteStats
	var ba, bs, fp sql.NullFloat64
	var period sql.NullString
	var fetched int64
	err := db.sql.QueryRowContext(ctx, `
		SELECT code, high_52w, low_52w, ytd_high, ytd_low, all_high, all_low, volume_52w, beta_aspi,
			beta_spsl20, beta_period, foreign_pct, fetched_at FROM quote_stats WHERE code = ?`, code).
		Scan(&s.Code, &s.High52W, &s.Low52W, &s.YTDHigh, &s.YTDLow, &s.AllHigh, &s.AllLow, &s.Volume52W,
			&ba, &bs, &period, &fp, &fetched)
	if err != nil {
		return QuoteStats{}, err
	}
	s.BetaASPI, s.BetaSPSL20, s.ForeignPct = ptrFloat(ba), ptrFloat(bs), ptrFloat(fp)
	s.BetaPeriod, s.FetchedAt = period.String, time.UnixMilli(fetched)
	return s, nil
}

// Bar is one daily price bar. Open, High and Low may be missing.
type Bar struct {
	Date   string // YYYY-MM-DD, Colombo
	Open   *float64
	High   *float64
	Low    *float64
	Close  float64
	Volume float64
}

// MergeBars upserts daily bars for code. Existing dates are updated and
// older dates are kept, so history grows past what one fetch returns.
func (t *Tx) MergeBars(ctx context.Context, code string, bars []Bar) error {
	for _, b := range bars {
		err := t.exec(ctx, `
			INSERT INTO price_bars (code, date, open, high, low, close, volume) VALUES (?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (code, date) DO UPDATE SET open = excluded.open, high = excluded.high,
				low = excluded.low, close = excluded.close, volume = excluded.volume`,
			code, b.Date, b.Open, b.High, b.Low, b.Close, b.Volume)
		if err != nil {
			return fmt.Errorf("merge bar %s %s: %w", code, b.Date, err)
		}
	}
	return nil
}

// Bars returns the stored bars for code between from and to (inclusive,
// YYYY-MM-DD; empty means unbounded), oldest first.
func (db *DB) Bars(ctx context.Context, code, from, to string) ([]Bar, error) {
	if to == "" {
		to = "9999-12-31"
	}
	rows, err := db.sql.QueryContext(ctx, `
		SELECT date, open, high, low, close, volume FROM price_bars
		WHERE code = ? AND date >= ? AND date <= ? ORDER BY date`, code, from, to)
	if err != nil {
		return nil, fmt.Errorf("query bars %s: %w", code, err)
	}
	defer rows.Close()
	var out []Bar
	for rows.Next() {
		var b Bar
		var o, h, l, v sql.NullFloat64
		if err := rows.Scan(&b.Date, &o, &h, &l, &b.Close, &v); err != nil {
			return nil, fmt.Errorf("scan bar: %w", err)
		}
		b.Open, b.High, b.Low, b.Volume = ptrFloat(o), ptrFloat(h), ptrFloat(l), v.Float64
		out = append(out, b)
	}
	return out, rows.Err()
}

// IndexBar is one daily value of a market or sector index.
type IndexBar struct {
	IndexID   string // ASPI, SPSL20 or a sector symbol such as EGY
	Date      string
	Name      string
	Value     float64
	Change    float64
	ChangePct float64
	Turnover  *float64
	Volume    *float64
	Trades    *float64
}

// UpsertIndexBars stores index values, one per index per day.
func (t *Tx) UpsertIndexBars(ctx context.Context, bars []IndexBar) error {
	for _, b := range bars {
		err := t.exec(ctx, `
			INSERT INTO index_bars (index_id, date, name, value, change, change_pct, turnover, volume, trades)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (index_id, date) DO UPDATE SET name = excluded.name, value = excluded.value,
				change = excluded.change, change_pct = excluded.change_pct, turnover = excluded.turnover,
				volume = excluded.volume, trades = excluded.trades`,
			b.IndexID, b.Date, nullStr(b.Name), b.Value, b.Change, b.ChangePct, b.Turnover, b.Volume, b.Trades)
		if err != nil {
			return fmt.Errorf("upsert index bar %s %s: %w", b.IndexID, b.Date, err)
		}
	}
	return nil
}

// IndexBars returns the stored values of one index, oldest first.
func (db *DB) IndexBars(ctx context.Context, indexID string) ([]IndexBar, error) {
	rows, err := db.sql.QueryContext(ctx, `
		SELECT index_id, date, COALESCE(name, ''), value, change, change_pct, turnover, volume, trades
		FROM index_bars WHERE index_id = ? ORDER BY date`, indexID)
	if err != nil {
		return nil, fmt.Errorf("query index bars %s: %w", indexID, err)
	}
	defer rows.Close()
	var out []IndexBar
	for rows.Next() {
		var b IndexBar
		var tu, vo, tr sql.NullFloat64
		if err := rows.Scan(&b.IndexID, &b.Date, &b.Name, &b.Value, &b.Change, &b.ChangePct, &tu, &vo, &tr); err != nil {
			return nil, fmt.Errorf("scan index bar: %w", err)
		}
		b.Turnover, b.Volume, b.Trades = ptrFloat(tu), ptrFloat(vo), ptrFloat(tr)
		out = append(out, b)
	}
	return out, rows.Err()
}

// MarketDay is one day of market-wide statistics.
type MarketDay struct {
	Date         string
	Turnover     float64
	Trades       float64
	ShareVolume  float64
	MarketCap    float64
	PER          float64
	PBV          float64
	DY           float64
	ASI          float64
	SPSL20       float64
	DomesticBuy  float64
	DomesticSell float64
	ForeignBuy   float64
	ForeignSell  float64
	Listed       float64
	Traded       float64
}

// UpsertMarketDays stores daily market statistics.
func (t *Tx) UpsertMarketDays(ctx context.Context, days []MarketDay) error {
	for _, d := range days {
		err := t.exec(ctx, `
			INSERT INTO market_daily (date, turnover, trades, share_volume, market_cap, per, pbv, dy, asi, spsl20,
				domestic_buy, domestic_sell, foreign_buy, foreign_sell, listed, traded)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (date) DO UPDATE SET turnover = excluded.turnover, trades = excluded.trades,
				share_volume = excluded.share_volume, market_cap = excluded.market_cap, per = excluded.per,
				pbv = excluded.pbv, dy = excluded.dy, asi = excluded.asi, spsl20 = excluded.spsl20,
				domestic_buy = excluded.domestic_buy, domestic_sell = excluded.domestic_sell,
				foreign_buy = excluded.foreign_buy, foreign_sell = excluded.foreign_sell,
				listed = excluded.listed, traded = excluded.traded`,
			d.Date, d.Turnover, d.Trades, d.ShareVolume, d.MarketCap, d.PER, d.PBV, d.DY, d.ASI, d.SPSL20,
			d.DomesticBuy, d.DomesticSell, d.ForeignBuy, d.ForeignSell, d.Listed, d.Traded)
		if err != nil {
			return fmt.Errorf("upsert market day %s: %w", d.Date, err)
		}
	}
	return nil
}

// MarketDays returns the stored market days, oldest first.
func (db *DB) MarketDays(ctx context.Context) ([]MarketDay, error) {
	rows, err := db.sql.QueryContext(ctx, `
		SELECT date, turnover, trades, share_volume, market_cap, per, pbv, dy, asi, spsl20,
			domestic_buy, domestic_sell, foreign_buy, foreign_sell, listed, traded
		FROM market_daily ORDER BY date`)
	if err != nil {
		return nil, fmt.Errorf("query market days: %w", err)
	}
	defer rows.Close()
	var out []MarketDay
	for rows.Next() {
		var d MarketDay
		if err := rows.Scan(&d.Date, &d.Turnover, &d.Trades, &d.ShareVolume, &d.MarketCap, &d.PER, &d.PBV,
			&d.DY, &d.ASI, &d.SPSL20, &d.DomesticBuy, &d.DomesticSell, &d.ForeignBuy, &d.ForeignSell,
			&d.Listed, &d.Traded); err != nil {
			return nil, fmt.Errorf("scan market day: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// SetMeta stores a small named value, such as the market status.
func (t *Tx) SetMeta(ctx context.Context, key, value string) error {
	err := t.exec(ctx, `
		INSERT INTO meta (key, value, updated_at) VALUES (?, ?, ?)
		ON CONFLICT (key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		key, value, ms(t.now))
	if err != nil {
		return fmt.Errorf("set meta %s: %w", key, err)
	}
	return nil
}

// Meta returns a stored value and when it was set, or sql.ErrNoRows.
func (db *DB) Meta(ctx context.Context, key string) (string, time.Time, error) {
	var v string
	var at int64
	if err := db.sql.QueryRowContext(ctx, `SELECT value, updated_at FROM meta WHERE key = ?`, key).Scan(&v, &at); err != nil {
		return "", time.Time{}, err
	}
	return v, time.UnixMilli(at), nil
}
