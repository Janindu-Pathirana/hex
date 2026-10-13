---
name: go-conventions
description: Go coding conventions for the hex CLI: package layout, error handling, context/timeouts, dependencies, formatting and linting. Use when writing or refactoring any Go code in this repo, adding a package, choosing a library, or running gofmt/go vet/staticcheck/golangci-lint.
---

# Go conventions for hex

Module `github.com/Janindu-Pathirana/hex`, Go 1.26, the only dependency is `spf13/cobra`.

## Layout
- `cmd/<group>.go`: cobra commands and flags only, with no HTTP or parsing logic.
- `cmd/<group>/`: logic for one command group. The package name is the folder name without dashes (e.g. `cmd/company` → `package company`).
- `internal/`: code shared by groups. Put new shared domain code here:
  - `internal/cse/`: the CSE API client (see `cse-api`)
  - `internal/analysis/`: pure functions for indicators and ratios, with no I/O (see `technical-analysis`, `fundamental-analysis`)
  - `internal/output/`: table, JSON and formatting helpers (see `terminal-output`)
  - `internal/cache/`: a generic cache read/write if a second cached source appears (see `cache-data`)
- New folders use lowercase names with no dashes.

## Errors
- Wrap with a lowercase "verb noun" prefix: `fmt.Errorf("fetch trade summary: %w", err)`.
- Library code (`internal/`, `cmd/<group>/`) **returns** errors. It never calls `log.Fatal`, `os.Exit` or `fmt.Println` for errors.
- Command code reports errors through cobra: prefer `RunE` and return the error, so `Execute()` prints it to stderr and exits 1. Existing commands use `Run` + `fmt.Println`. Migrate them when you touch them.
- Use sentinel errors for cases callers branch on, e.g. `var ErrUnknownSymbol = errors.New("unknown symbol")`, checked with `errors.Is`.

## HTTP and context
- Send every request through `httpclient.Default` (or a client injected for tests).
- Use `http.NewRequestWithContext(cmd.Context(), ...)` so Ctrl-C cancels the request.
- Always `defer resp.Body.Close()`. Check `resp.StatusCode` before decoding, and decode with `json.NewDecoder(resp.Body)`.
- For fan-out across many symbols, use bounded concurrency (`errgroup` with `SetLimit`, or a semaphore channel) capped at around 4 in-flight requests, since cse.lk is a public site.

## Types
- Prices and ratios use `float64`. They are display and analysis values, not accounting. Round only at output.
- CSE timestamps are Unix **milliseconds**. Convert with `time.UnixMilli(ms).In(colombo)`.
- Use pointer fields (`*string`, `*float64`) for JSON values that can be `null`.
- Exported identifiers get doc comments. Keep comments sparse elsewhere.

## Dependencies
Use the standard library first. Add a dependency only if it saves substantial code and is well maintained; good candidates are `golang.org/x/sync/errgroup` and `golang.org/x/term`. Run `go mod tidy` after any change.

## Before finishing
```bash
gofmt -l . && go vet ./... && go test ./...
# optional, if installed:
staticcheck ./... ; golangci-lint run
```
Make sure the `hex` binary at the repo root stays git-ignored, and don't commit it.

For deeper idiomatic Go (generics, concurrency patterns, pprof), also use the `golang-pro` skill.
