// Package reportjobs holds the scrape-engine jobs that download financial
// report PDFs, extract their text and turn statements into fundamentals.
package reportjobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Janindu-Pathirana/hex/internal/engine"
	"github.com/Janindu-Pathirana/hex/internal/pdfx"
	"github.com/Janindu-Pathirana/hex/internal/store"
)

// Job kinds.
const (
	KindPDFs         = "pdfs"
	KindFundamentals = "fundamentals"
)

// Options select which reports are processed.
type Options struct {
	Quarterly int  // newest quarterly (interim) reports per company
	Annual    int  // newest annual reports per company (large files)
	Reparse   bool // re-extract even if already processed (uses cached PDFs)
	Backend   pdfx.Backend
	Limits    pdfx.Limits
}

// DefaultOptions take the last four interim reports, which carry both the
// quarter and the year-to-date (12M in Q4) figures, and no annual reports.
var DefaultOptions = Options{Quarterly: 4, Limits: pdfx.DefaultLimits}

// Select returns the newest annual and quarterly reports allowed by o.
func (o Options) Select(reports []store.Report) []store.Report {
	var out []store.Report
	nq, na := 0, 0
	for _, r := range reports { // newest period first
		switch {
		case r.Kind == store.ReportQuarterly && nq < o.Quarterly:
			nq++
			out = append(out, r)
		case r.Kind == store.ReportAnnual && na < o.Annual:
			na++
			out = append(out, r)
		}
	}
	return out
}

// PDFs downloads the selected reports of code (unless cached) and stores
// their text page by page.
func PDFs(code string, o Options) engine.Job {
	ttl := 24 * time.Hour
	if o.Reparse {
		ttl = 0
	}
	return engine.FuncJob{K: KindPDFs + ":" + code, Kd: KindPDFs, T: ttl, F: func(ctx context.Context, env *engine.Env) error {
		if env.Files == nil {
			return errors.New("no pdf store configured")
		}
		reports, err := env.Store.Reports(ctx, code)
		if err != nil {
			return err
		}
		var errs []error
		for _, r := range o.Select(reports) {
			if err := ctx.Err(); err != nil {
				return err
			}
			if err := ProcessPDF(ctx, env, r, o); err != nil {
				errs = append(errs, fmt.Errorf("report %d (%s): %w", r.ID, r.Title, err))
			}
		}
		return errors.Join(errs...)
	}}
}

// ProcessPDF downloads one report (unless cached) and stores its text. It
// does nothing when the text is already stored, unless o.Reparse is set.
func ProcessPDF(ctx context.Context, env *engine.Env, r store.Report, o Options) error {
	haveText := r.TextStatus == store.TextOK || r.TextStatus == store.TextScanned
	cached := r.SHA256 != "" && env.Files.Has(r.SHA256)
	if haveText && !o.Reparse {
		return nil
	}

	var data []byte
	var err error
	if cached {
		data, err = env.Files.Get(r.SHA256)
	} else {
		max := o.Limits.MaxBytes
		if max <= 0 {
			max = pdfx.DefaultLimits.MaxBytes
		}
		data, err = env.CSE.DownloadPDF(ctx, r.Path, max)
		if err == nil {
			var sum string
			if sum, err = env.Files.Put(data); err == nil {
				r.SHA256 = sum
				err = env.Store.Update(ctx, func(tx *store.Tx) error { return tx.SetReportFile(ctx, r.ID, sum, int64(len(data))) })
			}
		}
	}
	if err != nil {
		return err
	}

	backend := o.Backend
	if backend == nil {
		backend = pdfx.DefaultBackend()
	}
	res, xerr := pdfx.Extract(ctx, backend, data, o.Limits)
	if xerr != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	status, msg := store.TextOK, ""
	var pages []store.ReportPage
	switch {
	case xerr != nil:
		status, msg = store.TextFailed, xerr.Error()
	case res.Scanned:
		status = store.TextScanned
	}
	for _, p := range res.Pages {
		b, err := json.Marshal(p)
		if err != nil {
			return fmt.Errorf("encode page %d: %w", p.N, err)
		}
		pages = append(pages, store.ReportPage{Page: p.N, Text: b})
	}
	if err := env.Store.Update(ctx, func(tx *store.Tx) error {
		return tx.SetReportText(ctx, r.ID, len(res.Pages), status, msg, pages)
	}); err != nil {
		return err
	}
	if xerr != nil {
		return fmt.Errorf("extract text: %w", xerr)
	}
	if res.Scanned {
		fmt.Fprintf(env.Log, "warning: %s report %d looks scanned (no text layer)\n", r.Code, r.ID)
	}
	return nil
}

// LoadPages decodes a report's stored pages.
func LoadPages(ctx context.Context, db *store.DB, reportID int64) ([]pdfx.Page, error) {
	rows, err := db.ReportPages(ctx, reportID)
	if err != nil {
		return nil, err
	}
	pages := make([]pdfx.Page, 0, len(rows))
	for _, r := range rows {
		var p pdfx.Page
		if err := json.Unmarshal(r.Text, &p); err != nil {
			return nil, fmt.Errorf("decode page %d of report %d: %w", r.Page, reportID, err)
		}
		pages = append(pages, p)
	}
	return pages, nil
}
