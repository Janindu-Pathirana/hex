package reportjobs_test

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Janindu-Pathirana/hex/internal/csetest"
	"github.com/Janindu-Pathirana/hex/internal/engine"
	"github.com/Janindu-Pathirana/hex/internal/extract"
	"github.com/Janindu-Pathirana/hex/internal/pdfx"
	"github.com/Janindu-Pathirana/hex/internal/pdfx/pdftest"
	"github.com/Janindu-Pathirana/hex/internal/sources/csejobs"
	"github.com/Janindu-Pathirana/hex/internal/sources/reportjobs"
	"github.com/Janindu-Pathirana/hex/internal/store"
)

const (
	q1Path = "/cdn/cmt/upload_report_file/508_1785229278745.pdf"
	q2Path = "/cdn/cmt/upload_report_file/508_1779789322176.pdf"
)

// statementPDF is a tiny interim report with a two-column income statement.
func statementPDF(revenue string) []byte {
	return pdftest.Make(
		[]pdftest.Text{{X: 50, Y: 800, S: "Interim Condensed Financial Statements"}},
		[]pdftest.Text{
			{X: 50, Y: 800, S: "CONSOLIDATED INCOME STATEMENT"},
			{X: 380, Y: 780, S: "2026"}, {X: 450, Y: 780, S: "2025"},
			{X: 50, Y: 760, S: "Revenue"}, {X: 360, Y: 760, S: revenue}, {X: 430, Y: 760, S: "108,712,150"},
			{X: 50, Y: 740, S: "Note: All values are in Rs '000s, unless otherwise stated."},
		},
	)
}

type fixture struct {
	srv  *csetest.Server
	env  *engine.Env
	hits atomic.Int32
}

func setup(t *testing.T) *fixture {
	t.Helper()
	srv, c := csetest.New(t)
	db, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "hex.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	f := &fixture{srv: srv, env: &engine.Env{CSE: c, Store: db, Files: &pdfx.Blobs{Dir: t.TempDir()}, Log: io.Discard}}
	serve := func(body []byte) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) { f.hits.Add(1); _, _ = w.Write(body) }
	}
	srv.Override(q1Path, serve(statementPDF("135,443,355")))
	srv.Override(q2Path, serve([]byte("<html>gone</html>")))

	// Catalogue JKH's reports first.
	if _, err := engine.Run(context.Background(), f.env, csejobs.Plan(csejobs.Scope{Reports: true}, []string{"JKH.N0000"}, time.Now()), engine.Options{}); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestPDFsDownloadsAndExtracts(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	o := reportjobs.Options{Quarterly: 2, Backend: pdfx.Pure{}, Limits: pdfx.DefaultLimits}

	sum, _ := engine.Run(ctx, f.env, []engine.Stage{{reportjobs.PDFs("JKH.N0000", o)}}, engine.Options{})
	// The second quarterly "PDF" is an HTML page, so the job reports a failure,
	// but the first report is still processed.
	if sum.Count(engine.Failed) != 1 {
		t.Fatalf("summary: %v", sum.Err())
	}
	reports, _ := f.env.Store.Reports(ctx, "JKH.N0000")
	var got store.Report
	for _, r := range reports {
		if r.ID == 52111 {
			got = r
		}
	}
	if got.TextStatus != store.TextOK || got.Pages != 2 || got.SHA256 == "" || !f.env.Files.Has(got.SHA256) {
		t.Fatalf("report = %+v", got)
	}
	pages, err := reportjobs.LoadPages(ctx, f.env.Store, got.ID)
	if err != nil || len(pages) != 2 || pages[1].Lines[2].Text() != "Revenue 135,443,355 108,712,150" {
		t.Fatalf("pages = %+v, %v", pages, err)
	}

	// Re-parsing works from the cached PDF without downloading it again.
	before := f.hits.Load()
	o.Reparse, o.Quarterly = true, 1
	sum, _ = engine.Run(ctx, f.env, []engine.Stage{{reportjobs.PDFs("JKH.N0000", o)}}, engine.Options{})
	if sum.Count(engine.Ran) != 1 || f.hits.Load() != before {
		t.Fatalf("reparse: ran %d, downloads %d: %v", sum.Count(engine.Ran), f.hits.Load()-before, sum.Err())
	}
}

func TestSelect(t *testing.T) {
	rs := []store.Report{
		{ID: 1, Kind: store.ReportQuarterly}, {ID: 2, Kind: store.ReportAnnual}, {ID: 3, Kind: store.ReportQuarterly},
		{ID: 4, Kind: store.ReportOther}, {ID: 5, Kind: store.ReportQuarterly},
	}
	got := reportjobs.Options{Quarterly: 2, Annual: 1}.Select(rs)
	if len(got) != 3 || got[0].ID != 1 || got[1].ID != 2 || got[2].ID != 3 {
		t.Fatalf("select = %+v", got)
	}
}

func TestFundamentalsFromStoredText(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	o := reportjobs.Options{Quarterly: 1, Backend: pdfx.Pure{}, Limits: pdfx.DefaultLimits}
	stages := []engine.Stage{{reportjobs.PDFs("JKH.N0000", o)}, {reportjobs.Fundamentals("JKH.N0000", o, nil)}}
	if sum, err := engine.Run(ctx, f.env, stages, engine.Options{}); err != nil || sum.Err() != nil {
		t.Fatalf("run: %v %v", err, sum.Err())
	}
	fs, err := f.env.Store.Fundamentals(ctx, "JKH.N0000", false)
	if err != nil || len(fs) != 1 {
		t.Fatalf("fundamentals = %+v, %v", fs, err)
	}
	got := fs[0]
	if got.Field != "revenue" || got.Value != 135_443_355_000 || got.PeriodEnd != "2026-06-30" || got.PeriodType != "3M" ||
		got.Source != store.SourcePDF || got.ReportID != 52111 || got.Page != 2 {
		t.Fatalf("value = %+v", got)
	}
	runs, _ := f.env.Store.Runs(ctx, 52111)
	if len(runs) != 1 || runs[0].Method != "rules" {
		t.Fatalf("runs = %+v", runs)
	}
	r, _ := f.env.Store.Report(ctx, 52111)
	if r.ParsedAt.IsZero() {
		t.Fatal("report not marked parsed")
	}

	// A manual override wins over the extracted value.
	manual := got
	manual.Source, manual.Value, manual.Confidence = store.SourceManual, 1, 1
	f.env.Store.Update(ctx, func(tx *store.Tx) error { return tx.UpsertFundamentals(ctx, []store.Fundamental{manual}) })
	fs, _ = f.env.Store.Fundamentals(ctx, "JKH.N0000", false)
	if fs[0].Source != store.SourceManual {
		t.Fatalf("manual should win: %+v", fs[0])
	}
}

type fakeFallback struct{ calls int }

func (f *fakeFallback) Name() string { return "fake-model" }
func (f *fakeFallback) Extract(_ context.Context, _ *engine.Env, r store.Report, meta extract.Meta, rules extract.Result) (extract.Result, error) {
	f.calls++
	v := extract.Value{Field: "pat", Statement: extract.IncomeStatement, Scope: extract.ScopeGroup, PeriodEnd: meta.PeriodEnd,
		PeriodType: "3M", Value: 42, Page: 2, Raw: "42"}
	return extract.Finish([]extract.Value{v}, meta, nil), nil
}

func TestLowConfidenceGoesToFallback(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	o := reportjobs.Options{Quarterly: 1, Backend: pdfx.Pure{}, Limits: pdfx.DefaultLimits}
	fb := &fakeFallback{}
	stages := []engine.Stage{{reportjobs.PDFs("JKH.N0000", o)}, {reportjobs.Fundamentals("JKH.N0000", o, fb)}}
	if sum, err := engine.Run(ctx, f.env, stages, engine.Options{}); err != nil || sum.Err() != nil {
		t.Fatalf("run: %v %v", err, sum.Err())
	}
	if fb.calls != 1 {
		t.Fatalf("fallback called %d times, want 1 (one value scores below %.1f)", fb.calls, reportjobs.LowConfidence)
	}
	all, _ := f.env.Store.Fundamentals(ctx, "JKH.N0000", true)
	sources := map[string]bool{}
	for _, v := range all {
		sources[v.Source] = true
	}
	if !sources[store.SourcePDF] || !sources[store.SourceLLM] {
		t.Fatalf("sources = %v", sources)
	}
	runs, _ := f.env.Store.Runs(ctx, 52111)
	if len(runs) != 2 || runs[0].Method != "llm" || runs[0].Model != "fake-model" {
		t.Fatalf("runs = %+v", runs)
	}
}
