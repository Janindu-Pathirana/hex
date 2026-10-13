package store

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"fmt"
	"io"
	"time"
)

// Report kinds.
const (
	ReportAnnual    = "annual"
	ReportQuarterly = "quarterly"
	ReportOther     = "other"
)

// Report text statuses.
const (
	TextOK      = "ok"
	TextScanned = "scanned"
	TextFailed  = "failed"
)

// Report is a catalogued financial report PDF and its processing state.
type Report struct {
	ID           int64
	Code         string
	Kind         string
	Title        string
	PeriodDate   string // manualDate as YYYY-MM-DD
	UploadedAt   time.Time
	AuthorizedAt time.Time
	Path         string
	SHA256       string
	Bytes        int64
	Pages        int
	TextStatus   string
	TextError    string
	ParsedAt     time.Time
}

// UpsertReports stores report catalogue entries. Download and parse state is kept.
func (t *Tx) UpsertReports(ctx context.Context, reports []Report) error {
	for _, r := range reports {
		err := t.exec(ctx, `
			INSERT INTO reports (id, code, kind, title, period_date, uploaded_at, authorized_at, path)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (id) DO UPDATE SET code = excluded.code, kind = excluded.kind, title = excluded.title,
				period_date = excluded.period_date, uploaded_at = excluded.uploaded_at,
				authorized_at = excluded.authorized_at, path = excluded.path`,
			r.ID, r.Code, r.Kind, r.Title, nullStr(r.PeriodDate), nullTime(r.UploadedAt), nullTime(r.AuthorizedAt), r.Path)
		if err != nil {
			return fmt.Errorf("upsert report %d: %w", r.ID, err)
		}
	}
	return nil
}

// SetReportFile records the downloaded PDF's hash and size.
func (t *Tx) SetReportFile(ctx context.Context, id int64, sha256 string, size int64) error {
	if err := t.exec(ctx, `UPDATE reports SET pdf_sha256 = ?, bytes = ? WHERE id = ?`, sha256, size, id); err != nil {
		return fmt.Errorf("set report file %d: %w", id, err)
	}
	return nil
}

// ReportPage is one page of extracted text. Text is the page as JSON; it is
// stored gzipped.
type ReportPage struct {
	Page  int
	Kind  string
	Score float64
	Text  []byte
}

// SetReportText records the text-extraction outcome and replaces the stored pages.
func (t *Tx) SetReportText(ctx context.Context, id int64, pageCount int, status, errText string, pages []ReportPage) error {
	if err := t.exec(ctx, `UPDATE reports SET pages = ?, text_status = ?, text_error = ? WHERE id = ?`,
		pageCount, status, nullStr(errText), id); err != nil {
		return fmt.Errorf("set report text %d: %w", id, err)
	}
	if err := t.exec(ctx, `DELETE FROM report_pages WHERE report_id = ?`, id); err != nil {
		return fmt.Errorf("clear report pages %d: %w", id, err)
	}
	for _, p := range pages {
		gz, err := gzipBytes(p.Text)
		if err != nil {
			return err
		}
		kind := p.Kind
		if kind == "" {
			kind = "other"
		}
		if err := t.exec(ctx, `INSERT INTO report_pages (report_id, page, kind, score, text_gz) VALUES (?, ?, ?, ?, ?)`,
			id, p.Page, kind, p.Score, gz); err != nil {
			return fmt.Errorf("insert report page %d/%d: %w", id, p.Page, err)
		}
	}
	return nil
}

// SetReportParsed stamps the report as parsed by the extractor.
func (t *Tx) SetReportParsed(ctx context.Context, id int64) error {
	if err := t.exec(ctx, `UPDATE reports SET parsed_at = ? WHERE id = ?`, ms(t.now), id); err != nil {
		return fmt.Errorf("set report parsed %d: %w", id, err)
	}
	return nil
}

const reportColumns = `id, code, kind, title, period_date, uploaded_at, authorized_at, path, pdf_sha256, bytes,
	pages, text_status, text_error, parsed_at`

func scanReport(sc interface{ Scan(...any) error }) (Report, error) {
	var r Report
	var period, sha, status, errText sql.NullString
	var up, auth, parsed, size, pages sql.NullInt64
	if err := sc.Scan(&r.ID, &r.Code, &r.Kind, &r.Title, &period, &up, &auth, &r.Path, &sha, &size,
		&pages, &status, &errText, &parsed); err != nil {
		return Report{}, err
	}
	r.PeriodDate, r.SHA256, r.TextStatus, r.TextError = period.String, sha.String, status.String, errText.String
	r.UploadedAt, r.AuthorizedAt, r.ParsedAt = fromMS(up), fromMS(auth), fromMS(parsed)
	r.Bytes, r.Pages = size.Int64, int(pages.Int64)
	return r, nil
}

// Reports returns the reports for code (all codes when empty), newest period first.
func (db *DB) Reports(ctx context.Context, code string) ([]Report, error) {
	rows, err := db.sql.QueryContext(ctx, `SELECT `+reportColumns+` FROM reports
		WHERE ? = '' OR code = ? ORDER BY period_date DESC, id DESC`, code, code)
	if err != nil {
		return nil, fmt.Errorf("query reports: %w", err)
	}
	defer rows.Close()
	var out []Report
	for rows.Next() {
		r, err := scanReport(rows)
		if err != nil {
			return nil, fmt.Errorf("scan report: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// Report returns one report by id, or sql.ErrNoRows.
func (db *DB) Report(ctx context.Context, id int64) (Report, error) {
	return scanReport(db.sql.QueryRowContext(ctx, `SELECT `+reportColumns+` FROM reports WHERE id = ?`, id))
}

// ReportPages returns the stored pages of a report in page order.
func (db *DB) ReportPages(ctx context.Context, id int64) ([]ReportPage, error) {
	rows, err := db.sql.QueryContext(ctx, `SELECT page, kind, score, text_gz FROM report_pages WHERE report_id = ? ORDER BY page`, id)
	if err != nil {
		return nil, fmt.Errorf("query report pages %d: %w", id, err)
	}
	defer rows.Close()
	var out []ReportPage
	for rows.Next() {
		var p ReportPage
		var gz []byte
		if err := rows.Scan(&p.Page, &p.Kind, &p.Score, &gz); err != nil {
			return nil, fmt.Errorf("scan report page: %w", err)
		}
		if p.Text, err = gunzipBytes(gz); err != nil {
			return nil, fmt.Errorf("decompress report page %d/%d: %w", id, p.Page, err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// SetPageKinds records the statement classification of pages (page → kind, score).
func (t *Tx) SetPageKinds(ctx context.Context, id int64, kinds map[int]string, scores map[int]float64) error {
	for page, kind := range kinds {
		if err := t.exec(ctx, `UPDATE report_pages SET kind = ?, score = ? WHERE report_id = ? AND page = ?`,
			kind, scores[page], id, page); err != nil {
			return fmt.Errorf("set page kind %d/%d: %w", id, page, err)
		}
	}
	return nil
}

func gzipBytes(b []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(b); err != nil {
		return nil, fmt.Errorf("compress: %w", err)
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("compress: %w", err)
	}
	return buf.Bytes(), nil
}

func gunzipBytes(b []byte) ([]byte, error) {
	zr, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	return io.ReadAll(zr)
}
