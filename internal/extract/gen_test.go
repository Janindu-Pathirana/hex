package extract_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Janindu-Pathirana/hex/internal/extract"
	"github.com/Janindu-Pathirana/hex/internal/pdfx"
	"github.com/Janindu-Pathirana/hex/internal/sources/reportjobs"
	"github.com/Janindu-Pathirana/hex/internal/store"
)

// TestGenGolden writes statement-page fixtures from a spike database
// (SPIKE_DB). It is not part of the normal run.
func TestGenGolden(t *testing.T) {
	path := os.Getenv("SPIKE_DB")
	if path == "" || os.Getenv("GEN_GOLDEN") == "" {
		t.Skip()
	}
	ctx := context.Background()
	db, err := store.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, id := range []int64{52111, 51319, 52504, 52411, 52587, 52370, 52517, 52264, 52684, 52628, 52119} {
		r, err := db.Report(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		pages, _ := reportjobs.LoadPages(ctx, db, id)
		sec, _ := db.Security(ctx, r.Code)
		prof, _, _ := db.Profile(ctx, r.Code)
		end, _ := extract.PeriodEnd(r.Title)
		meta := extract.Meta{Annual: r.Kind == store.ReportAnnual, PeriodEnd: end, Shares: sec.Issued, FYEnd: extract.FYEndMonth(prof.FYEnd)}
		res := extract.Extract(pages, meta)
		var keep []pdfx.Page
		for _, p := range pages {
			if _, ok := res.Pages[p.N]; ok {
				keep = append(keep, p)
			}
		}
		name := fmt.Sprintf("%s_%d", strings.Split(r.Code, ".")[0], id)
		writeJSON(t, filepath.Join("..", "..", "testdata", "extract", name+".json"), fixture{Title: r.Title, Meta: meta, Pages: keep})
		want := map[string]float64{}
		for _, v := range res.Values {
			want[wantKey(v)] = v.Value
		}
		writeJSON(t, filepath.Join("..", "..", "testdata", "extract", name+".want.json"), want)
	}
}

func writeJSON(t *testing.T, path string, v any) {
	var b []byte
	var err error
	if _, isWant := v.(map[string]float64); isWant {
		b, err = json.MarshalIndent(v, "", " ")
	} else {
		b, err = json.Marshal(v) // page fixtures: compact
	}
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}
