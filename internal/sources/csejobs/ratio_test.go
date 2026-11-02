package csejobs_test

import (
	"math"
	"testing"

	"github.com/Janindu-Pathirana/hex/internal/sources/csejobs"
	"github.com/Janindu-Pathirana/hex/internal/store"
)

// The texts are announcements seen on cse.lk on 2026-11-02.
func TestParseRatio(t *testing.T) {
	for _, c := range []struct {
		name, kind, text string
		num, den         float64
		ok               bool
	}{
		{"split digits", store.ActionSplit, "Sub-division of every 01 Ordinary Share into 20 Ordinary Shares.", 20, 1, true},
		{"split words", store.ActionSplit, "Subdividing every one (1) existing ordinary share into four (4) ordinary shares for increasing from 29,582,029 ordinary shares to 118,328,116 ordinary shares, with no change to the stated capital of the Company", 4, 1, true},
		{"split to", store.ActionSplit, "Sub Division of UML Shares- Every existing one (01)share to be sub divided to ten(10) shares", 10, 1, true},
		{"split colon", store.ActionSplit, "SUB DIVISION OF ORDINARY VOTING SHARES OF LANKA MILK FOODS (CWE) PLC AT THE RATIO OF 1:10", 10, 1, true},
		{"split attachment", store.ActionSplit, "Refer to the Attachment.", 0, 0, false},
		{"split title only", store.ActionSplit, "SUB-DIVISION OF SHARES", 0, 0, false},
		{"split without a ratio", store.ActionSplit, "Sub-division of Shares 2024", 0, 0, false},
		{"split empty", store.ActionSplit, "", 0, 0, false},
		{"split must grow the share count", store.ActionSplit, "every 4 shares into 1 share", 0, 0, false},
		{"rights every", store.ActionRights, "03 Ordinary Voting share for every 14 existing Ordinary Voting shares", 3, 14, true},
		{"rights new for every", store.ActionRights, "06 new ordinary shares for every 01 existing ordinary share", 6, 1, true},
		{"rights words and digits", store.ActionRights, "One (01) new Ordinary Voting Share for every One (01) Ordinary Voting Share held", 1, 1, true},
		{"rights allotted to every", store.ActionRights, "04 (Four) new Ordinary Voting Shares will be provisionally allotted to every 11 (Eleven) Ordinary Voting Shares", 4, 11, true},
		{"rights two-word numbers", store.ActionRights, "Twenty eight (28) new ordinary voting share for every sixty one (61) ordinary voting shares held", 28, 61, true},
		{"rights abbreviation", store.ActionRights, "Five new Ord. Shares for every Six Ord. Shares", 5, 6, true},
		{"rights hyphenated numbers", store.ActionRights, "Thirty -Two (32) new Ordinary Shares for every Twenty-Nine (29) Ordinary Voting Shares", 32, 29, true},
		{"rights forty-five", store.ActionRights, "Fourteen (14) New Ordinary Shares for every Forty-Five (45) existing Ordinary Shares", 14, 45, true},
		{"scrip glued for", store.ActionScrip, "1for115.2508544528", 1, 115.2508544528, true},
		{"rights seventeen", store.ActionRights, "Ten (10) new Ordinary Voting Shares for every Seventeen (17) existing Ordinary Voting Share", 10, 17, true},
		{"rights for existing", store.ActionRights, "03 new Shares for existing 04 Shares", 3, 4, true},
		{"rights typo everone", store.ActionRights, "ONE(01) NEW ORDINARY SHARE FOR EVERONE (01) ORDINARY SHARE AS AT THE RECORD DATE.", 1, 1, true},
		{"rights shares held", store.ActionRights, "07 shares for every 04 shares held", 7, 4, true},
		{"scrip for", store.ActionScrip, "1 for 43.90", 1, 43.9, true},
		{"scrip for every", store.ActionScrip, "1 for every 202.32562400", 1, 202.325624, true},
		{"scrip bare number", store.ActionScrip, "168.200036052", 1, 168.200036052, true},
		{"scrip bare small number is not a ratio", store.ActionScrip, "0.0343999821", 0, 0, false},
		{"scrip words with decimals", store.ActionScrip, "One (01) share being issued for each existing thirty three decimal zero two three two five five nine one four one (33.0232559141)", 1, 33.0232559141, true},
		{"scrip no space after every", store.ActionScrip, "01 new Ordinary share for every12.2000000724", 1, 12.2000000724, true},
		{"scrip each existing", store.ActionScrip, "01 new share for each existing 12.2000000724", 1, 12.2000000724, true},
		{"scrip colon", store.ActionScrip, "1:23.5294118079", 1, 23.5294118079, true},
		{"bonus share for every", store.ActionBonus, "1 share for every 17.5287357329", 1, 17.5287357329, true},
		{"colon typo is rejected", store.ActionScrip, "1:250000000899", 0, 0, false},
		{"no ratio in text", store.ActionRights, "AMANA BANK PLC RIGHTS ISSUE - LEVEL OF SUBSCRIPTION", 0, 0, false},
		{"zero side", store.ActionRights, "0 shares for every 4", 0, 0, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			num, den, ok := csejobs.ParseRatio(c.kind, c.text)
			if ok != c.ok || (ok && (math.Abs(num-c.num) > 1e-9 || math.Abs(den-c.den) > 1e-9)) {
				t.Fatalf("ParseRatio(%q) = %v, %v, %v; want %v, %v, %v", c.text, num, den, ok, c.num, c.den, c.ok)
			}
			if !ok && (num != 0 || den != 0) {
				t.Errorf("a failed parse must return zeros, got %v, %v", num, den)
			}
		})
	}
}
