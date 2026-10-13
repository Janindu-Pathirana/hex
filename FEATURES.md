# hex features

A quick reference to what `hex` can do today. For build and install steps, see [readme.md](readme.md).

`hex` is a Go command-line tool that pulls data from the Colombo Stock Exchange website (cse.lk). It lists companies and, with its **scrape engine** (`hex sync`), stores market data, quotes, company detail, price history, profiles, report catalogues and announcements in a local database, and reads income statement, balance sheet and cash flow figures out of the report PDFs. Analysis features on top of that data are planned (see [Roadmap](#roadmap)).

> Analysis output is informational only and is not investment advice.

## Feature index

| Feature | Command | Notes |
|---|---|---|
| [List companies](#list-companies) | `hex company list` | Every listed security, or only names or tickers |
| [Company profile](#company-profile) | `hex company show-profile <SYMBOL>` | Company info, directors, summaries, as JSON |
| [Financial reports](#financial-reports) | `hex company show-financial <SYMBOL>` | Annual/quarterly/other report PDF links |
| [Scrape engine](#scrape-engine) | `hex sync [group...]`, `hex sync status` | Incremental, resumable, rate-limited |
| [Report fundamentals](#report-fundamentals) | `hex sync fundamentals`, `hex reports`, `hex fundamentals` | Figures read from PDFs, with confidence; manual overrides |
| [Symbol resolution](#symbol-resolution) | any `<SYMBOL>` argument | `jkh`, `JKH`, `CIC.X0000` |
| [Caching](#caching) | `hex clean all`, `hex clean securities`, `hex clean pdf`, `hex clean db`, `hex clean raw` | Symbol list 24h, PDFs by hash, database kept by `clean all` |
| [Reliable networking](#networking-and-reliability) | all commands | Retries, 30s timeout, Ctrl-C cancels |
| [Shell completion](#cli-experience) | `hex completion <shell>` | Completes ticker symbols |
| [Version](#cli-experience) | `hex version` | Version set at build time |
| [Distribution](#distribution) | `brew install`, `go install` | Private Homebrew tap |

---

## Company data

### List companies

```bash
hex company list              # NAME,CODE for every security (N, X and U series)
hex company list --symbols    # unique tickers, e.g. JKH
hex company list --names      # unique company names
```

- `--symbols` / `-s`: unique tickers, in list order.
- `--names` / `-n`: unique company names. Spaces become dashes (kept for compatibility).
- You can't use `--names` and `--symbols` together.
- The old flag names `--onlyNames` and `--onlySymbol` still work.
- It reads from the [cached](#caching) symbol list when that list is fresh.

### Company profile

```bash
hex company show-profile JKH
hex company profile cic.x0000   # alias
```

Prints the company's cse.lk profile as JSON:

- **Summary**: name, symbol, sector, board type, registered office, phone, fax, email, website, year established, financial year end, quarter date, capital (authorised and issued), secretaries and auditors
- **People**: directors and key executives
- **Content**: business summary, articles (PDFs), top posts, logos and block ads

### Financial reports

```bash
hex company show-financial JKH        # all sections
hex company show-financial JKH -q     # quarterly only
hex company financials JKH -a         # aliases: financials, reports
```

With no flag, every section is shown along with the resolved `symbol`. Each section flag shows one section only, and you can use just one at a time:

| Flag | Section |
|---|---|
| `-a`, `--annual` | Annual reports |
| `-q`, `--quarterly` | Quarterly reports |
| `-o`, `--other` | Other filings |
| `-w`, `--web-link` | Web links |
| `-r`, `--req-financial` | Extra financial data elements |
| `-b`, `--banner-ad` | Company banner ad |

Each report entry has:

```json
{ "title": "...", "date": "...", "uploaded": "...", "authorized": "...", "url": "https://.../report.pdf" }
```

- Dates and times are in **Asia/Colombo** time.
- `authorized` is `null` when the CSE gives no authorisation time.
- The old camelCase flags (`--showAnnualData`, `--showQuarterlyData`, …) still work. `--showCompanyBannerAd` / `-c` is deprecated; use `-b` instead.

## Symbol resolution

Every `<SYMBOL>` argument goes through the same resolver:

- **Case-insensitive**: `jkh`, `JKH` and ` Jkh ` all work.
- **Bare ticker** → picks the voting `.N0000` series (for example `JKH` → `JKH.N0000`). If there is no N series, it picks the first matching code.
- **Full code** → that exact series, so you can choose non-voting `X` and `U` series (`CIC.X0000`).
- **Unknown symbol** → an error that suggests `hex company list --symbols`.

## Caching

| Cache | File | TTL | Clear with |
|---|---|---|---|
| Securities (symbol list) | `securities.json` | 24h | `hex clean securities` (alias `available-companies`) |
| Report PDFs | `pdf/<sha256>.pdf` | kept | `hex clean pdf` (text stays in the database) |
| Everything | (all of the above, plus the old `available_companies.txt`) | — | `hex clean all` |

The database is not a cache: `hex clean db` deletes it (including manual values) and `hex clean raw` drops only the kept raw responses.

- Location: `os.UserCacheDir()/hex/` (on macOS, `~/Library/Caches/hex/`).
- Set `HEX_CACHE_DIR` to use a different folder.
- Writes are atomic, so a crash never leaves a half-written cache file.

## Networking and reliability

- All cse.lk traffic goes through one client (`internal/cse.Client`) that uses a shared HTTP client with a **30s timeout**.
- **Retries**: up to 2 extra attempts on network errors and HTTP 5xx responses, with exponential backoff (500ms, then 1s).
- **Ctrl-C** cancels requests that are still running.
- If cse.lk returns an empty answer (204, `{}` or `[]`), hex reports it as "no data".

## CLI experience

```bash
hex completion zsh > "${fpath[1]}/_hex"   # also bash, fish, powershell
hex version
```

- **Shell completion** fills in ticker symbols for `show-profile` and `show-financial` as you type.
- **Output** is JSON (pretty-printed) for profile and financials, and plain lines for `company list`.
- **Errors** go to stderr and the exit code is 1. Usage text is not printed on runtime errors.
- **`hex version`** prints the version baked in with `-ldflags "-X .../cmd.Version=x.y.z"` (`dev` otherwise).

## Distribution

- Private Homebrew tap: `Formula/hex.rb` (`brew install --HEAD hex`, or a tagged release).
- `go install .` from the repo.

## Testing (for contributors)

- Tests never call the real cse.lk or the Anthropic API. `internal/csetest` runs a fake server backed by `testdata/cse/` fixtures; `pdftest` builds small PDFs.
- `testdata/extract/` holds the statement pages of 11 real reports with expected values (`TestGoldenReports`).
- Golden files in `testdata/golden/` lock down command output. Regenerate them with `go test ./cmd -update`.
- `go vet ./... && go test -race -cover ./...`

---

## Known limitations

- Output is JSON only. There are no tables and no `--json` toggle yet.
- `company list --names` replaces spaces with dashes.
- `company show-profile` / `show-financial` still fetch live rather than reading the database.
- Announcement dates (XD, payment) are not in the list endpoint, so corporate actions have no dates yet.
- PDF extraction is heuristic: check low-confidence values against the PDF, or set them by hand.

## Roadmap

| Phase | Scope |
|---|---|
| 0 | Foundation ✅ |
| 1 | Local SQLite store and sync ✅ (table output and global `--json` still to come) |
| 2 | Price history ✅ (accumulated daily bars) |
| 3 | Corporate actions: announcements stored ✅; dates and price adjustment to come |
| 4 | Technical analysis (SMA/EMA, RSI, MACD, volatility) |
| 5 | Fundamentals from report PDFs ✅; ratios on top of them (P/E, P/B, ROE, yield) to come |
| 6 | CBSL macro data |
| 7 | Screener, watchlist, portfolio P&L, and release |
