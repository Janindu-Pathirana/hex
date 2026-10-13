package cmd

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Janindu-Pathirana/hex/internal/cache"
	"github.com/Janindu-Pathirana/hex/internal/csetest"
	"github.com/Janindu-Pathirana/hex/internal/store"
)

func TestSyncThenStatus(t *testing.T) {
	t.Setenv(cache.EnvDir, t.TempDir())
	t.Setenv(store.EnvDir, t.TempDir())
	srv, client := csetest.New(t)

	out, _, err := runWith(t, srv, client, "sync", "market", "securities", "quotes", "info", "history", "profiles", "reports", "actions", "--symbols", "jkh,COMB")
	if err != nil {
		t.Fatalf("sync: %v\n%s", err, out)
	}
	if !strings.Contains(out, "sync: 12 ok, 0 fresh, 0 failed") {
		t.Fatalf("unexpected summary:\n%s", out)
	}
	if !strings.Contains(out, "history:JKH.N0000 ok") {
		t.Fatalf("missing progress line:\n%s", out)
	}

	before := srv.Hits.Load()
	out, _, err = runWith(t, srv, client, "sync", "market", "securities", "quotes", "info", "history", "profiles", "reports", "actions", "--symbols", "jkh,COMB")
	if err != nil || !strings.Contains(out, "0 ok, 12 fresh") {
		t.Fatalf("second sync: %v\n%s", err, out)
	}
	// Only the symbol-list lookup may hit the network (and it is cached).
	if n := srv.Hits.Load() - before; n != 0 {
		t.Fatalf("second sync made %d requests", n)
	}

	out, _, err = runWith(t, srv, client, "sync", "status")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"KIND", "history", "reports", "market"} {
		if !strings.Contains(out, want) {
			t.Fatalf("status missing %q:\n%s", want, out)
		}
	}

	out, _, err = runWith(t, srv, client, "sync", "status", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var states []jobStateJSON
	if err := json.Unmarshal([]byte(out), &states); err != nil || len(states) != 12 {
		t.Fatalf("status json: %v, %d states\n%s", err, len(states), out)
	}
}

func TestSyncReportsFailures(t *testing.T) {
	t.Setenv(cache.EnvDir, t.TempDir())
	t.Setenv(store.EnvDir, t.TempDir())
	srv, client := csetest.New(t)
	srv.Override("/api/companyInfoSummery", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	})

	out, _, err := runWith(t, srv, client, "sync", "info", "--symbols", "JKH")
	if err == nil || !strings.Contains(err.Error(), "1 job(s) failed") {
		t.Fatalf("err = %v\n%s", err, out)
	}
	out, _, _ = runWith(t, srv, client, "sync", "status")
	if !strings.Contains(out, "failed info:JKH.N0000") {
		t.Fatalf("status should list the failure:\n%s", out)
	}
}

func TestSyncDryRunFetchesNothing(t *testing.T) {
	out, srv, err := run(t, "sync", "--dry-run")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "3 would run") || srv.Hits.Load() != 0 {
		t.Fatalf("dry run made %d requests:\n%s", srv.Hits.Load(), out)
	}
}

func TestSyncPerCompanyNeedsSymbols(t *testing.T) {
	if _, _, err := run(t, "sync", "history"); err == nil || !strings.Contains(err.Error(), "--symbols") {
		t.Fatalf("err = %v", err)
	}
}

func TestSyncRejectsUnknownGroup(t *testing.T) {
	if _, _, err := run(t, "sync", "bogus"); err == nil {
		t.Fatal("expected an error for an unknown group")
	}
}

func TestVersionDoesNotCreateDatabase(t *testing.T) {
	if _, _, err := run(t, "version"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(os.Getenv(store.EnvDir), store.FileName)); !os.IsNotExist(err) {
		t.Fatalf("version created the database: %v", err)
	}
}

func TestCleanDB(t *testing.T) {
	out, _, err := run(t, "sync", "--dry-run")
	if err != nil {
		t.Fatal(err, out)
	}
	path := filepath.Join(os.Getenv(store.EnvDir), store.FileName)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("database not created: %v", err)
	}
	var buf strings.Builder
	root := NewRootCmd()
	root.SetOut(&buf)
	root.SetArgs([]string{"clean", "db"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("database still exists: %v", err)
	}
}

func TestSyncPDFsPlansCatalogueFirst(t *testing.T) {
	out, _, err := run(t, "sync", "pdfs", "--symbols", "JKH", "--dry-run")
	if err != nil {
		t.Fatal(err)
	}
	// reports:JKH (catalogue) and pdfs:JKH.
	if !strings.Contains(out, "reports:JKH.N0000 would run") || !strings.Contains(out, "pdfs:JKH.N0000 would run") {
		t.Fatalf("unexpected plan:\n%s", out)
	}
}
