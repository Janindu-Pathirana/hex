package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Janindu-Pathirana/hex/cmd/analyze"
	"github.com/Janindu-Pathirana/hex/internal/analysis"
	"github.com/Janindu-Pathirana/hex/internal/output"
	"github.com/Janindu-Pathirana/hex/internal/store"
	"github.com/spf13/cobra"
)

func newAnalyzeCmd(a *app) *cobra.Command {
	var asJSON bool
	var sections []string
	cmd := &cobra.Command{
		Use:   "analyze <SYMBOL>",
		Short: "Analyze a company from stored data: ratios, trends and findings",
		Long: `Analyze a company using only what hex has stored (prices, statements, profile).
No network calls and no LLM: every metric shows its period and inputs, and every
finding is a rule applied to those numbers. Run 'hex sync' first to fill the store.

Ratios use trailing twelve months where the quarters are stored, and say so.
Findings use fixed thresholds until sector benchmarks are added.`,
		Example: `  hex analyze jkh
  hex analyze COMB --section valuation,growth
  hex analyze jkh --json`,
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
			snap, err := analyze.Load(cmd.Context(), db, sec.Code, a.now())
			if errors.Is(err, analyze.ErrNoFundamentals) {
				return fmt.Errorf("%w; run 'hex sync' for %s first", err, sec.Ticker())
			}
			if err != nil {
				return err
			}
			rep := analysis.Analyze(snap)
			if len(sections) > 0 {
				if err := rep.Filter(sections); err != nil {
					return err
				}
			}
			if asJSON {
				return output.JSON(cmd.OutOrStdout(), rep)
			}
			return rep.Render(cmd.OutOrStdout())
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Print the analysis as JSON")
	cmd.Flags().StringSliceVar(&sections, "section", nil, "Only these sections: "+strings.Join(analysis.Sections(), ", "))
	_ = cmd.RegisterFlagCompletionFunc("section", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return analysis.Sections(), cobra.ShellCompDirectiveNoFileComp
	})
	return cmd
}
