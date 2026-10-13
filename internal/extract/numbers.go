package extract

import (
	"regexp"
	"strconv"
	"strings"
)

// numberToken matches an amount as printed in statements: 1,234 (1,234)
// -1,234 0.004 (0.046), optionally followed by one stray letter that some
// PDFs attach (seen as "(1,255,043)v").
var numberToken = regexp.MustCompile(`^(\(?)(-?)(\d{1,3}(?:,\d{3})+|\d+)(\.\d+)?(\)?)[a-zA-Z]?$`)

// dashToken is a nil value printed as a dash.
var dashToken = regexp.MustCompile(`^[-\x{2013}\x{2014}]+$`)

// parseNumber parses a statement amount. Dashes are zero. ok is false for
// anything else (labels, percentages, ">100").
func parseNumber(s string) (v float64, ok bool) {
	s = strings.TrimSpace(s)
	if dashToken.MatchString(s) {
		return 0, true
	}
	m := numberToken.FindStringSubmatch(s)
	if m == nil {
		return 0, false
	}
	if (m[1] == "(") != (m[5] == ")") {
		return 0, false
	}
	v, err := strconv.ParseFloat(strings.ReplaceAll(m[3], ",", "")+m[4], 64)
	if err != nil {
		return 0, false
	}
	if m[1] == "(" || m[2] == "-" {
		v = -v
	}
	return v, true
}

// isAmount reports whether s looks like a statement figure rather than a
// note number or year: it has thousands separators, brackets or decimals.
func isAmount(s string) bool {
	if _, ok := parseNumber(s); !ok || dashToken.MatchString(s) {
		return false
	}
	return strings.ContainsAny(s, ",()") || strings.Contains(s, ".")
}

var (
	unitThousands = regexp.MustCompile(`(?i)(rs\.?|lkr|rupees)\s*'?\s*000|'000|\bthousands?\b|rs\s+000`)
	unitMillions  = regexp.MustCompile(`(?i)\b(rs\.?|lkr)\s*(mn|million)|\bmillions?\b|rs\.?\s*m\b`)
	unitBillions  = regexp.MustCompile(`(?i)\b(rs\.?|lkr)\s*(bn|billion)`)
)

// unitScale finds the amount unit stated in text: 1000 for Rs.'000, 1e6 for
// millions, 1e9 for billions, 0 when none is stated.
func unitScale(text string) float64 {
	switch {
	case unitThousands.MatchString(text):
		return 1e3
	case unitMillions.MatchString(text):
		return 1e6
	case unitBillions.MatchString(text):
		return 1e9
	}
	return 0
}
