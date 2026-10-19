package cmd

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Janindu-Pathirana/hex/internal/cache"
	"github.com/Janindu-Pathirana/hex/internal/csetest"
	"github.com/Janindu-Pathirana/hex/internal/store"
)

// seedAnalysis stores a small March-year-end company for JKH.N0000. The
// values match internal/analysis's fixture: TTM revenue 1,100, profit 120.
func seedAnalysis(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	db, err := store.OpenDefault(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	row := func(end, typ, field string, v float64) store.Fundamental {
		return store.Fundamental{Code: "JKH.N0000", PeriodEnd: end, PeriodType: typ, Scope: "group", Field: field,
			Source: store.SourcePDF, Value: v, Confidence: 0.9}
	}
	err = db.Update(ctx, func(tx *store.Tx) error {
		if err := tx.UpsertSecurities(ctx, []store.Security{{Code: "JKH.N0000", Ticker: "JKH", Series: "N0000", Name: "John Keells Holdings PLC"}}); err != nil {
			return err
		}
		if err := tx.SetSecurityDetail(ctx, store.Security{Code: "JKH.N0000", StockID: 1, Issued: 100}); err != nil {
			return err
		}
		if err := tx.UpsertQuotes(ctx, []store.Quote{{Code: "JKH.N0000", Price: 60}}); err != nil {
			return err
		}
		// 30 daily bars ending 2026-01-14: a zigzag drifting up, 4 days short of
		// the history SMA 50 and the one-year figures need.
		var bars []store.Bar
		var idx []store.IndexBar
		start := time.Date(2025, 12, 16, 0, 0, 0, 0, time.UTC)
		for i := 0; i < 30; i++ {
			d := start.AddDate(0, 0, i).Format("2006-01-02")
			c := 55 + float64(i)/6 + float64(i%3)
			bars = append(bars, store.Bar{Date: d, Close: c, Volume: 1000})
			idx = append(idx, store.IndexBar{IndexID: "ASPI", Date: d, Value: 20000 + 10*float64(i)})
		}
		if err := tx.MergeBars(ctx, "JKH.N0000", bars); err != nil {
			return err
		}
		if err := tx.UpsertIndexBars(ctx, idx); err != nil {
			return err
		}
		return tx.UpsertFundamentals(ctx, []store.Fundamental{
			row("2025-03-31", "12M", "revenue", 1000), row("2025-12-31", "9M", "revenue", 800), row("2024-12-31", "9M", "revenue", 700),
			row("2025-03-31", "12M", "pat_parent", 100), row("2025-12-31", "9M", "pat_parent", 90), row("2024-12-31", "9M", "pat_parent", 70),
			row("2025-03-31", "12M", "eps_basic", 10), row("2025-12-31", "9M", "eps_basic", 9), row("2024-12-31", "9M", "eps_basic", 7),
			row("2025-12-31", "PIT", "equity_parent", 1200), row("2024-12-31", "PIT", "equity_parent", 1000),
			row("2025-12-31", "PIT", "total_assets", 3000), row("2024-12-31", "PIT", "total_assets", 2500),
		})
	})
	if err != nil {
		t.Fatal(err)
	}
}

func runAnalyze(t *testing.T, seed bool, args ...string) (string, error) {
	t.Helper()
	t.Setenv(cache.EnvDir, t.TempDir())
	t.Setenv(store.EnvDir, t.TempDir())
	if seed {
		seedAnalysis(t)
	}
	srv, client := csetest.New(t)
	_ = srv
	var out strings.Builder
	root := newRootCmd(&app{cse: client, rps: 1000, now: func() time.Time { return time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC) }})
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), err
}

func TestAnalyzeText(t *testing.T) {
	out, err := runAnalyze(t, true, "analyze", "jkh")
	if err != nil {
		t.Fatal(err)
	}
	golden(t, "analyze-jkh", out)
}

func TestAnalyzeJSONAndSection(t *testing.T) {
	out, err := runAnalyze(t, true, "analyze", "jkh", "--json", "--section", "valuation")
	if err != nil {
		t.Fatal(err)
	}
	var rep struct {
		Metrics    []struct{ Group string }
		Findings   []struct{ Dimension string }
		Disclaimer string
	}
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("bad JSON: %v\n%s", err, out)
	}
	if len(rep.Metrics) == 0 || rep.Disclaimer == "" {
		t.Fatalf("empty report: %s", out)
	}
	for _, m := range rep.Metrics {
		if m.Group != "valuation" {
			t.Errorf("metric group %q leaked through --section valuation", m.Group)
		}
	}
	for _, f := range rep.Findings {
		if f.Dimension != "valuation" && f.Dimension != "data quality" {
			t.Errorf("finding dimension %q leaked through --section valuation", f.Dimension)
		}
	}
}

func TestAnalyzeUnknownSection(t *testing.T) {
	if _, err := runAnalyze(t, true, "analyze", "jkh", "--section", "vibes"); err == nil || !strings.Contains(err.Error(), "unknown section") {
		t.Fatalf("got %v", err)
	}
}

func TestAnalyzeWithoutData(t *testing.T) {
	_, err := runAnalyze(t, false, "analyze", "jkh")
	if err == nil || !strings.Contains(err.Error(), "hex sync") {
		t.Fatalf("got %v, want a hint to run hex sync", err)
	}
}
