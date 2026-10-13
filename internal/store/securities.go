package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Security is a listed security and its identifiers.
type Security struct {
	Code      string
	Ticker    string
	Series    string
	Name      string
	StockID   int64
	ISIN      string
	Issued    float64
	ParValue  float64
	UpdatedAt time.Time
}

// UpsertSecurities inserts or renames securities from the symbol list. It
// leaves stock_id, ISIN and share counts untouched.
func (t *Tx) UpsertSecurities(ctx context.Context, secs []Security) error {
	for _, s := range secs {
		err := t.exec(ctx, `
			INSERT INTO securities (code, ticker, series, name, updated_at) VALUES (?, ?, ?, ?, ?)
			ON CONFLICT (code) DO UPDATE SET ticker = excluded.ticker, series = excluded.series,
				name = excluded.name, updated_at = excluded.updated_at`,
			s.Code, s.Ticker, s.Series, s.Name, ms(t.now))
		if err != nil {
			return fmt.Errorf("upsert security %s: %w", s.Code, err)
		}
	}
	return nil
}

// SetSecurityDetail records the identifiers from companyInfoSummery or
// tradeSummary. Zero values do not overwrite stored ones.
func (t *Tx) SetSecurityDetail(ctx context.Context, s Security) error {
	err := t.exec(ctx, `
		INSERT INTO securities (code, ticker, series, name, stock_id, isin, issued, par_value, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (code) DO UPDATE SET
			name      = CASE WHEN excluded.name <> '' THEN excluded.name ELSE securities.name END,
			stock_id  = COALESCE(excluded.stock_id, securities.stock_id),
			isin      = COALESCE(excluded.isin, securities.isin),
			issued    = COALESCE(excluded.issued, securities.issued),
			par_value = COALESCE(excluded.par_value, securities.par_value),
			updated_at = excluded.updated_at`,
		s.Code, s.Ticker, s.Series, s.Name, nullInt(s.StockID), nullStr(s.ISIN),
		nullFloat(s.Issued), nullFloat(s.ParValue), ms(t.now))
	if err != nil {
		return fmt.Errorf("set security detail %s: %w", s.Code, err)
	}
	return nil
}

// Securities returns every stored security ordered by code.
func (db *DB) Securities(ctx context.Context) ([]Security, error) {
	rows, err := db.sql.QueryContext(ctx, `
		SELECT code, ticker, series, name, stock_id, isin, issued, par_value, updated_at
		FROM securities ORDER BY code`)
	if err != nil {
		return nil, fmt.Errorf("query securities: %w", err)
	}
	defer rows.Close()
	var out []Security
	for rows.Next() {
		var s Security
		var stockID sql.NullInt64
		var isin sql.NullString
		var issued, par sql.NullFloat64
		var updated int64
		if err := rows.Scan(&s.Code, &s.Ticker, &s.Series, &s.Name, &stockID, &isin, &issued, &par, &updated); err != nil {
			return nil, fmt.Errorf("scan security: %w", err)
		}
		s.StockID, s.ISIN, s.Issued, s.ParValue = stockID.Int64, isin.String, issued.Float64, par.Float64
		s.UpdatedAt = time.UnixMilli(updated)
		out = append(out, s)
	}
	return out, rows.Err()
}

// Security returns one security by full code, or sql.ErrNoRows.
func (db *DB) Security(ctx context.Context, code string) (Security, error) {
	secs, err := db.Securities(ctx)
	if err != nil {
		return Security{}, err
	}
	for _, s := range secs {
		if s.Code == code {
			return s, nil
		}
	}
	return Security{}, sql.ErrNoRows
}

func nullFloat(v float64) any {
	if v == 0 {
		return nil
	}
	return v
}
