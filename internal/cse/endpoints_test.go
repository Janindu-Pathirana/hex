package cse_test

import (
	"context"
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
