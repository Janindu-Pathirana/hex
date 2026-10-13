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
