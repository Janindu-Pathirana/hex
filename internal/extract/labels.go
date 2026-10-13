package extract

import (
	"regexp"
	"strings"
)

// Statement kinds.
const (
	IncomeStatement = "pl"
	BalanceSheet    = "bs"
	CashFlow        = "cf"
)

// Field describes one canonical value and the labels that identify it.
type Field struct {
	Name      string
	Statement string
	PerShare  bool // not scaled by the amount unit
	Magnitude bool // stored as a positive amount (expenses printed in brackets)
	Patterns  []*regexp.Regexp
}

func f(name, stmt string, perShare, magnitude bool, patterns ...string) Field {
	fd := Field{Name: name, Statement: stmt, PerShare: perShare, Magnitude: magnitude}
	for _, p := range patterns {
		fd.Patterns = append(fd.Patterns, regexp.MustCompile(`^(?:`+p+`)$`))
	}
	return fd
}

const profitWord = `(?:profit|loss|profit\s*/\s*\(loss\)|\(loss\)\s*/\s*profit|profit\s*/\s*loss|\(loss\)|profit\s*\(loss\))`

// Fields is the label dictionary, matched against normalised labels (see
// normLabel). Patterns are anchored, so section headings do not match.
var Fields = []Field{
	// Income statement, general.
	f("revenue", IncomeStatement, false, false,
		`total revenue`, `(?:net )?revenue`, `revenue from contracts with customers`, `gross revenue`, `(?:total |net )?turnover`),
	f("cost_of_sales", IncomeStatement, false, true, `cost of (?:sales|revenue|goods sold)`),
	f("gross_profit", IncomeStatement, false, false, `gross profit`),
	f("operating_profit", IncomeStatement, false, false,
		`results from operating activities`, `operating profits?`, `profit from operations`, `profit from operating activities`),
	f("finance_income", IncomeStatement, false, false, `finance income`),
	f("finance_cost", IncomeStatement, false, true, `finance (?:costs?|expenses?)`),
	f("pbt", IncomeStatement, false, false, profitWord+` before (?:income )?tax(?:ation|es)?`),
	f("tax", IncomeStatement, false, true, `(?:income )?tax(?:ation)? (?:expenses?|charge)`, `taxation`),
	f("deferred_tax", IncomeStatement, false, true, `deferred tax(?:ation)?(?: (?:expense|charge))?`),
	f("pat", IncomeStatement, false, false, profitWord+` for the (?:period|year|quarter)`, profitWord+` after tax(?:ation)?`),
	f("pat_parent", IncomeStatement, false, false,
		`(?:attributable to:? )?(?:equity holders|owners|shareholders|equity shareholders) of the (?:parent|company|bank)(?: company)?`),
	f("eps_basic", IncomeStatement, true, false,
		`basic(?: earnings per (?:ordinary )?share)?(?: for the (?:period|year|quarter))?`,
		`earnings per (?:ordinary )?share basic`, `basic eps`, `earnings per (?:ordinary )?share(?: \(eps\))? basic(?: earnings per (?:ordinary )?share)?`,
		`earnings per (?:ordinary )?share(?: \(eps\))?(?: for the (?:period|year|quarter))?`),
	f("eps_diluted", IncomeStatement, true, false,
		`diluted(?: earnings per (?:ordinary )?share)?(?: for the (?:period|year|quarter))?`,
		`earnings per (?:ordinary )?share diluted`, `diluted eps`, `earnings per (?:ordinary )?share(?: \(eps\))? diluted(?: earnings per (?:ordinary )?share)?`),
	f("dps", IncomeStatement, true, false, `dividends? per (?:ordinary )?share`),

	// Income statement, banks and finance companies.
	f("gross_income", IncomeStatement, false, false, `gross income`),
	f("interest_income", IncomeStatement, false, false, `interest income`),
	f("interest_expense", IncomeStatement, false, true, `interest expenses?`),
	f("net_interest_income", IncomeStatement, false, false, `net interest income`),
	f("net_fee_income", IncomeStatement, false, false, `net fee and commission income`),
	f("total_operating_income", IncomeStatement, false, false, `total operating income`),
	f("impairment", IncomeStatement, false, true,
		`impairment (?:charges?|losses?)(?: and other losses)?(?: (?:on|for) .*)?`, `impairment charges? (?:for|on) loans .*`),
	f("operating_expenses", IncomeStatement, false, true, `(?:total )?operating expenses`),

	// Income statement, insurers.
	f("gwp", IncomeStatement, false, false, `gross written premiums?`),
	f("net_earned_premium", IncomeStatement, false, false, `net earned premiums?`),

	// Balance sheet.
	f("total_assets", BalanceSheet, false, false, `total assets`),
	f("total_liabilities", BalanceSheet, false, false, `total liabilities`),
	f("total_equity", BalanceSheet, false, false, `total equity`, `total shareholders'? equity`),
	f("total_equity_and_liabilities", BalanceSheet, false, false, `total equity and liabilities`, `total liabilities and equity`),
	f("equity_parent", BalanceSheet, false, false,
		`(?:total )?equity attributable to (?:the )?(?:equity holders|owners|shareholders|equity shareholders) of the (?:parent|company|bank)(?: company)?`),
	f("current_assets", BalanceSheet, false, false, `total current assets`),
	f("current_liabilities", BalanceSheet, false, false, `total current liabilities`),
	f("cash", BalanceSheet, false, false, `cash (?:and|&) cash equivalents`, `cash in hand and at bank`, `cash (?:and|&) bank balances`),
	f("nav_per_share", BalanceSheet, true, false, `net assets? (?:value )?per (?:ordinary )?share`),
	f("loans", BalanceSheet, false, false,
		`(?:financial assets at amortised cost - )?loans and (?:advances|receivables) to customers`, `loans and advances`),
	f("deposits", BalanceSheet, false, false,
		`(?:financial liabilities at amortised cost - )?due to depositors`, `deposits from customers`, `customer deposits`),

	// Cash flow.
	f("ocf", CashFlow, false, false, `net cash.* operating activities`),
	f("icf", CashFlow, false, false, `net cash.* investing activities`),
	f("fcf_financing", CashFlow, false, false, `net cash.* financing activities`),
	f("capex", CashFlow, false, true,
		`(?:purchase|acquisition|additions?) (?:of|to) property,? plant (?:and|&) equipment`),
	f("dividends_paid", CashFlow, false, true, `dividends? paid(?: to .*)?`),
}

var (
	labelNoise   = regexp.MustCompile(`\*+|\(rs\.?\)|\(lkr\)|\brs\.?$`)
	labelPrefix  = regexp.MustCompile(`^(?:less|add|add/\(less\)|add\s*/\s*\(less\))\s*:\s*`)
	labelTrailer = regexp.MustCompile(`[\s:]+$`)
	space        = regexp.MustCompile(`\s+`)
	junkSuffix   = regexp.MustCompile(`([a-z]{2,})[pvk]\b`)
)

// normLabel lowercases a label, drops "Less:"/"Add:" prefixes, footnote
// markers and unit tags, and collapses whitespace.
func normLabel(s string) string {
	s = strings.ToLower(s)
	s = labelNoise.ReplaceAllString(s, "")
	s = space.ReplaceAllString(strings.TrimSpace(s), " ")
	s = labelPrefix.ReplaceAllString(s, "")
	s = labelTrailer.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

// matchField returns the field for a label in a statement and the index of
// the pattern that matched (lower is a better match: "Total revenue" beats
// "Turnover"), or nil. When the label as printed does not match, it retries
// without a stray trailing "p"/"v"/"k" on words (an artefact of some PDF fonts).
func matchField(stmt, label string) (*Field, int) {
	n := normLabel(label)
	if n == "" {
		return nil, 0
	}
	for _, cand := range []string{n, normLabel(junkSuffix.ReplaceAllString(n, "$1"))} {
		for i := range Fields {
			fd := &Fields[i]
			if fd.Statement != stmt {
				continue
			}
			for pi, p := range fd.Patterns {
				if p.MatchString(cand) {
					return fd, pi
				}
			}
		}
	}
	return nil, 0
}
