package csejobs_test

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Janindu-Pathirana/hex/internal/cse"
	"github.com/Janindu-Pathirana/hex/internal/csetest"
	"github.com/Janindu-Pathirana/hex/internal/engine"
	"github.com/Janindu-Pathirana/hex/internal/output"
	"github.com/Janindu-Pathirana/hex/internal/sources/csejobs"
	"github.com/Janindu-Pathirana/hex/internal/store"
)

var all = csejobs.Scope{Securities: true, Market: true, Index: true, Quotes: true, Info: true, History: true, Profiles: true, Reports: true, Actions: true}

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
	// Today's snapshot plus the five chart points, on distinct dates.
	if aspi, _ := db.IndexBars(ctx, "ASPI"); len(aspi) != 6 || aspi[0].Date != "2025-11-06" || aspi[0].Value != 23115.12 {
		t.Fatalf("aspi bars = %+v", aspi)
	}
	if snp, _ := db.IndexBars(ctx, "SPSL20"); len(snp) != 5 || snp[len(snp)-1].Date != "2026-10-30" {
		t.Fatalf("s&p sl20 bars = %+v", snp)
	}
	acts, _ := db.Actions(ctx, "")
	if len(acts) != 7 {
		t.Fatalf("corporate actions = %d, want 7", len(acts))
	}
	byID := map[int64]store.CorporateAction{}
	for _, a := range acts {
		byID[a.ID] = a
	}
	// JKH dividend: dates parsed, DPS stored, matched to the N series.
	if d := byID[39812]; d.Code != "JKH.N0000" || d.ExDate != "2026-11-09" || d.PaymentDate != "2026-11-27" || d.DPS == nil || *d.DPS != 0.8 {
		t.Fatalf("dividend = %+v", d)
	}
	// COMB sub-division 1 into 4, trading from 2026-03-20.
	if sp := byID[36079]; sp.Kind != store.ActionSplit || sp.ExDate != "2026-03-20" || sp.RatioNum == nil || *sp.RatioNum != 4 {
		t.Fatalf("split = %+v", sp)
	}
	// A split with no stated ratio keeps its dates and leaves the ratio empty.
	if sp := byID[22456]; sp.Kind != store.ActionSplit || sp.RatioNum != nil || sp.ExDate != "" {
		t.Fatalf("split without ratio = %+v", sp)
	}
	// A deleted security's id is never searched, but the fixture returns its row anyway; it has no code.
	if r := byID[22083]; r.Code != "" {
		t.Fatalf("unmatched rights row = %+v", r)
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

func TestActionsSearchesTickersInTheSymbolListOnly(t *testing.T) {
	srv, env := setup(t)
	var bodies []string
	srv.Override("/api/smd", func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(b))
		_, _ = w.Write([]byte(`{"Announcement":{}}`))
	})
	since := time.Date(2018, 1, 1, 0, 0, 0, 0, output.Colombo)
	ctx := context.Background()
	if sum, _ := engine.Run(ctx, env, csejobs.Plan(csejobs.Scope{Securities: true, Actions: true, ActionsSince: since}, nil, time.Now()), engine.Options{}); sum.Err() != nil {
		t.Fatal(sum.Err())
	}
	if len(bodies) != 1 {
		t.Fatalf("searches = %d, want 1 (three listed tickers fit in one chunk)", len(bodies))
	}
	// JKH 508, COMB 369, CIC 372; the deleted and the unlisted security are left out.
	for _, want := range []string{`"companyIds":[369,372,508]`, `"fromDate":"2018-01-01"`, `"SUB-DIVISION OF SHARES"`} {
		if !strings.Contains(bodies[0], want) {
			t.Errorf("request %s missing %s", bodies[0], want)
		}
	}
	if strings.Contains(bodies[0], "RIGHTS ISSUE (DATES)") {
		t.Error("RIGHTS ISSUE (DATES) is answered with HTTP 500 and must not be requested")
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
	if len(stages) != 3 || len(stages[0]) != 4 || len(stages[2]) != 1 {
		t.Fatalf("stages = %d", len(stages))
	}
	if stages[0][3].TTL() != 5*time.Minute {
		t.Fatalf("quotes TTL in market hours = %v", stages[0][2].TTL())
	}
	if csejobs.Plan(all, nil, closed)[0][3].TTL() != 12*time.Hour {
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
		{Code: "CIC.X0000", Ticker: "CIC", Series: "X0000", Name: "C I C HOLDINGS PLC"},
		{Code: "CIC.N0000", Ticker: "CIC", Series: "N0000", Name: "C I C HOLDINGS PLC"},
		{Code: "COMB.N0000", Ticker: "COMB", Series: "N0000", Name: "COMMERCIAL BANK OF CEYLON PLC"},
	}
	rows := csejobs.ActionRows([]cse.SMDAnnouncement{
		{ID: 1, SecID: 372, Symbol: "cic", Company: "C I C HOLDINGS PLC", Category: "SCRIP DIVIDEND", Date: "28-11-2025",
			XD: "13-02-2026", RecordDate: "16-02-2026", Payment: "05-03-2026", VotingProportion: "1 for 43.90", VotingConsider: "43.9"},
		{ID: 2, Symbol: "COMB", Company: "COMMERCIAL BANK OF CEYLON PLC", Category: "SUB-DIVISION OF SHARES", Date: "02-02-2026",
			Remarks: "Sub-division of every 01 Ordinary Share into 20 Ordinary Shares.", TradingCommence: "20-03-2026", EntitlementAsAt: "16-03-2026"},
		{ID: 3, Symbol: "COMB", Category: "RIGHTS ISSUE", Date: "06-02-2026", XR: "17-04-2026", Allotment: "27-04-2026",
			RightsProportion: "03 Ordinary Voting share for every 14 existing Ordinary Voting shares", RightsPrice: "19"},
		{ID: 4, Symbol: "COMB", Category: "CAPITALIZATION OF RESERVES", Date: "01-12-2022", XC: "13-12-2022", DateAllotment: "15-12-2022"},
		{ID: 5, Symbol: "COMB", Category: "CASH DIVIDEND", Date: "29-10-2026", XD: "09-11-2026", VotingDivPerShare: "0.8"},
		{ID: 6, Symbol: "NOPE", Category: "CASH DIVIDEND", Date: "bad date", XD: "31-02-2026"},
	}, secs)

	scrip := rows[0]
	if scrip.Code != "CIC.N0000" || scrip.Ticker != "CIC" || scrip.SecID != 372 || scrip.Kind != store.ActionScrip ||
		scrip.Announced != "2025-11-28" || scrip.ExDate != "2026-02-13" || scrip.XD != "2026-02-13" ||
		scrip.RecordDate != "2026-02-16" || scrip.PaymentDate != "2026-03-05" {
		t.Fatalf("scrip = %+v", scrip)
	}
	if scrip.RatioNum == nil || *scrip.RatioNum != 1 || *scrip.RatioDen != 43.9 || scrip.Price == nil || *scrip.Price != 43.9 {
		t.Fatalf("scrip ratio = %v %v price %v", scrip.RatioNum, scrip.RatioDen, scrip.Price)
	}

	split := rows[1]
	if split.Kind != store.ActionSplit || split.ExDate != "2026-03-20" || split.EntitlementDate != "2026-03-16" ||
		split.RatioNum == nil || *split.RatioNum != 20 || *split.RatioDen != 1 {
		t.Fatalf("split = %+v", split)
	}
	rights := rows[2]
	if rights.Kind != store.ActionRights || rights.ExDate != "2026-04-17" || rights.AllotmentDate != "2026-04-27" ||
		rights.RatioNum == nil || *rights.RatioNum != 3 || *rights.RatioDen != 14 || rights.Price == nil || *rights.Price != 19 {
		t.Fatalf("rights = %+v", rights)
	}
	bonus := rows[3]
	if bonus.Kind != store.ActionBonus || bonus.ExDate != "2022-12-13" || bonus.AllotmentDate != "2022-12-15" || bonus.RatioNum != nil {
		t.Fatalf("bonus without a stated ratio = %+v", bonus)
	}
	div := rows[4]
	if div.Kind != store.ActionDividend || div.ExDate != "2026-11-09" || div.DPS == nil || *div.DPS != 0.8 || div.RatioNum != nil {
		t.Fatalf("dividend = %+v", div)
	}
	odd := rows[5]
	if odd.Code != "" || odd.Announced != "" || odd.XD != "" || odd.ExDate != "" {
		t.Fatalf("unmatched row with bad dates = %+v", odd)
	}
}

func TestActionKind(t *testing.T) {
	for cat, want := range map[string]string{
		"CASH DIVIDEND": store.ActionDividend, "CASH DIVIDEND (DATES TO BE NOTIFIED)": store.ActionDividend,
		"SCRIP DIVIDEND": store.ActionScrip, "SCRIP DIVIDEND (DATES)": store.ActionScrip,
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

func TestIndexChartRows(t *testing.T) {
	d1 := time.Date(2026, 10, 29, 14, 30, 0, 0, output.Colombo).UnixMilli()
	d2 := time.Date(2026, 10, 30, 9, 30, 0, 0, output.Colombo).UnixMilli()
	d3 := time.Date(2026, 10, 30, 14, 30, 0, 0, output.Colombo).UnixMilli() // same day as d2: the later point wins
	rows := csejobs.IndexChartRows("ASPI", "All Share Price Index", []cse.IndexPoint{
		{D: d1, Value: 100, Percent: 1}, {D: d2, Value: 103, Percent: 3}, {D: d3, Value: 102, Percent: 2},
	})
	if len(rows) != 2 || rows[0].Date != "2026-10-29" || rows[0].Change != 0 {
		t.Fatalf("rows = %+v", rows)
	}
	if rows[1].Date != "2026-10-30" || rows[1].Value != 102 || rows[1].Change != -1 || rows[1].ChangePct != 2 {
		t.Fatalf("last row = %+v", rows[1])
	}
}

func TestIndexHistoryNoDataFails(t *testing.T) {
	srv, env := setup(t)
	srv.Override("/api/chartData", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("[]")) })
	sum, _ := engine.Run(context.Background(), env, []engine.Stage{{csejobs.IndexHistory()}}, engine.Options{})
	if sum.Count(engine.Failed) != 1 {
		t.Fatalf("an empty chart should fail the job: %+v", sum)
	}
}
