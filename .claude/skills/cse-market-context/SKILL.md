---
name: cse-market-context
description: Domain knowledge about the Sri Lankan stock market (Colombo Stock Exchange) needed to build hex features correctly, covering indices (ASPI, S&P SL20), GICS sectors, listing boards (Main, Diri Savi, Empower), security codes, trading hours and holidays, settlement, currency, regulators and circuit breakers. Use when a feature depends on market rules or terminology, e.g. market-hours logic, index/sector displays, board filters or symbol parsing.
---

# Sri Lankan market context

Each fact is tagged **[verified]** (confirmed against live data or a source, with date) or **[verify]** (from general knowledge or older sources; confirm on cse.lk / sec.gov.lk before relying on it in code).

## Exchange and regulators
- **Colombo Stock Exchange (CSE)**, cse.lk, is the only stock exchange in Sri Lanka.
- Regulator: **Securities and Exchange Commission of Sri Lanka (SEC)**, sec.gov.lk.
- **CDS** (Central Depository Systems) holds shares electronically. `dailyMarketSummery` reports CDS holdings split into domestic and foreign.
- Currency: **LKR** (Sri Lankan rupee), written `Rs.` or `LKR`.

## Trading session [verified 2026-10-13]
- Monday to Friday, continuous trading **09:30–14:30 Asia/Colombo** (UTC+05:30, no DST). Live intraday data on 2026-10-13 starts at 04:00 UTC (= 09:30).
- Market closes on public holidays, including the monthly **Poya (full moon) days**. Poya dates move each year, so don't hard-code them. Options: (a) trust `marketStatus`; (b) treat days with no `dailyMarketSummery` / daily bar as holidays; (c) keep a yearly holiday list from cse.lk that is updated with each release.
- Use the `marketStatus` endpoint as the source of truth for "is the market open now". Use the clock only for cache TTLs.

## Settlement [verify]
- Older sources say T+3. CSE has announced moves to a shorter cycle; confirm the current cycle on cse.lk before showing settlement dates.

## Indices
- **ASPI** (All Share Price Index) [verified]: covers all listed shares. `aspiData`; `sectorId` 1; history via `chartData chartId=1`.
- **S&P SL20** [verified]: 20 large, liquid companies, run with S&P Dow Jones Indices. `snpData`; `sectorId` 40.
- **Sector indices** [verified 2026-10-13]: `allSectors` returns "S&P/CSE <Industry Group> Index" entries, which are **GICS industry groups** (e.g. Energy, Banks, Capital Goods, Food Beverage & Tobacco, Diversified Financials…). Use these names as the sector taxonomy.
- `triasi` / `spt` in `dailyMarketSummery` are probably total-return versions of ASPI / SL20 [verify].

## Listing boards [verified via news sources]
- **Main Board**: larger companies.
- **Diri Savi Board**: medium, small and start-up companies.
- **Empower Board**: SMEs (stated capital about Rs. 25–100 Mn at listing). Companies can move up between boards.
- A company's board is in `companyProfile → reqComSumInfo[].boardType`.

## Security codes [verified]
- Format `TICKER.SERIES`, e.g. `JKH.N0000`.
- `N` = voting ordinary shares, `X` = non-voting ordinary shares. Both can exist for one company and trade at different prices. Other letters cover rights, warrants and debentures [verify the exact letters before parsing them].
- The "symbol" users type is the ticker (`JKH`). Map it to `.N0000` by default.

## Market mechanics
- **Crossings**: negotiated large trades reported separately (`crossingVolume`, `crossingTradeVol` in `tradeSummary`). Analysis may want to exclude them from "normal" volume.
- **Circuit breakers** [verify, introduced 2020]: market-wide halts at an S&P SL20 drop of 5% (30-minute halt), a further 2.5% (30 minutes), and a further 2.5% (closed for the day).
- **Price bands / tick sizes** for individual stocks [verify]: check the CSE trading rules before building any order-related feature (hex does not place orders).
- **Foreign participation** is tracked daily (`equityForeignPurchase/Sales`). Net foreign flow is a commonly quoted market indicator in Sri Lanka.

## Corporate actions
`approvedAnnouncement` carries XD (ex-dividend), XR (ex-rights), XC, record date, payment date and AGM date. Use XD dates for dividend yield calculations and to explain price drops in the history.

## Financial reporting
- Most companies' financial year ends **31 March**. Some use 31 December (banks, some others). Read `finYearEnd` from `companyProfile`.
- Quarterly interim reports and annual reports are PDFs (see `cse-api` → `financials`). There is no structured financial-statement API, so fundamentals need parsing or manual input (see `fundamental-analysis`).
- Market-wide valuation benchmarks (PER, PBV, DY) come daily from `dailyMarketSummery`.
