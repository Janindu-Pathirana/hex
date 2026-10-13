---
name: fundamental-analysis
description: Implement fundamental/valuation analysis for CSE companies in hex, covering market cap, EPS, P/E, P/B, NAV per share, dividend yield, payout, ROE/ROA, margins, leverage, growth and beta, where each input comes from on cse.lk, and how to handle missing or negative values. Use when building "hex analyze" fundamentals, valuation comparisons against sector/market, or parsing financial report data.
---

# Fundamental analysis

Formulas, edge cases and test values are in [references/ratios.md](references/ratios.md).

## Where the inputs come from
| Input | Source |
|---|---|
| Price, previous close, market cap, shares issued (`quantityIssued`), beta, 52w hi/lo | `companyInfoSummery` [verified] |
| Sector, board, financial year end, auditors | `companyProfile → reqComSumInfo` |
| Market PER / PBV / DY benchmark | `dailyMarketSummery` (`per`, `pbv`, `dy`) |
| Sector benchmark | compute from peers in the same GICS group (see `screening-and-portfolio`) |
| Earnings, equity, revenue, dividends, total assets, debt | **Financial report PDFs** only (`financials` → `https://cdn.cse.lk/<path>`) |
| Dividend per share, XD dates | `approvedAnnouncement` (XD entries) + annual reports |

The CSE has no structured financials API. In order of preference:
1. **User-supplied fundamentals file**: `os.UserConfigDir()/hex/fundamentals/<SYMBOL>.yaml|json` with `period`, `revenue`, `net_profit_attributable`, `equity_attributable`, `total_assets`, `total_debt`, `dps`. Add a `hex fundamentals set` command to fill it in. This is reliable and simple, so build it first.
2. **PDF extraction** (later and optional): download the PDF, extract text (`ledongthuc/pdf` or shell out to `pdftotext` if present), and pull income-statement lines with regexes. Results are fragile because layouts vary by company. Always show the extracted values so the user can check them, and keep the manual override.

## Implementation
- Put pure functions in `internal/analysis/fundamental.go`. Inputs are plain structs, output is `Ratios` with `*float64` fields (nil = not computable).
- Annualise interim results carefully: prefer **TTM** (sum of the last 4 quarters) over ×4 extrapolation, and label which one was used.
- Use profit **attributable to equity holders of the parent**, not total profit, and the **weighted average** share count when available, otherwise `quantityIssued`.
- Watch units: Sri Lankan reports are usually in **Rs. '000** or Rs. Mn. Store everything in rupees and record the unit in the input file.
- Negative earnings: P/E is "n/m" (not meaningful). Don't print a negative P/E.
- Voting and non-voting shares: market cap = Σ (price × shares) across **both** series when computing company-level valuation.

## Presenting
- Show each ratio next to the **sector median** and **market** (`dailyMarketSummery`) values, e.g. `P/E 8.4 (sector 7.1, market 9.8)`.
- Show the period the numbers come from (`FY2025/26`, `TTM Jun-2026`).
- Add the disclaimer line (see `terminal-output`).
- For deeper narrative work (thesis, earnings review), the `equity-research:*` skills can help structure the write-up.
