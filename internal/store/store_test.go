package store_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/Janindu-Pathirana/hex/internal/store"
)

func open(t *testing.T) *store.DB {
	t.Helper()
	db, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "hex.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func update(t *testing.T, db *store.DB, fn func(*store.Tx) error) {
	t.Helper()
	if err := db.Update(context.Background(), fn); err != nil {
		t.Fatal(err)
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "hex.db")
	for i := 0; i < 2; i++ {
		db, err := store.Open(ctx, path)
		if err != nil {
			t.Fatalf("open %d: %v", i, err)
		}
		v, err := db.SchemaVersion(ctx)
		if err != nil || v != 1 {
			t.Fatalf("schema version = %d, %v", v, err)
		}
		db.Close()
	}
}

func TestPathHonoursEnv(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(store.EnvDir, dir)
	p, err := store.Path()
	if err != nil || p != filepath.Join(dir, "hex.db") {
		t.Fatalf("Path = %q, %v", p, err)
	}
}

func TestSecurities(t *testing.T) {
	db := open(t)
	ctx := context.Background()
	update(t, db, func(tx *store.Tx) error {
		return tx.UpsertSecurities(ctx, []store.Security{
			{Code: "JKH.N0000", Ticker: "JKH", Series: "N0000", Name: "JOHN KEELLS HOLDINGS PLC"},
		})
	})
	update(t, db, func(tx *store.Tx) error {
		return tx.SetSecurityDetail(ctx, store.Security{Code: "JKH.N0000", Ticker: "JKH", Series: "N0000", StockID: 297, ISIN: "LK0092N00003"})
	})
	// A later rename from the symbol list keeps the detail.
	update(t, db, func(tx *store.Tx) error {
		return tx.UpsertSecurities(ctx, []store.Security{
			{Code: "JKH.N0000", Ticker: "JKH", Series: "N0000", Name: "JOHN KEELLS HOLDINGS PLC (NEW)"},
		})
	})
	s, err := db.Security(ctx, "JKH.N0000")
	if err != nil {
		t.Fatal(err)
	}
	if s.StockID != 297 || s.ISIN != "LK0092N00003" || s.Name != "JOHN KEELLS HOLDINGS PLC (NEW)" {
		t.Fatalf("security = %+v", s)
	}
	if _, err := db.Security(ctx, "NOPE.N0000"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("missing security: %v", err)
	}
}

func TestMergeBarsKeepsHistory(t *testing.T) {
	db := open(t)
	ctx := context.Background()
	o := 10.0
	update(t, db, func(tx *store.Tx) error {
		return tx.MergeBars(ctx, "JKH.N0000", []store.Bar{{Date: "2024-01-02", Close: 1}, {Date: "2024-01-03", Close: 2}})
	})
	update(t, db, func(tx *store.Tx) error {
		return tx.MergeBars(ctx, "JKH.N0000", []store.Bar{{Date: "2024-01-03", Close: 3, Open: &o}, {Date: "2024-01-04", Close: 4}})
	})
	bars, err := db.Bars(ctx, "JKH.N0000", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(bars) != 3 || bars[1].Close != 3 || bars[1].Open == nil || *bars[1].Open != 10 || bars[0].Open != nil {
		t.Fatalf("bars = %+v", bars)
	}
	if got, _ := db.Bars(ctx, "JKH.N0000", "2024-01-03", "2024-01-03"); len(got) != 1 {
		t.Fatalf("range filter returned %d bars", len(got))
	}
}

func TestFundamentalsPrecedence(t *testing.T) {
	db := open(t)
	ctx := context.Background()
	base := store.Fundamental{Code: "JKH.N0000", PeriodEnd: "2026-03-31", PeriodType: "12M", Scope: "group", Field: "eps_basic"}
	pdf, llm, manual := base, base, base
	pdf.Source, pdf.Value, pdf.Confidence = store.SourcePDF, 1.0, 0.6
	llm.Source, llm.Value, llm.Confidence = store.SourceLLM, 2.0, 0.9
	manual.Source, manual.Value, manual.Confidence = store.SourceManual, 3.0, 1

	update(t, db, func(tx *store.Tx) error { return tx.UpsertFundamentals(ctx, []store.Fundamental{pdf, llm}) })
	got, err := db.Fundamentals(ctx, "JKH.N0000", false)
	if err != nil || len(got) != 1 || got[0].Source != store.SourceLLM {
		t.Fatalf("best without manual = %+v, %v", got, err)
	}
	update(t, db, func(tx *store.Tx) error { return tx.UpsertFundamentals(ctx, []store.Fundamental{manual}) })
	got, _ = db.Fundamentals(ctx, "JKH.N0000", false)
	if len(got) != 1 || got[0].Source != store.SourceManual || got[0].Value != 3 {
		t.Fatalf("manual should win: %+v", got)
	}
	all, _ := db.Fundamentals(ctx, "JKH.N0000", true)
	if len(all) != 3 {
		t.Fatalf("all = %d rows, want 3", len(all))
	}
}

func TestReportsAndPages(t *testing.T) {
	db := open(t)
	ctx := context.Background()
	r := store.Report{ID: 51321, Code: "JKH.N0000", Kind: store.ReportAnnual, Title: "Annual Report", PeriodDate: "2026-03-31", Path: "a.pdf"}
	update(t, db, func(tx *store.Tx) error { return tx.UpsertReports(ctx, []store.Report{r}) })
	update(t, db, func(tx *store.Tx) error {
		if err := tx.SetReportFile(ctx, r.ID, "abc", 123); err != nil {
			return err
		}
		return tx.SetReportText(ctx, r.ID, 2, store.TextOK, "", []store.ReportPage{{Page: 1, Text: []byte(`{"lines":[]}`)}})
	})
	// Re-cataloguing keeps download and text state.
	update(t, db, func(tx *store.Tx) error { return tx.UpsertReports(ctx, []store.Report{r}) })
	got, err := db.Report(ctx, r.ID)
	if err != nil || got.SHA256 != "abc" || got.Pages != 2 || got.TextStatus != store.TextOK {
		t.Fatalf("report = %+v, %v", got, err)
	}
	pages, err := db.ReportPages(ctx, r.ID)
	if err != nil || len(pages) != 1 || string(pages[0].Text) != `{"lines":[]}` || pages[0].Kind != "other" {
		t.Fatalf("pages = %+v, %v", pages, err)
	}
}

func TestRawKeepsLastN(t *testing.T) {
	db := open(t)
	ctx := context.Background()
	now := time.Unix(1_700_000_000, 0)
	for i := 0; i < store.RawKeep+2; i++ {
		db.SetClock(func() time.Time { return now.Add(time.Duration(i) * time.Second) })
		body := []byte{byte('a' + i)}
		update(t, db, func(tx *store.Tx) error { return tx.SaveRaw(ctx, "cse", "tradeSummary", "", body) })
	}
	body, _, err := db.LatestRaw(ctx, "cse", "tradeSummary", "")
	if err != nil || string(body) != "e" {
		t.Fatalf("latest = %q, %v", body, err)
	}
	n, err := db.ClearRaw(ctx)
	if err != nil || n != store.RawKeep {
		t.Fatalf("cleared %d rows, want %d (%v)", n, store.RawKeep, err)
	}
}

func TestJobStates(t *testing.T) {
	db := open(t)
	ctx := context.Background()
	if _, found, err := db.JobState(ctx, "quotes"); found || err != nil {
		t.Fatalf("unexpected job state: %v %v", found, err)
	}
	update(t, db, func(tx *store.Tx) error {
		return tx.PutJobState(ctx, store.JobState{Key: "quotes", Kind: "quotes", Status: store.JobRunning, Attempts: 1})
	})
	if err := db.ResetRunning(ctx); err != nil {
		t.Fatal(err)
	}
	s, found, err := db.JobState(ctx, "quotes")
	if err != nil || !found || s.Status != store.JobPending || s.Attempts != 1 {
		t.Fatalf("state = %+v %v %v", s, found, err)
	}
}

func TestUpdateRollsBack(t *testing.T) {
	db := open(t)
	ctx := context.Background()
	boom := errors.New("boom")
	err := db.Update(ctx, func(tx *store.Tx) error {
		if err := tx.SetMeta(ctx, "k", "v"); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
	if _, _, err := db.Meta(ctx, "k"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("meta should be rolled back: %v", err)
	}
}
