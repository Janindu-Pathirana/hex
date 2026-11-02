package cmd

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDataAuditText(t *testing.T) {
	out, err := runAnalyze(t, true, "data", "audit")
	if err != nil {
		t.Fatal(err)
	}
	golden(t, "data-audit", out)
}

func TestDataAuditJSONAndSymbol(t *testing.T) {
	out, err := runAnalyze(t, true, "data", "audit", "jkh", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var rep struct {
		Market struct {
			Indexes    []struct{ ID string }
			MarketDays int
		}
		Companies []struct {
			Code string
			Bars *struct {
				Count      int
				First      string
				MaxGapDays int
			}
			Fundamentals *struct{ Periods int }
			Flags        []string
		}
		Flagged int
	}
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("not json: %v\n%s", err, out)
	}
	if len(rep.Companies) != 1 || rep.Companies[0].Code != "JKH.N0000" {
		t.Fatalf("companies = %+v", rep.Companies)
	}
	c := rep.Companies[0]
	if c.Bars == nil || c.Bars.Count != 30 || c.Bars.First != "2025-12-16" || c.Bars.MaxGapDays != 1 {
		t.Fatalf("bars = %+v", c.Bars)
	}
	if c.Fundamentals == nil || c.Fundamentals.Periods != 3 {
		t.Fatalf("fundamentals = %+v", c.Fundamentals)
	}
	if rep.Market.MarketDays != 30 || len(rep.Market.Indexes) != 1 || rep.Market.Indexes[0].ID != "ASPI" {
		t.Fatalf("market = %+v", rep.Market)
	}
	joined := strings.Join(c.Flags, "|")
	for _, want := range []string{"short price history: 30 bars", "only 3 statement periods"} {
		if !strings.Contains(joined, want) {
			t.Errorf("flags %q missing %q", joined, want)
		}
	}
	if rep.Flagged != 1 {
		t.Errorf("flagged = %d", rep.Flagged)
	}
}

func TestDataAuditEmptyStore(t *testing.T) {
	out, err := runAnalyze(t, false, "data", "audit")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"0 of 0 companies have flags", "ASPI: no stored levels", "SPSL20: no stored levels"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestDataAuditUnknownSymbol(t *testing.T) {
	if _, err := runAnalyze(t, true, "data", "audit", "nope"); err == nil {
		t.Fatal("an unknown symbol should be an error")
	}
}
