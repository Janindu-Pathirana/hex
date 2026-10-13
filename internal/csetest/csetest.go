// Package csetest provides a fake cse.lk server backed by testdata fixtures.
package csetest

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Janindu-Pathirana/hex/internal/cse"
)

// Server is a fake cse.lk. Requests records every request it served.
type Server struct {
	*httptest.Server
	Hits atomic.Int32

	mu    sync.Mutex
	forms []map[string]string
}

// Forms returns the endpoint path and "symbol" form value of each request.
func (s *Server) Forms() []map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]map[string]string(nil), s.forms...)
}

// Fixture returns the contents of testdata/cse/<name>.
func Fixture(t testing.TB, name string) []byte {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	b, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "testdata", "cse", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return b
}

// New starts a fake server that knows JKH.N0000 (profile, financials) and
// serves the trimmed symbol list. Unknown symbols get what cse.lk returns:
// 204 for companyProfile and {} for financials.
func New(t testing.TB) (*Server, *cse.Client) {
	t.Helper()
	s := &Server{}
	mux := http.NewServeMux()

	symbolList := Fixture(t, "company_symbol_list.csv")
	profile := Fixture(t, "companyProfile_JKH.json")
	financials := Fixture(t, "financials_JKH.json")

	mux.HandleFunc("GET /chat/company_symbol_list.csv", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(symbolList)
	})
	mux.HandleFunc("POST /api/companyProfile", func(w http.ResponseWriter, r *http.Request) {
		if r.FormValue("symbol") != "JKH.N0000" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		_, _ = w.Write(profile)
	})
	mux.HandleFunc("POST /api/financials", func(w http.ResponseWriter, r *http.Request) {
		if r.FormValue("symbol") != "JKH.N0000" {
			_, _ = w.Write([]byte("{}"))
			return
		}
		_, _ = w.Write(financials)
	})

	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.Hits.Add(1)
		_ = r.ParseForm()
		s.mu.Lock()
		s.forms = append(s.forms, map[string]string{"path": r.URL.Path, "symbol": r.FormValue("symbol")})
		s.mu.Unlock()
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
		HTTP:    hc,
		Retries: 2,
		Backoff: time.Millisecond,
	}
}
