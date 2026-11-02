package csejobs

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/Janindu-Pathirana/hex/internal/store"
)

var (
	// A parenthetical repeats a number in another form ("One (01)", "04 (Four)")
	// unless it holds a decimal, which is then the real figure.
	parenthetical  = regexp.MustCompile(`\(([^)]*)\)`)
	splitRatio     = regexp.MustCompile(`(\d+(?:\.\d+)?)\s+(?:[a-z.]+\s+){0,4}?shares?\b.*?\b(?:into|to)\s+(\d+(?:\.\d+)?)`)
	colonRatio     = regexp.MustCompile(`(\d+(?:\.\d+)?)\s*:\s*(\d+(?:\.\d+)?)`)
	forRatio       = regexp.MustCompile(`(\d+(?:\.\d+)?)\s+(?:[a-z.]+\s+){0,8}?(?:for|to)\s+(?:(?:every|each|ever|existing)\s*)*(\d+(?:\.\d+)?)`)
	bareNumber     = regexp.MustCompile(`^\s*(\d+(?:\.\d+)?)\s*$`)
	decimalNumber  = regexp.MustCompile(`\d+\.\d+`)
	digitBeforeFor = regexp.MustCompile(`(\d)for`) // "1for115.25"
	forBeforeDigit = regexp.MustCompile(`\bfor(\d)`)
)

var unitWords = map[string]int{
	"one": 1, "two": 2, "three": 3, "four": 4, "five": 5, "six": 6, "seven": 7, "eight": 8, "nine": 9, "ten": 10,
	"eleven": 11, "twelve": 12, "thirteen": 13, "fourteen": 14, "fifteen": 15, "sixteen": 16, "seventeen": 17,
	"eighteen": 18, "nineteen": 19,
}

var tensWords = map[string]int{
	"twenty": 20, "thirty": 30, "forty": 40, "fifty": 50, "sixty": 60, "seventy": 70, "eighty": 80, "ninety": 90,
}

// maxRatioSide rejects readings such as "1:250000000899", which is a typo in a
// real announcement rather than a ratio.
const maxRatioSide = 1e6

// ParseRatio reads the share ratio a corporate action states in free text.
//
//   - For a sub-division it finds "every 1 ordinary share into 4" (or "1:10")
//     and returns shares after and shares before (4, 1).
//   - For a rights issue, scrip dividend or capitalisation it finds "3 new
//     shares for every 14 held", "1 for 43.90", "1:23.5" or, for scrip, a bare
//     "168.2" (one share per 168.2 held) and returns new shares and existing
//     shares (3, 14).
//
// ok is false when the text has no ratio or the numbers are not plausible.
// It never guesses: a missing ratio is left for a person to enter. Most
// sub-divisions on cse.lk state the ratio only in an attachment, so ok is
// false for them more often than not.
func ParseRatio(kind, text string) (num, den float64, ok bool) {
	t := normaliseRatioText(text)

	if kind == store.ActionSplit {
		if m := splitRatio.FindStringSubmatch(t); m != nil {
			return plausible(parse(m[2]), parse(m[1]), true)
		}
		if m := colonRatio.FindStringSubmatch(t); m != nil {
			return plausible(parse(m[2]), parse(m[1]), true)
		}
		return 0, 0, false
	}

	if kind == store.ActionScrip {
		if m := bareNumber.FindStringSubmatch(t); m != nil {
			if v := parse(m[1]); v > 1 {
				return plausible(1, v, false)
			}
			return 0, 0, false
		}
	}
	m := colonRatio.FindStringSubmatch(t)
	if m == nil {
		m = forRatio.FindStringSubmatch(t)
	}
	if m == nil {
		return 0, 0, false
	}
	n, d := parse(m[1]), parse(m[2])
	// "thirty three decimal zero two three ... (33.0232559141)": the digits in
	// brackets are the figure, the words only spell it out.
	if strings.Contains(t, " decimal ") {
		if ds := decimalNumber.FindAllString(t, -1); len(ds) > 0 {
			d = parse(ds[len(ds)-1])
		}
	}
	return plausible(n, d, false)
}

// plausible returns the ratio if both sides are sensible. For a split the
// share count must grow.
func plausible(num, den float64, split bool) (float64, float64, bool) {
	if num <= 0 || den <= 0 || num > maxRatioSide || den > maxRatioSide || (split && num <= den) {
		return 0, 0, false
	}
	return num, den, true
}

func normaliseRatioText(text string) string {
	t := strings.ToLower(text)
	t = parenthetical.ReplaceAllStringFunc(t, func(p string) string {
		inner := strings.Trim(p, "()")
		if strings.Contains(inner, ".") {
			return " " + strings.TrimSpace(inner) + " "
		}
		return " "
	})
	t = strings.ReplaceAll(t, ",", "")
	t = strings.ReplaceAll(t, "-", " ") // "thirty-two", "Thirty -Two"
	t = strings.ReplaceAll(t, "everone", "every one")
	t = digitBeforeFor.ReplaceAllString(t, "$1 for ")
	t = forBeforeDigit.ReplaceAllString(t, " for $1")
	words := strings.Fields(t)
	out := make([]string, 0, len(words))
	for i := 0; i < len(words); i++ {
		w := strings.Trim(words[i], ".;")
		if tens, isTens := tensWords[w]; isTens {
			if i+1 < len(words) {
				if u, isUnit := unitWords[strings.Trim(words[i+1], ".;")]; isUnit && u < 10 {
					out = append(out, strconv.Itoa(tens+u))
					i++
					continue
				}
			}
			out = append(out, strconv.Itoa(tens))
			continue
		}
		if u, isUnit := unitWords[w]; isUnit {
			out = append(out, strconv.Itoa(u))
			continue
		}
		out = append(out, words[i])
	}
	return strings.Join(out, " ")
}

func parse(s string) float64 {
	v, _ := strconv.ParseFloat(s, 64)
	return v
}
