// Package csetest provides a fake cse.lk server backed by testdata fixtures.
package csetest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Janindu-Pathirana/hex/internal/cse"
)

// Server is a fake cse.lk. Hits counts every request it served. Mux can be
// used to add routes and Override to replace one.
type Server struct {
	*httptest.Server
	Mux  *http.ServeMux
	Hits atomic.Int32

	mu        sync.Mutex
	forms     []map[string]string
	overrides map[string]http.HandlerFunc
}

// Override serves every request for path (e.g. "/api/companyInfoSummery")
// with h instead of the fixture route.
func (s *Server) Override(path string, h http.HandlerFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.overrides == nil {
		s.overrides = map[string]http.HandlerFunc{}
	}
	s.overrides[path] = h
}

// Forms returns the endpoint path and "symbol" form value of each request.
func (s *Server) Forms() []map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]map[string]string(nil), s.forms...)
}

// Paths returns the request path of each request, in order.
func (s *Server) Paths() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, len(s.forms))
	for i, f := range s.forms {
		out[i] = f["path"]
	}
	return out
}

// TestdataDir returns the repository's testdata directory.
func TestdataDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata")
}

// Fixture returns the contents of testdata/cse/<name>.
func Fixture(t testing.TB, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(TestdataDir(), "cse", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return b
}

// New starts a fake server that knows JKH.N0000 and COMB.N0000 (profile,
// financials, info summary, history) plus the market-wide endpoints, and
// serves report PDFs from testdata/pdf under /cdn/. Unknown symbols get what
// cse.lk returns: 204 for companyProfile and {} elsewhere.
func New(t testing.TB) (*Server, *cse.Client) {
	t.Helper()
	s := &Server{Mux: http.NewServeMux()}
	mux := s.Mux

	static := func(pattern, fixture string) {
		body := Fixture(t, fixture)
		mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write(body)
		})
	}
	bySymbol := func(endpoint string, unknown func(http.ResponseWriter), fixtures map[string]string) {
		bodies := make(map[string][]byte, len(fixtures))
		for sym, f := range fixtures {
			bodies[sym] = Fixture(t, f)
		}
		mux.HandleFunc("POST /api/"+endpoint, func(w http.ResponseWriter, r *http.Request) {
			b, ok := bodies[r.FormValue("symbol")]
			if !ok {
				unknown(w)
				return
			}
			_, _ = w.Write(b)
		})
	}
	noContent := func(w http.ResponseWriter) { w.WriteHeader(http.StatusNoContent) }
	emptyObject := func(w http.ResponseWriter) { _, _ = w.Write([]byte("{}")) }

	static("GET /chat/company_symbol_list.csv", "company_symbol_list.csv")
	for _, ep := range []string{"marketStatus", "marketSummery", "aspiData", "snpData", "tradeSummary",
		"allSectors", "dailyMarketSummery", "approvedAnnouncement", "getFinancialAnnouncement"} {
		static("POST /api/"+ep, ep+".json")
	}
	bySymbol("companyProfile", noContent, map[string]string{
		"JKH.N0000": "companyProfile_JKH.json", "COMB.N0000": "companyProfile_COMB.json",
	})
	bySymbol("financials", emptyObject, map[string]string{
		"JKH.N0000": "financials_JKH.json", "COMB.N0000": "financials_COMB.json",
	})
	bySymbol("companyInfoSummery", emptyObject, map[string]string{
		"JKH.N0000": "companyInfoSummery_JKH.json", "COMB.N0000": "companyInfoSummery_COMB.json",
	})
	chart := Fixture(t, "companyChartDataByStock_297.json")
	mux.HandleFunc("POST /api/companyChartDataByStock", func(w http.ResponseWriter, r *http.Request) {
		switch r.FormValue("stockId") {
		case "297", "208": // JKH, COMB: both get the JKH bars
			_, _ = w.Write(chart)
		default:
			_, _ = w.Write([]byte(`{"chartData":[],"id":0}`))
		}
	})
	static("GET /api/cntSecurity", "cntSecurity.json")
	smd := Fixture(t, "smd.json")
	mux.HandleFunc("POST /api/smd", func(w http.ResponseWriter, r *http.Request) {
		var q struct {
			CompanyIDs []int64  `json:"companyIds"`
			Categories []string `json:"categories"`
		}
		_ = json.NewDecoder(r.Body).Decode(&q)
		switch { // the validation messages cse.lk answers with
		case len(q.CompanyIDs) == 0:
			http.Error(w, `{"status":"CLIENT_ERROR","statusCode":400,"validationFailures":[{"field":"Company error","code":"one or more company should select"}]}`, http.StatusBadRequest)
		case len(q.Categories) == 0:
			http.Error(w, `{"status":"CLIENT_ERROR","statusCode":400,"validationFailures":[{"field":"Category error","code":"one or more category should select"}]}`, http.StatusBadRequest)
		default:
			_, _ = w.Write(smd)
		}
	})
	indexCharts := map[string][]byte{"1": Fixture(t, "chartData_1.json"), "40": Fixture(t, "chartData_40.json")}
	mux.HandleFunc("POST /api/chartData", func(w http.ResponseWriter, r *http.Request) {
		if b, ok := indexCharts[r.FormValue("chartId")]; ok {
			_, _ = w.Write(b)
			return
		}
		_, _ = w.Write([]byte(`[]`)) // what cse.lk returns for an unknown chart id
	})
	mux.HandleFunc("GET /cdn/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(TestdataDir(), "pdf", path.Base(r.URL.Path)))
	})

	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.Hits.Add(1)
		_ = r.ParseForm()
		s.mu.Lock()
		s.forms = append(s.forms, map[string]string{"path": r.URL.Path, "symbol": r.FormValue("symbol")})
		h := s.overrides[r.URL.Path]
		s.mu.Unlock()
		if h != nil {
			h(w, r)
			return
		}
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(s.Close)

	return s, Client(s.URL, s.Client())
}

// Client returns a cse.Client pointed at base with fast retries.
func Client(base string, hc *http.Client) *cse.Client {
	return &cse.Client{
		BaseURL: base + "/api/",
		SiteURL: base + "/",
		CDNBase: base + "/cdn/",
		HTTP:    hc,
		Retries: 2,
		Backoff: time.Millisecond,
	}
}
