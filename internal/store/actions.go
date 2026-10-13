package store

import (
	"context"
	"database/sql"
	"fmt"
)

// Corporate action kinds.
const (
	ActionDividend = "dividend"
	ActionRights   = "rights"
	ActionSplit    = "split"
	ActionBonus    = "bonus"
	ActionAGM      = "agm"
	ActionOther    = "other"
)

// CorporateAction is a stored CSE announcement with its classified kind.
type CorporateAction struct {
	ID          int64
	Code        string // empty when the company is not matched to a security
	Company     string
	Category    string
	Kind        string
	Announced   string // YYYY-MM-DD
	XD          string
	RecordDate  string
	PaymentDate string
	Remarks     string
	Raw         string // original JSON
}

// UpsertActions stores announcements by their CSE id.
func (t *Tx) UpsertActions(ctx context.Context, actions []CorporateAction) error {
	for _, a := range actions {
		err := t.exec(ctx, `
			INSERT INTO corporate_actions (id, code, company, category, kind, announced, xd, record_date,
				payment_date, remarks, raw)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (id) DO UPDATE SET code = excluded.code, company = excluded.company,
				category = excluded.category, kind = excluded.kind, announced = excluded.announced,
				xd = excluded.xd, record_date = excluded.record_date, payment_date = excluded.payment_date,
				remarks = excluded.remarks, raw = excluded.raw`,
			a.ID, nullStr(a.Code), a.Company, a.Category, a.Kind, a.Announced, nullStr(a.XD),
			nullStr(a.RecordDate), nullStr(a.PaymentDate), nullStr(a.Remarks), a.Raw)
		if err != nil {
			return fmt.Errorf("upsert corporate action %d: %w", a.ID, err)
		}
	}
	return nil
}

// Actions returns the stored actions for code (all codes when empty), newest first.
func (db *DB) Actions(ctx context.Context, code string) ([]CorporateAction, error) {
	rows, err := db.sql.QueryContext(ctx, `
		SELECT id, code, company, category, kind, announced, xd, record_date, payment_date, remarks, raw
		FROM corporate_actions WHERE ? = '' OR code = ? ORDER BY announced DESC, id DESC`, code, code)
	if err != nil {
		return nil, fmt.Errorf("query corporate actions: %w", err)
	}
	defer rows.Close()
	var out []CorporateAction
	for rows.Next() {
		var a CorporateAction
		var c, xd, rd, pd, rm sql.NullString
		if err := rows.Scan(&a.ID, &c, &a.Company, &a.Category, &a.Kind, &a.Announced, &xd, &rd, &pd, &rm, &a.Raw); err != nil {
			return nil, fmt.Errorf("scan corporate action: %w", err)
		}
		a.Code, a.XD, a.RecordDate, a.PaymentDate, a.Remarks = c.String, xd.String, rd.String, pd.String, rm.String
		out = append(out, a)
	}
	return out, rows.Err()
}
