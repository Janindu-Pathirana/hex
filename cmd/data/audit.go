// Package data builds the stored-data audit behind "hex data audit".
package data

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/Janindu-Pathirana/hex/internal/store"
)

// Thresholds that turn counts into flags. They are guides, not rules.
const (
	minBars         = 120 // fewer daily bars than this is a short history
	maxGapDays      = 10  // calendar days between two bars before it is called a gap
	staleBarDays    = 7   // newest bar older than this is out of date
	zeroVolumeShare = 0.2 // share of bars without volume before it is flagged
	minPeriods      = 5   // statement periods needed for TTM and year-on-year
	staleStatement  = 200 // days since the newest statement period
	lowConfShare    = 0.3 // share of low-confidence values before it is flagged
	staleQuoteDays  = 3
	lowConfidence   = 0.7 // matches reportjobs.LowConfidence
)

// Report is the audit of what the store holds.
type Report struct {
	AsOf      string    `json:"asOf"`
	Market    Market    `json:"market"`
	Companies []Company `json:"companies"`
	Flagged   int       `json:"flagged"` // companies with at least one flag
}

// Market covers the data that is not tied to one company.
type Market struct {
	Securities int      `json:"securities"`
	Indexes    []Index  `json:"indexes"`       // ASPI and S&P SL20 only
	Sectors    int      `json:"sectorIndexes"` // other stored index series
	MarketDays int      `json:"marketDays"`
	Actions    int      `json:"actions"`
	Flags      []string `json:"flags,omitempty"`
}

// Index is one stored index series.
type Index struct {
	ID    string `json:"id"`
	Days  int    `json:"days"`
	First string `json:"first"`
	Last  string `json:"last"`
}

// Company is the audit of one company.
type Company struct {
	Code         string        `json:"code"`
	Name         string        `json:"name,omitempty"`
	Bars         *Bars         `json:"bars"`
	Fundamentals *Fundamentals `json:"fundamentals"`
	Reports      *Reports      `json:"reports"`
	QuoteAgeDays *int          `json:"quoteAgeDays"`
	Flags        []string      `json:"flags,omitempty"`
}

// Bars summarises price history.
type Bars struct {
	Count      int    `json:"count"`
	First      string `json:"first"`
	Last       string `json:"last"`
	MaxGapDays int    `json:"maxGapDays"`
	ZeroVolume int    `json:"zeroVolume"`
}

// Fundamentals summarises statement values.
type Fundamentals struct {
	Periods int     `json:"periods"`
	Values  int     `json:"values"`
	Newest  string  `json:"newest"`
	Low     int     `json:"lowConfidence"`
	MinConf float64 `json:"minConfidence"`
}

// Reports summarises the report pipeline.
type Reports struct {
	Quarterly  int `json:"quarterly"`
	Annual     int `json:"annual"`
	Downloaded int `json:"downloaded"`
	Unreadable int `json:"unreadable"` // scanned or failed text
	Parsed     int `json:"parsed"`
}

// Build audits the companies in codes, or every company with stored price,
// statement or report data when codes is empty.
func Build(ctx context.Context, db *store.DB, now time.Time, codes []string) (Report, error) {
	secs, err := db.Securities(ctx)
	if err != nil {
		return Report{}, err
	}
	names := map[string]string{}
	for _, s := range secs {
		names[s.Code] = s.Name
	}
	bars, err := db.BarStats(ctx)
	if err != nil {
		return Report{}, err
	}
	funds, err := db.FundamentalStats(ctx, lowConfidence)
	if err != nil {
		return Report{}, err
	}
	reps, err := db.ReportStats(ctx)
	if err != nil {
		return Report{}, err
	}
	quotes, err := db.QuoteTimes(ctx)
	if err != nil {
		return Report{}, err
	}
	idx, err := db.IndexStats(ctx)
	if err != nil {
		return Report{}, err
	}
	days, err := db.MarketDays(ctx)
	if err != nil {
		return Report{}, err
	}
	acts, err := db.Actions(ctx, "")
	if err != nil {
		return Report{}, err
	}

	bm := map[string]store.BarStat{}
	for _, b := range bars {
		bm[b.Code] = b
	}
	fm := map[string]store.FundamentalStat{}
	for _, f := range funds {
		fm[f.Code] = f
	}
	rm := map[string]store.ReportStat{}
	for _, r := range reps {
		rm[r.Code] = r
	}

	if len(codes) == 0 {
		seen := map[string]bool{}
		for c := range bm {
			seen[c] = true
		}
		for c := range fm {
			seen[c] = true
		}
		for _, r := range reps {
			if r.Downloaded > 0 {
				seen[r.Code] = true
			}
		}
		for c := range seen {
			codes = append(codes, c)
		}
	}
	sort.Strings(codes)

	rep := Report{AsOf: now.Format("2006-01-02")}
	rep.Market = Market{Securities: len(secs), MarketDays: len(days), Actions: len(acts)}
	for _, i := range idx {
		if i.IndexID != "ASPI" && i.IndexID != "SPSL20" {
			rep.Market.Sectors++
			continue
		}
		rep.Market.Indexes = append(rep.Market.Indexes, Index{ID: i.IndexID, Days: i.Days, First: i.First, Last: i.Last})
	}
	rep.Market.Flags = marketFlags(rep.Market)

	for _, code := range codes {
		c := Company{Code: code, Name: names[code]}
		if b, ok := bm[code]; ok {
			c.Bars = &Bars{Count: b.Bars, First: b.First, Last: b.Last, MaxGapDays: b.MaxGapDays, ZeroVolume: b.ZeroVolume}
		}
		if f, ok := fm[code]; ok {
			c.Fundamentals = &Fundamentals{Periods: f.Periods, Values: f.Values, Newest: f.Newest, Low: f.Low, MinConf: f.MinConf}
		}
		if r, ok := rm[code]; ok {
			c.Reports = &Reports{Quarterly: r.Quarterly, Annual: r.Annual, Downloaded: r.Downloaded,
				Unreadable: r.Scanned + r.Failed, Parsed: r.Parsed}
		}
		if at, ok := quotes[code]; ok {
			age := int(now.Sub(at).Hours() / 24)
			c.QuoteAgeDays = &age
		}
		c.Flags = companyFlags(c, now)
		if len(c.Flags) > 0 {
			rep.Flagged++
		}
		rep.Companies = append(rep.Companies, c)
	}
	return rep, nil
}

func marketFlags(m Market) []string {
	var fl []string
	have := map[string]Index{}
	for _, i := range m.Indexes {
		have[i.ID] = i
	}
	for _, id := range []string{"ASPI", "SPSL20"} {
		i, ok := have[id]
		switch {
		case !ok:
			fl = append(fl, id+": no stored levels; run 'hex sync index'")
		case i.Days < 200:
			fl = append(fl, fmt.Sprintf("%s: only %d days stored; run 'hex sync index'", id, i.Days))
		}
	}
	if m.MarketDays < 21 {
		fl = append(fl, fmt.Sprintf("market statistics cover %d days; turnover and foreign flow need 21 and build up as 'hex sync market' keeps running", m.MarketDays))
	}
	return fl
}

func companyFlags(c Company, now time.Time) []string {
	var fl []string
	if b := c.Bars; b == nil {
		fl = append(fl, "no price history; run 'hex sync history'")
	} else {
		if b.Count < minBars {
			fl = append(fl, fmt.Sprintf("short price history: %d bars", b.Count))
		}
		if b.MaxGapDays > maxGapDays {
			fl = append(fl, fmt.Sprintf("gap of %d days in prices", b.MaxGapDays))
		}
		if last, err := time.Parse("2006-01-02", b.Last); err == nil && int(now.Sub(last).Hours()/24) > staleBarDays {
			fl = append(fl, "prices end "+b.Last)
		}
		if b.Count > 0 && float64(b.ZeroVolume)/float64(b.Count) > zeroVolumeShare {
			fl = append(fl, fmt.Sprintf("no volume recorded on %d of %d bars", b.ZeroVolume, b.Count))
		}
	}
	if f := c.Fundamentals; f == nil {
		fl = append(fl, "no statement values; run 'hex sync pdfs fundamentals'")
	} else {
		if f.Periods < minPeriods {
			fl = append(fl, fmt.Sprintf("only %d statement periods; trailing twelve months and year-on-year need %d", f.Periods, minPeriods))
		}
		if end, err := time.Parse("2006-01-02", f.Newest); err == nil && int(now.Sub(end).Hours()/24) > staleStatement {
			fl = append(fl, "newest statement period is "+f.Newest)
		}
		if f.Values > 0 && float64(f.Low)/float64(f.Values) > lowConfShare {
			fl = append(fl, fmt.Sprintf("%d of %d statement values have low confidence", f.Low, f.Values))
		}
	}
	if r := c.Reports; r != nil {
		if r.Downloaded > 0 && r.Parsed == 0 {
			fl = append(fl, "reports downloaded but no values extracted")
		}
		if r.Unreadable > 0 {
			fl = append(fl, fmt.Sprintf("%d reports have no readable text", r.Unreadable))
		}
	}
	switch {
	case c.QuoteAgeDays == nil:
		fl = append(fl, "no stored quote")
	case *c.QuoteAgeDays > staleQuoteDays:
		fl = append(fl, fmt.Sprintf("quote is %d days old", *c.QuoteAgeDays))
	}
	return fl
}

// Render writes the audit as plain text.
func (r Report) Render(w io.Writer) error {
	fmt.Fprintf(w, "Stored data as of %s\n\n", r.AsOf)
	fmt.Fprintf(w, "Market: %d securities, %d market days, %d announcements\n", r.Market.Securities, r.Market.MarketDays, r.Market.Actions)
	for _, i := range r.Market.Indexes {
		fmt.Fprintf(w, "  %-8s %4d days  %s to %s\n", i.ID, i.Days, i.First, i.Last)
	}
	if r.Market.Sectors > 0 {
		fmt.Fprintf(w, "  plus %d sector index series\n", r.Market.Sectors)
	}
	for _, f := range r.Market.Flags {
		fmt.Fprintf(w, "  ! %s\n", f)
	}
	fmt.Fprintln(w)

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "COMPANY\tBARS\tFIRST\tLAST\tGAP\tPERIODS\tNEWEST\tLOW\tREPORTS\tFLAGS")
	for _, c := range r.Companies {
		bars, first, last, gap := "-", "-", "-", "-"
		if b := c.Bars; b != nil {
			bars, first, last, gap = fmt.Sprint(b.Count), b.First, b.Last, fmt.Sprint(b.MaxGapDays)
		}
		periods, newest, low := "-", "-", "-"
		if f := c.Fundamentals; f != nil {
			periods, newest, low = fmt.Sprint(f.Periods), f.Newest, fmt.Sprintf("%d/%d", f.Low, f.Values)
		}
		reports := "-"
		if rp := c.Reports; rp != nil {
			reports = fmt.Sprintf("%d/%d/%d", rp.Parsed, rp.Downloaded, rp.Quarterly+rp.Annual)
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%d\n", c.Code, bars, first, last, gap, periods, newest, low, reports, len(c.Flags))
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	fmt.Fprintln(w, "\nBARS daily price bars; GAP longest gap in days; PERIODS statement periods; LOW low-confidence/all values; REPORTS parsed/downloaded/catalogued.")
	fmt.Fprintf(w, "%d of %d companies have flags.\n", r.Flagged, len(r.Companies))
	for _, c := range r.Companies {
		if len(c.Flags) == 0 {
			continue
		}
		fmt.Fprintf(w, "\n%s\n  %s\n", c.Code, strings.Join(c.Flags, "\n  "))
	}
	return nil
}
