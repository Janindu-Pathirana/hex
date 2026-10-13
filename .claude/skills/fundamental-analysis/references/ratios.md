# Fundamental ratios: formulas and edge cases

All money values are in LKR (rupees). "Shares" = ordinary shares in issue (voting + non-voting when computing company-level values).

| Ratio | Formula | Edge cases |
|---|---|---|
| Market cap | Σ price_i × shares_i over series | Cross-check with `marketCap` from `companyInfoSummery` |
| EPS | net profit attributable to parent ÷ weighted avg shares | Use TTM for interim; label the period |
| P/E | price ÷ EPS | EPS ≤ 0 → nil ("n/m") |
| Earnings yield | EPS ÷ price | Can be negative; fine to show |
| NAV / BVPS | equity attributable to parent ÷ shares | Equity ≤ 0 → P/B nil |
| P/B (PBV) | price ÷ NAV per share | |
| Dividend yield | DPS (last 12 months, by XD date) ÷ price | No dividends → 0%, not nil |
| Payout ratio | DPS ÷ EPS | EPS ≤ 0 → nil |
| ROE | net profit attributable ÷ avg equity attributable | Use (open + close) / 2 if both are available |
| ROA | net profit ÷ avg total assets | |
| Net margin | net profit ÷ revenue | Revenue 0 → nil |
| Gross / operating margin | gross profit or operating profit ÷ revenue | Banks have no "revenue" in the usual sense; skip for the Banks GICS group |
| Debt / equity | total interest-bearing debt ÷ total equity | Banks/finance: skip, not meaningful |
| Current ratio | current assets ÷ current liabilities | Banks: skip |
| Growth (YoY) | (X_t − X_{t−4q}) ÷ |X_{t−4q}| | Base ≤ 0 → nil or "turnaround" label |
| Growth (QoQ) | (X_t − X_{t−1q}) ÷ |X_{t−1q}| | Seasonal; prefer YoY |
| PEG | P/E ÷ EPS growth (in %) | Growth ≤ 0 → nil |
| Graham number | √(22.5 × EPS × BVPS) | Either ≤ 0 → nil |
| Beta | from `reqSymbolBetaInfo` (`triASIBetaValue` vs ASPI, `betaValueSPSL` vs SL20) | Show the period/quarter |

## Test values (hand-computed)
- price 100, EPS 12.5 → P/E 8.0, earnings yield 12.5%
- price 50, NAV 40 → P/B 1.25
- price 80, DPS 4 → DY 5.0%; EPS 10 → payout 40%
- profit 1,200, equity open 9,000 / close 11,000 → ROE 12.0%
- EPS −2 → P/E nil
- EPS growth: 10 → 12 = +20%; −5 → 3 = nil (turnaround)

## Sector-specific notes
- **Banks / Diversified Financials / Insurance**: use P/B, ROE, net interest margin and capital adequacy (from reports) rather than D/E or margins.
- **Holding companies / conglomerates** (common on the CSE, e.g. JKH): consolidated figures; minority interest matters, so always use the "attributable to parent" line.
- **Plantations / commodities**: highly cyclical. Use multi-year averages for P/E context.
