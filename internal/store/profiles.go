package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Profile is a company's registration details and business summary.
type Profile struct {
	Code            string
	Name            string
	Sector          string
	Board           string
	FYEnd           string
	Auditors        string
	Secretaries     string
	Established     string
	Address         string
	Web             string
	Email           string
	BusinessSummary string
	UpdatedAt       time.Time
}

// Director is a board member or key officer, in the order CSE lists them.
type Director struct {
	Name string
	Role string
}

// UpsertProfile replaces the profile and director list for a security.
func (t *Tx) UpsertProfile(ctx context.Context, p Profile, directors []Director) error {
	err := t.exec(ctx, `
		INSERT INTO profiles (code, name, sector, board, fy_end, auditors, secretaries, established, address,
			web, email, business_summary, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (code) DO UPDATE SET name = excluded.name, sector = excluded.sector, board = excluded.board,
			fy_end = excluded.fy_end, auditors = excluded.auditors, secretaries = excluded.secretaries,
			established = excluded.established, address = excluded.address, web = excluded.web,
			email = excluded.email, business_summary = excluded.business_summary, updated_at = excluded.updated_at`,
		p.Code, nullStr(p.Name), nullStr(p.Sector), nullStr(p.Board), nullStr(p.FYEnd), nullStr(p.Auditors),
		nullStr(p.Secretaries), nullStr(p.Established), nullStr(p.Address), nullStr(p.Web), nullStr(p.Email),
		nullStr(p.BusinessSummary), ms(t.now))
	if err != nil {
		return fmt.Errorf("upsert profile %s: %w", p.Code, err)
	}
	if err := t.exec(ctx, `DELETE FROM directors WHERE code = ?`, p.Code); err != nil {
		return fmt.Errorf("clear directors %s: %w", p.Code, err)
	}
	for i, d := range directors {
		if err := t.exec(ctx, `INSERT INTO directors (code, position, name, role) VALUES (?, ?, ?, ?)`,
			p.Code, i, d.Name, nullStr(d.Role)); err != nil {
			return fmt.Errorf("insert director %s: %w", p.Code, err)
		}
	}
	return nil
}

// Profile returns the stored profile and directors for code, or sql.ErrNoRows.
func (db *DB) Profile(ctx context.Context, code string) (Profile, []Director, error) {
	var p Profile
	var f [11]sql.NullString
	var updated int64
	err := db.sql.QueryRowContext(ctx, `
		SELECT code, name, sector, board, fy_end, auditors, secretaries, established, address, web, email,
			business_summary, updated_at FROM profiles WHERE code = ?`, code).
		Scan(&p.Code, &f[0], &f[1], &f[2], &f[3], &f[4], &f[5], &f[6], &f[7], &f[8], &f[9], &f[10], &updated)
	if err != nil {
		return Profile{}, nil, err
	}
	p.Name, p.Sector, p.Board, p.FYEnd, p.Auditors = f[0].String, f[1].String, f[2].String, f[3].String, f[4].String
	p.Secretaries, p.Established, p.Address, p.Web = f[5].String, f[6].String, f[7].String, f[8].String
	p.Email, p.BusinessSummary, p.UpdatedAt = f[9].String, f[10].String, time.UnixMilli(updated)

	rows, err := db.sql.QueryContext(ctx, `SELECT name, COALESCE(role, '') FROM directors WHERE code = ? ORDER BY position`, code)
	if err != nil {
		return Profile{}, nil, fmt.Errorf("query directors %s: %w", code, err)
	}
	defer rows.Close()
	var ds []Director
	for rows.Next() {
		var d Director
		if err := rows.Scan(&d.Name, &d.Role); err != nil {
			return Profile{}, nil, fmt.Errorf("scan director: %w", err)
		}
		ds = append(ds, d)
	}
	return p, ds, rows.Err()
}
