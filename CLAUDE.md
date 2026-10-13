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
| `main.go` | Calls `cmd.Execute()` |
| `cmd/root.go` | Root cobra command `hex` |
| `cmd/<group>.go` | Cobra wiring for a command group: commands, flags, `init()` registration (`company.go`, `clean-cash.go`, `version.go`) |
| `cmd/<group>/` | Logic for that group: fetching, parsing, caching (`cmd/company/`, `cmd/clean-cash/`) |
| `internal/const.go` | `CacheDir` and `CacheConfig` (file + TTL) for each cached data source |
| `internal/httpclient/` | `httpclient.Default`, the shared `*http.Client` (30s timeout) |
| `internal/custom-structs/` | JSON structs for CSE API responses (package `customStructs`) |
| `internal/util/` | `PrettyPrintJSON` |

## Conventions

- Wrap errors with context: `fmt.Errorf("request company profile: %w", err)`, using a lowercase "verb noun" prefix.
- Send all HTTP through `httpclient.Default`. CSE API calls are form-encoded `POST`s with browser-like `Origin`/`Referer`/`User-Agent` headers (see the `cse-api` skill).
- Cache files live in `os.UserCacheDir()/hex/<file>`. Every new cache gets a `CacheConfig` in `internal/const.go` **and** a subcommand under `hex clean` in `cmd/clean-cash.go`.
- Symbols: the user types `JKH`; the API usually wants the full security code `JKH.N0000`.
- Analysis output is informational, not investment advice. Keep that disclaimer on analysis commands.

## Known tech debt (fix when touching the area)

- The CSE POST boilerplate is duplicated in `cmd/company/root.go`. Extract a shared `cse` client with an injectable base URL so it can be tested.
- `ValidArgs: getValidArgs()` in `cmd/company.go` runs a network fetch during `init()` on **every** invocation. Replace it with a lazy `ValidArgsFunction`.
- In `show-financial`, `-b` (`showBannerAd`) and `-c` (`showCompanyBannerAd`) print the same field.
- "cash" vs "cache" naming is mixed (`clean-cash`, `CleanAllCash`). Use "cache" in new code.
- `util.PrettyPrintJSON` calls `log.Fatalf`. Return errors instead.
- There are no tests yet.

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
