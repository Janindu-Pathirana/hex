package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/Janindu-Pathirana/hex/cmd/company"
	"github.com/Janindu-Pathirana/hex/internal/engine"
	"github.com/Janindu-Pathirana/hex/internal/output"
	"github.com/Janindu-Pathirana/hex/internal/sources/csejobs"
	"github.com/Janindu-Pathirana/hex/internal/store"
	"github.com/spf13/cobra"
)

// defaultRPS is the polite request rate for cse.lk.
const defaultRPS = 5

// syncGroups are the data groups "hex sync" accepts, in help order.
var syncGroups = []string{"market", "securities", "quotes", "info", "history", "profiles", "reports", "actions", "all"}

type syncOptions struct {
	symbols     []string
	allSymbols  bool
	force       bool
	dryRun      bool
	concurrency int
}

func newSyncCmd(a *app) *cobra.Command {
	var o syncOptions
	cmd := &cobra.Command{
		Use:   "sync [group...]",
		Short: "Scrape cse.lk into the local database",
		Long: `Scrape cse.lk into the local database (` + "`hex sync status`" + ` shows what is stored).

Groups: market (status, indices, sectors, daily stats), securities (symbol
list), quotes (today's prices), and per company: info (stockId, ranges,
beta), history (daily bars, accumulated beyond one year), profiles,
reports (PDF catalogue). actions stores recent corporate announcements.
"all" selects every group.

With no group, hex syncs market, securities and quotes, plus info, history,
profiles and reports for any --symbols. Work done within its freshness
window is skipped unless --force is given; an interrupted sync resumes
where it stopped. Requests are rate-limited and run at most 4 at a time.`,
		Example: `  hex sync
  hex sync --symbols jkh,comb
  hex sync all --all-symbols
  hex sync history --symbols JKH --force
  hex sync status`,
		ValidArgs: syncGroups,
		Args:      cobra.OnlyValidArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runSync(cmd, args, o)
		},
	}
	f := cmd.Flags()
	f.StringSliceVarP(&o.symbols, "symbols", "s", nil, "Companies to sync (tickers or full codes, comma-separated)")
	f.BoolVar(&o.allSymbols, "all-symbols", false, "Sync every listed security")
	f.BoolVarP(&o.force, "force", "f", false, "Ignore freshness and fetch everything again")
	f.BoolVar(&o.dryRun, "dry-run", false, "Show which jobs would run without fetching")
	f.IntVarP(&o.concurrency, "concurrency", "c", engine.MaxConcurrency, "Jobs to run at once (max 4)")
	cmd.MarkFlagsMutuallyExclusive("symbols", "all-symbols")
	_ = cmd.RegisterFlagCompletionFunc("symbols", a.completeSymbols)

	cmd.AddCommand(newSyncStatusCmd())
	return cmd
}

// syncScope turns the group arguments into a scope.
func syncScope(groups []string, haveSymbols bool) csejobs.Scope {
	if len(groups) == 0 {
		return csejobs.Scope{Market: true, Securities: true, Quotes: true,
			Info: haveSymbols, History: haveSymbols, Profiles: haveSymbols, Reports: haveSymbols}
	}
	has := func(g string) bool { return slices.Contains(groups, g) || slices.Contains(groups, "all") }
	return csejobs.Scope{
		Market: has("market"), Securities: has("securities"), Quotes: has("quotes"), Info: has("info"),
		History: has("history"), Profiles: has("profiles"), Reports: has("reports"), Actions: has("actions"),
	}
}

func (a *app) runSync(cmd *cobra.Command, groups []string, o syncOptions) error {
	ctx := cmd.Context()
	codes, err := a.syncCodes(ctx, cmd, o)
	if err != nil {
		return err
	}
	scope := syncScope(groups, len(codes) > 0)
	perCompany := scope.Info || scope.History || scope.Profiles || scope.Reports
	if perCompany && len(codes) == 0 {
		return errors.New("per-company groups need --symbols or --all-symbols")
	}
	if scope.Actions {
		scope.Securities = true // matching announcements needs the symbol list
	}

	dir, err := store.Dir()
	if err != nil {
		return err
	}
	unlock, err := engine.Lock(dir)
	if err != nil {
		return err
	}
	defer unlock()

	db, err := store.OpenDefault(ctx)
	if err != nil {
		return err
	}
	defer db.Close()

	log := cmd.ErrOrStderr()
	client := *a.cse
	engine.RecordRaw(&client, db, log)
	env := &engine.Env{
		CSE:   engine.Limit(&client, engine.NewLimiter(a.rps, engine.MaxConcurrency)),
		Store: db,
		Log:   log,
	}

	start := time.Now()
	stages := csejobs.Plan(scope, codes, start)
	sum, runErr := engine.Run(ctx, env, stages, engine.Options{Concurrency: o.concurrency, Force: o.force, DryRun: o.dryRun})

	w := cmd.OutOrStdout()
	if o.dryRun {
		_, err = fmt.Fprintf(w, "sync (dry run): %d would run, %d fresh\n", sum.Count(engine.Planned), sum.Count(engine.Skipped))
		return err
	}
	if _, err := fmt.Fprintf(w, "sync: %d ok, %d fresh, %d failed, %d pending (%.1fs)\n",
		sum.Count(engine.Ran), sum.Count(engine.Skipped), sum.Count(engine.Failed), sum.Count(engine.Pending),
		time.Since(start).Seconds()); err != nil {
		return err
	}
	if runErr != nil {
		return fmt.Errorf("sync interrupted; run it again to resume: %w", runErr)
	}
	if n := sum.Count(engine.Failed); n > 0 {
		return fmt.Errorf("sync: %d job(s) failed (see 'hex sync status')", n)
	}
	return nil
}

// syncCodes resolves --symbols / --all-symbols to full security codes.
func (a *app) syncCodes(ctx context.Context, cmd *cobra.Command, o syncOptions) ([]string, error) {
	if o.allSymbols {
		secs, err := company.Securities(ctx, a.cse)
		if err != nil {
			return nil, err
		}
		codes := make([]string, len(secs))
		for i, s := range secs {
			codes[i] = s.Code
		}
		return codes, nil
	}
	var codes []string
	for _, in := range o.symbols {
		if strings.TrimSpace(in) == "" {
			continue
		}
		sec, err := a.resolve(cmd, in)
		if err != nil {
			return nil, err
		}
		if !slices.Contains(codes, sec.Code) {
			codes = append(codes, sec.Code)
		}
	}
	return codes, nil
}

func newSyncStatusCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show what the local database holds and how fresh it is",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := store.OpenDefault(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()
			states, err := db.JobStates(cmd.Context())
			if err != nil {
				return err
			}
			if asJSON {
				return output.JSON(cmd.OutOrStdout(), jobStatesJSON(states))
			}
			return writeSyncStatus(cmd.OutOrStdout(), states)
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Print every job's state as JSON")
	return cmd
}

type jobStateJSON struct {
	Key       string  `json:"key"`
	Kind      string  `json:"kind"`
	Status    string  `json:"status"`
	Attempts  int     `json:"attempts"`
	LastOK    *string `json:"lastOk"`
	LastRun   *string `json:"lastRun"`
	LastError string  `json:"lastError,omitempty"`
}

func jobStatesJSON(states []store.JobState) []jobStateJSON {
	out := make([]jobStateJSON, len(states))
	for i, s := range states {
		out[i] = jobStateJSON{Key: s.Key, Kind: s.Kind, Status: s.Status, Attempts: s.Attempts,
			LastOK: fmtTime(s.LastOKAt), LastRun: fmtTime(s.LastRunAt), LastError: s.LastError}
	}
	return out
}

func fmtTime(t time.Time) *string {
	if t.IsZero() {
		return nil
	}
	s := output.DateTime(t.UnixMilli())
	return &s
}

// writeSyncStatus prints one row per job kind, then each failed job.
func writeSyncStatus(w io.Writer, states []store.JobState) error {
	if len(states) == 0 {
		_, err := fmt.Fprintln(w, "Nothing synced yet. Run 'hex sync'.")
		return err
	}
	type agg struct {
		jobs, ok, failed, pending int
		lastOK                    time.Time
	}
	var kinds []string
	byKind := map[string]*agg{}
	for _, s := range states {
		g, seen := byKind[s.Kind]
		if !seen {
			g = &agg{}
			byKind[s.Kind] = g
			kinds = append(kinds, s.Kind)
		}
		g.jobs++
		switch s.Status {
		case store.JobDone:
			g.ok++
		case store.JobFailed:
			g.failed++
		default:
			g.pending++
		}
		if s.LastOKAt.After(g.lastOK) {
			g.lastOK = s.LastOKAt
		}
	}

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "KIND\tJOBS\tOK\tFAILED\tPENDING\tLAST OK")
	for _, k := range kinds {
		g := byKind[k]
		last := "-"
		if p := fmtTime(g.lastOK); p != nil {
			last = *p
		}
		fmt.Fprintf(tw, "%s\t%d\t%d\t%d\t%d\t%s\n", k, g.jobs, g.ok, g.failed, g.pending, last)
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	for _, s := range states {
		if s.Status == store.JobFailed {
			if _, err := fmt.Fprintf(w, "failed %s (attempt %d): %s\n", s.Key, s.Attempts, s.LastError); err != nil {
				return err
			}
		}
	}
	return nil
}
