package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Janindu-Pathirana/hex/internal/cse"
	"github.com/Janindu-Pathirana/hex/internal/engine"
	"github.com/Janindu-Pathirana/hex/internal/extract"
	"github.com/Janindu-Pathirana/hex/internal/llm"
	"github.com/Janindu-Pathirana/hex/internal/output"
	"github.com/Janindu-Pathirana/hex/internal/sources/csejobs"
	"github.com/Janindu-Pathirana/hex/internal/sources/reportjobs"
	"github.com/Janindu-Pathirana/hex/internal/store"
	"github.com/spf13/cobra"
)

func newReportsCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reports",
		Short: "Financial report PDFs: what is stored and what was read from them",
	}
	cmd.AddCommand(newReportsListCmd(a), newReportsExtractCmd(a))
	return cmd
}

type reportView struct {
	ID         int64    `json:"id"`
	Kind       string   `json:"kind"`
	Title      string   `json:"title"`
	PeriodEnd  string   `json:"periodEnd,omitempty"`
	Uploaded   *string  `json:"uploaded"`
	URL        string   `json:"url"`
	Downloaded bool     `json:"downloaded"`
	Pages      int      `json:"pages,omitempty"`
	Text       string   `json:"text,omitempty"` // ok, scanned, failed
	Parsed     *string  `json:"parsed"`
	Confidence *float64 `json:"confidence"`
	Method     string   `json:"method,omitempty"`
}

func newReportsListCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:               "list <SYMBOL>",
		Short:             "List a company's catalogued reports and their processing state",
		Long:              "List a company's reports from the local database (run 'hex sync reports --symbols SYM' first).",
		Example:           "  hex reports list jkh",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: a.completeSymbols,
		RunE: func(cmd *cobra.Command, args []string) error {
			sec, err := a.resolve(cmd, args[0])
			if err != nil {
				return err
			}
			db, err := store.OpenDefault(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()
			reports, err := db.Reports(cmd.Context(), sec.Code)
			if err != nil {
				return err
			}
			if len(reports) == 0 {
				return fmt.Errorf("no reports stored for %s; run 'hex sync reports --symbols %s'", sec.Code, sec.Ticker())
			}
			views := make([]reportView, 0, len(reports))
			for _, r := range reports {
				v := reportView{ID: r.ID, Kind: r.Kind, Title: r.Title, URL: cse.AssetURL(r.Path),
					Downloaded: r.SHA256 != "", Pages: r.Pages, Text: r.TextStatus, Uploaded: fmtTime(r.UploadedAt), Parsed: fmtTime(r.ParsedAt)}
				if end, ok := extract.PeriodEnd(r.Title); ok {
					v.PeriodEnd = end.Format("2006-01-02")
				}
				if runs, err := db.Runs(cmd.Context(), r.ID); err == nil && len(runs) > 0 {
					c := runs[0].Confidence
					v.Confidence, v.Method = &c, runs[0].Method
				}
				views = append(views, v)
			}
			return output.JSON(cmd.OutOrStdout(), views)
		},
	}
}

type extractView struct {
	Symbol     string            `json:"symbol"`
	Report     reportView        `json:"report"`
	Confidence float64           `json:"confidence"`
	Values     []fundamentalView `json:"values"`
	Issues     []string          `json:"issues,omitempty"`
	Pages      map[int]string    `json:"pages,omitempty"`
	Disclaimer string            `json:"disclaimer"`
}

func newReportsExtractCmd(a *app) *cobra.Command {
	var period string
	var useLLM, showPages bool
	var model string
	cmd := &cobra.Command{
		Use:   "extract <SYMBOL>",
		Short: "Download (if needed) and read one report's statements now",
		Long: `Read the income statement, balance sheet and cash flow of one report and
store the values. By default the newest interim report; --period picks the
report for a period end (YYYY-MM-DD). The report is re-read even if it was
read before. --llm also asks Claude when the rules score below 0.7.`,
		Example:           "  hex reports extract jkh\n  hex reports extract COMB --period 2026-03-31 --show-pages",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: a.completeSymbols,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			sec, err := a.resolve(cmd, args[0])
			if err != nil {
				return err
			}
			env, done, err := a.engineEnv(cmd)
			if err != nil {
				return err
			}
			defer done()

			// Catalogue, shares and year end, then the PDF text (each skipped when fresh).
			o := reportjobs.Options{Limits: reportjobs.DefaultOptions.Limits}
			stages := csejobs.Plan(csejobs.Scope{Reports: true, Info: true, Profiles: true}, []string{sec.Code}, time.Now())
			if _, err := engine.Run(ctx, env, stages, engine.Options{}); err != nil {
				return err
			}
			r, err := pickReport(ctx, env.Store, sec.Code, period)
			if err != nil {
				return err
			}
			if r.TextStatus == "" {
				if err := reportjobs.ProcessPDF(ctx, env, r, o); err != nil {
					return fmt.Errorf("read report %d: %w", r.ID, err)
				}
				if r, err = env.Store.Report(ctx, r.ID); err != nil {
					return err
				}
			}

			info, _ := env.Store.Security(ctx, sec.Code)
			prof, _, _ := env.Store.Profile(ctx, sec.Code)
			end, _ := extract.PeriodEnd(r.Title)
			meta := extract.Meta{Annual: r.Kind == store.ReportAnnual, PeriodEnd: end, Shares: info.Issued, FYEnd: extract.FYEndMonth(prof.FYEnd)}
			var fb reportjobs.Fallback
			if useLLM {
				fb = llm.New(model, 1, a.llmOpts...)
			}
			if err := reportjobs.ExtractReport(ctx, env, r, meta, fb); err != nil {
				return err
			}

			view := extractView{Symbol: sec.Code, Disclaimer: pdfDisclaimer}
			if r, err = env.Store.Report(ctx, r.ID); err != nil {
				return err
			}
			view.Report = reportView{ID: r.ID, Kind: r.Kind, Title: r.Title, PeriodEnd: end.Format("2006-01-02"), URL: cse.AssetURL(r.Path),
				Downloaded: r.SHA256 != "", Pages: r.Pages, Text: r.TextStatus, Parsed: fmtTime(r.ParsedAt)}
			runs, _ := env.Store.Runs(ctx, r.ID)
			if len(runs) > 0 {
				view.Confidence = runs[0].Confidence
				if runs[0].Errors != "" {
					view.Issues = strings.Split(runs[0].Errors, "\n")
				}
			}
			all, err := env.Store.Fundamentals(ctx, sec.Code, true)
			if err != nil {
				return err
			}
			for _, f := range all {
				if f.ReportID == r.ID {
					view.Values = append(view.Values, toFundamentalView(f))
				}
			}
			if showPages {
				pages, _ := reportjobs.LoadPages(ctx, env.Store, r.ID)
				view.Pages = map[int]string{}
				used := map[int]bool{}
				for _, v := range view.Values {
					used[v.Page] = true
				}
				for _, p := range pages {
					if used[p.N] {
						view.Pages[p.N] = p.Text()
					}
				}
			}
			return output.JSON(cmd.OutOrStdout(), view)
		},
	}
	cmd.Flags().StringVar(&period, "period", "", "Period end of the report to read (YYYY-MM-DD); default the newest interim report")
	cmd.Flags().BoolVar(&useLLM, "llm", false, "Ask Claude when the rules are not confident (needs ANTHROPIC_API_KEY; costs money)")
	cmd.Flags().StringVar(&model, "llm-model", llm.DefaultModel, "Claude model for --llm")
	cmd.Flags().BoolVar(&showPages, "show-pages", false, "Include the text of the statement pages that values came from")
	return cmd
}

// pickReport returns the newest interim report, or the report (interim
// first, then annual) whose title gives the requested period end.
func pickReport(ctx context.Context, db *store.DB, code, period string) (store.Report, error) {
	reports, err := db.Reports(ctx, code)
	if err != nil {
		return store.Report{}, err
	}
	for _, kind := range []string{store.ReportQuarterly, store.ReportAnnual} {
		for _, r := range reports {
			if r.Kind != kind {
				continue
			}
			end, ok := extract.PeriodEnd(r.Title)
			if period == "" && kind == store.ReportQuarterly {
				return r, nil
			}
			if ok && end.Format("2006-01-02") == period {
				return r, nil
			}
		}
	}
	if period == "" {
		return store.Report{}, fmt.Errorf("no interim reports found for %s", code)
	}
	return store.Report{}, fmt.Errorf("no report for period %s found for %s (see 'hex reports list')", period, code)
}
