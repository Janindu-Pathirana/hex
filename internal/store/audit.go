package store

import (
	"context"
	"fmt"
	"time"
)

// BarStat summarises a company's stored daily price bars.
type BarStat struct {
	Code       string
	Bars       int
	First      string // YYYY-MM-DD
	Last       string // YYYY-MM-DD
	MaxGapDays int    // longest run between two stored bars, in calendar days
	ZeroVolume int    // bars with no volume recorded
}

// BarStats returns one row per company that has price bars, ordered by code.
func (db *DB) BarStats(ctx context.Context) ([]BarStat, error) {
	rows, err := db.sql.QueryContext(ctx, `
		SELECT code, COUNT(*), MIN(date), MAX(date),
			COALESCE(MAX(CAST(julianday(date) - julianday(prev) AS INTEGER)), 0),
			SUM(CASE WHEN COALESCE(volume, 0) = 0 THEN 1 ELSE 0 END)
		FROM (SELECT code, date, volume, LAG(date) OVER (PARTITION BY code ORDER BY date) AS prev FROM price_bars)
		GROUP BY code ORDER BY code`)
	if err != nil {
		return nil, fmt.Errorf("query bar stats: %w", err)
	}
	defer rows.Close()
	var out []BarStat
	for rows.Next() {
		var s BarStat
		if err := rows.Scan(&s.Code, &s.Bars, &s.First, &s.Last, &s.MaxGapDays, &s.ZeroVolume); err != nil {
			return nil, fmt.Errorf("scan bar stats: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// FundamentalStat summarises the stored statement values of a company. Each
// (period, field) counts once, using the value that wins when read.
type FundamentalStat struct {
	Code    string
	Periods int    // distinct period ends
	Values  int    // stored values, all sources
	Newest  string // newest period end
	Low     int    // values below the given confidence
	MinConf float64
}

// FundamentalStats returns one row per company with statement values. Values
// with confidence below low count as low.
func (db *DB) FundamentalStats(ctx context.Context, low float64) ([]FundamentalStat, error) {
	rows, err := db.sql.QueryContext(ctx, `
		SELECT code, COUNT(DISTINCT period_end), COUNT(*), MAX(period_end),
			SUM(CASE WHEN confidence < ? THEN 1 ELSE 0 END), MIN(confidence)
		FROM fundamentals GROUP BY code ORDER BY code`, low)
	if err != nil {
		return nil, fmt.Errorf("query fundamental stats: %w", err)
	}
	defer rows.Close()
	var out []FundamentalStat
	for rows.Next() {
		var s FundamentalStat
		if err := rows.Scan(&s.Code, &s.Periods, &s.Values, &s.Newest, &s.Low, &s.MinConf); err != nil {
			return nil, fmt.Errorf("scan fundamental stats: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// ReportStat counts a company's catalogued reports and how far each got.
type ReportStat struct {
	Code       string
	Quarterly  int
	Annual     int
	Downloaded int // PDF fetched and its text read (text_status set)
	Scanned    int // text_status scanned: no text layer
	Failed     int // text_status failed
	Parsed     int // statement values extracted
}

// ReportStats returns one row per company with catalogued reports.
func (db *DB) ReportStats(ctx context.Context) ([]ReportStat, error) {
	rows, err := db.sql.QueryContext(ctx, `
		SELECT code,
			SUM(CASE WHEN kind = 'quarterly' THEN 1 ELSE 0 END),
			SUM(CASE WHEN kind = 'annual' THEN 1 ELSE 0 END),
			SUM(CASE WHEN text_status IS NOT NULL THEN 1 ELSE 0 END),
			SUM(CASE WHEN text_status = 'scanned' THEN 1 ELSE 0 END),
			SUM(CASE WHEN text_status = 'failed' THEN 1 ELSE 0 END),
			SUM(CASE WHEN parsed_at IS NOT NULL THEN 1 ELSE 0 END)
		FROM reports GROUP BY code ORDER BY code`)
	if err != nil {
		return nil, fmt.Errorf("query report stats: %w", err)
	}
	defer rows.Close()
	var out []ReportStat
	for rows.Next() {
		var s ReportStat
		if err := rows.Scan(&s.Code, &s.Quarterly, &s.Annual, &s.Downloaded, &s.Scanned, &s.Failed, &s.Parsed); err != nil {
			return nil, fmt.Errorf("scan report stats: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// QuoteTimes returns when each stored quote was fetched.
func (db *DB) QuoteTimes(ctx context.Context) (map[string]time.Time, error) {
	rows, err := db.sql.QueryContext(ctx, `SELECT code, fetched_at FROM quotes`)
	if err != nil {
		return nil, fmt.Errorf("query quote times: %w", err)
	}
	defer rows.Close()
	out := map[string]time.Time{}
	for rows.Next() {
		var code string
		var at int64
		if err := rows.Scan(&code, &at); err != nil {
			return nil, fmt.Errorf("scan quote time: %w", err)
		}
		out[code] = time.UnixMilli(at)
	}
	return out, rows.Err()
}

// IndexStat summarises one stored index series.
type IndexStat struct {
	IndexID string
	Days    int
	First   string
	Last    string
}

// IndexStats returns one row per stored index, ordered by id.
func (db *DB) IndexStats(ctx context.Context) ([]IndexStat, error) {
	rows, err := db.sql.QueryContext(ctx, `
		SELECT index_id, COUNT(*), MIN(date), MAX(date) FROM index_bars GROUP BY index_id ORDER BY index_id`)
	if err != nil {
		return nil, fmt.Errorf("query index stats: %w", err)
	}
	defer rows.Close()
	var out []IndexStat
	for rows.Next() {
		var s IndexStat
		if err := rows.Scan(&s.IndexID, &s.Days, &s.First, &s.Last); err != nil {
			return nil, fmt.Errorf("scan index stats: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
