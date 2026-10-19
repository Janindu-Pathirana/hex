# Tasks: toward accurate prediction

Derived from the gaps in [features.md](features.md) and the catalogue in [prediction-research.md](prediction-research.md). Order matters: data correctness first, then features, then validation, then models. Project skills to load are in brackets.

Legend: `[ ]` open, `[x]` done. Each task ends with "Done when".

## P0: Foundation and audit
- [x] **Retire stale `FEATURES.md`**: point it at `doc/features.md` or delete it. Done when: one source of truth.
- [ ] **Audit stored data**: per symbol, count `price_bars`, earliest date, gaps; count fundamentals periods and confidence. Done when: a data-availability matrix is in `doc/data-audit.md`.
- [ ] **Start accumulating history now**: schedule `hex sync history` daily (history grows only while syncing). Done when: a documented cron/launchd recipe exists.
- [ ] **Fix `debt-equity` in `analyze`**: feed a debt input or drop the metric. [fundamental-analysis]

## P1: Price correctness (prerequisite for every return feature)
- [ ] **Corporate-action dates**: verify `getFinancialAnnouncement` (unused client method), fill `xd`, `record_date`, `payment_date`. [cse-api, scrape-engine]
- [ ] **Adjustment factors**: split, bonus, rights, scrip. Store adjusted close beside raw. [sqlite-store]
- [ ] **Holiday-aware market-open calendar** for TTLs and gaps. [cse-market-context]
- [ ] **Tests** with fixtures for each action type. [go-testing]

## P2: Technical indicators (`internal/analysis/indicators.go`)
- [ ] SMA / EMA (20, 50, 200), price vs MA, cross state. [technical-analysis]
- [ ] RSI, stochastic, MACD, Bollinger, ATR.
- [ ] Momentum 1/3/6/12 and 12-1; mean-reversion z-score.
- [ ] Realised volatility, downside deviation, max drawdown.
- [ ] Volume features: volume vs average, OBV, spikes.
- [ ] Beta and correlation vs ASPI and S&P SL20 (rolling, from `price_bars` + `index_bars`).
- [ ] `hex analyze --section technical` (text and `--json`); minimum-history guard; golden files.
- [ ] Done when: indicators match a reference implementation on a fixture within tolerance.

## P3: Liquidity and market context
- [ ] Traded-days ratio, zero-return days, Amihud illiquidity, average turnover.
- [ ] **Liquidity gate**: mark symbols too illiquid to score; surface as a finding.
- [ ] Relative strength vs ASPI, S&P SL20 and sector index.
- [ ] Market breadth, market volatility and turnover regimes (from `market_daily`, `index_bars`).
- [ ] Foreign flow features from `market_daily`.
- [ ] Sector and market benchmarks to replace absolute thresholds in `rules.go`.

## P4: Fundamental extensions
- [ ] Margin and ROE trends (YoY bps change), growth acceleration.
- [ ] ROIC, asset turnover, EV/EBITDA, FCF yield.
- [ ] Piotroski F-score, Altman Z, Beneish M-score, accruals. [fundamental-analysis]
- [ ] Earnings surprise (SUE vs same quarter last year).
- [ ] Sector-relative z-scores.
- [ ] **Bank rules**: NIM, NPL, capital adequacy, LDR, cost/income, provision coverage (extend `extract.Fields`). [pdf-extraction]
- [ ] **Insurer rules**: combined ratio, solvency, GWP growth.
- [ ] Record **publish date** per fundamental value for point-in-time use.

## P5: Events and text
- [ ] Results-date calendar; pre/post-earnings drift features.
- [ ] Dividend calendar and history (consistency, growth, special).
- [ ] Rights / placement dilution flags; buybacks; takeovers; delistings.
- [ ] Director dealings and substantial-holder changes (profile and announcements).
- [ ] LLM extraction of audit opinion, going concern, outlook (verify against page text). [pdf-extraction]
- [ ] Announcement classifier (tone) and optional news ingest. Opt-in, last.

## P6: CBSL macro (new source)
- [ ] Add CBSL jobs: policy rates, AWPLR, T-bill / bond yields, CCPI, USD/LKR, reserves, money supply, credit growth. [scrape-engine, sqlite-store]
- [ ] Migration `0002` for `macro_series` (series_id, date, value, published_at).
- [ ] Derived: real rate, yield slope, FX change, inflation change.
- [ ] Manual event calendar: IMF reviews, budget, elections.
- [ ] Add `hex clean` entry and `CacheConfig` for each cache. [cache-data]

## P7: Feature store and labels
- [ ] `features` table: (code, as_of, name, value), built only from data published on or before `as_of`.
- [ ] `hex features build` command. [add-command]
- [ ] Labels: forward 5/20/60-day excess return vs ASPI; triple-barrier variant.
- [ ] Survivorship: keep delisted and suspended symbols in the universe.
- [ ] Look-ahead test: a unit test that fails if any feature reads data after `as_of`.

## P8: Backtest and validation harness
- [ ] Walk-forward splits with purging and embargo.
- [ ] Cost model (brokerage, levies, spread) and liquidity-capped position sizes.
- [ ] Baselines: no-change, buy-and-hold ASPI, momentum rank, value rank.
- [ ] Metrics: hit rate, rank IC, top-minus-bottom spread, Brier score, calibration curve, max drawdown.
- [ ] Multiple-testing guard: hold-out era, count of tried variants, deflated Sharpe.
- [ ] `hex backtest` command with `--json`. Done when: baselines reproduce and a leakage test passes.

## P9: Scoring and prediction
- [ ] **Transparent composite score**: value, quality, growth, momentum, low-risk z-scores plus liquidity gate. [screening-and-portfolio]
- [ ] Regularised logistic model, then gradient boosting (monotonic constraints). Decide Go-native vs exported model.
- [ ] Probability calibration (isotonic / Platt).
- [ ] `hex predict <SYMBOL>`: probability, expected range, confidence, data coverage, top drivers (SHAP-style), disclaimer.
- [ ] Optional `--narrate` (LLM explanation of already-computed numbers only).
- [ ] Only ship a model that beats the baselines out of sample.

## P10: Product and operations
- [ ] Screener, watchlist, portfolio P&L. [screening-and-portfolio]
- [ ] Global `--json` and table output (`internal/output`). [terminal-output]
- [ ] Drift monitoring and retrain trigger; `hex sync status` extended for model freshness.
- [ ] Move `company show-profile` / `show-financial` onto the store.
- [ ] Re-runnable normalisers from `raw_responses`.
- [ ] Release and Homebrew formula update. [release]

## Suggested first sprint
P0 (audit + schedule history) → P1 (adjusted prices) → P2 (indicators) → P3 (liquidity gate). These fix data trust and give the first usable score inputs without new data sources.
