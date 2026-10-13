package extract

import (
	"fmt"
	"math"
	"time"
)

// check is one accounting identity over the values of a statement period.
type check struct {
	name   string
	fields []string
	ok     func(v map[string]float64) (bool, bool) // (applicable, holds)
}

func near(a, b, rel float64) bool {
	tol := math.Max(math.Abs(a), math.Abs(b)) * rel
	return math.Abs(a-b) <= math.Max(tol, 2000) // ±Rs.2,000 for rounding
}

func have(v map[string]float64, names ...string) bool {
	for _, n := range names {
		if _, ok := v[n]; !ok {
			return false
		}
	}
	return true
}

func checks(shares float64) []check {
	cs := []check{
		{"gross profit = revenue - cost of sales", []string{"revenue", "cost_of_sales", "gross_profit"}, func(v map[string]float64) (bool, bool) {
			if !have(v, "revenue", "cost_of_sales", "gross_profit") {
				return false, false
			}
			return true, near(v["revenue"]-v["cost_of_sales"], v["gross_profit"], 0.005)
		}},
		{"profit = profit before tax - tax", []string{"pbt", "tax", "pat"}, func(v map[string]float64) (bool, bool) {
			if !have(v, "pbt", "tax", "pat") {
				return false, false
			}
			tax := v["tax"] + v["deferred_tax"] // deferred_tax is 0 when not shown separately
			return true, near(v["pbt"]-tax, v["pat"], 0.005) || near(v["pbt"]+tax, v["pat"], 0.005) ||
				near(v["pbt"]-v["tax"], v["pat"], 0.005)
		}},
		{"net interest income = interest income - interest expense", []string{"interest_income", "interest_expense", "net_interest_income"}, func(v map[string]float64) (bool, bool) {
			if !have(v, "interest_income", "interest_expense", "net_interest_income") {
				return false, false
			}
			return true, near(v["interest_income"]-v["interest_expense"], v["net_interest_income"], 0.005)
		}},
		{"total assets = total equity and liabilities", []string{"total_assets", "total_equity_and_liabilities"}, func(v map[string]float64) (bool, bool) {
			if !have(v, "total_assets", "total_equity_and_liabilities") {
				return false, false
			}
			return true, near(v["total_assets"], v["total_equity_and_liabilities"], 0.001)
		}},
		{"total assets = equity + liabilities", []string{"total_assets", "total_equity", "total_liabilities"}, func(v map[string]float64) (bool, bool) {
			if !have(v, "total_assets", "total_equity", "total_liabilities") {
				return false, false
			}
			return true, near(v["total_assets"], v["total_equity"]+v["total_liabilities"], 0.001)
		}},
		{"parent equity <= total equity", []string{"equity_parent", "total_equity"}, func(v map[string]float64) (bool, bool) {
			if !have(v, "equity_parent", "total_equity") || v["total_equity"] <= 0 {
				return false, false
			}
			return true, v["equity_parent"] <= v["total_equity"]*1.001
		}},
	}
	if shares > 0 {
		cs = append(cs,
			check{"EPS ~ parent profit / shares", []string{"eps_basic", "pat_parent"}, func(v map[string]float64) (bool, bool) {
				if !have(v, "eps_basic", "pat_parent") || math.Abs(v["pat_parent"]) < shares*0.01 {
					return false, false
				}
				return true, near(v["pat_parent"]/shares, v["eps_basic"], 0.15)
			}},
			check{"NAV per share ~ parent equity / shares", []string{"nav_per_share", "equity_parent"}, func(v map[string]float64) (bool, bool) {
				if !have(v, "nav_per_share", "equity_parent") {
					return false, false
				}
				return true, near(v["equity_parent"]/shares, v["nav_per_share"], 0.1)
			}},
		)
	}
	return cs
}

// score sets each value's confidence from the identities its statement
// period satisfies, and returns the failed checks as issues.
func score(vals []Value, meta Meta) []string {
	type period struct {
		scope, ptype string
		end          time.Time
	}
	// Balance-sheet and income values of the same scope and date are checked
	// together (EPS needs P&L, NAV needs the balance sheet).
	groups := map[period][]int{}
	for i, v := range vals {
		p := period{v.Scope, v.PeriodType, v.PeriodEnd}
		groups[p] = append(groups[p], i)
	}
	var issues []string
	for p, idx := range groups {
		m := map[string]float64{}
		for _, i := range idx {
			m[vals[i].Field] = vals[i].Value
		}
		passed, failed := 0, 0
		bad := map[string]bool{}
		for _, c := range checks(meta.Shares) {
			applicable, holds := c.ok(m)
			if !applicable {
				continue
			}
			if holds {
				passed++
				continue
			}
			failed++
			for _, f := range c.fields {
				bad[f] = true
			}
			issues = append(issues, fmt.Sprintf("%s %s %s: %s fails", p.scope, p.ptype, p.end.Format("2006-01-02"), c.name))
		}
		base := 0.5
		if passed == 0 && failed == 0 && len(idx) < 4 {
			base = 0.4
		}
		conf := base + 0.15*float64(passed) - 0.25*float64(failed)
		for _, i := range idx {
			c := conf
			if bad[vals[i].Field] {
				c -= 0.2
			}
			vals[i].Confidence = math.Round(math.Min(0.99, math.Max(0.05, c))*100) / 100
		}
	}
	return issues
}
