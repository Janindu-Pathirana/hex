// Package extract turns the positioned text of CSE financial-report PDFs into
// statement values: it finds the income statement, balance sheet and cash
// flow pages, rebuilds their columns from the headers, maps line labels to
// canonical fields, scales units and scores each result with accounting
// identities.
package extract

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

var months = map[string]time.Month{
	"jan": 1, "feb": 2, "mar": 3, "apr": 4, "may": 5, "jun": 6,
	"jul": 7, "aug": 8, "sep": 9, "oct": 10, "nov": 11, "dec": 12,
}

var (
	// "30th June 2026", "31 March 2026", "31st December 2025"
	dmyWords = regexp.MustCompile(`(?i)\b(\d{1,2})(?:st|nd|rd|th)?\s+(jan|feb|mar|apr|may|jun|jul|aug|sep|oct|nov|dec)[a-z]*,?\s+(\d{4})\b`)
	// "June 30, 2026"
	mdyWords = regexp.MustCompile(`(?i)\b(jan|feb|mar|apr|may|jun|jul|aug|sep|oct|nov|dec)[a-z]*\s+(\d{1,2})(?:st|nd|rd|th)?,?\s+(\d{4})\b`)
	// "30-Jun-26", "30-Jun-2026"
	dmyShort = regexp.MustCompile(`(?i)\b(\d{1,2})-(jan|feb|mar|apr|may|jun|jul|aug|sep|oct|nov|dec)[a-z]*-(\d{2}|\d{4})\b`)
	// "30.06.2026", "30/06/2026", "30-06-2026"
	dmyNumeric = regexp.MustCompile(`\b(\d{1,2})[./-](\d{1,2})[./-](\d{4})\b`)
	// "2024/25", "2024/2025"
	fiscalYear = regexp.MustCompile(`\b(20\d{2})\s*/\s*(\d{2}|20\d{2})\b`)
	// "Annual Report 2024"
	plainYear = regexp.MustCompile(`\b(20\d{2})\b`)
)

func date(y, m, d int) (time.Time, bool) {
	if m < 1 || m > 12 || d < 1 || d > 31 {
		return time.Time{}, false
	}
	t := time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC)
	if t.Day() != d {
		return time.Time{}, false
	}
	return t, true
}

// parseDate finds the first full date in s.
func parseDate(s string) (time.Time, bool) {
	if m := dmyWords.FindStringSubmatch(s); m != nil {
		d, _ := strconv.Atoi(m[1])
		y, _ := strconv.Atoi(m[3])
		return date(y, int(months[strings.ToLower(m[2][:3])]), d)
	}
	if m := mdyWords.FindStringSubmatch(s); m != nil {
		d, _ := strconv.Atoi(m[2])
		y, _ := strconv.Atoi(m[3])
		return date(y, int(months[strings.ToLower(m[1][:3])]), d)
	}
	if m := dmyShort.FindStringSubmatch(s); m != nil {
		d, _ := strconv.Atoi(m[1])
		y, _ := strconv.Atoi(m[3])
		if y < 100 {
			y += 2000
		}
		return date(y, int(months[strings.ToLower(m[2][:3])]), d)
	}
	if m := dmyNumeric.FindStringSubmatch(s); m != nil {
		d, _ := strconv.Atoi(m[1])
		mo, _ := strconv.Atoi(m[2])
		y, _ := strconv.Atoi(m[3])
		return date(y, mo, d)
	}
	return time.Time{}, false
}

// PeriodEnd works out the period a report covers from its CSE title, e.g.
// "Interim Financial Statements for the Quarter ended 30th June 2026" or
// "Annual Report 2024/25" (a split year ends on 31 March, a single year on
// 31 December). ok is false when the title has no usable date.
func PeriodEnd(title string) (time.Time, bool) {
	if t, ok := parseDate(title); ok {
		return t, true
	}
	if m := fiscalYear.FindStringSubmatch(title); m != nil {
		end, _ := strconv.Atoi(m[2])
		if end < 100 {
			start, _ := strconv.Atoi(m[1])
			end += start / 100 * 100
		}
		return date(end, 3, 31)
	}
	if m := plainYear.FindStringSubmatch(title); m != nil {
		y, _ := strconv.Atoi(m[1])
		return date(y, 12, 31)
	}
	return time.Time{}, false
}
