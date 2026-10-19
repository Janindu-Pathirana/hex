package analysis

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"
)

// Disclaimer goes on every analysis output.
const Disclaimer = "Data: cse.lk, may be delayed. Figures read from report PDFs should be checked against the report. For information only, not investment advice."

// Coverage says which stored values the analysis used and what it lacked.
type Coverage struct {
	Used    []string `json:"used"`    // stored fields that fed at least one metric
	Unused  []string `json:"unused"`  // stored fields no metric or rule reads yet
	Missing []string `json:"missing"` // metrics that could not be computed, with the reason
}

// Report is the full analysis of one company.
type Report struct {
	Symbol     string    `json:"symbol"`
	Name       string    `json:"name,omitempty"`
	Sector     string    `json:"sector,omitempty"`
	AsOf       string    `json:"asOf"`
	Price      *float64  `json:"price"`
	Metrics    []Metric  `json:"metrics"`
	Findings   []Finding `json:"findings"`
	Coverage   Coverage  `json:"coverage"`
	Disclaimer string    `json:"disclaimer"`
}

// Analyze computes the metrics and findings for a snapshot.
func Analyze(s *Snapshot) Report {
	m := Compute(s)
	r := Report{Symbol: s.Code, Name: s.Name, Sector: s.Sector, AsOf: s.AsOf.Format(dateLayout),
		Metrics: m.List, Findings: Evaluate(s, m), Disclaimer: Disclaimer}
	if s.Price > 0 {
		p := s.Price
		r.Price = &p
	}
	r.Coverage = coverage(s, m)
	return r
}

func coverage(s *Snapshot, m Metrics) Coverage {
	used := map[string]bool{}
	var cov Coverage
	for _, x := range m.List {
		for _, f := range x.Used {
			used[f] = true
		}
		if x.Value == nil && !x.Skipped && x.Note != "" {
			cov.Missing = append(cov.Missing, x.ID+": "+x.Note)
		}
	}
	for _, f := range s.Fields() {
		if used[f] {
			cov.Used = append(cov.Used, f)
		} else {
			cov.Unused = append(cov.Unused, f)
		}
	}
	return cov
}

const groupTechnical = "technical"

var groupOrder = []string{"valuation", "profitability", "growth", "health", "cashflow", "income", groupTechnical}

var groupTitle = map[string]string{
	"valuation": "Valuation", "profitability": "Profitability", "growth": "Growth",
	"health": "Financial health", "cashflow": "Cash flow", "income": "Dividends",
	groupTechnical: "Price and volume",
}

// groupDimension maps a metric group to the finding dimension that covers it.
var groupDimension = map[string]string{
	"valuation": DimValuation, "profitability": DimProfitability, "growth": DimGrowth,
	"health": DimHealth, "cashflow": DimCash, "income": DimDividends,
	groupTechnical: DimTechnical,
}

// Sections lists the names accepted by Filter.
func Sections() []string { return append([]string(nil), groupOrder...) }

// Filter keeps only the named metric groups and their findings. Data quality
// findings always stay.
func (r *Report) Filter(sections []string) error {
	keep := map[string]bool{}
	for _, s := range sections {
		s = strings.ToLower(strings.TrimSpace(s))
		if _, ok := groupDimension[s]; !ok {
			return fmt.Errorf("unknown section %q; choose from %s", s, strings.Join(groupOrder, ", "))
		}
		keep[s] = true
	}
	var ms []Metric
	for _, m := range r.Metrics {
		if keep[m.Group] {
			ms = append(ms, m)
		}
	}
	var fs []Finding
	for _, f := range r.Findings {
		if f.Dimension == DimData || keep[dimensionGroup(f.Dimension)] {
			fs = append(fs, f)
		}
	}
	r.Metrics, r.Findings = ms, fs
	return nil
}

func dimensionGroup(dim string) string {
	for g, d := range groupDimension {
		if d == dim {
			return g
		}
	}
	return ""
}

// Render writes the report as plain text.
func (r Report) Render(w io.Writer) error {
	title := r.Symbol
	if r.Name != "" {
		title += "  " + r.Name
	}
	fmt.Fprintln(w, title)
	var meta []string
	if r.Sector != "" {
		meta = append(meta, r.Sector)
	}
	if r.Price != nil {
		meta = append(meta, "price "+Num(*r.Price))
	}
	meta = append(meta, "as of "+r.AsOf)
	fmt.Fprintln(w, strings.Join(meta, " · "))

	for _, g := range groupOrder {
		var rows []Metric
		for _, m := range r.Metrics {
			if m.Group == g && !m.Skipped {
				rows = append(rows, m)
			}
		}
		if len(rows) == 0 {
			continue
		}
		fmt.Fprintf(w, "\n%s\n", groupTitle[g])
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		for _, m := range rows {
			detail := m.Period
			if m.Value == nil {
				detail = m.Note
			} else if m.Confidence > 0 && m.Confidence < lowConfidence {
				detail += " (low confidence)"
			}
			fmt.Fprintf(tw, "  %s\t%s\t%s\n", m.Name, m.Display, detail)
		}
		if err := tw.Flush(); err != nil {
			return err
		}
	}

	fmt.Fprintln(w, "\nFindings")
	if len(r.Findings) == 0 {
		fmt.Fprintln(w, "  none: not enough stored data")
	}
	for _, sev := range []Severity{Risk, Watch, Positive, Neutral} {
		for _, f := range r.Findings {
			if f.Severity == sev {
				fmt.Fprintf(w, "  [%s] %s: %s. %s\n", sev, f.Dimension, f.Title, f.Detail)
			}
		}
	}

	fmt.Fprintf(w, "\nCoverage: %d stored fields used", len(r.Coverage.Used))
	if len(r.Coverage.Unused) > 0 {
		fmt.Fprintf(w, "; not read by any metric yet: %s", strings.Join(r.Coverage.Unused, ", "))
	}
	fmt.Fprintln(w)
	if len(r.Coverage.Missing) > 0 {
		missing := append([]string(nil), r.Coverage.Missing...)
		sort.Strings(missing)
		fmt.Fprintln(w, "Not computed:")
		for _, m := range missing {
			fmt.Fprintln(w, "  "+m)
		}
	}
	fmt.Fprintf(w, "\n%s\n", r.Disclaimer)
	return nil
}
