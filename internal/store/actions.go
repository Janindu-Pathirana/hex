package store

import (
	"context"
	"database/sql"
	"fmt"
)

// Corporate action kinds.
const (
	ActionDividend = "dividend"
	ActionScrip    = "scrip" // dividend paid in new shares
	ActionRights   = "rights"
	ActionSplit    = "split" // sub-division of shares
	ActionBonus    = "bonus" // capitalisation of reserves
	ActionAGM      = "agm"
	ActionOther    = "other"
)

// CorporateAction is a stored CSE announcement with its classified kind. All
// dates are YYYY-MM-DD in Colombo time and empty when the CSE gave none.
type CorporateAction struct {
	ID        int64  // the CSE announcement id
	Code      string // empty when the company is not matched to a security
	Ticker    string
	SecID     int64
	Company   string
	Category  string
	Kind      string
	Announced string
	Remarks   string
	Raw       string // original JSON

	ExDate              string // the date prices adjust: xd, xr, xc or trading commencement
	XD                  string
	XR                  string
	XC                  string
	RecordDate          string
	PaymentDate         string
	TradingCommencement string
	TradingSuspended    string
	AllotmentDate       string
	EntitlementDate     string
	AGMDate             string

	Proportion string   // as published, e.g. "03 Ordinary Voting share for every 14 existing"
	RatioNum   *float64 // parsed new shares (split: shares after); nil when the text could not be read
	RatioDen   *float64 // parsed existing shares (split: shares before)
	Price      *float64 // rights issue price or scrip reference price
	DPS        *float64 // cash dividend per voting share
	ChildID    int64
}

// UpsertActions stores announcements by their CSE id.
func (t *Tx) UpsertActions(ctx context.Context, actions []CorporateAction) error {
	for _, a := range actions {
		err := t.exec(ctx, `
			INSERT INTO corporate_actions (id, code, ticker, sec_id, company, category, kind, announced, remarks, raw,
				ex_date, xd, xr, xc, record_date, payment_date, trading_commencement, trading_suspended,
				allotment_date, entitlement_date, agm_date, proportion, ratio_num, ratio_den, price, dps, child_id)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (id) DO UPDATE SET code = excluded.code, ticker = excluded.ticker, sec_id = excluded.sec_id,
				company = excluded.company, category = excluded.category, kind = excluded.kind,
				announced = excluded.announced, remarks = excluded.remarks, raw = excluded.raw,
				ex_date = excluded.ex_date, xd = excluded.xd, xr = excluded.xr, xc = excluded.xc,
				record_date = excluded.record_date, payment_date = excluded.payment_date,
				trading_commencement = excluded.trading_commencement, trading_suspended = excluded.trading_suspended,
				allotment_date = excluded.allotment_date, entitlement_date = excluded.entitlement_date,
				agm_date = excluded.agm_date, proportion = excluded.proportion, ratio_num = excluded.ratio_num,
				ratio_den = excluded.ratio_den, price = excluded.price, dps = excluded.dps, child_id = excluded.child_id`,
			a.ID, nullStr(a.Code), nullStr(a.Ticker), nullInt(a.SecID), a.Company, a.Category, a.Kind, a.Announced,
			nullStr(a.Remarks), a.Raw, nullStr(a.ExDate), nullStr(a.XD), nullStr(a.XR), nullStr(a.XC),
			nullStr(a.RecordDate), nullStr(a.PaymentDate), nullStr(a.TradingCommencement), nullStr(a.TradingSuspended),
			nullStr(a.AllotmentDate), nullStr(a.EntitlementDate), nullStr(a.AGMDate), nullStr(a.Proportion),
			a.RatioNum, a.RatioDen, a.Price, a.DPS, nullInt(a.ChildID))
		if err != nil {
			return fmt.Errorf("upsert corporate action %d: %w", a.ID, err)
		}
	}
	return nil
}

const actionColumns = `id, code, ticker, sec_id, company, category, kind, announced, remarks, raw,
	ex_date, xd, xr, xc, record_date, payment_date, trading_commencement, trading_suspended,
	allotment_date, entitlement_date, agm_date, proportion, ratio_num, ratio_den, price, dps, child_id`

// Actions returns the stored actions for code (all codes when empty), newest first.
func (db *DB) Actions(ctx context.Context, code string) ([]CorporateAction, error) {
	rows, err := db.sql.QueryContext(ctx, `
		SELECT `+actionColumns+` FROM corporate_actions WHERE ? = '' OR code = ?
		ORDER BY announced DESC, id DESC`, code, code)
	if err != nil {
		return nil, fmt.Errorf("query corporate actions: %w", err)
	}
	defer rows.Close()
	var out []CorporateAction
	for rows.Next() {
		var a CorporateAction
		var s [15]sql.NullString
		var secID, child sql.NullInt64
		var num, den, price, dps sql.NullFloat64
		if err := rows.Scan(&a.ID, &s[0], &s[1], &secID, &a.Company, &a.Category, &a.Kind, &a.Announced, &s[2], &a.Raw,
			&s[3], &s[4], &s[5], &s[6], &s[7], &s[8], &s[9], &s[10], &s[11], &s[12], &s[13], &s[14],
			&num, &den, &price, &dps, &child); err != nil {
			return nil, fmt.Errorf("scan corporate action: %w", err)
		}
		a.Code, a.Ticker, a.Remarks = s[0].String, s[1].String, s[2].String
		a.ExDate, a.XD, a.XR, a.XC = s[3].String, s[4].String, s[5].String, s[6].String
		a.RecordDate, a.PaymentDate = s[7].String, s[8].String
		a.TradingCommencement, a.TradingSuspended = s[9].String, s[10].String
		a.AllotmentDate, a.EntitlementDate, a.AGMDate, a.Proportion = s[11].String, s[12].String, s[13].String, s[14].String
		a.SecID, a.ChildID = secID.Int64, child.Int64
		a.RatioNum, a.RatioDen, a.Price, a.DPS = floatPtr(num), floatPtr(den), floatPtr(price), floatPtr(dps)
		out = append(out, a)
	}
	return out, rows.Err()
}

func floatPtr(v sql.NullFloat64) *float64 {
	if !v.Valid {
		return nil
	}
	f := v.Float64
	return &f
}
