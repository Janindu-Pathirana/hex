---
name: technical-analysis
description: Implement price/volume technical indicators for CSE stocks in hex, covering returns, SMA/EMA, RSI, MACD, Bollinger Bands, ATR, volatility, max drawdown, 52-week range, volume averages and relative strength vs ASPI, with warm-up handling and test vectors. Use when building "hex analyze", price history views, signals, or any indicator math in internal/analysis.
---

# Technical analysis

Exact formulas and reference test values are in [references/formulas.md](references/formulas.md).

## Data
- Daily bars: `companyChartDataByStock` with `period=5` gives about 1 year (~240 bars). The fields are `t`, `p` (close), `h`, `l`, `o` (often null), `q` (volume). See `cse-api`.
- Benchmark: ASPI from `chartData chartId=1 period=5`.
- For more than 1 year, accumulate history locally (see `cache-data`).
- Normalise into `type Bar struct { Date time.Time; Open *float64; High, Low, Close, Volume float64 }`, sorted ascending by date with duplicates removed.
- Open is often null. Indicators that need it are rare; ATR uses H/L/prev close, so it works without open.
- Many CSE stocks are **illiquid**: days with no trades may be missing or repeat the previous price. Report "trading days in window" and warn if a 14-day indicator rests on fewer than ~10 real trades.

## Implementation rules
- Put pure functions in `internal/analysis/technical.go`, with no I/O and no printing.
- Signature style: `func SMA(xs []float64, n int) []float64` returns a slice the **same length** as the input, with `math.NaN()` during warm-up. Callers take the last value. Alternatively, return `(value float64, ok bool)` helpers for the "latest" case.
- Validate `n > 0` and `len(xs) >= n`, and return NaN/ok=false rather than panicking.
- Use `float64` throughout. Compare in tests with a tolerance (`math.Abs(a-b) < 1e-9`, or 1e-4 for EMA-based values against published references).
- RSI and ATR use **Wilder smoothing** (α = 1/n). EMA uses α = 2/(n+1), seeded with the SMA of the first n values. State this in doc comments, since tools differ.
- Annualise volatility with √(trading days per year). Use **~240** for the CSE, which is what period=5 returns, rather than 252.

## Default indicator set for `hex analyze <SYMBOL>`
| Metric | Default |
|---|---|
| Last price, change, % change | `companyInfoSummery` |
| Returns | 1W, 1M, 3M, YTD, 1Y |
| SMA | 20, 50, 200 (200 only if enough bars); price vs SMA in % |
| EMA | 12, 26 |
| RSI | 14 (Wilder); zones <30 / >70 |
| MACD | 12/26/9: line, signal, histogram |
| Bollinger | 20, 2σ; %B |
| ATR | 14 (Wilder), also as % of price |
| Volatility | 20-day and 1Y annualised stdev of log returns |
| Max drawdown | over the window, with peak and trough dates |
| 52w range | `p12HiPrice`/`p12LowPrice`; distance from high/low |
| Volume | 20-day average vs today (`tdyShareVolume`) |
| Relative strength | stock 3M/1Y return − ASPI return; beta from `companyInfoSummery` |

## Signals and wording
Describe conditions ("RSI 74 — above 70 (overbought zone)", "price 6% above SMA50", "SMA50 crossed above SMA200 on 2026-09-02"). **Never** print buy/sell recommendations. Include the disclaimer from `terminal-output`.

## Testing
Use the test vectors in `references/formulas.md`, and add table-driven tests for the warm-up edges (`len == n-1`, `len == n`) and a constant series (RSI with zero losses = 100; stdev = 0). See `go-testing`.
