# hex: CLI for analyzing the Sri Lankan stock market (CSE)

`hex` is a Go command-line tool built with cobra. It pulls data from the Colombo Stock Exchange website (cse.lk) and analyzes listed companies. It is distributed through a private Homebrew tap (`Formula/hex.rb`).

## Commands

```bash
go build -o hex .                     # build
go run . --help                       # run without building
go vet ./... && go test ./...         # check
go test -race -cover ./...            # full test run
go build -ldflags "-X github.com/Janindu-Pathirana/hex/cmd.Version=0.1.0" -o hex .   # versioned build
```

## Layout

| Path | Role |
|---|---|
| `main.go` | Calls `cmd.Execute()`; embeds tzdata |
| `cmd/root.go` | `NewRootCmd()` builds the tree; `app` holds injectable deps (the CSE client) |
| `cmd/<group>.go` | Cobra wiring for a group: `new<Group>Cmd(a *app)` constructors, flags (`company.go`, `clean.go`, `version.go`) |
| `cmd/calc.go` | `hex calc <ratio>`: one subcommand generated per `analysis.Registry` entry, `hex calc list` |
| `internal/analysis/` | Pure ratio engine: `Registry` of `Ratio{Inputs, Formula, Compute}` returning `Result{Value, Display, Steps}` |
| `cmd/args.go` | Shared symbol completion (`ValidArgsFunction`) and legacy flag-name normalisation |
| `cmd/<group>/` | Logic for that group (`cmd/company/`: cached securities, symbol resolution, report views) |
| `internal/cse/` | cse.lk client: `Client{BaseURL, SiteURL, HTTP}`, `post`/`get` with retries, `ErrNoData`, `ErrUnknownSymbol`, `Resolve`, response types |
| `internal/csetest/` | Fake cse.lk server for tests, backed by `testdata/cse/` fixtures |
| `internal/cache/` | JSON file cache: `Load`/`Save` (atomic)/`Remove`/`RemoveAll`; `HEX_CACHE_DIR` overrides the root |
| `internal/const.go` | `CacheDir`, `CacheConfig` per cached source, `AllCaches` |
| `internal/output/` | `JSON` writer, Asia/Colombo date helpers |
| `internal/httpclient/` | `httpclient.Default`, the shared `*http.Client` (30s timeout) |
| `testdata/` | `cse/` API fixtures, `golden/` expected command output (`go test ./cmd -update` rewrites) |

## Conventions

- Wrap errors with context: `fmt.Errorf("fetch company profile for %s: %w", code, err)`, using a lowercase "verb noun" prefix.
- Commands use `RunE` and write to `cmd.OutOrStdout()`; cobra prints errors to stderr and `Execute` exits 1.
- Send all CSE HTTP through `internal/cse.Client` (which uses `httpclient.Default`). Never hard-code cse.lk URLs elsewhere.
- Cache files live in `os.UserCacheDir()/hex/<file>` (or `$HEX_CACHE_DIR`). Every new cache gets a `CacheConfig` in `internal/const.go`, an entry in `AllCaches`, **and** a subcommand under `hex clean` in `cmd/clean.go`.
- Symbols: users type `jkh` or `CIC.X0000`; resolve with `company.Resolve` (case-insensitive, prefers `.N0000`, keeps X/U series). The API gets the full code.
- Tests never hit the real cse.lk. Use `csetest.New(t)` and `t.Setenv(cache.EnvDir, t.TempDir())`.
- Analysis output is informational, not investment advice. Keep that disclaimer on analysis commands.

## Roadmap

The approved data-platform plan (CSE + CBSL sources, SQLite store, PDF fundamentals, analysis) is phased: 0 foundation (done), 1 store and sync, 2 history, 3 corporate actions, 4 technical analysis, 5 fundamentals from PDFs, 6 CBSL macro, 7 screener/watchlist/portfolio and release.

## Known tech debt (fix when touching the area)

- Output is JSON only. Tables and a global `--json` toggle come with `internal/output` in Phase 1 (see `terminal-output`).
- `company list --names` replaces spaces with dashes (kept for compatibility).
- Profile and financials responses are not cached yet.

## Skill map

Project skills (`.claude/skills/`):

| Task | Skill |
|---|---|
| Go style, structure, linting | `go-conventions` |
| New cobra command / subcommand | `add-command` |
| Calling cse.lk, new endpoints, response structs | `cse-api` |
| Caching a data source | `cache-data` |
| Tables, `--json`, number/date/colour formatting | `terminal-output` |
| Sri Lankan market rules, indices, boards, hours | `cse-market-context` |
| Ratios: P/E, P/B, dividend yield, ROE… | `fundamental-analysis` |
| Indicators: SMA/EMA, RSI, MACD, volatility… | `technical-analysis` |
| Screener, watchlist, portfolio P&L | `screening-and-portfolio` |
| Tests, fakes, fixtures, golden files | `go-testing` |
| Version, tag, Homebrew formula, CI | `release` |
| Committing: grouping changes, message format (`feat:`, `bugfix:`, `refac:`…) | `my-commits` |

General installed skills that also apply: `golang-pro` (idiomatic Go, concurrency), `cli-developer` (CLI UX, completions), `test-master`, `debugging-wizard`, `code-reviewer`, `security-reviewer` / `secure-code-guardian`, `code-documenter` (readme/help text), `data:statistical-analysis`, `equity-research:screen` / `:earnings` / `:sector` / `:thesis` (analysis ideas), `dataviz` (charts) and `devops-engineer` (GitHub Actions).
