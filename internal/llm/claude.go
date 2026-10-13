// Package llm re-reads financial statements with Claude when the rule-based
// extractor is not confident or a report is scanned. Every value Claude
// returns is checked before it is stored: the printed figure must appear on
// the page it cites, and the accounting identities score it like the rules.
package llm

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync/atomic"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/packages/param"
	"github.com/anthropics/anthropic-sdk-go/shared/constant"

	"github.com/Janindu-Pathirana/hex/internal/engine"
	"github.com/Janindu-Pathirana/hex/internal/extract"
	"github.com/Janindu-Pathirana/hex/internal/sources/reportjobs"
	"github.com/Janindu-Pathirana/hex/internal/store"
)

// DefaultModel is the Claude model used unless --llm-model says otherwise.
const DefaultModel = "claude-opus-5-5"

// PromptVersion identifies the prompt and schema; it is stored with each run.
const PromptVersion = "llm-1"

// maxPDFBytes keeps scanned PDFs under the API's request size limit.
const maxPDFBytes = 30 << 20

// Errors returned by Extract.
var (
	ErrBudget  = errors.New("llm report budget used up")
	ErrRefused = errors.New("model declined the request")
)

// Claude extracts statements with the Anthropic API. It is safe for
// concurrent use; Max caps how many reports one run may send.
type Claude struct {
	client anthropic.Client
	Model  string
	Max    int32
	used   atomic.Int32
}

// New returns a Claude reader. Credentials come from the environment
// (ANTHROPIC_API_KEY, or an `ant auth login` profile); opts are for tests.
func New(model string, max int, opts ...option.RequestOption) *Claude {
	if model == "" {
		model = DefaultModel
	}
	return &Claude{client: anthropic.NewClient(opts...), Model: model, Max: int32(max)}
}

// Name implements reportjobs.Fallback.
func (c *Claude) Name() string { return c.Model }

const system = `You read the primary financial statements of companies listed on the Colombo Stock Exchange (Sri Lanka) and record their figures exactly as printed.

Record only figures for the current reporting period of the report: for income statements and cash flows, every period the report shows ending on the report date (for example the quarter and the year to date); for the balance sheet, the report date. Skip comparative (prior-year) columns, percentage columns and note numbers.

For each figure give the field, the scope (group = consolidated; company = the parent or bank alone; use group when only one set is shown), the period type, the period end date, the figure exactly as printed in "raw" (keep brackets, commas and decimals), the unit stated on the page, the page number and the line label. Leave out fields the statements do not show. Never compute or estimate a figure.

When you have read the statements, call record_statements once with everything you found.`

func fieldNames() []string {
	names := make([]string, len(extract.Fields))
	for i, f := range extract.Fields {
		names[i] = f.Name
	}
	return names
}

func tool() anthropic.BetaToolUnionParam {
	value := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"field":       map[string]any{"type": "string", "enum": fieldNames()},
			"scope":       map[string]any{"type": "string", "enum": []string{extract.ScopeGroup, extract.ScopeCompany}},
			"period_type": map[string]any{"type": "string", "enum": []string{"3M", "6M", "9M", "12M", extract.PointInTime}},
			"period_end":  map[string]any{"type": "string", "description": "YYYY-MM-DD"},
			"raw":         map[string]any{"type": "string", "description": "the figure exactly as printed"},
			"unit":        map[string]any{"type": "string", "enum": []string{"rupees", "thousands", "millions", "billions", "per_share"}},
			"page":        map[string]any{"type": "integer"},
			"label":       map[string]any{"type": "string"},
		},
		"required":             []string{"field", "scope", "period_type", "period_end", "raw", "unit", "page", "label"},
		"additionalProperties": false,
	}
	t := anthropic.BetaToolParam{
		Name:        "record_statements",
		Description: anthropic.String("Record the current-period figures read from the financial statements."),
		Strict:      param.NewOpt(true),
		InputSchema: anthropic.BetaToolInputSchemaParam{
			Properties: map[string]any{
				"values": map[string]any{"type": "array", "items": value},
			},
			Required:    []string{"values"},
			ExtraFields: map[string]any{"additionalProperties": false},
		},
	}
	return anthropic.BetaToolUnionParam{OfTool: &t}
}

// recorded is one figure as Claude reports it.
type recorded struct {
	Field      string `json:"field"`
	Scope      string `json:"scope"`
	PeriodType string `json:"period_type"`
	PeriodEnd  string `json:"period_end"`
	Raw        string `json:"raw"`
	Unit       string `json:"unit"`
	Page       int    `json:"page"`
	Label      string `json:"label"`
}

// Extract implements reportjobs.Fallback.
func (c *Claude) Extract(ctx context.Context, env *engine.Env, r store.Report, meta extract.Meta, rules extract.Result) (extract.Result, error) {
	if c.Max > 0 && c.used.Add(1) > c.Max {
		return extract.Result{}, ErrBudget
	}

	var content []anthropic.BetaContentBlockParamUnion
	var pages map[int]string // page → text, for verification (nil when scanned)
	intro := fmt.Sprintf("Report: %q, period ending %s.", r.Title, meta.PeriodEnd.Format("2006-01-02"))
	if meta.FYEnd != 0 {
		intro += fmt.Sprintf(" The financial year ends in %s.", meta.FYEnd)
	}

	if r.TextStatus == store.TextScanned {
		if env.Files == nil || r.SHA256 == "" || !env.Files.Has(r.SHA256) {
			return extract.Result{}, errors.New("scanned pdf not in the cache")
		}
		data, err := env.Files.Get(r.SHA256)
		if err != nil {
			return extract.Result{}, err
		}
		if len(data) > maxPDFBytes {
			return extract.Result{}, fmt.Errorf("scanned pdf too large for the api (%d bytes)", len(data))
		}
		content = append(content, anthropic.NewBetaDocumentBlock(anthropic.BetaBase64PDFSourceParam{Data: base64.StdEncoding.EncodeToString(data)}))
		content = append(content, anthropic.NewBetaTextBlock(intro+" The PDF is scanned; read the statement pages from the images."))
	} else {
		all, err := reportjobs.LoadPages(ctx, env.Store, r.ID)
		if err != nil {
			return extract.Result{}, err
		}
		pages = map[int]string{}
		want := map[int]bool{}
		for _, n := range extract.CandidatePages(all) {
			want[n] = true
		}
		var b strings.Builder
		b.WriteString(intro + " Statement pages follow, as extracted text (columns are separated by spaces).\n")
		for _, p := range all {
			if !want[p.N] {
				continue
			}
			text := p.Text()
			pages[p.N] = text
			fmt.Fprintf(&b, "\n=== Page %d ===\n%s", p.N, text)
		}
		if len(pages) == 0 {
			return extract.Result{}, errors.New("no statement pages to send")
		}
		content = append(content, anthropic.NewBetaTextBlock(b.String()))
	}

	resp, err := c.client.Beta.Messages.New(ctx, anthropic.BetaMessageNewParams{
		Model:      anthropic.Model(c.Model),
		MaxTokens:  16000,
		System:     []anthropic.BetaTextBlockParam{{Text: system}},
		Messages:   []anthropic.BetaMessageParam{anthropic.NewBetaUserMessage(content...)},
		Tools:      []anthropic.BetaToolUnionParam{tool()},
		ToolChoice: anthropic.BetaToolChoiceUnionParam{OfAuto: &anthropic.BetaToolChoiceAutoParam{}},
		// A policy decline is re-served by the fallback model the API picks.
		Fallbacks: anthropic.BetaFallbacksParamUnion{OfDefault: constant.ValueOf[constant.Default]()},
		Betas:     []anthropic.AnthropicBeta{anthropic.AnthropicBetaServerSideFallback2026_07_01},
	})
	if err != nil {
		return extract.Result{}, fmt.Errorf("call claude: %w", err)
	}
	if resp.StopReason == anthropic.BetaStopReasonRefusal {
		return extract.Result{}, ErrRefused
	}

	var got []recorded
	found := false
	for _, block := range resp.Content {
		if tu, ok := block.AsAny().(anthropic.BetaToolUseBlock); ok && tu.Name == "record_statements" {
			var in struct {
				Values []recorded `json:"values"`
			}
			raw, err := json.Marshal(tu.Input)
			if err != nil {
				return extract.Result{}, fmt.Errorf("decode claude output: %w", err)
			}
			if err := json.Unmarshal(raw, &in); err != nil {
				return extract.Result{}, fmt.Errorf("decode claude output: %w", err)
			}
			got, found = in.Values, true
		}
	}
	if !found {
		return extract.Result{}, fmt.Errorf("claude did not record any values (stop reason %s)", resp.StopReason)
	}
	vals, issues := Verify(got, pages, meta)
	if pages == nil {
		issues = append(issues, "scanned report: figures not checked against text")
	}
	res := extract.Finish(vals, meta, issues)
	if pages == nil { // nothing to check the figures against: cap the score
		sum := 0.0
		for i := range res.Values {
			res.Values[i].Confidence = math.Min(res.Values[i].Confidence, 0.6)
			sum += res.Values[i].Confidence
		}
		if len(res.Values) > 0 {
			res.Confidence = sum / float64(len(res.Values))
		}
	}
	return res, nil
}

var unitScale = map[string]float64{"rupees": 1, "thousands": 1e3, "millions": 1e6, "billions": 1e9, "per_share": 1}

// Verify turns recorded figures into values. A figure is dropped when its
// field is unknown, its raw text does not parse, its date is not a date, or
// (for text reports) the raw figure is not printed on the page it cites.
func Verify(got []recorded, pages map[int]string, meta extract.Meta) ([]extract.Value, []string) {
	var vals []extract.Value
	var issues []string
	seen := map[string]bool{}
	for _, g := range got {
		fd := extract.FieldByName(g.Field)
		if fd == nil {
			issues = append(issues, fmt.Sprintf("llm: unknown field %q", g.Field))
			continue
		}
		v, ok := extract.ParseAmount(strings.TrimSpace(g.Raw))
		if !ok {
			issues = append(issues, fmt.Sprintf("llm: %s: unreadable figure %q", g.Field, g.Raw))
			continue
		}
		end, err := time.Parse("2006-01-02", g.PeriodEnd)
		if err != nil {
			issues = append(issues, fmt.Sprintf("llm: %s: bad date %q", g.Field, g.PeriodEnd))
			continue
		}
		if !meta.PeriodEnd.IsZero() && end.After(meta.PeriodEnd.AddDate(0, 0, 1)) {
			issues = append(issues, fmt.Sprintf("llm: %s: period %s is after the report date", g.Field, g.PeriodEnd))
			continue
		}
		if pages != nil && !printedOn(pages[g.Page], g.Raw) {
			issues = append(issues, fmt.Sprintf("llm: %s %q not found on page %d", g.Field, g.Raw, g.Page))
			continue
		}
		if fd.Statement == extract.BalanceSheet {
			g.PeriodType = extract.PointInTime
		}
		key := g.Scope + g.PeriodType + g.PeriodEnd + g.Field
		if seen[key] {
			continue
		}
		seen[key] = true
		if !fd.PerShare {
			scale, ok := unitScale[g.Unit]
			if !ok || g.Unit == "per_share" {
				scale = 1e3
			}
			v *= scale
		}
		if fd.Magnitude {
			v = math.Abs(v)
		}
		vals = append(vals, extract.Value{
			Field: fd.Name, Statement: fd.Statement, Scope: g.Scope, PeriodEnd: end, PeriodType: g.PeriodType,
			Value: v, Page: g.Page, Label: g.Label, Raw: g.Raw,
		})
	}
	return vals, issues
}

// printedOn reports whether the figure appears as a whole token in text.
func printedOn(text, raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" || text == "" {
		return false
	}
	for _, tok := range strings.Fields(text) {
		if tok == raw || strings.TrimRight(tok, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ") == raw {
			return true
		}
	}
	return false
}

var _ reportjobs.Fallback = (*Claude)(nil)
