package cmd

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Janindu-Pathirana/hex/internal/cache"
	"github.com/Janindu-Pathirana/hex/internal/csetest"
)

var update = flag.Bool("update", false, "update golden files")

// run executes hex with args against a fake CSE server and an empty cache.
func run(t *testing.T, args ...string) (out string, srv *csetest.Server, err error) {
	t.Helper()
	t.Setenv(cache.EnvDir, t.TempDir())
	srv, client := csetest.New(t)

	var buf bytes.Buffer
	root := newRootCmd(&app{cse: client})
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(args)
	err = root.Execute()
	return buf.String(), srv, err
}

func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("..", "testdata", "golden", name+".txt")
	if *update {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden (run with -update to create): %v", err)
	}
	if got != string(want) {
		t.Fatalf("output mismatch for %s\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
	}
}

func TestVersionMakesNoRequests(t *testing.T) {
	out, srv, err := run(t, "version")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "dev" {
		t.Fatalf("got %q", out)
	}
	if n := srv.Hits.Load(); n != 0 {
		t.Fatalf("version made %d requests, want 0", n)
	}
}

func TestCompanyList(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"all", []string{"company", "list"}, "C I C HOLDINGS PLC,CIC.X0000\nC I C HOLDINGS PLC,CIC.N0000\nCOMMERCIAL BANK OF CEYLON PLC,COMB.N0000\nCOMMERCIAL BANK OF CEYLON PLC,COMB.X0000\nJOHN KEELLS HOLDINGS PLC,JKH.N0000\n"},
		{"symbols", []string{"company", "list", "--symbols"}, "CIC\nCOMB\nJKH\n"},
		{"legacy flag", []string{"company", "list", "--onlySymbol"}, "CIC\nCOMB\nJKH\n"},
		{"names", []string{"company", "list", "-n"}, "C-I-C-HOLDINGS-PLC\nCOMMERCIAL-BANK-OF-CEYLON-PLC\nJOHN-KEELLS-HOLDINGS-PLC\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, _, err := run(t, tc.args...)
			if err != nil {
				t.Fatal(err)
			}
			if out != tc.want {
				t.Fatalf("got\n%s\nwant\n%s", out, tc.want)
			}
		})
	}
}

func TestShowProfileResolvesLowercaseTicker(t *testing.T) {
	out, srv, err := run(t, "company", "show-profile", "jkh")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "JOHN KEELLS HOLDINGS PLC") {
		t.Fatalf("profile output missing company name:\n%s", out)
	}
	forms := srv.Forms()
	last := forms[len(forms)-1]
	if last["path"] != "/api/companyProfile" || last["symbol"] != "JKH.N0000" {
		t.Fatalf("profile requested with %v, want full code JKH.N0000", last)
	}
}

func TestUnknownSymbol(t *testing.T) {
	_, srv, err := run(t, "company", "show-profile", "NOPE")
	if err == nil || !strings.Contains(err.Error(), "hex company list --symbols") {
		t.Fatalf("got %v, want unknown-symbol hint", err)
	}
	for _, f := range srv.Forms() {
		if f["path"] == "/api/companyProfile" {
			t.Fatal("profile endpoint should not be called for an unknown symbol")
		}
	}
}

func TestShowFinancialAnnual(t *testing.T) {
	out, _, err := run(t, "company", "show-financial", "JKH", "--annual")
	if err != nil {
		t.Fatal(err)
	}
	golden(t, "show-financial-annual", out)
}

func TestShowFinancialLegacyFlag(t *testing.T) {
	legacy, _, err := run(t, "company", "show-financial", "JKH", "--showAnnualData")
	if err != nil {
		t.Fatal(err)
	}
	golden(t, "show-financial-annual", legacy)
}

func TestShowFinancialSectionsAreExclusive(t *testing.T) {
	if _, _, err := run(t, "company", "show-financial", "JKH", "-a", "-q"); err == nil {
		t.Fatal("expected an error when combining -a and -q")
	}
}

func TestCleanAllTwice(t *testing.T) {
	t.Setenv(cache.EnvDir, t.TempDir())
	for i := 0; i < 2; i++ {
		var buf bytes.Buffer
		root := NewRootCmd()
		root.SetOut(&buf)
		root.SetArgs([]string{"clean", "all"})
		if err := root.Execute(); err != nil {
			t.Fatalf("run %d: %v", i+1, err)
		}
	}
}

func TestCompletionFiltersByPrefix(t *testing.T) {
	out, _, err := run(t, "__complete", "company", "show-profile", "c")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "CIC\nCOMB\n:4\n") {
		t.Fatalf("unexpected completion output:\n%s", out)
	}
}

func TestCalcROE(t *testing.T) {
	out, srv, err := run(t, "calc", "roe", "--net-profit", "1200", "--equity-open", "9000", "--equity-close", "11000")
	if err != nil {
		t.Fatal(err)
	}
	golden(t, "calc-roe", out)
	if n := srv.Hits.Load(); n != 0 {
		t.Fatalf("calc made %d requests, want 0", n)
	}
}

func TestCalcMissingInput(t *testing.T) {
	_, _, err := run(t, "calc", "roe", "--net-profit", "1200")
	if err == nil || !strings.Contains(err.Error(), "--equity-close") {
		t.Fatalf("got %v, want missing --equity-close", err)
	}
}

func TestCalcJSON(t *testing.T) {
	out, _, err := run(t, "calc", "pe", "--price", "100", "--eps", "12.5", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"value": 8`) || !strings.Contains(out, `"display": "8.00x"`) {
		t.Fatalf("unexpected json:\n%s", out)
	}
}

func TestCalcListCoversRegistry(t *testing.T) {
	out, _, err := run(t, "calc", "list")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"roe", "net-margin", "pe", "dividend-yield", "growth"} {
		if !strings.Contains(out, "\n"+id+" ") {
			t.Errorf("calc list missing %s:\n%s", id, out)
		}
	}
}
