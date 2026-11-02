# cse.lk data sources: what was verified live

Checked on **2026-11-02** against www.cse.lk, using `hex sync` into a scratch store and the website's own requests in a browser. Anything not listed here is unverified. Re-check before relying on it: the site changes (it moved to a new front end with `/api/smd`).

## 1. Announcements with dates and ratios: `POST /api/smd`

The old `approvedAnnouncement` list that hex uses today carries **no dates, ratios or symbols** (0 of 63 rows had `xd`, `recordDate` or `paymentDate`, including 8 cash dividends; 49 of 63 resolved to a code by company name). The website's Announcements page gets complete data from a different endpoint.

- **Request:** `POST https://www.cse.lk/api/smd`, `Content-Type: application/json`.
  ```json
  {"companyIds": [728], "categories": ["CASH DIVIDEND"], "fromDate": "2018-01-01", "toDate": "2026-11-02"}
  ```
  - `companyIds` are `securityId` values from `GET /api/cntSecurity` (`{securityId, name, symbol, boardId, deleted}`), also called `secId` in responses. Empty or missing gives HTTP 400 `one or more company should select`.
  - `categories` are names from `GET /api/smd/categories` (about 55 strings). Missing gives 400 `one or more category should select`.
  - `fromDate` / `toDate` are ISO dates. Other spellings (`startDate`, `from`, `dateFrom`...) are accepted but ignored (the result is then only the latest day). History back to 2018 came back for one company (15 cash dividends).
- **Response:** an object with keys `Financial Statements`, `Announcement` and `CSE Reports`. `Announcement` maps category name to a list of rows. Dates in rows are `DD-MM-YYYY`.
- **Limits seen:** 20 companies per request with 6 categories worked (18 requests covered all 356 securities from 2018). Asking for all companies at once, or `RIGHTS ISSUE (DATES)` back to 2018 for one company, returned HTTP 500 `Database error`. Chunk by 20 companies and treat a 500 as "retry smaller" rather than "no data".
- **Related:** `POST getAnnouncementById` (form `announcementId=`), `getGeneralAnnouncementById`, `announcementById` (form `id=`) exist in the site's code; not called yet. Rows link to follow-ups through `childAnnouncement`.

### Fields by category (non-null counts from 2018 to 2026-11-02)

| Category | Rows | What it carries |
|---|---|---|
| `CASH DIVIDEND` | many | `xd`, `recordDate`, `payment`, `votingDivPerShare`, `nonVotingDivPerShare`, interim/final flags, `remarks` |
| `SCRIP DIVIDEND` | 105 | `votingPropotion` like `"1 for 43.90"` or `"1:23.5294118079"`, `votingConsideration` (price), `xd`, `recordDate`, `payment` on 18 rows |
| `SCRIP DIVIDEND (DATES)` | 84 | `xd` (70), `recordDate` (43), `payment` (20), `agm`, proportion text |
| `SUB-DIVISION OF SHARES` | 64 | **ratio only in `remarks` text** ("every 01 Ordinary Share into 20", "every one (1) ... into four (4)"); 20 rows have no ratio at all (e.g. JKH 2024: "Sub-division of Shares 2024"); `tradingCommencement`, `tradingSuspended`, `subDivisionBasedOnShareholdingsAsAt` on about a third |
| `SUB-DIVISION OF SHARES (DATES)` | 15 | `tradingSuspended`, `tradingCommencement` (all), entitlement date, `egm` |
| `CAPITALIZATION OF RESERVES` (bonus) | 2 | `xc` (ex-capitalisation date), `dateAllotment`; proportion text on 1 row (`"1 share for every 17.5287357329"`) |
| `RIGHTS ISSUE` | 252 | `votingShrsPropToBeIssued` text ("03 Ordinary Voting share for every 14 existing", "One (01) new ... for every One (01)"), `votingShareConsideration` (issue price), `numOfVotingShrsIssued`; `xr`, `recordDate`, `allotment`, `tradingCommencement` only on 27 rows (the later "dates" announcements); the same rights issue appears as several rows |
| `RIGHTS ISSUE (DATES)` | ? | HTTP 500 for a wide range; try recent ranges |

Not seen: any structured numeric ratio field for splits or bonus issues. Ratios must be parsed from text and are sometimes absent.

## 2. Price history is raw (unadjusted)

`companyChartDataByStock` (period 5, about 240 daily bars) returns prices as traded. WAPO (sub-division 1 into 4, trading resumed 2026-03-23): close **102.75 on 2026-03-16, 23.60 on 2026-03-23**, volume unchanged in scale. So every return, indicator, volatility and drawdown in `hex analyze` is wrong across a split or bonus. `open` is empty for most bars; `high`/`low` are present.

History depth is one year per fetch. After the first sync, bars accumulate only while `hex sync history` keeps running.

## 3. Real-data run: what `hex analyze` did on live data

Synced `securities market quotes actions` plus `info history profiles reports` for JKH, COMB, WAPO, HHL, SAMP, and downloaded and extracted reports for JKH, COMB, SAMP. Everything synced without errors (5 req/s limiter). Findings:

| # | Finding | Status |
|---|---|---|
| 1 | One-year windows failed by a few days ("needs history from 2025-10-30", first bar 2025-11-03) | **Fixed here**: a window may start up to 7 days before the first bar |
| 2 | `index_bars` and `market_daily` hold only 2 days, so every ASPI/S&P SL20 comparison and market trend is `n/m` | **Fixed for the indices**: `hex sync index` stores a year of ASPI and S&P SL20 levels. `market_daily` still has 2 days (the endpoint returns only the latest), so turnover and foreign flow build up with daily syncs |
| 3 | Default of 4 quarterly reports gives no prior-year period, so growth, margin change, Piotroski show `n/m` (JKH: 2 of 8 tests) | **Fixed**: default is 6 reports; JKH now shows revenue growth +24.1%, margin changes and Piotroski 5 of 6 |
| 4 | SAMP (bank): "no statement values found" for a report; EPS, ROE, cash flow all missing | Open: bank statement layouts need extractor work (P4 bank rules) |
| 5 | JKH: capex and debt not extracted, so FCF and debt/equity are missing | Open: label dictionary |
| 6 | Several values at confidence 0.65 ("few identities could be checked") | Expected; shows in the data-quality finding |
| 7 | Large numbers print with stray decimals (`69,385,571.4`, `3,828,396.45`) | Open: round large values to whole numbers |
| 8 | No crashes, NaN or Inf in any section | |

## 4. Consequences for the plan

- Corporate-action dates and ratios exist, but from `/api/smd`, not from `approvedAnnouncement`: switch the Actions job to it (per-company, chunked, categories from `/api/smd/categories`).
- Adjustment factors need a text ratio parser with a manual override, because sub-division ratios are free text and sometimes missing. Rights issues need the issue price and the pre-ex close to compute a theoretical ex-rights factor; that data exists (`votingShareConsideration`, proportion text).
- The chart is raw, so adjustment is required, not optional.

## 5. Index charts: `POST /api/chartData`

Verified 2026-11-02. Form body `chartId=<id>&period=5`. `chartId=1` is the ASPI and **`chartId=40` is the S&P SL20** (found in the site's own code; ids 2 to 8 return `[]`). The response is a plain array of `{d (ms), v (level), pc (percent change)}`, 240 daily points for `period=5`. `d` is a time on the trade date, not midnight. The `fromDate`/`toDate` fields the site can send are ignored for `period=5`, so history beyond a year has to be accumulated.
