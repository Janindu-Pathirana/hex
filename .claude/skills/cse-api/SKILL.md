---
name: cse-api
description: How to call the Colombo Stock Exchange (cse.lk) web API from hex, covering request format, headers, symbol codes, the known endpoints (prices, trade summary, company info, chart/history, indices, sectors, movers, announcements, financials), mapping JSON to Go structs, and handling API changes. Use whenever fetching CSE data, adding an endpoint, writing response structs, or debugging a failing CSE request.
---

# CSE web API

The cse.lk website runs on an undocumented JSON API at `https://www.cse.lk/api/<endpoint>`. It is not an official public API, so expect it to change, and be polite with request volume.

## Request format (verified 2026-10-13)
- Method `POST`, body `application/x-www-form-urlencoded`. An empty body is fine for market-wide endpoints.
- Headers (set by `Client.post` in `internal/cse/client.go`):
  ```
  Content-Type: application/x-www-form-urlencoded; charset=UTF-8
  Accept: application/json, text/plain, */*
  Origin: https://www.cse.lk
  Referer: https://www.cse.lk/
  User-Agent: Mozilla/5.0 (...) Chrome Safari
  ```
- Responses are JSON, and timestamps are **Unix milliseconds**. Display them in `Asia/Colombo` (UTC+05:30, no DST).
- The company list is a CSV GET: `https://www.cse.lk/chat/company_symbol_list.csv`. Its header row is followed by `COMPANY NAME,SYMBOL.N0000` lines.

## Symbols
- The full security code is `TICKER.SERIES`: `JKH.N0000` for voting shares, `.X0000` for non-voting. Other series exist for rights and warrants.
- `companyInfoSummery` **requires the full code** (`JKH` returns `{}`). `companyProfile` and `financials` accept either form.
- `companyChartDataByStock` takes a numeric `stockId`. That is the `id` from `reqSymbolInfo` in `companyInfoSummery`, or `id` in `tradeSummary` (JKH = 297).

## Endpoints
Full field lists are in [references/endpoints.md](references/endpoints.md). Summary:

| Endpoint | Params | Gives |
|---|---|---|
| `marketStatus` | none | `{"status":"Regular Trading"}` (or a closed state) |
| `marketSummery` | none | today's turnover, share volume, trades |
| `aspiData` / `snpData` | none | ASPI / S&P SL20 value, high, low, change, % |
| `tradeSummary` | none | **every traded security today**: price, change, high/low/open/close, volume, turnover, market cap |
| `todaySharePrice` | none | lighter price list for all securities |
| `detailedTrades` | none | per-security trades/qty today |
| `topGainers` / `topLooses` / `mostActiveTrades` | none | market movers |
| `allSectors` | none | S&P/CSE GICS industry-group indices and today's sector stats |
| `dailyMarketSummery` | none | daily history: turnover, foreign flows, market cap, **market PER, PBV, DY**, ASPI |
| `companyInfoSummery` | `symbol=JKH.N0000` | quote, WTD/MTD/YTD/52w/all-time hi-lo, volumes, market cap, **beta**, ISIN, issued qty, `id` (stockId) |
| `companyChartDataByStock` | `stockId=297&period=N` | price history (see periods) |
| `chartData` | `chartId=1&period=N` | ASPI history (`chartId=1`). Other ids are unverified |
| `companyProfile` | `symbol` | directors, business summary, sector, auditors, fin. year end |
| `financials` | `symbol` | annual/quarterly report **PDF links** (`https://cdn.cse.lk/` + `path`, verified) |
| `approvedAnnouncement` | none | corporate disclosures, XD/AGM dates |
| `getFinancialAnnouncement` | none | latest financial report uploads market-wide |

**Periods** (both chart endpoints): `1` intraday (per trade or minute), `2` ~1 week, `3` ~1 month, `4` ~3 months, `5` ~1 year (daily bars). Daily bars are stamped at 00:00 Colombo time (18:30 UTC the previous day). Field keys for stock history are `t` time, `p` close/last, `h` high, `l` low, `o` open (often null), `q` volume. Anything beyond 1 year must be accumulated locally (see `cache-data`).

## Client design (target)
Implemented in `internal/cse` (Phase 0). Add new endpoints as methods on `Client`:
```go
type Client struct {
	BaseURL string       // "https://www.cse.lk/api/" — overridable in tests
	HTTP    *http.Client // httpclient.Default
}
func (c *Client) post(ctx context.Context, endpoint string, form url.Values, out any) error
func (c *Client) TradeSummary(ctx context.Context) ([]TradeSummary, error)
```
`post` sets the headers, checks the status, and decodes JSON. It treats an empty `{}` / `[]` response as `ErrNoData` (an unknown symbol usually returns `{}` with status 200, not a 404).

## Structs
- Put them in `internal/cse/` next to the method that returns them.
- Keep the API's spelling in JSON tags (`reqTradeSummery`, `topLooses`, `sharevolume`) but use correct Go names (`TradeSummary`, `ShareVolume`).
- Any field seen as `null` becomes a pointer. Numbers that may arrive as `1.0` go in `float64`, even when they look like ints.
- Map only the fields you use. Unknown fields are ignored by `encoding/json`.

## Discovering or confirming an endpoint
1. Open the page on cse.lk → DevTools → Network → filter `api/`. Note the endpoint, form params and response.
2. Reproduce it with curl:
   ```bash
   curl -s -X POST https://www.cse.lk/api/companyInfoSummery \
     -H 'Content-Type: application/x-www-form-urlencoded; charset=UTF-8' \
     -H 'Origin: https://www.cse.lk' -H 'Referer: https://www.cse.lk/' \
     --data 'symbol=JKH.N0000' | python3 -m json.tool | head -50
   ```
3. Save a trimmed response as a fixture in `testdata/cse/<endpoint>.json` (see `go-testing`).
4. Add the endpoint to `references/endpoints.md` with the date you verified it.

## Etiquette and robustness
- At most ~4 concurrent requests. Prefer one market-wide call (`tradeSummary`) over N per-symbol calls.
- Cache aggressively outside market hours (see `cse-market-context` for hours).
- On a decode failure, include the endpoint name and the first ~200 bytes of the body in the error, so schema changes are easy to diagnose.
