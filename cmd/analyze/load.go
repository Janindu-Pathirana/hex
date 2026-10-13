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
	return analysis.NewSnapshot(code, name, sector, price, shares, now, rows), nil
}
