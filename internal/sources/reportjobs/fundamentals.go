package reportjobs

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Janindu-Pathirana/hex/internal/engine"
	"github.com/Janindu-Pathirana/hex/internal/extract"
	"github.com/Janindu-Pathirana/hex/internal/store"
)

// LowConfidence is the run confidence below which a result is flagged (and,
// with the LLM fallback enabled, re-extracted by the model).
const LowConfidence = 0.7

// Fallback re-extracts a report the rules could not read well. It returns
// values to store under store.SourceLLM; implementations live in internal/llm.
type Fallback interface {
	Name() string // model name, recorded with the run
	Extract(ctx context.Context, env *engine.Env, r store.Report, meta extract.Meta, rules extract.Result) (extract.Result, error)
}

// Fundamentals extracts statement values from code's processed reports that
// have not been parsed yet (all of them with Reparse). Values below
// LowConfidence are stored but flagged; fb, when set, gets a second try.
func Fundamentals(code string, o Options, fb Fallback) engine.Job {
	ttl := 24 * time.Hour
	if o.Reparse {
		ttl = 0
	}
	return engine.FuncJob{K: KindFundamentals + ":" + code, Kd: KindFundamentals, T: ttl, F: func(ctx context.Context, env *engine.Env) error {
		reports, err := env.Store.Reports(ctx, code)
		if err != nil {
			return err
		}
		sec, _ := env.Store.Security(ctx, code)
		prof, _, err := env.Store.Profile(ctx, code)
		if err != nil {
			prof = store.Profile{}
		}
		var errs []error
		for _, r := range o.Select(reports) {
			if r.TextStatus == "" || r.TextStatus == store.TextFailed || (!r.ParsedAt.IsZero() && !o.Reparse) {
				continue
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			end, ok := extract.PeriodEnd(r.Title)
			if !ok {
				end, _ = time.Parse("2006-01-02", r.PeriodDate)
			}
			meta := extract.Meta{Annual: r.Kind == store.ReportAnnual, PeriodEnd: end, Shares: sec.Issued, FYEnd: extract.FYEndMonth(prof.FYEnd)}
			if err := ExtractReport(ctx, env, r, meta, fb); err != nil {
				errs = append(errs, fmt.Errorf("report %d (%s): %w", r.ID, r.Title, err))
			}
		}
		return errors.Join(errs...)
	}}
}

// ExtractReport runs the rules (and the fallback when they fall short) on one
// report and replaces its stored values.
func ExtractReport(ctx context.Context, env *engine.Env, r store.Report, meta extract.Meta, fb Fallback) error {
	var res extract.Result
	started := env.Now()
	if r.TextStatus != store.TextScanned {
		pages, err := LoadPages(ctx, env.Store, r.ID)
		if err != nil {
			return err
		}
		res = extract.Extract(pages, meta)
	} else {
		res.Issues = []string{"scanned report: no text layer"}
	}
	if err := saveRun(ctx, env, r, "rules", extract.Version, "", started, res, store.SourcePDF); err != nil {
		return err
	}

	if fb != nil && res.Confidence < LowConfidence {
		started = env.Now()
		llmRes, err := fb.Extract(ctx, env, r, meta, res)
		if err != nil {
			fmt.Fprintf(env.Log, "warning: %s report %d: llm fallback: %v\n", r.Code, r.ID, err)
			llmRes = extract.Result{Issues: []string{err.Error()}}
		}
		if err := saveRun(ctx, env, r, "llm", "llm-1", fb.Name(), started, llmRes, store.SourceLLM); err != nil {
			return err
		}
	} else if res.Confidence < LowConfidence {
		fmt.Fprintf(env.Log, "warning: %s report %d: low confidence %.2f (%s)\n", r.Code, r.ID, res.Confidence, firstIssue(res))
	}
	return env.Store.Update(ctx, func(tx *store.Tx) error { return tx.SetReportParsed(ctx, r.ID) })
}

func saveRun(ctx context.Context, env *engine.Env, r store.Report, method, version, model string, started time.Time, res extract.Result, source string) error {
	status := store.RunOK
	switch {
	case len(res.Values) == 0:
		status = store.RunFailed
	case res.Confidence < LowConfidence:
		status = store.RunLowConfidence
	}
	return env.Store.Update(ctx, func(tx *store.Tx) error {
		runID, err := tx.InsertRun(ctx, store.ExtractionRun{
			ReportID: r.ID, Method: method, Version: version, Model: model, StartedAt: started,
			Confidence: res.Confidence, Status: status, Errors: strings.Join(res.Issues, "\n"),
		})
		if err != nil {
			return err
		}
		if err := tx.DeleteReportFundamentals(ctx, r.ID, source); err != nil {
			return err
		}
		rows := make([]store.Fundamental, 0, len(res.Values))
		for _, v := range res.Values {
			rows = append(rows, store.Fundamental{
				Code: r.Code, PeriodEnd: v.PeriodEnd.Format("2006-01-02"), PeriodType: v.PeriodType, Scope: v.Scope,
				Field: v.Field, Source: source, Value: v.Value, ReportID: r.ID, RunID: runID, Page: v.Page,
				RawLabel: v.Label, Confidence: v.Confidence,
			})
		}
		return tx.UpsertFundamentals(ctx, rows)
	})
}

// firstIssue explains a low score: a failed identity if there is one.
func firstIssue(res extract.Result) string {
	for _, is := range res.Issues {
		if strings.HasSuffix(is, " fails") || strings.HasPrefix(is, "scanned") || strings.HasPrefix(is, "no statement") {
			return is
		}
	}
	return "few identities could be checked"
}
