package cse_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Janindu-Pathirana/hex/internal/cse"
	"github.com/Janindu-Pathirana/hex/internal/csetest"
)

func TestMarketEndpoints(t *testing.T) {
	_, c := csetest.New(t)
	ctx := context.Background()

	status, err := c.MarketStatus(ctx)
	if err != nil || status != "Market Closed" {
		t.Fatalf("MarketStatus = %q, %v", status, err)
	}
	sum, err := c.MarketSummary(ctx)
	if err != nil || sum.Turnover <= 0 || sum.TradeDate == 0 {
		t.Fatalf("MarketSummary = %+v, %v", sum, err)
	}
	aspi, err := c.ASPI(ctx)
	if err != nil || aspi.SectorID != 1 || aspi.Value <= 0 {
		t.Fatalf("ASPI = %+v, %v", aspi, err)
	}
	snp, err := c.SNP(ctx)
	if err != nil || snp.SectorID != 40 {
		t.Fatalf("SNP = %+v, %v", snp, err)
	}
	sectors, err := c.Sectors(ctx)
	if err != nil || len(sectors) != 3 || sectors[0].IndexName == "" {
		t.Fatalf("Sectors = %+v, %v", sectors, err)
	}
	days, err := c.DailyMarketSummary(ctx)
	if err != nil || len(days) != 2 {
		t.Fatalf("DailyMarketSummary = %d days, %v", len(days), err)
	}
	if days[0].PER <= 0 || days[0].TradeDate <= days[1].TradeDate {
		t.Fatalf("daily rows not newest-first with PER: %+v", days)
	}
}

func TestTradeSummary(t *testing.T) {
	_, c := csetest.New(t)
	rows, err := c.TradeSummary(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var jkh *cse.TradeSummary
	for i := range rows {
		if rows[i].Symbol == "JKH.N0000" {
			jkh = &rows[i]
		}
	}
	if len(rows) != 5 || jkh == nil {
		t.Fatalf("got %d rows, JKH %v", len(rows), jkh)
	}
	if jkh.StockID != 297 || jkh.Price <= 0 || jkh.MarketCap <= 0 {
		t.Fatalf("JKH = %+v", jkh)
	}
}

func TestCompanyInfoSummary(t *testing.T) {
	_, c := csetest.New(t)
	ctx := context.Background()

	ci, err := c.CompanyInfoSummary(ctx, "JKH.N0000")
	if err != nil {
		t.Fatal(err)
	}
	if ci.Info.StockID != 297 || ci.Info.ISIN != "LK0092N00003" || ci.Info.High52W < ci.Info.Low52W {
		t.Fatalf("info = %+v", ci.Info)
	}
	if ci.Beta == nil || ci.Beta.ASPI <= 0 {
		t.Fatalf("beta = %+v", ci.Beta)
	}
	if ci.Info.ForeignHoldings != nil {
		t.Error("null foreignHoldings should decode as nil")
	}
	if _, err := c.CompanyInfoSummary(ctx, "JKH"); !errors.Is(err, cse.ErrNoData) {
		t.Fatalf("bare ticker: got %v, want ErrNoData", err)
	}
}

func TestChartByStock(t *testing.T) {
	_, c := csetest.New(t)
	ctx := context.Background()

	bars, err := c.ChartByStock(ctx, 297, cse.PeriodYear)
	if err != nil {
		t.Fatal(err)
	}
	if len(bars) != 5 || bars[0].Close <= 0 || bars[0].T >= bars[4].T {
		t.Fatalf("bars = %+v", bars)
	}
	if bars[0].Open != nil {
		t.Error("null open should decode as nil")
	}
	if _, err := c.ChartByStock(ctx, 1, cse.PeriodYear); !errors.Is(err, cse.ErrNoData) {
		t.Fatalf("unknown stock: got %v, want ErrNoData", err)
	}
}

func TestAnnouncements(t *testing.T) {
	_, c := csetest.New(t)
	ctx := context.Background()

	anns, err := c.Announcements(ctx)
	if err != nil || len(anns) == 0 {
		t.Fatalf("Announcements = %d, %v", len(anns), err)
	}
	if anns[0].Company == "" || anns[0].Category == "" || anns[0].CreatedDate == 0 {
		t.Fatalf("announcement = %+v", anns[0])
	}
	fin, err := c.FinancialAnnouncements(ctx)
	if err != nil || len(fin) != 2 || fin[0].Path == "" || fin[0].Symbol != "ASPH" {
		t.Fatalf("FinancialAnnouncements = %+v, %v", fin, err)
	}
}

func TestDownloadPDF(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		max     int64
		wantErr error
	}{
		{"pdf", "%PDF-1.7\nhello", 1 << 20, nil},
		{"html error page", "<html>not found</html>", 1 << 20, cse.ErrNotPDF},
		{"too large", "%PDF-1.7\n" + strings.Repeat("x", 100), 50, cse.ErrTooLarge},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var gotPath string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			b, err := csetest.Client(srv.URL, srv.Client()).DownloadPDF(context.Background(), "cmt/upload_report_file/1_2.pdf", tc.max)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
			if gotPath != "/cdn/cmt/upload_report_file/1_2.pdf" {
				t.Fatalf("path = %q", gotPath)
			}
			if tc.wantErr == nil && string(b) != tc.body {
				t.Fatalf("body = %q", b)
			}
		})
	}
}

func TestIndexChart(t *testing.T) {
	_, c := csetest.New(t)
	ctx := context.Background()

	aspi, err := c.IndexChart(ctx, cse.ChartASPI, cse.PeriodYear)
	if err != nil || len(aspi) != 5 || aspi[0].Value != 23115.12 || aspi[0].D != 1762397160000 {
		t.Fatalf("ASPI chart = %+v, %v", aspi, err)
	}
	snp, err := c.IndexChart(ctx, cse.ChartSPSL20, cse.PeriodYear)
	if err != nil || len(snp) != 4 {
		t.Fatalf("S&P SL20 chart = %d, %v", len(snp), err)
	}
	// cse.lk answers [] for an id it does not know.
	if _, err := c.IndexChart(ctx, 2, cse.PeriodYear); !errors.Is(err, cse.ErrNoData) {
		t.Fatalf("unknown chart: got %v, want ErrNoData", err)
	}
}

func TestSecurityIDsAndSearchAnnouncements(t *testing.T) {
	_, c := csetest.New(t)
	ctx := context.Background()

	ids, err := c.SecurityIDs(ctx)
	if err != nil || len(ids) != 4 || ids["JKH"] != 508 || ids["COMB"] != 369 {
		t.Fatalf("SecurityIDs = %v, %v", ids, err)
	}
	if _, ok := ids["OLDX"]; ok {
		t.Error("a deleted security must not be returned")
	}

	rows, err := c.SearchAnnouncements(ctx, cse.SMDQuery{CompanyIDs: []int64{508, 369}, Categories: []string{"CASH DIVIDEND", "RIGHTS ISSUE"}, FromDate: "2018-01-01"})
	if err != nil || len(rows) != 3 {
		t.Fatalf("SearchAnnouncements = %d rows, %v", len(rows), err)
	}
	div := rows[0]
	if div.ID != 39812 || div.Symbol != "JKH" || div.XD != "09-11-2026" || div.Payment != "27-11-2026" {
		t.Fatalf("dividend = %+v", div)
	}
	if v, ok := div.VotingDivPerShare.Float(); !ok || v != 0.8 {
		t.Errorf("DPS = %q", div.VotingDivPerShare)
	}
	rights := rows[1]
	// A number in the JSON (price) and a string (share count) both decode to text.
	if p, ok := rights.RightsPrice.Float(); !ok || p != 19 || rights.RightsShares != "52854857" {
		t.Errorf("rights price %q shares %q", rights.RightsPrice, rights.RightsShares)
	}
	if rights.RightsProportion != "03 Ordinary Voting share for every 14 existing Ordinary Voting shares" {
		t.Errorf("proportion = %q", rights.RightsProportion)
	}
}

func TestSearchAnnouncementsValidation(t *testing.T) {
	_, c := csetest.New(t)
	ctx := context.Background()
	for name, q := range map[string]cse.SMDQuery{
		"no company":  {Categories: []string{"CASH DIVIDEND"}},
		"no category": {CompanyIDs: []int64{508}},
	} {
		if _, err := c.SearchAnnouncements(ctx, q); err == nil || !strings.Contains(err.Error(), "400") {
			t.Errorf("%s: err = %v, want an HTTP 400", name, err)
		}
	}
}

func TestTextDecodesStringsNumbersAndNull(t *testing.T) {
	var v struct {
		A, B, C, D cse.Text
	}
	if err := json.Unmarshal([]byte(`{"A":"1 for 43.90","B":2.4,"C":null,"D":" 17,400 "}`), &v); err != nil {
		t.Fatal(err)
	}
	if v.A != "1 for 43.90" || v.B != "2.4" || v.C != "" || v.D != "17,400" {
		t.Fatalf("decoded = %+v", v)
	}
	if f, ok := v.D.Float(); !ok || f != 17400 {
		t.Errorf("Float with a thousands separator = %v, %v", f, ok)
	}
	if _, ok := v.A.Float(); ok {
		t.Error("text that is not a number must not parse")
	}
}
