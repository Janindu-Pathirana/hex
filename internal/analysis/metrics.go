package analysis

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Metric is one computed number with its working, its period and where the
// inputs came from.
type Metric struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Group      string   `json:"group"`
	Unit       Unit     `json:"unit"`
	Value      *float64 `json:"value"`
	Display    string   `json:"display"`
	Period     string   `json:"period,omitempty"`
	Steps      []string `json:"steps,omitempty"`
	Note       string   `json:"note,omitempty"`
	Skipped    bool     `json:"skipped,omitempty"`    // not applicable to this kind of company
	Confidence float64  `json:"confidence,omitempty"` // lowest confidence among the inputs, 0 when none are stored
	Sources    []string `json:"sources,omitempty"`    // fundamental sources used (pdf, llm, manual)
	Missing    []string `json:"missing,omitempty"`    // inputs that were not available
	Used       []string `json:"-"`                    // stored fields that fed this metric
}

// Metrics is the full set computed for a snapshot.
type Metrics struct{ List []Metric }

// Get returns a metric by ID.
func (m Metrics) Get(id string) (Metric, bool) {
	for _, x := range m.List {
		if x.ID == id {
			return x, true
		}
	}
	return Metric{}, false
}

// Val returns a metric's value when it was computed.
func (m Metrics) Val(id string) (float64, bool) {
	x, ok := m.Get(id)
	if !ok || x.Value == nil {
		return 0, false
	}
	return *x.Value, true
}

// src collects the inputs for one metric and where they came from.
type src struct {
	vals    Values
	used    []string
	conf    float64
	sources map[string]bool
	end     time.Time
	problem string
	period  string
}

func newSrc() *src { return &src{vals: Values{}, sources: map[string]bool{}} }

func (x *src) note(conf float64, source string) {
	if conf > 0 && (x.conf == 0 || conf < x.conf) {
		x.conf = conf
	}
	if source != "" {
		x.sources[source] = true
	}
}

// flow adds an income or cash-flow amount and checks its period matches the others.
func (x *src) flow(key, field string, f Flow, ok bool) *src {
	if !ok {
		return x
	}
	x.vals[key] = f.Value
	x.used = append(x.used, field)
	for _, s := range f.Sources {
		x.note(0, s)
	}
	x.note(f.Confidence, "")
	if x.period == "" {
		x.period = f.Basis
	}
	if f.Stale {
		x.period = f.Basis + " (interim periods incomplete)"
	} else if !x.end.IsZero() && !x.end.Equal(f.End) {
		x.problem = fmt.Sprintf("periods differ (%s vs %s)", x.end.Format(dateLayout), f.End.Format(dateLayout))
	}
	x.end = f.End
	return x
}

// point adds a balance-sheet value.
func (x *src) point(key, field string, p Point, ok bool) *src {
	if !ok {
		return x
	}
	x.vals[key] = p.Value
	x.used = append(x.used, field)
	x.note(p.Confidence, p.Source)
	return x
}

func (x *src) set(key string, v float64) *src {
	if v > 0 {
		x.vals[key] = v
	}
	return x
}

type builder struct {
	s   *Snapshot
	fin bool
	out []Metric
}

// Compute derives every metric the stored data supports. Formulas live in
// Registry; this layer maps stored fields onto their inputs.
func Compute(s *Snapshot) Metrics {
	b := &builder{s: s, fin: IsFinancial(s.Sector)}

	profitField := firstField(s, "pat_parent", "pat")
	revField := firstField(s, "revenue", "gross_income", "total_operating_income")
	profit, hasProfit := s.TTM(profitField)

	// Valuation
	b.emit("market-cap", "market-cap", "", newSrc().set("price", s.Price).set("shares", s.Shares), "")
	eps, hasEPS := b.eps(profit, hasProfit)
	b.emit("pe", "pe", "", newSrc().set("price", s.Price).setOK("eps", eps, hasEPS), "")
	b.emit("earnings-yield", "earnings-yield", "", newSrc().setOK("eps", eps, hasEPS).set("price", s.Price), "")
	nav, hasNAV := b.nav()
	b.emit("pb", "pb", "", newSrc().set("price", s.Price).setOK("nav_per_share", nav, hasNAV), "")
	b.emit("graham", "graham", "", newSrc().setOK("eps", eps, hasEPS).setOK("nav_per_share", nav, hasNAV), "")

	// Profitability
	b.returnOn("roe", "equity_parent", "equity", profitField, profit, hasProfit)
	b.returnOn("roa", "total_assets", "assets", profitField, profit, hasProfit)
	rev, hasRev := s.TTM(revField)
	for _, m := range []struct{ id, key, field string }{
		{"gross-margin", "gross_profit", "gross_profit"},
		{"operating-margin", "operating_profit", "operating_profit"},
		{"net-margin", "net_profit", profitField},
	} {
		p, ok := s.TTM(m.field)
		b.emit(m.id, m.id, "", newSrc().flow(m.key, m.field, p, ok).flow("revenue", revField, rev, hasRev), "")
	}

	// Financial health
	{
		x := newSrc()
		ca, ok1 := s.Balance("current_assets")
		cl, ok2 := s.Balance("current_liabilities")
		x.point("current_assets", "current_assets", ca, ok1).point("current_liabilities", "current_liabilities", cl, ok2)
		if ok1 && ok2 && !ca.End.Equal(cl.End) {
			x.problem = "balance sheet dates differ"
		}
		b.emit("current-ratio", "current-ratio", "", x, "")
	}
	{
		te, ok := s.Balance("total_equity")
		b.emit("debt-equity", "debt-equity", "", newSrc().point("total_equity", "total_equity", te, ok), "")
	}
	{
		x := newSrc()
		op, ok1 := s.TTM("operating_profit")
		fc, ok2 := s.TTM("finance_cost")
		x.flow("operating_profit", "operating_profit", op, ok1).flow("finance_cost", "finance_cost", fc, ok2)
		b.emit("interest-cover", "interest-cover", "", x, "")
	}

	// Cash flow
	{
		ocf, ok1 := s.TTM("ocf")
		x := newSrc().flow("ocf", "ocf", ocf, ok1)
		if hasProfit {
			x.flow("net_profit", profitField, profit, true)
		}
		b.emit("cash-conversion", "cash-conversion", "", x, "")
		capex, ok2 := s.TTM("capex")
		b.emit("fcf", "fcf", "", newSrc().flow("ocf", "ocf", ocf, ok1).flow("capex", "capex", capex, ok2), "")
	}

	// Income
	{
		dps, ok := s.TTM("dps")
		b.emit("dividend-yield", "dividend-yield", "", newSrc().flow("dps", "dps", dps, ok).set("price", s.Price), "")
		b.emit("payout", "payout", "", newSrc().flow("dps", "dps", dps, ok).setOK("eps", eps, hasEPS), "")
	}

	// Growth
	b.growth("revenue-growth", "Revenue growth", revField)
	profG := b.growth("profit-growth", "Profit growth", profitField)
	epsG := b.growth("eps-growth", "EPS growth", "eps_basic")
	g, hasG := epsG, epsG != nil
	if !hasG {
		g, hasG = profG, profG != nil
	}
	x := newSrc().set("price", s.Price).setOK("eps", eps, hasEPS)
	if hasG {
		x.vals["eps_growth"] = *g * 100
	}
	b.emit("peg", "peg", "", x, "")

	list := append(b.out, computeTrends(s, b.fin, profitField, revField)...)
	list = append(list, computePiotroski(s, b.fin, profitField, revField)...)
	list = append(list, computeTechnical(s)...)
	list = append(list, computeLiquidity(s)...)
	return Metrics{List: append(list, computeMarket(s)...)}
}

func (x *src) setOK(key string, v float64, ok bool) *src {
	if ok {
		x.vals[key] = v
	}
	return x
}

func firstField(s *Snapshot, names ...string) string {
	for _, n := range names {
		if len(s.Series(n)) > 0 {
			return n
		}
	}
	return names[0]
}

// emit runs a registry ratio on the collected inputs and stores the result.
func (b *builder) emit(id, ratioID, name string, x *src, period string) {
	r, _ := Lookup(ratioID)
	if name == "" {
		name = r.Name
	}
	m := Metric{ID: id, Name: name, Group: r.Group, Unit: r.Unit, Display: "-"}
	if period == "" {
		period = x.period
	}
	m.Period = period
	switch {
	case r.SkipFinancials && b.fin:
		m.Skipped, m.Display, m.Note = true, "n/a", "not meaningful for banks, finance and insurance companies"
	case x.problem != "":
		m.Note = x.problem
	default:
		if miss := r.Missing(x.vals); len(miss) > 0 {
			for _, i := range miss {
				m.Missing = append(m.Missing, i.Label)
			}
			m.Note = "missing: " + strings.Join(m.Missing, "; ")
			break
		}
		res := r.Compute(x.vals)
		m.Value, m.Display, m.Steps = res.Value, res.Display, res.Steps
		if res.Note != "" {
			m.Note = res.Note
		}
		m.Used, m.Confidence = dedupe(x.used), x.conf
		for s := range x.sources {
			m.Sources = append(m.Sources, s)
		}
		sort.Strings(m.Sources)
	}
	b.out = append(b.out, m)
}

// eps returns trailing-twelve-month EPS: the reported figure when stored,
// otherwise profit ÷ shares in issue.
func (b *builder) eps(profit Flow, hasProfit bool) (float64, bool) {
	if f, ok := b.s.TTM("eps_basic"); ok {
		v := f.Value
		b.out = append(b.out, Metric{ID: "eps", Name: "Earnings per share (EPS)", Group: "valuation", Unit: LKR,
			Value: &v, Display: Format(v, LKR), Period: f.Basis, Used: []string{"eps_basic"}, Confidence: f.Confidence,
			Sources: f.Sources, Steps: []string{"EPS = reported basic EPS, " + f.Basis}})
		return v, true
	}
	x := newSrc().set("shares", b.s.Shares)
	if hasProfit {
		x.flow("net_profit", firstField(b.s, "pat_parent", "pat"), profit, true)
	}
	b.emit("eps", "eps", "", x, "")
	m, _ := Metrics{List: b.out}.Get("eps")
	if m.Value == nil {
		return 0, false
	}
	return *m.Value, true
}

// nav returns book value per share: the reported figure when stored,
// otherwise parent equity ÷ shares in issue.
func (b *builder) nav() (float64, bool) {
	if p, ok := b.s.Balance("nav_per_share"); ok {
		v := p.Value
		b.out = append(b.out, Metric{ID: "bvps", Name: "Book value (NAV) per share", Group: "valuation", Unit: LKR,
			Value: &v, Display: Format(v, LKR), Period: "at " + p.End.Format(dateLayout), Used: []string{"nav_per_share"},
			Confidence: p.Confidence, Sources: []string{p.Source}, Steps: []string{"NAV per share = reported, at " + p.End.Format(dateLayout)}})
		return v, true
	}
	eq, ok := b.s.Balance("equity_parent")
	x := newSrc().point("equity_close", "equity_parent", eq, ok).set("shares", b.s.Shares)
	if ok {
		x.period = "at " + eq.End.Format(dateLayout)
	}
	b.emit("bvps", "bvps", "", x, "")
	m, _ := Metrics{List: b.out}.Get("bvps")
	if m.Value == nil {
		return 0, false
	}
	return *m.Value, true
}

// returnOn emits ROE or ROA on trailing profit and the balance at the end of
// that period, averaged with the balance a year earlier when stored.
func (b *builder) returnOn(id, field, what, profitField string, profit Flow, hasProfit bool) {
	x := newSrc()
	if hasProfit {
		x.flow("net_profit", profitField, profit, true)
		closing, ok := b.s.BalanceNear(field, profit.End)
		if !ok {
			closing, ok = b.s.Balance(field)
		}
		x.point(what+"_close", field, closing, ok)
		if ok {
			if open, ok := b.s.BalanceNear(field, yearBefore(closing.End)); ok {
				x.point(what+"_open", field, open, true)
			}
		}
	}
	b.emit(id, id, "", x, "")
}

// growth emits year-on-year growth of a field: the latest period against the
// same period a year earlier. It returns the growth as a fraction when computed.
func (b *builder) growth(id, name, field string) *float64 {
	cur, prev, ok := b.s.YoY(field)
	x := newSrc()
	if ok {
		x.vals["current"], x.vals["previous"] = cur.Value, prev.Value
		x.used = append(x.used, field)
		x.note(min(cur.Confidence, prev.Confidence), cur.Source)
		x.period = fmt.Sprintf("%s to %s vs %s", cur.Type, cur.End.Format(dateLayout), prev.End.Format(dateLayout))
	}
	b.emit(id, "growth", name, x, "")
	m := b.out[len(b.out)-1]
	m.Group = "growth"
	b.out[len(b.out)-1] = m
	return m.Value
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
