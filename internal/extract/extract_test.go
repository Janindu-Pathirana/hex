package extract_test

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Janindu-Pathirana/hex/internal/extract"
	"github.com/Janindu-Pathirana/hex/internal/pdfx"
)

// fixture is the statement pages of one real CSE report, as extracted text.
type fixture struct {
	Title string       `json:"title"`
	Meta  extract.Meta `json:"meta"`
	Pages []pdfx.Page  `json:"pages"`
}

func wantKey(v extract.Value) string {
	return strings.Join([]string{v.Scope, v.PeriodType, v.PeriodEnd.Format("2006-01-02"), v.Field}, "|")
}

// keyFields were checked by hand against the PDFs; the rest of each want file
// guards against regressions.
var keyFields = map[string]bool{
	"revenue": true, "gross_income": true, "gwp": true, "pat": true, "pat_parent": true,
	"eps_basic": true, "total_assets": true, "equity_parent": true, "nav_per_share": true,
}

func TestGoldenReports(t *testing.T) {
	files, _ := filepath.Glob(filepath.Join("..", "..", "testdata", "extract", "*.want.json"))
	if len(files) == 0 {
		t.Fatal("no golden fixtures")
	}
	totalKey, okKey := 0, 0
	for _, wf := range files {
		name := strings.TrimSuffix(filepath.Base(wf), ".want.json")
		t.Run(name, func(t *testing.T) {
			var fx fixture
			var want map[string]float64
			readJSON(t, strings.TrimSuffix(wf, ".want.json")+".json", &fx)
			readJSON(t, wf, &want)

			res := extract.Extract(fx.Pages, fx.Meta)
			got := map[string]float64{}
			for _, v := range res.Values {
				got[wantKey(v)] = v.Value
			}
			var keys []string
			for k := range want {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				field := k[strings.LastIndex(k, "|")+1:]
				g, ok := got[k]
				match := ok && math.Abs(g-want[k]) <= math.Abs(want[k])*0.005+1e-9
				if keyFields[field] {
					totalKey++
					if match {
						okKey++
					}
				}
				if !match {
					t.Errorf("%s: got %v (found %v), want %v", k, g, ok, want[k])
				}
			}
			if res.Confidence < 0.5 {
				t.Errorf("confidence %.2f < 0.5: %v", res.Confidence, res.Issues)
			}
		})
	}
	t.Logf("key fields: %d/%d match", okKey, totalKey)
}

func readJSON(t *testing.T, path string, v any) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatal(err)
	}
}

func TestPeriodEnd(t *testing.T) {
	for title, want := range map[string]string{
		"Interim Financial Statements for the Quarter ended 30th June 2026":     "2026-06-30",
		"Annual Report as at 31st March 2026":                                   "2026-03-31",
		"Annual Report 2024/25":                                                 "2025-03-31",
		"Commercial Bank of Ceylon PLC - Annual Report 2024":                    "2024-12-31",
		"Interim Financial Statements for the Quarter ended 31st December 2025": "2025-12-31",
	} {
		got, ok := extract.PeriodEnd(title)
		if !ok || got.Format("2006-01-02") != want {
			t.Errorf("PeriodEnd(%q) = %v, %v; want %s", title, got, ok, want)
		}
	}
	if _, ok := extract.PeriodEnd("Circular to shareholders"); ok {
		t.Error("undated title should fail")
	}
}

func TestFYEndMonth(t *testing.T) {
	if extract.FYEndMonth("1") != time.March || extract.FYEndMonth("2") != time.December || extract.FYEndMonth("") != 0 {
		t.Fatal("FYEndMonth mapping changed")
	}
}
