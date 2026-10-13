package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/Janindu-Pathirana/hex/internal/cache"
	"github.com/Janindu-Pathirana/hex/internal/csetest"
	"github.com/Janindu-Pathirana/hex/internal/pdfx/pdftest"
	"github.com/Janindu-Pathirana/hex/internal/store"
)

func statementPDF() []byte {
	return pdftest.Make(
		[]pdftest.Text{{X: 50, Y: 800, S: "Interim Condensed Financial Statements"}},
		[]pdftest.Text{
			{X: 50, Y: 800, S: "CONSOLIDATED INCOME STATEMENT"},
			{X: 380, Y: 780, S: "2026"}, {X: 450, Y: 780, S: "2025"},
			{X: 50, Y: 760, S: "Total revenue"}, {X: 360, Y: 760, S: "141,647,460"}, {X: 430, Y: 760, S: "114,153,194"},
			{X: 50, Y: 740, S: "Cost of sales"}, {X: 356, Y: 740, S: "(117,062,877)"}, {X: 426, Y: 740, S: "(94,018,750)"},
			{X: 50, Y: 720, S: "Gross profit"}, {X: 364, Y: 720, S: "24,584,583"}, {X: 434, Y: 720, S: "20,134,444"},
			{X: 50, Y: 700, S: "Note: All values are in Rs '000s, unless otherwise stated."},
		},
	)
}

func TestReportsExtractListAndFundamentals(t *testing.T) {
	t.Setenv(cache.EnvDir, t.TempDir())
	t.Setenv(store.EnvDir, t.TempDir())
	srv, client := csetest.New(t)
	pdf := statementPDF()
	srv.Override("/cdn/cmt/upload_report_file/508_1785229278745.pdf", func(w http.ResponseWriter, r *http.Request) { w.Write(pdf) })

	out, _, err := runWith(t, srv, client, "reports", "extract", "jkh")
	if err != nil {
		t.Fatalf("extract: %v\n%s", err, out)
	}
	var ev extractView
	if err := json.Unmarshal([]byte(out[strings.Index(out, "{"):]), &ev); err != nil {
		t.Fatalf("decode: %v\n%s", err, out)
	}
	if ev.Report.ID != 52111 || len(ev.Values) != 3 || ev.Confidence < 0.6 {
		t.Fatalf("extract view = %+v", ev)
	}

	out, _, err = runWith(t, srv, client, "reports", "list", "JKH")
	if err != nil || !strings.Contains(out, `"text": "ok"`) || !strings.Contains(out, `"method": "rules"`) {
		t.Fatalf("list: %v\n%s", err, out)
	}

	out, _, err = runWith(t, srv, client, "fundamentals", "set", "jkh", "--period", "2026-06-30", "--field", "revenue=1")
	if err != nil {
		t.Fatalf("set: %v\n%s", err, out)
	}
	out, _, err = runWith(t, srv, client, "fundamentals", "show", "jkh")
	if err != nil {
		t.Fatalf("show: %v\n%s", err, out)
	}
	var shown struct {
		Values     []fundamentalView `json:"values"`
		Disclaimer string            `json:"disclaimer"`
	}
	json.Unmarshal([]byte(out), &shown)
	var rev fundamentalView
	for _, v := range shown.Values {
		if v.Field == "revenue" {
			rev = v
		}
	}
	if rev.Source != store.SourceManual || rev.Value != 1 || shown.Disclaimer == "" {
		t.Fatalf("manual value should win: %+v\n%s", rev, out)
	}
}

func TestFundamentalsSetValidates(t *testing.T) {
	for _, args := range [][]string{
		{"fundamentals", "set", "jkh", "--period", "30/06/2026", "--field", "revenue=1"},
		{"fundamentals", "set", "jkh", "--period", "2026-06-30", "--field", "nonsense=1"},
		{"fundamentals", "set", "jkh", "--period", "2026-06-30"},
	} {
		if _, _, err := run(t, args...); err == nil {
			t.Errorf("%v: expected an error", args)
		}
	}
}
