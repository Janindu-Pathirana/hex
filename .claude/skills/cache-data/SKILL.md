---
name: cache-data
description: Add or change local caching of CSE data in hex: CacheConfig entries, TTL choices by data type, read/write helpers under the user cache dir, storing accumulated price history, and wiring a matching "hex clean" subcommand. Use when a command re-fetches data that could be cached, when adding price history storage, or when touching internal/const.go or cmd/clean-cash.go.
---

# Caching in hex

## Existing pattern
- `internal/const.go`: `CacheDir = "hex"`, `CacheConfig{CacheFile, CacheTTL}`, and `AvailableCompaniesCache` (24h).
- `cmd/company/available_companies.go`: path = `os.UserCacheDir()/hex/<file>`. The cache is valid if the file's mtime is within the TTL. Save with `MkdirAll(0o755)` + `WriteFile(0o644)`.
- `cmd/clean-cash/root.go` + `cmd/clean-cash.go`: `hex clean all` and `hex clean <name>`.

## Adding a cached source
1. Add a `CacheConfig` var in `internal/const.go`:
   ```go
   var TradeSummaryCache = CacheConfig{CacheFile: "trade_summary.json", CacheTTL: 5 * time.Minute}
   ```
2. Once there are two or more caches, extract a generic helper instead of copying load/save:
   ```go
   // internal/cache
   func Load[T any](cfg internal.CacheConfig, ttl time.Duration) (v T, ok bool, err error)
   func Save[T any](cfg internal.CacheConfig, v T) error   // JSON; write to tmp file then os.Rename (atomic)
   func Path(cfg internal.CacheConfig) (string, error)
   ```
   Allow a root override (e.g. `HEX_CACHE_DIR` env) so tests can use `t.TempDir()`.
3. Keep a list of all configs, e.g. `internal.AllCaches = []CacheConfig{...}`. Make `CleanAllCash` loop over it, and have it **ignore `os.ErrNotExist`**: today it fails when the file is missing.
4. Add `hex clean <name>` in `cmd/clean-cash.go`.
5. Add a `--refresh` (or `--no-cache`) persistent flag that bypasses reads but still writes.

## TTL guide
| Data | TTL |
|---|---|
| Company/symbol list | 24h |
| Company profile, financial report links | 24h–7d |
| Quotes, trade summary, movers, sectors, during market hours | 1–5 min |
| Same data outside market hours | until the next market open (see `cse-market-context`) |
| Daily history (period 5) | until the next trading day |
| Market status | don't cache, or cache for ≤1 min |

A helper `marketAwareTTL(now)` can return a short TTL during 09:30–14:30 Asia/Colombo on weekdays and a long one otherwise.

## Accumulating price history
The API gives at most ~1 year of daily bars. For longer analysis:
- Store per-symbol daily bars in `os.UserCacheDir()/hex/history/<SYMBOL>.json` (or CSV), keyed by trade date (Colombo date).
- On fetch, merge new bars by date: replace same-date entries and keep older ones. This is history to preserve, so `hex clean all` should prompt before deleting it, or skip it unless `--include-history` is passed.
- If it grows, consider SQLite (`modernc.org/sqlite`, pure Go, so no cgo and Homebrew builds stay simple). Justify it first per `go-conventions`.

## User data vs cache
Watchlists and portfolios are **user data**, not cache. Keep them in `os.UserConfigDir()/hex/`, which `hex clean` never touches (see `screening-and-portfolio`).
