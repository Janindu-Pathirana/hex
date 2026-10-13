package analysis

import (
	"math"
	"strings"
	"testing"
)

// Hand-computed cases, mostly from the fundamental-analysis skill's ratios.md.
// want is nil when the ratio should be "n/m".
var cases = map[string][]struct {
	name string
	in   Values
	want *float64
}{
	"net-margin":       {{"basic", Values{"net_profit": 150, "revenue": 1000}, f(0.15)}, {"no revenue", Values{"net_profit": 1, "revenue": 0}, nil}},
	"gross-margin":     {{"basic", Values{"gross_profit": 400, "revenue": 1000}, f(0.40)}},
	"operating-margin": {{"basic", Values{"operating_profit": 250, "revenue": 1000}, f(0.25)}},
	"roe": {
		{"average equity", Values{"net_profit": 1200, "equity_open": 9000, "equity_close": 11000}, f(0.12)},
		{"closing only", Values{"net_profit": 1200, "equity_close": 10000}, f(0.12)},
		{"negative equity", Values{"net_profit": 10, "equity_close": -5}, nil},
	},
	"roa":            {{"average assets", Values{"net_profit": 50, "assets_open": 900, "assets_close": 1100}, f(0.05)}},
	"market-cap":     {{"basic", Values{"price": 18.2, "shares": 1000}, f(18200)}},
	"eps":            {{"basic", Values{"net_profit": 1250, "shares": 100}, f(12.5)}},
	"pe":             {{"basic", Values{"price": 100, "eps": 12.5}, f(8)}, {"loss", Values{"price": 100, "eps": -2}, nil}},
	"earnings-yield": {{"basic", Values{"eps": 12.5, "price": 100}, f(0.125)}},
	"bvps":           {{"basic", Values{"equity_close": 4000, "shares": 100}, f(40)}},
	"pb":             {{"basic", Values{"price": 50, "nav_per_share": 40}, f(1.25)}, {"negative book", Values{"price": 50, "nav_per_share": -1}, nil}},
	"peg":            {{"basic", Values{"price": 100, "eps": 10, "eps_growth": 20}, f(0.5)}, {"shrinking", Values{"price": 100, "eps": 10, "eps_growth": -5}, nil}},
	"graham":         {{"basic", Values{"eps": 2, "nav_per_share": 20}, f(math.Sqrt(900))}, {"loss", Values{"eps": -1, "nav_per_share": 20}, nil}},
	"dividend-yield": {{"basic", Values{"dps": 4, "price": 80}, f(0.05)}, {"no dividend", Values{"dps": 0, "price": 80}, f(0)}},
	"payout":         {{"basic", Values{"dps": 4, "eps": 10}, f(0.4)}, {"loss", Values{"dps": 4, "eps": -1}, nil}},
	"debt-equity":    {{"basic", Values{"debt": 500, "total_equity": 1000}, f(0.5)}},
	"current-ratio":  {{"basic", Values{"current_assets": 300, "current_liabilities": 200}, f(1.5)}},
	"growth": {
		{"up", Values{"current": 12, "previous": 10}, f(0.2)},
		{"turnaround", Values{"current": 3, "previous": -5}, nil},
	},
}

func f(v float64) *float64 { return &v }

func TestRegistry(t *testing.T) {
	seen := map[string]bool{}
	for _, r := range Registry {
		if seen[r.ID] {
			t.Errorf("duplicate ratio id %q", r.ID)
		}
		seen[r.ID] = true
		tcs, ok := cases[r.ID]
		if !ok {
			t.Errorf("ratio %q has no test cases", r.ID)
			continue
		}
		for _, tc := range tcs {
			t.Run(r.ID+"/"+tc.name, func(t *testing.T) {
				got, err := r.Calc(tc.in)
				if err != nil {
					t.Fatal(err)
				}
				switch {
				case tc.want == nil && got.Value != nil:
					t.Fatalf("got %v, want n/m", *got.Value)
				case tc.want == nil:
					if got.Display != "n/m" || got.Note == "" {
						t.Fatalf("n/m result should explain itself: %+v", got)
					}
				case got.Value == nil:
					t.Fatalf("got n/m (%s), want %v", got.Note, *tc.want)
				case math.Abs(*got.Value-*tc.want) > 1e-9:
					t.Fatalf("got %v, want %v", *got.Value, *tc.want)
				case len(got.Steps) == 0:
					t.Fatal("computed result has no steps")
				}
			})
		}
	}
	for id := range cases {
		if !seen[id] {
			t.Errorf("test cases for unknown ratio %q", id)
		}
	}
}

func TestCalcReportsMissingFlags(t *testing.T) {
	r, _ := Lookup("roe")
	_, err := r.Calc(Values{"net_profit": 1})
	if err == nil || !strings.Contains(err.Error(), "--equity-close") {
		t.Fatalf("got %v, want missing --equity-close", err)
	}
}

func TestROESteps(t *testing.T) {
	r, _ := Lookup("roe")
	got, _ := r.Calc(Values{"net_profit": 1200, "equity_open": 9000, "equity_close": 11000})
	want := []string{
		"average equity = (9,000 + 11,000) ÷ 2 = 10,000",
		"ROE = 1,200 ÷ 10,000 = 12.00%",
	}
	if strings.Join(got.Steps, "\n") != strings.Join(want, "\n") {
		t.Fatalf("steps:\n%s", strings.Join(got.Steps, "\n"))
	}
}

func TestNum(t *testing.T) {
	tests := map[float64]string{
		0:            "0",
		0.004:        "0.004",
		-0.046:       "-0.046",
		18.2:         "18.2",
		1051.75:      "1,051.75",
		141647460000: "141,647,460,000",
		-117062877:   "-117,062,877",
		-0.00001:     "0",
	}
	for in, want := range tests {
		if got := Num(in); got != want {
			t.Errorf("Num(%v) = %q, want %q", in, got, want)
		}
	}
}
