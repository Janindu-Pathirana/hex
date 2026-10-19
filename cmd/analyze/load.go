// Package analyze loads a company's stored data into an analysis snapshot.
package analyze

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Janindu-Pathirana/hex/internal/analysis"
	"github.com/Janindu-Pathirana/hex/internal/store"
)

// ErrNoFundamentals means no statement values are stored for the company.
var ErrNoFundamentals = errors.New("no fundamentals stored")

// Load reads everything the store holds for code. Missing quote, profile or
// share count are not errors; the analysis reports what it could not compute.
func Load(ctx context.Context, db *store.DB, code string, now time.Time) (*analysis.Snapshot, error) {
	fs, err := db.Fundamentals(ctx, code, false)
	if err != nil {
		return nil, fmt.Errorf("load fundamentals for %s: %w", code, err)
	}
	if len(fs) == 0 {
		return nil, fmt.Errorf("%w for %s", ErrNoFundamentals, code)
	}
	rows := make([]analysis.Row, len(fs))
	for i, f := range fs {
		rows[i] = analysis.Row{PeriodEnd: f.PeriodEnd, PeriodType: f.PeriodType, Scope: f.Scope, Field: f.Field,
			Source: f.Source, Value: f.Value, Confidence: f.Confidence}
	}

	var price, shares float64
	q, err := db.Quote(ctx, code)
	switch {
	case err == nil:
		price = q.Price
	case !errors.Is(err, sql.ErrNoRows):
		return nil, fmt.Errorf("load quote for %s: %w", code, err)
	}
	sec, err := db.Security(ctx, code)
	switch {
	case err == nil:
		shares = sec.Issued
	case !errors.Is(err, sql.ErrNoRows):
		return nil, fmt.Errorf("load security %s: %w", code, err)
	}
	var name, sector string
	p, _, err := db.Profile(ctx, code)
	switch {
	case err == nil:
		name, sector = p.Name, p.Sector
	case !errors.Is(err, sql.ErrNoRows):
		return nil, fmt.Errorf("load profile for %s: %w", code, err)
	}
	if name == "" {
		name = sec.Name
	}
	bars, err := db.Bars(ctx, code, "", "")
	if err != nil {
		return nil, fmt.Errorf("load price bars for %s: %w", code, err)
	}
	idx, err := db.IndexBars(ctx, "ASPI")
	if err != nil {
		return nil, fmt.Errorf("load index bars for ASPI: %w", err)
	}
	days, err := db.MarketDays(ctx)
	if err != nil {
		return nil, fmt.Errorf("load market days: %w", err)
	}
	snap := analysis.NewSnapshot(code, name, sector, price, shares, now, rows)
	return snap.WithBars(barsFromStore(bars), indexFromStore(idx)).WithMarket(marketFromStore(days)), nil
}

func barsFromStore(in []store.Bar) []analysis.Bar {
	out := make([]analysis.Bar, 0, len(in))
	for _, b := range in {
		d, err := time.Parse("2006-01-02", b.Date)
		if err != nil {
			continue
		}
		out = append(out, analysis.Bar{Date: d, High: b.High, Low: b.Low, Close: b.Close, Volume: b.Volume})
	}
	return out
}

func indexFromStore(in []store.IndexBar) []analysis.Bar {
	out := make([]analysis.Bar, 0, len(in))
	for _, b := range in {
		d, err := time.Parse("2006-01-02", b.Date)
		if err != nil {
			continue
		}
		out = append(out, analysis.Bar{Date: d, Close: b.Value})
	}
	return out
}

func marketFromStore(in []store.MarketDay) []analysis.MarketDay {
	out := make([]analysis.MarketDay, 0, len(in))
	for _, d := range in {
		t, err := time.Parse("2006-01-02", d.Date)
		if err != nil {
			continue
		}
		out = append(out, analysis.MarketDay{Date: t, ASI: d.ASI, SPSL20: d.SPSL20, Turnover: d.Turnover,
			ForeignBuy: d.ForeignBuy, ForeignSell: d.ForeignSell, Listed: d.Listed, Traded: d.Traded, PER: d.PER, DY: d.DY})
	}
	return out
}
