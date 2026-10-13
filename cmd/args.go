package cmd

import (
	"strings"

	"github.com/Janindu-Pathirana/hex/cmd/company"
	"github.com/Janindu-Pathirana/hex/internal/cse"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// completeSymbols completes the first argument with listed tickers. It only
// touches the network (or cache) when the shell actually asks for completions.
func (a *app) completeSymbols(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	secs, err := company.Securities(cmd.Context(), a.cse)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	prefix := strings.ToUpper(toComplete)
	var out []string
	for _, t := range cse.Tickers(secs) {
		if strings.HasPrefix(t, prefix) {
			out = append(out, t)
		}
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

// legacyFlags maps old camelCase flag names to their kebab-case replacements
// so existing scripts keep working.
func legacyFlags(names map[string]string) func(*pflag.FlagSet, string) pflag.NormalizedName {
	return func(_ *pflag.FlagSet, name string) pflag.NormalizedName {
		if n, ok := names[name]; ok {
			return pflag.NormalizedName(n)
		}
		return pflag.NormalizedName(name)
	}
}
