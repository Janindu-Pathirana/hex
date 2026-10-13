# CSE API endpoint reference

Base: `POST https://www.cse.lk/api/<endpoint>` (form-encoded). Field lists come from live responses on **2026-10-13** unless marked otherwise. `ms` means Unix milliseconds. `?` means the field was observed as `null`, so use a pointer.

## Market-wide

### `marketStatus`
`{"status": "Regular Trading"}`. Other values appear when the market is closed (e.g. "Market Closed"); record them as you see them.

### `marketSummery`
`id, tradeVolume (float, = turnover LKR), shareVolume (int), tradeDate (ms), trades (int)`

### `aspiData`, `snpData`
`id, value, lowValue, highValue, change, percentage, sectorId, timestamp (ms)`
ASPI is `sectorId` 1 and S&P SL20 is `sectorId` 40.

### `tradeSummary` → `{"reqTradeSummery": [...]}`
`id (stockId), name, symbol (JKH.N0000), logoUrl, quantity, percentageChange, change, price, previousClose, high, low, open, closingPrice, lastTradedTime (ms), issueDate, turnover, sharevolume, tradevolume, marketCap, marketCapPercentage, crossingVolume, crossingTradeVol, status`
This is the best single source for a screener: one call returns all securities.

### `todaySharePrice` → `[...]`
`id, symbol, open, high, low, lastTradedPrice, change, changePercentage, crossingVolume, tradesTime (ms), quantity`

### `detailedTrades` → `{"reqDetailTrades": [...]}`
`id, securityId?, name, symbol, price, qty, trades, change, changePercentage, logoUrl`

### `topGainers`, `topLooses` → `[...]`
`id, securityId, symbol, price, change, changePercentage, tradeDate (ms)`

### `mostActiveTrades` → `[...]`
`id, securityId, symbol, tradeVolume, shareVolume, turnover, percentageShareVolume`

### `allSectors` → `[...]`
`id, sectorId, symbol (e.g. EGY), indexCode, indexCodeSp (e.g. SPCSEEIP), indexName ("S&P/CSE Energy Industry Group Index"), name, indexValue, change, percentage, sectorTradeToday, sectorVolumeToday, sectorTurnoverToday, sectorPreviousClose, transactionTime (ms)`

### `dailyMarketSummery` → `[[...]]` (note the nested array)
`id, tradeDate (ms), marketTurnover, marketTrades, marketDomestic, marketForeign, equityTurnover, equityDomesticPurchase, equityDomesticSales, equityForeignPurchase, equityForeignSales, volumeOfTurnOverNumber, volumeOfTurnoverDomestic, volumeOfTurnoverForeign, tradesNo, tradesNoDomestic, tradesNoForeign, listedCompanyNumber, tradeCompanyNumber, marketCap, cdsHoldings{Total,Domestic,Foreign}, cdsMarketValue{Total,Domestic,Foreign}, corporateDebtValue, governmentDebtValue, triasi, spt, pbv, dy, per, spp, trimpi?, mpi?, asi`
- `per` / `pbv` / `dy` are market-wide P/E, P/BV and dividend yield, a useful benchmark for company ratios.
- Foreign net flow = `equityForeignPurchase - equityForeignSales`.

### `chartData` (params `chartId`, `period`) → `[{d (ms), v, pc?}]`
`chartId=1` is ASPI. `chartId=2` returned nothing, so the S&P SL20 id is **unverified**; find it in DevTools on the index page.

## Per-company

### `companyInfoSummery` (param `symbol=JKH.N0000`, full code required)
- `reqSymbolInfo`: `id (stockId), symbol, name, isin, issueDate, quantityIssued, parValue, issuedPrice?, lastTradedPrice, previousClose, closingPrice, change, changePercentage, hiTrade, lowTrade, marketCap, marketCapPercentage, {wtd,mtd,ytd,p12,all}HiPrice, {wtd,mtd,ytd,p12,all}LowPrice, {tdy,wdy,mtd,ytd,p12}ShareVolume, {tdy,wtd?,mtd?,ytd?,p12?}TradeVolume, {tdy,wtd,mtd,ytd,p12?}Turnover, foreignHoldings?, foreignPercentage?, symbolIndexShareVolume`
  - `p12*` = trailing 52 weeks.
- `reqSymbolBetaInfo`: `securityId, triASIBetaValue, betaValueSPSL, triASIBetaPeriod, quarter`
- `reqLogo`: `{id, path, secId}`, `reqTagsLogo?`

### `companyChartDataByStock` (params `stockId`, `period`) → `{"chartData": [...]}`
`t (ms), p (price/close), h, l, o?, q (volume), c?, pc?, s, n?, id`
- Periods: 1 = intraday ticks (e.g. 508 rows), 2 = 1W, 3 = 1M, 4 = 3M, 5 = 1Y daily (~241 rows). Period 6 behaved like 2.
- Daily bars are stamped 18:30 UTC (= 00:00 Asia/Colombo of the trade date).

### `companyProfile` (param `symbol`)
Mapped in `internal/cse/company.go` (`CompanyProfile`): directors, `reqComSumInfo` (sector, boardType, finYearEnd, auditors…), business summary, logos.

### `financials` (param `symbol`)
Mapped in `internal/cse/company.go` (`Financials`, `Report`): `infoAnnualData`, `infoQuarterlyData`, `infoOtherData`, each with `path` → PDF at `https://cdn.cse.lk/<path>` (verified), `fileText` (e.g. "Annual Report as at 31st March 2026"), `manualDate`/`uploadedDate`/`authorizedDate` (ms).

## Announcements

### `approvedAnnouncement` → `{"approvedAnnouncements": [...]}`
`id, createdDate (ms), dateOfAnnouncement ("12 Oct 2026"), announcementId, announcementCategory, company, type?, symbol?, recordDate?, allotment?, xr?, xc?, xd?, tradingCommencement?, tradingSuspended?, paymentDate?, agmDate?, dateOfAgmInitial?, logoUrl, remarks`

### `getFinancialAnnouncement` → `{"reqFinancialAnnouncemnets": [...]}`
`id, path, manualDate (ms), uploadedDate (string "12 Oct 2026 06:05:35 PM"), fileText, name, symbol (short, e.g. ASPH), logoUrl`

## Static

### `GET https://www.cse.lk/chat/company_symbol_list.csv`
A header row, then `COMPANY NAME,TICKER.SERIES`. Parsed by `cse.ParseSymbolList`, cached for 24h by `company.Securities` (`cmd/company/company.go`). Unknown symbols: `companyProfile` returns **204** with an empty body, `financials` returns `{}` (verified 2026-10-13); both map to `cse.ErrNoData`.
