package cse_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Janindu-Pathirana/hex/internal/cse"
	"github.com/Janindu-Pathirana/hex/internal/csetest"
)

func TestCompanyProfile(t *testing.T) {
	srv, c := csetest.New(t)

	p, err := c.CompanyProfile(context.Background(), "JKH.N0000")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Summary) == 0 || p.Summary[0].Name != "JOHN KEELLS HOLDINGS PLC" {
		t.Fatalf("unexpected summary: %+v", p.Summary)
	}
	if len(p.Directors) == 0 {
		t.Fatal("expected directors")
	}
	if got := srv.Forms()[0]; got["path"] != "/api/companyProfile" || got["symbol"] != "JKH.N0000" {
		t.Fatalf("unexpected request %v", got)
	}
}

func TestFinancials(t *testing.T) {
	_, c := csetest.New(t)

	f, err := c.Financials(context.Background(), "JKH.N0000")
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Annual) != 2 {
		t.Fatalf("got %d annual reports, want 2", len(f.Annual))
	}
	if f.Annual[0].AuthorizedDate == nil {
		t.Error("first annual report should be authorised")
	}
	if f.Annual[1].AuthorizedDate != nil {
		t.Error("null authorizedDate should decode as nil")
	}
}

func TestUnknownSymbolIsErrNoData(t *testing.T) {
	_, c := csetest.New(t)
	ctx := context.Background()

	if _, err := c.CompanyProfile(ctx, "NOPE.N0000"); !errors.Is(err, cse.ErrNoData) {
		t.Errorf("profile (204): got %v, want ErrNoData", err)
	}
	if _, err := c.Financials(ctx, "NOPE.N0000"); !errors.Is(err, cse.ErrNoData) {
		t.Errorf("financials ({}): got %v, want ErrNoData", err)
	}
}

func TestRequestHeaders(t *testing.T) {
	var got http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		_, _ = w.Write([]byte(`{"infoAnnualData":[]}`))
	}))
	defer srv.Close()

	if _, err := csetest.Client(srv.URL, srv.Client()).Financials(context.Background(), "JKH"); err != nil {
		t.Fatal(err)
	}
	for k, want := range map[string]string{
		"Content-Type": "application/x-www-form-urlencoded; charset=UTF-8",
		"Origin":       "https://www.cse.lk",
		"Referer":      "https://www.cse.lk/",
	} {
		if got.Get(k) != want {
			t.Errorf("%s = %q, want %q", k, got.Get(k), want)
		}
	}
	if got.Get("User-Agent") == "" {
		t.Error("missing User-Agent")
	}
}

func TestRetries(t *testing.T) {
	tests := []struct {
		name     string
		statuses []int
		wantErr  bool
		wantHits int32
	}{
		{"5xx then ok", []int{500, 502, 200}, false, 3},
		{"5xx exhausts retries", []int{500, 500, 500, 500}, true, 3},
		{"4xx is not retried", []int{404, 200}, true, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var hits atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				n := hits.Add(1)
				w.WriteHeader(tc.statuses[n-1])
				_, _ = w.Write([]byte(`{"infoAnnualData":[]}`))
			}))
			defer srv.Close()

			_, err := csetest.Client(srv.URL, srv.Client()).Financials(context.Background(), "JKH")
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if hits.Load() != tc.wantHits {
				t.Fatalf("hits = %d, want %d", hits.Load(), tc.wantHits)
			}
		})
	}
}

func TestMalformedJSONIncludesBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html>maintenance</html>`))
	}))
	defer srv.Close()

	_, err := csetest.Client(srv.URL, srv.Client()).Financials(context.Background(), "JKH")
	if err == nil || !strings.Contains(err.Error(), "decode financials") || !strings.Contains(err.Error(), "maintenance") {
		t.Fatalf("got %v, want decode error with body snippet", err)
	}
}

func TestContextCancel(t *testing.T) {
	_, c := csetest.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := c.CompanyProfile(ctx, "JKH.N0000"); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want context.Canceled", err)
	}
}

func TestAssetURL(t *testing.T) {
	tests := map[string]string{
		"cmt/upload_report_file/1.pdf": "https://cdn.cse.lk/cmt/upload_report_file/1.pdf",
		"/upload_logo/x.jpeg":          "https://cdn.cse.lk/upload_logo/x.jpeg",
		"https://example.com/a.pdf":    "https://example.com/a.pdf",
		"":                             "",
	}
	for in, want := range tests {
		if got := cse.AssetURL(in); got != want {
			t.Errorf("AssetURL(%q) = %q, want %q", in, got, want)
		}
	}
}
