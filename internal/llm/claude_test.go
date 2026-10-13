package llm_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/Janindu-Pathirana/hex/internal/engine"
	"github.com/Janindu-Pathirana/hex/internal/extract"
	"github.com/Janindu-Pathirana/hex/internal/llm"
	"github.com/Janindu-Pathirana/hex/internal/pdfx"
	"github.com/Janindu-Pathirana/hex/internal/store"
)

const fakeKey = "sk-ant-test-secret-key"

// fakeAPI answers every request with a record_statements tool call carrying
// values, and keeps the last request body.
func fakeAPI(t *testing.T, values []map[string]any) (*httptest.Server, *[]byte) {
	t.Helper()
	var last []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		last, _ = io.ReadAll(r.Body)
		if r.Header.Get("X-Api-Key") != fakeKey {
			http.Error(w, `{"type":"error","error":{"type":"authentication_error","message":"bad key"}}`, 401)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id": "msg_1", "type": "message", "role": "assistant", "model": llm.DefaultModel,
			"content": []map[string]any{{"type": "tool_use", "id": "tu_1", "name": "record_statements",
				"input": map[string]any{"values": values}}},
			"stop_reason": "tool_use", "usage": map[string]any{"input_tokens": 10, "output_tokens": 10},
		})
	}))
	t.Cleanup(srv.Close)
	return srv, &last
}

// setup stores a report whose page 2 is an income statement as text.
func setup(t *testing.T) (*engine.Env, store.Report, *bytes.Buffer) {
	t.Helper()
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "hex.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	r := store.Report{ID: 1, Code: "JKH.N0000", Kind: store.ReportQuarterly, Title: "Interim Financial Statements for the Quarter ended 30th June 2026", Path: "x.pdf"}
	page := pdfx.Page{N: 2, Lines: []pdfx.Line{
		{Words: []pdfx.Word{{X: 50, Text: "CONSOLIDATED"}, {X: 120, Text: "INCOME"}, {X: 170, Text: "STATEMENT"}}},
		{Words: []pdfx.Word{{X: 50, Text: "Revenue"}, {X: 360, Text: "135,443,355"}, {X: 430, Text: "108,712,150"}}},
		{Words: []pdfx.Word{{X: 50, Text: "Profit"}, {X: 80, Text: "for"}, {X: 95, Text: "the"}, {X: 110, Text: "period"}, {X: 370, Text: "773,225"}, {X: 440, Text: "717,429"}}},
	}}
	b, _ := json.Marshal(page)
	db.Update(ctx, func(tx *store.Tx) error {
		if err := tx.UpsertReports(ctx, []store.Report{r}); err != nil {
			return err
		}
		return tx.SetReportText(ctx, r.ID, 2, store.TextOK, "", []store.ReportPage{{Page: 2, Text: b}})
	})
	r, _ = db.Report(ctx, r.ID)
	var log bytes.Buffer
	return &engine.Env{Store: db, Log: &log, Now: time.Now}, r, &log
}

func value(field, raw string, page int) map[string]any {
	return map[string]any{"field": field, "scope": "group", "period_type": "3M", "period_end": "2026-06-30",
		"raw": raw, "unit": "thousands", "page": page, "label": field}
}

func TestExtractVerifiesAgainstPageText(t *testing.T) {
	env, r, _ := setup(t)
	srv, last := fakeAPI(t, []map[string]any{
		value("revenue", "135,443,355", 2),
		value("pat", "773,225", 2),
		value("gross_profit", "24,584,583", 2), // fabricated: not on the page
		value("revenue", "135,443,355", 2),     // duplicate
		value("bogus_field", "1", 2),           // unknown field
		value("tax", "(3,307,157)", 9),         // page not sent
	})
	c := llm.New("", 5, option.WithBaseURL(srv.URL), option.WithAPIKey(fakeKey))
	end, _ := extract.PeriodEnd(r.Title)
	res, err := c.Extract(context.Background(), env, r, extract.Meta{PeriodEnd: end}, extract.Result{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Values) != 2 {
		t.Fatalf("values = %+v\nissues = %v", res.Values, res.Issues)
	}
	if res.Values[0].Field != "revenue" || res.Values[0].Value != 135_443_355_000 {
		t.Fatalf("revenue = %+v", res.Values[0])
	}
	joined := strings.Join(res.Issues, "\n")
	for _, want := range []string{`gross_profit "24,584,583" not found on page 2`, "unknown field", "not found on page 9"} {
		if !strings.Contains(joined, want) {
			t.Errorf("issues missing %q:\n%s", want, joined)
		}
	}

	var req map[string]any
	json.Unmarshal(*last, &req)
	if req["model"] != llm.DefaultModel {
		t.Errorf("model = %v", req["model"])
	}
	if tc := req["tool_choice"].(map[string]any); tc["type"] != "auto" {
		t.Errorf("tool_choice = %v (forced tool use is rejected by current models)", tc)
	}
	if !strings.Contains(string(*last), "135,443,355") || !strings.Contains(string(*last), "=== Page 2 ===") {
		t.Error("statement page text was not sent")
	}
}

func TestBudget(t *testing.T) {
	env, r, _ := setup(t)
	srv, _ := fakeAPI(t, []map[string]any{value("revenue", "135,443,355", 2)})
	c := llm.New("", 1, option.WithBaseURL(srv.URL), option.WithAPIKey(fakeKey))
	ctx := context.Background()
	if _, err := c.Extract(ctx, env, r, extract.Meta{}, extract.Result{}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Extract(ctx, env, r, extract.Meta{}, extract.Result{}); err != llm.ErrBudget {
		t.Fatalf("second call: %v, want ErrBudget", err)
	}
}

func TestKeyNeverLogged(t *testing.T) {
	env, r, log := setup(t)
	srv, _ := fakeAPI(t, nil)
	c := llm.New("", 5, option.WithBaseURL(srv.URL), option.WithAPIKey("sk-ant-wrong-key"), option.WithMaxRetries(0))
	_, err := c.Extract(context.Background(), env, r, extract.Meta{}, extract.Result{})
	if err == nil {
		t.Fatal("expected an auth error")
	}
	if strings.Contains(err.Error(), "sk-ant-wrong-key") || strings.Contains(log.String(), "sk-ant-wrong-key") {
		t.Fatalf("api key leaked: %v / %s", err, log.String())
	}
}
