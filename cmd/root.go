package cmd

import (
	"context"
	"os"
	"os/signal"

	"github.com/Janindu-Pathirana/hex/internal/cse"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/spf13/cobra"
)

// app carries the dependencies commands share, so tests can inject fakes.
type app struct {
	cse     *cse.Client
	rps     float64                // sync request rate; tests raise it
	llmOpts []option.RequestOption // Anthropic client options; tests point it at a fake
}

// NewRootCmd builds the hex command tree using the live cse.lk client.
func NewRootCmd() *cobra.Command {
	return newRootCmd(&app{cse: cse.Default})
}

func newRootCmd(a *app) *cobra.Command {
	if a.rps == 0 {
		a.rps = defaultRPS
	}
	root := &cobra.Command{
		Use:           "hex",
		Short:         "Analyze companies on the Sri Lanka stock market",
		Long:          "hex is a CLI tool for analyzing companies listed on the Sri Lanka stock market.",
		SilenceUsage:  true,
		SilenceErrors: false,
	}
	root.AddCommand(
		newCompanyCmd(a),
		newCalcCmd(),
		newSyncCmd(a),
		newReportsCmd(a),
		newFundamentalsCmd(a),
		newCleanCmd(),
		newVersionCmd(),
	)
	return root
}

// Execute runs hex and exits non-zero on error. Ctrl-C cancels in-flight requests.
func Execute() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := NewRootCmd().ExecuteContext(ctx); err != nil {
		os.Exit(1)
	}
}
