package csejobs_test

import (
	"context"
	"io"
	"path/filepath"
	"testing"
	"time"

	"github.com/Janindu-Pathirana/hex/internal/cse"
	"github.com/Janindu-Pathirana/hex/internal/csetest"
	"github.com/Janindu-Pathirana/hex/internal/engine"
	"github.com/Janindu-Pathirana/hex/internal/output"
	"github.com/Janindu-Pathirana/hex/internal/sources/csejobs"
	"github.com/Janindu-Pathirana/hex/internal/store"
)

var all = csejobs.Scope{Securities: true, Market: true, Quotes: true, Info: true, History: true, Profiles: true, Reports: true, Actions: true}

func setup(t *testing.T) (*csetest.Server, *engine.Env) {
	t.Helper()
	srv, c := csetest.New(t)
	db, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "hex.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	engine.RecordRaw(c, db, io.Discard)
	return srv, &engine.Env{CSE: engine.Limit(c, engine.NewLimiter(1000, 4)), Store: db, Log: io.Discard}
}

func TestFullSync(t *testing.T) {
	srv, env := setup(t)
	ctx := context.Background()
	codes := []string{"JKH.N0000", "COMB.N0000"}
	sum, err := engine.Run(ctx, env, csejobs.Plan(all, codes, time.Now()), engine.Options{})
	if err != nil || sum.Err() != nil {
		t.Fatalf("run: %v / %v", err, sum.Err())
	}
	db := env.Store

	if secs, _ := db.Securities(ctx); len(secs) != 5 {
		t.Fatalf("securities = %d, want 5", len(secs))
	}
	jkh, _ := db.Security(ctx, "JKH.N0000")
	if jkh.StockID != 297 || jkh.ISIN != "LK0092N00003" || jkh.Issued == 0 {
		t.Fatalf("JKH = %+v", jkh)
	}
	if q, err := db.Quote(ctx, "JKH.N0000"); err != nil || q.Price <= 0 {
		t.Fatalf("quote = %+v, %v", q, err)
	}
	if st, err := db.QuoteStats(ctx, "JKH.N0000"); err != nil || st.BetaASPI == nil || st.High52W == 0 {
		t.Fatalf("stats = %+v, %v", st, err)
	}
	if bars, _ := db.Bars(ctx, "JKH.N0000", "", ""); len(bars) != 5 {
		t.Fatalf("bars = %d, want 5", len(bars))
	}
	if p, ds, err := db.Profile(ctx, "COMB.N0000"); err != nil || p.Sector != "Banks" || len(ds) == 0 {
		t.Fatalf("profile = %+v %d, %v", p, len(ds), err)
	}
	if rs, _ := db.Reports(ctx, "JKH.N0000"); len(rs) != 6 {
		t.Fatalf("JKH reports = %d", len(rs))
	}
	if days, _ := db.MarketDays(ctx); len(days) != 2 || days[1].PER == 0 {
		t.Fatalf("market days = %+v", days)
	}
	if aspi, _ := db.IndexBars(ctx, "ASPI"); len(aspi) != 1 {
		t.Fatalf("aspi bars = %d", len(aspi))
	}
	if acts, _ := db.Actions(ctx, ""); len(acts) == 0 {
		t.Fatal("no corporate actions")
	}
	if st, _, err := db.Meta(ctx, "market_status"); err != nil || st == "" {
		t.Fatalf("market status %q %v", st, err)
	}
	if body, _, err := db.LatestRaw(ctx, "cse", "tradeSummary", ""); err != nil || len(body) == 0 {
		t.Fatalf("raw tradeSummary missing: %v", err)
	}

	// A second run inside the TTLs makes no requests.
	before := srv.Hits.Load()
	sum, _ = engine.Run(ctx, env, csejobs.Plan(all, codes, time.Now()), engine.Options{})
	if srv.Hits.Load() != before || sum.Count(engine.Ran) != 0 {
		t.Fatalf("second run made %d requests, ran %d", srv.Hits.Load()-before, sum.Count(engine.Ran))
	}
}

func TestUnknownSymbolFailsOnlyItsJobs(t *testing.T) {
	_, env := setup(t)
	scope := csejobs.Scope{Info: true, Reports: true}
	sum, _ := engine.Run(context.Background(), env, csejobs.Plan(scope, []string{"JKH.N0000", "NOPE.N0000"}, time.Now()), engine.Options{})
	// info:NOPE fails ({}), reports:NOPE has no data and succeeds empty.
	if sum.Count(engine.Failed) != 1 || sum.Count(engine.Ran) != 3 {
		t.Fatalf("ran=%d failed=%d: %v", sum.Count(engine.Ran), sum.Count(engine.Failed), sum.Err())
	}
}

func TestHistoryNeedsStockID(t *testing.T) {
	_, env := setup(t)
	sum, _ := engine.Run(context.Background(), env, []engine.Stage{{csejobs.History("JKH.N0000")}}, engine.Options{})
	if sum.Count(engine.Failed) != 1 {
		t.Fatalf("history without stockId should fail: %+v", sum)
	}
}

func TestPlanStagesAndTTL(t *testing.T) {
	open := time.Date(2026, 10, 13, 10, 0, 0, 0, output.Colombo)   // Tuesday
	closed := time.Date(2026, 10, 11, 10, 0, 0, 0, output.Colombo) // Sunday
	stages := csejobs.Plan(all, []string{"JKH.N0000"}, open)
	if len(stages) != 3 || len(stages[0]) != 3 || len(stages[2]) != 1 {
		t.Fatalf("stages = %d", len(stages))
	}
	if stages[0][2].TTL() != 5*time.Minute {
		t.Fatalf("quotes TTL in market hours = %v", stages[0][2].TTL())
	}
	if csejobs.Plan(all, nil, closed)[0][2].TTL() != 12*time.Hour {
		t.Fatal("quotes TTL when closed should be 12h")
	}
	if !csejobs.MarketOpen(open) || csejobs.MarketOpen(closed) {
		t.Fatal("MarketOpen wrong")
	}
}

func TestBarRowsDedupesByColomboDate(t *testing.T) {
	day := time.Date(2026, 10, 12, 0, 0, 0, 0, output.Colombo).UnixMilli()
	rows := csejobs.BarRows([]cse.Bar{{T: day, Close: 1}, {T: day + 3600_000, Close: 2}, {T: day + 86400_000, Close: 3}})
	if len(rows) != 2 || rows[0].Date != "2026-10-12" || rows[0].Close != 2 {
		t.Fatalf("rows = %+v", rows)
	}
}

func TestActionRows(t *testing.T) {
	secs := []store.Security{
		{Code: "CIC.X0000", Series: "X0000", Name: "C I C HOLDINGS PLC"},
		{Code: "CIC.N0000", Series: "N0000", Name: "C I C HOLDINGS PLC"},
	}
	rows := csejobs.ActionRows([]cse.Announcement{
		{ID: 1, Date: "09 Oct 2026", Category: "CASH DIVIDEND", Company: "C I C  HOLDINGS PLC"},
		{ID: 2, Date: "06 Oct 2026", Category: "RIGHTS ISSUE (DATES)", Company: "UNKNOWN PLC"},
	}, secs)
	if rows[0].Code != "CIC.N0000" || rows[0].Kind != store.ActionDividend || rows[0].Announced != "2026-10-09" {
		t.Fatalf("row 0 = %+v", rows[0])
	}
	if rows[1].Code != "" || rows[1].Kind != store.ActionRights {
		t.Fatalf("row 1 = %+v", rows[1])
	}
}

func TestActionKind(t *testing.T) {
	for cat, want := range map[string]string{
		"CASH DIVIDEND": store.ActionDividend, "SCRIP DIVIDEND": store.ActionDividend,
		"RIGHTS ISSUE": store.ActionRights, "SUB-DIVISION OF SHARES": store.ActionSplit,
		"CAPITALIZATION OF RESERVES": store.ActionBonus, "ANNUAL GENERAL MEETING - APPROVED": store.ActionAGM,
		"DEALINGS BY DIRECTORS": store.ActionOther,
	} {
		if got := csejobs.ActionKind(cat); got != want {
			t.Errorf("ActionKind(%q) = %q, want %q", cat, got, want)
		}
	}
}

func TestProfileRowsSplitsRoles(t *testing.T) {
	_, c := csetest.New(t)
	p, err := c.CompanyProfile(context.Background(), "JKH.N0000")
	if err != nil {
		t.Fatal(err)
	}
	row, ds := csejobs.ProfileRows("JKH.N0000", p)
	if row.Sector == "" || row.FYEnd == "" || len(ds) == 0 {
		t.Fatalf("profile = %+v, %d directors", row, len(ds))
	}
	for _, d := range ds {
		if d.Name == "" || (d.Role != "" && d.Name[len(d.Name)-1] == ')') {
			t.Fatalf("bad director split: %+v", d)
		}
	}
}
