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
| `cmd/<group>.go` | Cobra wiring for a group: `new<Group>Cmd(a *app)` constructors, flags (`company.go`, `sync.go`, `reports.go`, `fundamentals.go`, `clean.go`, `version.go`) |
| `cmd/analyze.go`, `cmd/analyze/` | `hex analyze <SYMBOL>`: reads only the store (`analyze.Load` builds the `Snapshot`), prints the report as text or `--json`, `--section` filters |
| `cmd/calc.go` | `hex calc <ratio>`: one subcommand generated per `analysis.Registry` entry, `hex calc list` |
| `internal/analysis/` | Pure ratio engine (`trends.go` adds year-on-year margin/ROE/growth changes, FCF yield, asset turnover): `Registry` of `Ratio{Inputs, Formula, Compute}` returning `Result{Value, Display, Steps}`. `Snapshot` (period series, TTM, YoY) → `Compute` (metrics) → `Evaluate` (rule findings) → `Report` (`Analyze`, `Render`, `Filter`). No I/O, no LLM |
| `cmd/args.go` | Shared symbol completion (`ValidArgsFunction`) and legacy flag-name normalisation |
| `cmd/<group>/` | Logic for that group (`cmd/company/`: cached securities, symbol resolution, report views) |
| `internal/cse/` | cse.lk client: `Client{BaseURL, SiteURL, CDNBase, HTTP, DownloadHTTP, Observe}`, `post`/`get` with retries and size caps, one method per endpoint, `DownloadPDF`, `ErrNoData`, `ErrUnknownSymbol`, `Resolve` |
| `internal/store/` | SQLite (`modernc.org/sqlite`) at `os.UserConfigDir()/hex/hex.db` (`HEX_DATA_DIR` overrides): embedded migrations, typed repos, writes only in `db.Update(func(*Tx))` |
| `internal/engine/` | Scrape engine: `Job`/`Stage`, `Run` (TTL skip, ≤4 concurrent, resume via `sync_jobs`), shared rate `Limiter`, sync `Lock`, `RecordRaw` |
| `internal/sources/csejobs/` | Jobs for cse.lk JSON (market, securities, quotes, info, history, profile, reports, actions), `Plan`, pure normalisers |
| `internal/sources/reportjobs/` | `PDFs` (download + text) and `Fundamentals` (extract, store, optional LLM fallback) jobs |
| `internal/pdfx/` | PDF text with positions (`Pure` ledongthuc/pdf, `Poppler` if installed), safety caps, scanned detection, `Blobs` (PDFs by SHA-256); `pdftest` builds test PDFs |
| `internal/extract/` | Statement reader: titles, dated header columns, label dictionary (`Fields`), units, identity checks, confidence |
| `internal/llm/` | Opt-in Claude fallback (`claude-opus-5-5`); verifies every figure against the page text |
| `internal/csetest/` | Fake cse.lk server for tests, backed by `testdata/cse/` fixtures |
| `internal/cache/` | JSON file cache: `Load`/`Save` (atomic)/`Remove`/`RemoveAll`; `HEX_CACHE_DIR` overrides the root |
| `internal/const.go` | `CacheDir`, `CacheConfig` per cached source, `AllCaches` |
| `internal/output/` | `JSON` writer, Asia/Colombo date helpers |
| `internal/httpclient/` | `httpclient.Default`, the shared `*http.Client` (30s timeout) |
| `testdata/` | `cse/` API fixtures, `extract/` statement pages of real reports with expected values, `golden/` expected command output (`go test ./cmd -update` rewrites) |

## Conventions

- Wrap errors with context: `fmt.Errorf("fetch company profile for %s: %w", code, err)`, using a lowercase "verb noun" prefix.
- Commands use `RunE` and write to `cmd.OutOrStdout()`; cobra prints errors to stderr and `Execute` exits 1.
- Send all CSE HTTP through `internal/cse.Client` (which uses `httpclient.Default`). Never hard-code cse.lk URLs elsewhere.
- Scraped data goes in the SQLite store through engine jobs (see `scrape-engine`, `sqlite-store`), never in ad-hoc files. Every job is idempotent and has a TTL; CSE requests go through the engine's limiter.
- Fundamentals carry `source` (pdf, llm, manual), report, page, label and confidence. Manual values win when read. Never store an LLM figure that was not checked (see `pdf-extraction`).
- Cache files live in `os.UserCacheDir()/hex/<file>` (or `$HEX_CACHE_DIR`). Every new cache gets a `CacheConfig` in `internal/const.go`, an entry in `AllCaches`, **and** a subcommand under `hex clean` in `cmd/clean.go`.
- Symbols: users type `jkh` or `CIC.X0000`; resolve with `company.Resolve` (case-insensitive, prefers `.N0000`, keeps X/U series). The API gets the full code.
- Tests never hit the real cse.lk or the Anthropic API. Use `csetest.New(t)` (`Override` to replace a route), `t.Setenv(cache.EnvDir, t.TempDir())` and `t.Setenv(store.EnvDir, t.TempDir())`.
- `go build` needs `GOFLAGS=-buildvcs=false` on this machine (`/usr/local/bin/git` is an x86 binary Go cannot run).
- Analysis output is informational, not investment advice. Keep that disclaimer on analysis commands.

## Roadmap

The approved data-platform plan (CSE + CBSL sources, SQLite store, PDF fundamentals, analysis) is phased: 0 foundation (done), 1 store and sync (done: scrape engine), 2 history (done: accumulated daily bars), 3 corporate actions (partial: announcements stored and classified by category; dates need the unverified detail endpoint), 4 technical analysis (`hex analyze` phase 1 done: fundamentals, valuation, growth, cash, dividends, data-quality findings; phase 2 done: price/volume indicators in `internal/analysis/technical.go`, `--section technical`, on unadjusted prices; liquidity metrics and gate in `liquidity.go`; market context from `market_daily` in `market.go`; benchmarks, scores, bank/insurer rules and opt-in `--narrate` are next), 5 fundamentals from PDFs (done: rules + opt-in LLM), 6 CBSL macro, 7 screener/watchlist/portfolio and release.

## Known tech debt (fix when touching the area)

- Output is JSON only. Tables and a global `--json` toggle come with `internal/output` in Phase 1 (see `terminal-output`).
- `company list --names` replaces spaces with dashes (kept for compatibility).
- `hex company show-profile` / `show-financial` still call cse.lk live instead of reading the store.
- Raw responses are kept (`raw_responses`) but JSON normalisers cannot be re-run from them yet; `--reparse` covers PDFs only.

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
| Scrape engine: jobs, stages, TTLs, adding a data source | `scrape-engine` |
| SQLite store: migrations, repos, upserts | `sqlite-store` |
| Reading report PDFs: pdfx, extract, golden fixtures, LLM fallback | `pdf-extraction` |
| Committing: grouping changes, message format (`feat:`, `bugfix:`, `refac:`…) | `my-commits` |

General installed skills that also apply: `golang-pro` (idiomatic Go, concurrency), `cli-developer` (CLI UX, completions), `test-master`, `debugging-wizard`, `code-reviewer`, `security-reviewer` / `secure-code-guardian`, `code-documenter` (readme/help text), `data:statistical-analysis`, `equity-research:screen` / `:earnings` / `:sector` / `:thesis` (analysis ideas), `dataviz` (charts) and `devops-engineer` (GitHub Actions).
