# hex: current features

What `hex` does today, verified against the code. For the roadmap see [tasks.md](tasks.md); for what is missing for prediction see [prediction-research.md](prediction-research.md).

Build and install steps are in the root [readme.md](../readme.md).

## 1. Commands

| Command | What it does | Flags | Code |
|---|---|---|---|
| `hex company list` | `NAME,CODE` for every security | `-n/--names`, `-s/--symbols` | `cmd/company.go` |
| `hex company show-profile <SYM>` | Live profile JSON (still calls cse.lk) | none | `cmd/company.go` |
| `hex company show-financial <SYM>` | Live report list JSON (still calls cse.lk) | `-a -q -o -b -w -r` | `cmd/company.go` |
| `hex calc <ratio>` | One subcommand per registry ratio: formula, inputs, steps, result | `--json`, one flag per input | `cmd/calc.go` |
| `hex calc list` | Table of ratios, groups, formulas, flags | none | `cmd/calc.go` |
| `hex analyze <SYM>` | Report from the store only (no network, no LLM) | `--json`, `--section` | `cmd/analyze.go`, `cmd/analyze/` |
| `hex sync [group...]` | Runs the scrape engine | `-s`, `--all-symbols`, `-f`, `--dry-run`, `-c`, `--quarterly-reports`, `--annual-reports`, `--reparse`, `--llm`, `--llm-model`, `--llm-max` | `cmd/sync.go` |
| `hex sync status` | Per-kind job table and failures | `--json` | `cmd/sync.go` |
| `hex reports list <SYM>` | Report catalogue with parse status | none | `cmd/reports.go` |
| `hex reports extract <SYM>` | Download and read one report now | `--period`, `--llm`, `--llm-model`, `--show-pages` | `cmd/reports.go` |
| `hex fundamentals show <SYM>` | Winning stored values with source, page, label, confidence | `--all`, `--scope`, `--period` | `cmd/fundamentals.go` |
| `hex fundamentals set <SYM>` | Manual override at confidence 1.0 | `--period`, `--type`, `--scope`, `--field name=value` | `cmd/fundamentals.go` |
| `hex clean all\|securities\|pdf\|db\|raw` | Clear caches / database | none | `cmd/clean.go` |
| `hex version`, `hex completion <shell>` | Version, shell completion (symbols complete) | none | `cmd/version.go`, `cmd/args.go` |

Symbol handling (`company.Resolve`): case-insensitive and whitespace-trimmed; a bare ticker such as `jkh` picks the voting `.N0000` series (first match if there is no N series); a full code such as `CIC.X0000` keeps its series; an unknown symbol errors with a hint to run `hex company list --symbols`.

Command notes:
- `company list --names` replaces spaces with dashes; `--names` and `--symbols` are mutually exclusive; the old `--onlyNames` / `--onlySymbol` still work. It reads the cached symbol list when fresh.
- `company show-profile` (alias `profile`) prints summary, people (directors, executives) and content (business summary, articles, logos) as JSON.
- `company show-financial` (aliases `financials`, `reports`) shows every section with the resolved `symbol` unless one section flag is given: `-a` annual, `-q` quarterly, `-o` other filings, `-w` web links, `-r` extra data elements, `-b` banner ad. Each report entry has `title`, `date`, `uploaded`, `authorized` (null when absent) and `url`, in Asia/Colombo time. The old camelCase flags still work; `-c/--showCompanyBannerAd` is deprecated.

## 2. Ratios (`internal/analysis`, 22 in `Registry`)

| Group | Ratios |
|---|---|
| Profitability | net-margin, gross-margin, operating-margin, roe, roa |
| Valuation | market-cap, eps, pe, earnings-yield, bvps, pb, peg, graham |
| Income | dividend-yield, payout |
| Health | debt-equity, current-ratio, interest-cover |
| Cash flow | cash-conversion, fcf |
| Growth | growth (YoY) |

- Banks, finance companies and insurers get "n/a" for margins, debt/equity, current ratio and interest cover (`SkipFinancials`).
- `Snapshot` builds TTM (YTD + last full year − prior YTD), YoY, average balances; flags stale data and mismatched periods.
- Every metric carries steps, confidence (lowest input), sources and missing inputs.

### Technical indicators (`technical.go`)

From stored daily bars, shown in the `technical` section: returns 1W/1M/3M/YTD/1Y, SMA 20/50/200 and price vs SMA, EMA 12/26, RSI 14 (Wilder), MACD 12/26/9, Bollinger %B, ATR 14 (falls back to close-to-close when high/low are missing), volatility 20d and 1Y (annualised with 240 days), max drawdown with peak/trough dates, distance from 52-week high/low, volume vs 20-day average, return vs ASPI 3M/1Y, days with a price change in the last 14. Indicators that lack history show what they need. Prices are not adjusted for splits, bonus or rights.

## 3. Rule findings (`rules.go`)

Severity levels: positive, neutral, watch, risk. Thresholds are absolute (no sector benchmark yet).

| Dimension | Rules |
|---|---|
| Valuation | P/E (loss-making, <8, >20), P/B (<1 with ROE ≥15%, >3), price vs Graham number |
| Profitability | ROE (<0, <8%, ≥15%), margin widening/narrowing |
| Growth | revenue growth, profit growth (≥10%, ≤−20%, <0) |
| Health | current ratio (<1, ≥1.5), interest cover (<1.5, <3) |
| Cash flow | cash conversion (<0, <0.5, ≥1), free cash flow sign |
| Dividends | payout above 100% |
| Technical | RSI zones (30/70), price vs SMA 50/200 and their order, MACD vs signal, near 52-week high/low, volatility >50%, drawdown <-30% |
| Data quality | low confidence (<0.7), stale (>200 days), no price, thinly traded (<10 price changes in 14 days), stale price history (>10 days) |

Report sections: valuation, profitability, growth, health, cashflow, income, technical (price and volume). Output as text or JSON, with a coverage line and an informational-not-advice disclaimer.

## 4. Data collection

**Client** (`internal/cse`): retries with backoff, 16 MB cap, `ErrNoData`, `ErrUnknownSymbol`. Endpoints: marketStatus, marketSummery, aspiData, snpData, tradeSummary, allSectors, dailyMarketSummery, companyInfoSummery, companyChartDataByStock, companyProfile, financials, approvedAnnouncement, getFinancialAnnouncement, symbol CSV, PDF download.

**Engine** (`internal/engine`): stages, ≤4 concurrent jobs, TTL skip, resume via `sync_jobs`, shared rate limiter (5 req/s), sync lock, raw response recording.

| Job | Writes | TTL |
|---|---|---|
| securities | `securities` | 24h |
| market | `meta`, `index_bars` (ASPI, S&P SL20, sectors), `market_daily` | 12h (15 min market open) |
| quotes | `quotes`, share counts | 12h (5 min open) |
| info | `quote_stats`, security detail | 24h |
| history | `price_bars` (accumulating daily bars) | 24h |
| profile | `profiles`, `directors` | 7d |
| reports | `reports` catalogue | 24h |
| actions | `corporate_actions` (classified) | 24h |
| pdfs | PDF blobs, text, `report_pages` | 24h |
| fundamentals | `extraction_runs`, `fundamentals` | 24h |

Market open = weekdays 09:30–14:30 Asia/Colombo (no holiday calendar).

## 5. Store (`internal/store`, SQLite)

Tables: `securities`, `quotes`, `quote_stats`, `price_bars`, `index_bars`, `market_daily`, `meta`, `profiles`, `directors`, `corporate_actions`, `reports`, `report_pages`, `extraction_runs`, `fundamentals`, `raw_responses`, `sync_jobs`.

`quote_stats` holds 52-week high/low, YTD, all-time range, beta vs ASPI and S&P SL20, foreign %. `market_daily` holds turnover, trades, breadth inputs (listed, traded), PER/PBV/DY, domestic and foreign buy/sell.

Fundamentals key: (code, period_end, period_type, scope, field, source). Winner: manual, then highest confidence, then newest.

## 6. PDF fundamentals

- **`pdfx`**: pure-Go and Poppler text with positions, size/page/time caps, scanned-page detection, PDFs stored by SHA-256.
- **`extract`**: finds income/balance/cash-flow pages, rebuilds dated columns, scales units, maps labels to 46 fields (income 22 incl. bank and insurer fields, balance 11, cash flow 5), checks accounting identities (gross profit, PAT, total assets, EPS and NAV vs shares), scores confidence.
- **`llm`** (opt-in `--llm`, `claude-opus-5-5`): runs when rules score <0.7 or the PDF is scanned; every figure is verified against the printed page before it is stored.

## 7. Caches

`securities.json` (24h), `pdf/<sha256>.pdf` (no TTL), legacy `available_companies.txt`. Root is `$HEX_CACHE_DIR` or the user cache dir. Cleared by `hex clean`.

Cache location: `os.UserCacheDir()/hex/` (macOS `~/Library/Caches/hex/`), writes are atomic. `hex clean db` deletes the database (manual values included); `hex clean raw` drops only the kept raw responses; `hex clean all` leaves the database alone.

## 7a. Networking and CLI experience

- All cse.lk traffic goes through `internal/cse.Client` on a shared 30s-timeout HTTP client; up to 2 retries on network errors and 5xx (500 ms, then 1 s backoff); Ctrl-C cancels in-flight requests; empty answers (204, `{}`, `[]`) become "no data".
- Output is JSON for profile and financials, plain lines for `company list`, tables/text/`--json` for `calc`, `analyze` and `sync status`.
- Errors go to stderr with exit code 1; usage is not printed on runtime errors.
- Shell completion: `hex completion zsh > "${fpath[1]}/_hex"` (also bash, fish, powershell).
- `hex version` prints the `-ldflags "-X .../cmd.Version=x.y.z"` value (`dev` otherwise).
- Distribution: private Homebrew tap `Formula/hex.rb`, or `go install .`.

## 8. Testing support

`internal/csetest` fake cse.lk server, `testdata/cse` fixtures, `testdata/extract` (11 real-report fixtures with expected values), `testdata/golden` command output (`go test ./cmd -update`).

## 9. Stored but unused

`hex analyze` now reads `price_bars` and the ASPI rows of `index_bars`. Nothing reads these yet: `market_daily`, `quote_stats`, `directors`, `corporate_actions`, other index rows. They are the raw material for the prediction work.

## 10. Known gaps

- No sector/market benchmarks, scores, bank/insurer rules, or `--narrate`. Technical indicators exist but use unadjusted prices.
- Corporate-action ex/record/payment dates never filled (detail endpoint unverified; `FinancialAnnouncements` client method unused).
- `debt-equity` is in the registry but `analyze` never feeds it a debt input.
- `company show-profile` / `show-financial` still hit cse.lk live.
- Raw JSON responses cannot be re-parsed (`--reparse` covers PDFs only).
- No CBSL macro, screener, watchlist or portfolio.
- Output has no tables or global `--json` toggle yet; `company list --names` dash replacement is kept for compatibility.
- PDF extraction is heuristic: check low-confidence values against the PDF or set them with `hex fundamentals set`.

## 11. Roadmap status

| Phase | Scope | State |
|---|---|---|
| 0 | Foundation | done |
| 1 | SQLite store and scrape engine | done (tables and global `--json` pending) |
| 2 | Price history (accumulated daily bars) | done |
| 3 | Corporate actions | announcements stored; dates and price adjustment pending |
| 4 | Technical analysis | fundamentals and price/volume indicators done; benchmarks, scores pending |
| 5 | Fundamentals from PDFs | done (rules + opt-in LLM) |
| 6 | CBSL macro | not started |
| 7 | Screener, watchlist, portfolio, release | not started |
