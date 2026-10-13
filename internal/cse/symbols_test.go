package cse_test

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/Janindu-Pathirana/hex/internal/cse"
	"github.com/Janindu-Pathirana/hex/internal/csetest"
)

func fixtureSecurities(t *testing.T) []cse.Security {
	t.Helper()
	secs, err := cse.ParseSymbolList(bytes.NewReader(csetest.Fixture(t, "company_symbol_list.csv")))
	if err != nil {
		t.Fatal(err)
	}
	return secs
}

func TestParseSymbolList(t *testing.T) {
	secs := fixtureSecurities(t)
	if len(secs) != 5 {
		t.Fatalf("got %d securities, want 5", len(secs))
	}
	if secs[0] != (cse.Security{Name: "C I C HOLDINGS PLC", Code: "CIC.X0000"}) {
		t.Fatalf("unexpected first row %+v", secs[0])
	}
}

func TestParseSymbolListErrors(t *testing.T) {
	for name, in := range map[string]string{
		"empty":       "",
		"header only": "Company Name,Symbol\n",
		"no codes":    "Company Name,Symbol\nFOO,\n",
	} {
		if _, err := cse.ParseSymbolList(strings.NewReader(in)); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestSecuritiesFetch(t *testing.T) {
	_, c := csetest.New(t)
	secs, err := c.Securities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(secs) != 5 {
		t.Fatalf("got %d, want 5", len(secs))
	}
}

func TestResolve(t *testing.T) {
	secs := fixtureSecurities(t)
	tests := []struct {
		in   string
		want string
		err  bool
	}{
		{"JKH", "JKH.N0000", false},
		{"jkh", "JKH.N0000", false},
		{" CIC ", "CIC.N0000", false}, // N0000 preferred even though X0000 is listed first
		{"cic.x0000", "CIC.X0000", false},
		{"COMB.X0000", "COMB.X0000", false},
		{"JKH.X0000", "", true},
		{"NOPE", "", true},
		{"", "", true},
	}
	for _, tc := range tests {
		got, err := cse.Resolve(tc.in, secs)
		if tc.err {
			if !errors.Is(err, cse.ErrUnknownSymbol) {
				t.Errorf("Resolve(%q): got %v, want ErrUnknownSymbol", tc.in, err)
			}
			continue
		}
		if err != nil || got.Code != tc.want {
			t.Errorf("Resolve(%q) = %q, %v; want %q", tc.in, got.Code, err, tc.want)
		}
	}
}

func TestResolveNonVotingOnly(t *testing.T) {
	secs := []cse.Security{{Name: "A", Code: "AAA.X0000"}, {Name: "A", Code: "AAA.U0000"}}
	got, err := cse.Resolve("aaa", secs)
	if err != nil || got.Code != "AAA.U0000" {
		t.Fatalf("got %q, %v; want first code alphabetically", got.Code, err)
	}
}

func TestTickers(t *testing.T) {
	got := cse.Tickers(fixtureSecurities(t))
	want := []string{"CIC", "COMB", "JKH"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
