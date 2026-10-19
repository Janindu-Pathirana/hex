// Package analysis computes fundamental ratios from plain numbers. It does no
// I/O: callers fill Values from flags, stored fundamentals or the CSE API.
package analysis

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Unit says how a ratio's value is displayed.
type Unit string

const (
	Percent Unit = "percent" // 0.12 → "12.00%"
	Times   Unit = "times"   // 8 → "8.00x"
	LKR     Unit = "lkr"     // 1234.5 → "1,234.50"
	Points  Unit = "points"  // 61.234 → "61.23" (indicator readings such as RSI)
	Change  Unit = "change"  // 0.0281 → "+2.81%", signed
	Count   Unit = "count"   // 9 → "9"
	PP      Unit = "pp"      // 0.0120 → "+1.20pp", a change in a percentage
)

// Input is one number a ratio needs. Key is shared with stored fundamentals.
type Input struct {
	Key   string
	Flag  string
	Label string
}

// Values holds input numbers by Input.Key.
type Values map[string]float64

// Has reports whether key was supplied.
func (v Values) Has(key string) bool {
	_, ok := v[key]
	return ok
}

// Result is a computed ratio with the working shown.
type Result struct {
	Value   *float64 `json:"value"`   // nil when not computable
	Display string   `json:"display"` // "12.00%", "n/m"
	Steps   []string `json:"steps"`
	Note    string   `json:"note,omitempty"` // why Value is nil, or caveats
}

// Ratio describes one calculator.
type Ratio struct {
	ID       string
	Name     string
	Group    string // profitability, valuation, income, health, growth
	Formula  string
	Unit     Unit
	Inputs   []Input // required
	Optional []Input // used when present
	// SkipFinancials marks ratios that are not meaningful for banks and
	// other financial companies (margins, debt/equity, current ratio).
	SkipFinancials bool
	Compute        func(in Values) Result
}

// AllInputs returns the required inputs followed by the optional ones.
func (r Ratio) AllInputs() []Input {
	return append(append([]Input(nil), r.Inputs...), r.Optional...)
}

// Missing returns the required inputs absent from in.
func (r Ratio) Missing(in Values) []Input {
	var miss []Input
	for _, i := range r.Inputs {
		if !in.Has(i.Key) {
			miss = append(miss, i)
		}
	}
	return miss
}

// Calc checks for missing inputs, then computes the ratio.
func (r Ratio) Calc(in Values) (Result, error) {
	if miss := r.Missing(in); len(miss) > 0 {
		names := make([]string, len(miss))
		for i, m := range miss {
			names[i] = "--" + m.Flag
		}
		return Result{}, fmt.Errorf("%s needs %s", r.ID, strings.Join(names, ", "))
	}
	return r.Compute(in), nil
}

// Lookup finds a ratio by ID.
func Lookup(id string) (Ratio, bool) {
	for _, r := range Registry {
		if r.ID == id {
			return r, true
		}
	}
	return Ratio{}, false
}

// ok builds a Result for a computed value.
func ok(v float64, u Unit, steps ...string) Result {
	return Result{Value: &v, Display: Format(v, u), Steps: steps}
}

// nm builds a "not meaningful" Result.
func nm(note string, steps ...string) Result {
	return Result{Display: "n/m", Note: note, Steps: steps}
}

// Format displays a value in unit u.
func Format(v float64, u Unit) string {
	switch u {
	case Percent:
		return strconv.FormatFloat(v*100, 'f', 2, 64) + "%"
	case Times:
		return strconv.FormatFloat(v, 'f', 2, 64) + "x"
	case Points:
		return strconv.FormatFloat(v, 'f', 2, 64)
	case Change:
		s := strconv.FormatFloat(v*100, 'f', 2, 64)
		if s != "0.00" && s != "-0.00" && v > 0 {
			s = "+" + s
		}
		if s == "-0.00" {
			s = "0.00"
		}
		return s + "%"
	case PP:
		p := strconv.FormatFloat(v*100, 'f', 2, 64)
		if p == "-0.00" {
			p = "0.00"
		}
		if v > 0 && p != "0.00" {
			p = "+" + p
		}
		return p + "pp"
	case Count:
		return strconv.FormatFloat(v, 'f', 0, 64)
	default:
		return Num(v)
	}
}

// Num formats a number with thousands separators. Values below 1 keep up to
// four decimals (EPS of 0.004 must not print as 0); others keep up to two.
func Num(v float64) string {
	prec := 2
	if math.Abs(v) < 1 {
		prec = 4
	}
	s := strconv.FormatFloat(v, 'f', prec, 64)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	}
	sign := ""
	if strings.HasPrefix(s, "-") {
		sign, s = "-", s[1:]
	}
	intPart, frac, hasFrac := strings.Cut(s, ".")
	var b strings.Builder
	for i, c := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	if hasFrac {
		b.WriteString("." + frac)
	}
	if sign != "" && b.String() == "0" {
		sign = ""
	}
	return sign + b.String()
}
