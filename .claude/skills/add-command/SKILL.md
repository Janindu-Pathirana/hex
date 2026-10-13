---
name: add-command
description: Scaffold a new hex CLI command or subcommand with cobra, following the repo's cmd/<group>.go + cmd/<group>/ pattern, including flags, symbol validation, shell completion, --json output and readme updates. Use when adding any new hex command (e.g. "hex price JKH", "hex market summary", "hex analyze").
---

# Adding a hex command

## 1. Pick the group
The existing groups are `company`, `clean` and `version`. Planned groups that fit the roadmap:
- `market`: status, summary, indices (ASPI, S&P SL20), sectors, gainers/losers/most-active
- `price`: today's quote and history for a symbol
- `analyze`: technical and fundamental metrics for a symbol
- `screen`, `watchlist`, `portfolio` (see `screening-and-portfolio`)
- `announcements`

If you're adding to an existing group, add the subcommand to `cmd/<group>.go`. For a new group, create both files below.

## 2. Wiring file `cmd/<group>.go`
```go
package cmd

var marketCmd = &cobra.Command{
	Use:   "market",
	Short: "Market-wide data: status, indices, sectors, movers",
}

var marketSummaryCmd = &cobra.Command{
	Use:   "summary",
	Short: "Show today's market summary",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := market.FetchSummary(cmd.Context())
		if err != nil {
			return err // cobra prints to stderr, Execute exits 1
		}
		return output.Render(cmd.OutOrStdout(), jsonOutput, s, market.SummaryTable(s))
	},
}

func init() {
	marketCmd.AddCommand(marketSummaryCmd)
	rootCmd.AddCommand(marketCmd)
}
```
Rules:
- Use `RunE`, not `Run`, so errors go to stderr with a non-zero exit.
- Write output to `cmd.OutOrStdout()` so tests can capture it.
- Flags use **kebab-case long names** with short aliases (`--only-symbol`, `-s`). The existing `--onlySymbol` camelCase flags are legacy. If you rename them, keep the old name hidden with `MarkDeprecated`.
- Add a persistent `--json` flag on `rootCmd` (once) and reuse it everywhere instead of per-command JSON flags.
- Use `Example:` strings for help text.

## 3. Logic package `cmd/<group>/`
- Fetch through the shared CSE client (see the `cse-api` skill). Use `a.cse` (an `*cse.Client` on the `app` struct in `cmd/root.go`); never build requests by hand.
- Return typed structs and errors, and leave printing to the caller.
- Cache if the data is reused (see `cache-data`).

## 4. Symbol arguments
Commands that take a symbol should share one helper instead of repeating the `Args` closure found in `cmd/company.go`:
```go
// cmd/args.go
func symbolArg(cmd *cobra.Command, args []string) error {
	if len(args) != 1 { return fmt.Errorf("expected 1 symbol, got %d", len(args)) }
	// normalise: upper-case, accept "JKH" or "JKH.N0000"
	...
}
func completeSymbols(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 { return nil, cobra.ShellCompDirectiveNoFileComp }
	syms, err := company.FetchAvailableCompanySymbols()
	if err != nil { return nil, cobra.ShellCompDirectiveError }
	return syms, cobra.ShellCompDirectiveNoFileComp
}
```
Existing helpers: `ValidArgsFunction: a.completeSymbols` (`cmd/args.go`) and `a.resolve(cmd, input)` (`cmd/company.go`). **Never** use `ValidArgs: <network call>`: it would fetch during command construction on every run.

Resolve the short symbol to its full security code(s) from the company list (`NAME,JKH.N0000`). A company can have both `.N0000` and `.X0000` lines. Default to `.N0000` and accept an explicit full code.

## 5. Finish
- Add a test (see `go-testing`): run the command with a fake CSE server and compare against a golden file.
- Document the command under a "Usage" section in `readme.md`.
- Run `go run . <group> <sub> --help` and check the help text reads well.
- Run `gofmt -l . && go vet ./... && go test ./...`.

Also see `cli-developer` for broader CLI UX (completions, prompts, progress).
