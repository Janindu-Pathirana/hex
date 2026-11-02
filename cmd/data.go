package cmd

import (
	"github.com/Janindu-Pathirana/hex/cmd/data"
	"github.com/Janindu-Pathirana/hex/internal/output"
	"github.com/Janindu-Pathirana/hex/internal/store"
	"github.com/spf13/cobra"
)

func newDataCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "data",
		Short: "Check what hex has stored",
	}
	cmd.AddCommand(newDataAuditCmd(a))
	return cmd
}

func newDataAuditCmd(a *app) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "audit [SYMBOL...]",
		Short: "Show how complete the stored data is, per company",
		Long: `Show what the local database holds for each company: daily price bars (count,
range, longest gap), statement periods and their confidence, report PDFs
(catalogued, downloaded, read) and the quote age. Companies with problems
that make 'hex analyze' unreliable get flags that say what to run.

With no symbols it covers every company that has price, statement or report
data; the index and market statistics are shown above the table. It reads the
store only and makes no network calls (except to resolve a ticker).`,
		Example: `  hex data audit
  hex data audit jkh comb
  hex data audit --json`,
		ValidArgsFunction: a.completeSymbols,
		RunE: func(cmd *cobra.Command, args []string) error {
			var codes []string
			for _, arg := range args {
				sec, err := a.resolve(cmd, arg)
				if err != nil {
					return err
				}
				codes = append(codes, sec.Code)
			}
			db, err := store.OpenDefault(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()
			rep, err := data.Build(cmd.Context(), db, a.now(), codes)
			if err != nil {
				return err
			}
			if asJSON {
				return output.JSON(cmd.OutOrStdout(), rep)
			}
			return rep.Render(cmd.OutOrStdout())
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Print the audit as JSON")
	return cmd
}
