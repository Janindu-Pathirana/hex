---
name: screening-and-portfolio
description: Build stock screener, watchlist, sector comparison and portfolio tracking features in hex, covering filter/sort specs over all CSE securities, peer and sector medians, local watchlist/portfolio files, P&L, allocation and dividend income math, and throttled bulk fetching. Use when adding "hex screen", "hex watchlist", "hex portfolio", "hex market sectors" or any feature that works across many symbols.
---

# Screening, watchlists and portfolios

## Bulk data: one call, not N
- `tradeSummary` returns **every traded security** with price, change, volume, turnover and market cap in one request. Build the screener universe from it.
- Add per-symbol detail (`companyInfoSummery`: 52w hi/lo, beta, shares) only for the rows that survive the cheap filters. Fetch with bounded concurrency (≤4, see `go-conventions`) and cache the results (see `cache-data`).
- Sector membership: `companyProfile → reqComSumInfo[].sector`. It is slow to fetch per symbol, so build a cached `symbol → sector` map (TTL 7d) the first time a sector feature runs, and show progress on stderr.

## Screener
```
hex screen --min-mcap 10bn --max-pe 10 --min-dy 5 --sector Banks --sort -turnover --limit 20
```
- Model each filter as `type Filter struct{ Field string; Op string; Value float64 }` and parse from flags, or from `--where "pe<10,dy>=5"`.
- Fields map to typed getters. Unknown field → error listing the valid fields.
- Rows with a nil metric **fail** numeric filters, and sort last.
- Accept human number suffixes (`k`, `mn`, `bn`) in flags.
- Built-in presets (`--preset value|momentum|dividend|liquid`) are combinations of filters. Document what each one means in `--help`.
- Liquidity guard: default `--min-turnover`, so illiquid stocks with stale prices don't flood the results. Allow it to be disabled.

## Sector view
`hex market sectors` uses `allSectors` (index value, change, turnover per GICS group). Add peer medians for P/E, P/B and DY by grouping screener rows by sector. Use the median, not the mean, because CSE outliers are extreme.

## Watchlist
- File: `os.UserConfigDir()/hex/watchlist.json` → `{"symbols": ["JKH.N0000", "COMB.N0000"]}`. This is user data, so `hex clean` never deletes it.
- Commands: `hex watchlist add|remove|list|show`. `show` renders price, % change, a sparkline and RSI for each symbol.
- Normalise and validate symbols against the company list on `add`.

## Portfolio
- File: `os.UserConfigDir()/hex/portfolio.json` with a list of transactions (not just holdings), so P&L can be rebuilt:
  ```json
  {"transactions":[{"date":"2026-03-02","symbol":"JKH.N0000","side":"buy","qty":1000,"price":19.40,"fees":0}]}
  ```
- Holdings: net quantity per symbol. Cost basis uses the **average cost** method (state it in help), with fees included in cost.
- Metrics: market value, unrealised P&L (LKR and %), realised P&L from sells, day change, allocation % by holding and by sector, dividend income (from user-entered dividends or XD announcements × qty held on the XD date).
- Brokerage fees on the CSE are a percentage of trade value, made up of several components [verify current rates]. Let the user enter `fees` per transaction rather than hard-coding rates.
- Write files atomically (tmp + rename) and keep a `.bak` of the previous version.

## Output
Use tables from `terminal-output`, colour P&L, put totals in a footer row, provide `--json`, and include the disclaimer.

## Tests
- Feed fixture `tradeSummary` JSON into the screener and check filter, sort and nil handling.
- Write portfolio math table tests, including partial sells and average-cost updates. See `go-testing`.
