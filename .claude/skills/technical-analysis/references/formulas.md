# Indicator formulas and test vectors

Notation: `C_t` close, `H_t` high, `L_t` low, `n` period. Series are oldest → newest.

## Returns
- Simple: `r_t = C_t / C_{t-1} − 1`
- Log: `ℓ_t = ln(C_t / C_{t-1})`
- Period return: `C_end / C_start − 1`, where `C_start` is the last close on or before the start date.

## SMA
`SMA_t = (C_{t-n+1} + … + C_t) / n`. The first valid index is `n−1`.

## EMA
`α = 2/(n+1)`. Seed `EMA_{n-1} = SMA_{n-1}`. Then `EMA_t = α·C_t + (1−α)·EMA_{t-1}`.

## RSI (Wilder)
1. `Δ_t = C_t − C_{t-1}`, `gain = max(Δ,0)`, `loss = max(−Δ,0)`.
2. Seed with `avgGain`, `avgLoss` = simple mean of the first n gains/losses (Δ_1..Δ_n). The first RSI is at index n.
3. Then `avgGain_t = (avgGain_{t-1}·(n−1) + gain_t)/n` (same for loss).
4. `RS = avgGain/avgLoss`, `RSI = 100 − 100/(1+RS)`. If `avgLoss == 0`, RSI = 100. If both are 0, RSI = 50.

## MACD
`MACD = EMA12 − EMA26`, `Signal = EMA9(MACD)` (computed over the valid MACD values only), `Hist = MACD − Signal`.

## Bollinger Bands
`mid = SMA20`, `σ` = **population** stdev of the same 20 closes, `upper/lower = mid ± 2σ`, `%B = (C − lower)/(upper − lower)`.

## ATR (Wilder)
`TR_t = max(H_t − L_t, |H_t − C_{t-1}|, |L_t − C_{t-1}|)`. Seed `ATR` = mean of the first n TRs, then `ATR_t = (ATR_{t-1}·(n−1) + TR_t)/n`.

## Volatility
`σ_daily` = sample stdev (n−1) of log returns over the window. Annualised: `σ_daily · √240`.

## Max drawdown
Track the running peak `P_t = max(C_0..C_t)`, `DD_t = C_t/P_t − 1`, `MDD = min DD_t`. Report the peak and trough dates.

## Beta (if computing locally)
`β = cov(r_stock, r_index) / var(r_index)` over aligned daily simple returns (inner join on date).

## Test vectors
Closes `[1,2,3,4,5,6,7,8,9,10]`:
- SMA(3) last = 9; SMA(3)[2] = 2; indices 0–1 NaN
- EMA(3): α = 0.5, seed EMA[2] = 2, EMA[3] = 3, …, EMA[9] = 9 (EMA_t = C_t − 1 from index 2 on)
- RSI(3) on a strictly rising series = 100
- Max drawdown = 0

Closes `[10, 8, 12, 6, 9]`:
- Peak 12 → trough 6: MDD = −50%

Constant closes `[5,5,5,5,5]`: stdev 0, Bollinger upper = lower = 5 (guard the division in %B), RSI = 50.

For EMA/RSI/MACD against real data, generate expected values once with a trusted tool (e.g. pandas-ta / TA-Lib in a scratch script), save them in `testdata/indicators/*.json`, and compare with a 1e-4 tolerance.
