package extract

import "testing"

func TestParseNumber(t *testing.T) {
	for in, want := range map[string]float64{
		"1,234": 1234, "(1,234)": -1234, "-1,234": -1234, "0.004": 0.004, "(0.046)": -0.046,
		"-": 0, "–": 0, "135,443,355": 135443355, "(1,255,043)v": -1255043, "12": 12,
	} {
		got, ok := parseNumber(in)
		if !ok || got != want {
			t.Errorf("parseNumber(%q) = %v, %v; want %v", in, got, ok, want)
		}
	}
	for _, in := range []string{"Revenue", "15%", ">100", "(1,234", "1,23", "2026/27"} {
		if _, ok := parseNumber(in); ok {
			t.Errorf("parseNumber(%q) should fail", in)
		}
	}
}

func TestUnitScale(t *testing.T) {
	for text, want := range map[string]float64{
		"Rs.'000":                          1e3,
		"Note: All values are in Rs '000s": 1e3,
		"(all amounts in Sri Lanka Rupees millions)":   1e6,
		"(all amounts in Sri Lankan Rupees thousands)": 1e3,
		"In Sri Lankan Rs.'000s":                       1e3,
		"Rs 000":                                       1e3,
		"For the quarter ended 30 June":                0,
	} {
		if got := unitScale(text); got != want {
			t.Errorf("unitScale(%q) = %v, want %v", text, got, want)
		}
	}
}

func TestMatchField(t *testing.T) {
	for label, want := range map[string]string{
		"Total revenue":                           "revenue",
		"Less : Interest expense":                 "interest_expense",
		"Profit/ (loss) for the period":           "pat",
		"Profit before taxation":                  "pbt",
		"Equity holders of the parent":            "pat_parent",
		"Basic earnings per ordinary share (Rs.)": "eps_basic",
		"Revenuev":                  "revenue",
		"Dividend per share (Rs)**": "dps",
	} {
		fd, _ := matchField(IncomeStatement, label)
		if fd == nil || fd.Name != want {
			t.Errorf("matchField(%q) = %v, want %s", label, fd, want)
		}
	}
	for _, label := range []string{"Current assets", "Other comprehensive income", "Attributable to:"} {
		if fd, _ := matchField(BalanceSheet, label); fd != nil {
			t.Errorf("heading %q matched %s", label, fd.Name)
		}
	}
	// "Total revenue" outranks "Turnover".
	_, total := matchField(IncomeStatement, "Total revenue")
	_, turnover := matchField(IncomeStatement, "Turnover")
	if total >= turnover {
		t.Error("total revenue should have a better priority than turnover")
	}
}

func TestStatementKind(t *testing.T) {
	for text, want := range map[string]string{
		"CONSOLIDATED INCOME STATEMENT":                                      IncomeStatement,
		"INCOME STATEMENT - GROUP 1":                                         IncomeStatement,
		"STATEMENT OF PROFIT OR LOSS AND OTHER COMPREHENSIVE INCOME - GROUP": IncomeStatement,
		"Condensed Consolidated Income Statementp":                           IncomeStatement,
		"STATEMENTS OF FINANCIAL POSITION":                                   BalanceSheet,
		"1 Statement of Financial Position":                                  BalanceSheet,
		"CONSOLIDATED STATEMENT OF CASH FLOWS":                               CashFlow,
		"CONSOLIDATED STATEMENT OF COMPREHENSIVE INCOME":                     "",
		"STATEMENT OF CHANGES IN EQUITY - GROUP":                             "",
		"Income Statement in subsequent periods":                             "",
		"profit or loss (FVTPL)":                                             "",
	} {
		if got := statementKind(text); got != want {
			t.Errorf("statementKind(%q) = %q, want %q", text, got, want)
		}
	}
}
