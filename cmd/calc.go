package cmd

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/Janindu-Pathirana/hex/internal/analysis"
	"github.com/Janindu-Pathirana/hex/internal/output"
	"github.com/spf13/cobra"
)

const disclaimer = "For information only, not investment advice."

func newCalcCmd() *cobra.Command {
	var asJSON bool
	calcCmd := &cobra.Command{
		Use:   "calc",
		Short: "Calculate financial ratios (ROE, P/E, margins…) and show the working",
		Long: `Calculate a financial ratio from numbers you supply.

Each ratio is its own subcommand. Every run prints the formula, the inputs,
each step of the calculation and the result. Use "hex calc list" to see all
ratios and the flags they need.`,
		Example: `  hex calc list
  hex calc roe --net-profit 1200 --equity-open 9000 --equity-close 11000
  hex calc pe --price 18.2 --eps 1.45 --json`,
	}
	calcCmd.PersistentFlags().BoolVar(&asJSON, "json", false, "Print the result as JSON")
	calcCmd.AddCommand(newCalcListCmd())
	for _, r := range analysis.Registry {
		calcCmd.AddCommand(newRatioCmd(r, &asJSON))
	}
	return calcCmd
}

func newCalcListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List every ratio, its formula and its inputs",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "RATIO\tGROUP\tFORMULA\tFLAGS")
			for _, r := range analysis.Registry {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", r.ID, r.Group, r.Formula, flagList(r))
			}
			return tw.Flush()
		},
	}
}

// flagList renders a ratio's flags, with optional ones in brackets.
func flagList(r analysis.Ratio) string {
	var parts []string
	for _, in := range r.Inputs {
		parts = append(parts, "--"+in.Flag)
	}
	for _, in := range r.Optional {
		parts = append(parts, "[--"+in.Flag+"]")
	}
	return strings.Join(parts, " ")
}

func newRatioCmd(r analysis.Ratio, asJSON *bool) *cobra.Command {
	vals := make(map[string]*float64)
	cmd := &cobra.Command{
		Use:     r.ID,
		Short:   r.Name,
		Long:    fmt.Sprintf("%s\n\nFormula: %s", r.Name, r.Formula),
		Example: "  hex calc " + r.ID + " " + exampleFlags(r),
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			in := analysis.Values{}
			for _, i := range r.AllInputs() {
				if cmd.Flags().Changed(i.Flag) {
					in[i.Key] = *vals[i.Key]
				}
			}
			res, err := r.Calc(in)
			if err != nil {
				return fmt.Errorf("%w (see hex calc %s --help)", err, r.ID)
			}
			if *asJSON {
				return output.JSON(cmd.OutOrStdout(), calcJSON{
					Ratio: r.ID, Name: r.Name, Formula: r.Formula, Inputs: in, Result: res,
				})
			}
			return writeCalc(cmd.OutOrStdout(), r, in, res)
		},
	}
	for _, i := range r.AllInputs() {
		vals[i.Key] = new(float64)
		cmd.Flags().Float64Var(vals[i.Key], i.Flag, 0, i.Label)
	}
	return cmd
}

type calcJSON struct {
	Ratio   string          `json:"ratio"`
	Name    string          `json:"name"`
	Formula string          `json:"formula"`
	Inputs  analysis.Values `json:"inputs"`
	analysis.Result
}

func exampleFlags(r analysis.Ratio) string {
	var parts []string
	for _, in := range r.Inputs {
		parts = append(parts, "--"+in.Flag+" <n>")
	}
	return strings.Join(parts, " ")
}

// writeCalc prints the formula, inputs, steps and result.
func writeCalc(w io.Writer, r analysis.Ratio, in analysis.Values, res analysis.Result) error {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\nFormula: %s\n\nInputs\n", r.Name, r.Formula)
	tw := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	for _, i := range r.AllInputs() {
		if v, ok := in[i.Key]; ok {
			fmt.Fprintf(tw, "  %s\t%s\n", i.Label, analysis.Num(v))
		}
	}
	_ = tw.Flush()
	if len(res.Steps) > 0 {
		b.WriteString("\nSteps\n")
		for _, s := range res.Steps {
			b.WriteString("  " + s + "\n")
		}
	}
	fmt.Fprintf(&b, "\nResult: %s\n", res.Display)
	if res.Note != "" {
		fmt.Fprintf(&b, "Note: %s\n", res.Note)
	}
	b.WriteString("\n" + disclaimer + "\n")
	_, err := io.WriteString(w, b.String())
	return err
}
