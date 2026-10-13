package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Fundamental sources, in override order: manual beats extracted values.
const (
	SourceManual = "manual"
	SourcePDF    = "pdf"
	SourceLLM    = "llm"
)

// Fundamental is one financial-statement value for a period.
type Fundamental struct {
	Code       string
	PeriodEnd  string // YYYY-MM-DD
	PeriodType string // 3M, 6M, 9M, 12M
	Scope      string // group, company
	Field      string // canonical name, e.g. revenue, eps_basic
	Source     string
	Value      float64
	ReportID   int64
	RunID      int64
	Page       int
	RawLabel   string
	Confidence float64
	UpdatedAt  time.Time
}

// UpsertFundamentals stores values keyed by period, scope, field and source.
func (t *Tx) UpsertFundamentals(ctx context.Context, fs []Fundamental) error {
	for _, f := range fs {
		err := t.exec(ctx, `
			INSERT INTO fundamentals (code, period_end, period_type, scope, field, source, value, report_id,
				run_id, page, raw_label, confidence, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (code, period_end, period_type, scope, field, source) DO UPDATE SET
				value = excluded.value, report_id = excluded.report_id, run_id = excluded.run_id,
				page = excluded.page, raw_label = excluded.raw_label, confidence = excluded.confidence,
				updated_at = excluded.updated_at`,
			f.Code, f.PeriodEnd, f.PeriodType, f.Scope, f.Field, f.Source, f.Value, nullInt(f.ReportID),
			nullInt(f.RunID), nullInt(int64(f.Page)), nullStr(f.RawLabel), f.Confidence, ms(t.now))
		if err != nil {
			return fmt.Errorf("upsert fundamental %s %s %s: %w", f.Code, f.PeriodEnd, f.Field, err)
		}
	}
	return nil
}

// DeleteReportFundamentals removes the values one source extracted from a
// report, before it is re-parsed.
func (t *Tx) DeleteReportFundamentals(ctx context.Context, reportID int64, source string) error {
	if err := t.exec(ctx, `DELETE FROM fundamentals WHERE report_id = ? AND source = ?`, reportID, source); err != nil {
		return fmt.Errorf("delete fundamentals for report %d: %w", reportID, err)
	}
	return nil
}

// Fundamentals returns the values for code, newest period first. Unless all is
// set, only the winning value per period, scope and field is returned:
// manual first, then the highest confidence.
func (db *DB) Fundamentals(ctx context.Context, code string, all bool) ([]Fundamental, error) {
	rows, err := db.sql.QueryContext(ctx, `
		SELECT code, period_end, period_type, scope, field, source, value, report_id, run_id, page, raw_label,
			confidence, updated_at
		FROM fundamentals WHERE code = ?
		ORDER BY period_end DESC, period_type, scope, field,
			CASE source WHEN 'manual' THEN 0 ELSE 1 END, confidence DESC, updated_at DESC`, code)
	if err != nil {
		return nil, fmt.Errorf("query fundamentals %s: %w", code, err)
	}
	defer rows.Close()
	var out []Fundamental
	var lastKey string
	for rows.Next() {
		var f Fundamental
		var rep, run, page sql.NullInt64
		var label sql.NullString
		var updated int64
		if err := rows.Scan(&f.Code, &f.PeriodEnd, &f.PeriodType, &f.Scope, &f.Field, &f.Source, &f.Value,
			&rep, &run, &page, &label, &f.Confidence, &updated); err != nil {
			return nil, fmt.Errorf("scan fundamental: %w", err)
		}
		f.ReportID, f.RunID, f.Page, f.RawLabel = rep.Int64, run.Int64, int(page.Int64), label.String
		f.UpdatedAt = time.UnixMilli(updated)
		key := f.PeriodEnd + "|" + f.PeriodType + "|" + f.Scope + "|" + f.Field
		if !all && key == lastKey {
			continue
		}
		lastKey = key
		out = append(out, f)
	}
	return out, rows.Err()
}

// Extraction run statuses.
const (
	RunOK            = "ok"
	RunLowConfidence = "low_confidence"
	RunFailed        = "failed"
)

// ExtractionRun records one attempt to extract fundamentals from a report.
type ExtractionRun struct {
	ID         int64
	ReportID   int64
	Method     string // rules, llm
	Version    string
	Model      string
	StartedAt  time.Time
	Confidence float64
	Status     string
	Errors     string
}

// InsertRun stores an extraction run and returns its id.
func (t *Tx) InsertRun(ctx context.Context, r ExtractionRun) (int64, error) {
	res, err := t.tx.ExecContext(ctx, `
		INSERT INTO extraction_runs (report_id, method, extractor_version, model, started_at, confidence, status, errors)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ReportID, r.Method, r.Version, nullStr(r.Model), ms(r.StartedAt), r.Confidence, r.Status, nullStr(r.Errors))
	if err != nil {
		return 0, fmt.Errorf("insert extraction run for report %d: %w", r.ReportID, err)
	}
	return res.LastInsertId()
}

// Runs returns the extraction runs for a report, newest first.
func (db *DB) Runs(ctx context.Context, reportID int64) ([]ExtractionRun, error) {
	rows, err := db.sql.QueryContext(ctx, `
		SELECT id, report_id, method, extractor_version, model, started_at, confidence, status, errors
		FROM extraction_runs WHERE report_id = ? ORDER BY id DESC`, reportID)
	if err != nil {
		return nil, fmt.Errorf("query extraction runs %d: %w", reportID, err)
	}
	defer rows.Close()
	var out []ExtractionRun
	for rows.Next() {
		var r ExtractionRun
		var model, errs sql.NullString
		var conf sql.NullFloat64
		var started int64
		if err := rows.Scan(&r.ID, &r.ReportID, &r.Method, &r.Version, &model, &started, &conf, &r.Status, &errs); err != nil {
			return nil, fmt.Errorf("scan extraction run: %w", err)
		}
		r.Model, r.Errors, r.Confidence, r.StartedAt = model.String, errs.String, conf.Float64, time.UnixMilli(started)
		out = append(out, r)
	}
	return out, rows.Err()
}
