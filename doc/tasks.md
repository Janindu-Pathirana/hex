# Tasks: toward accurate prediction

Derived from the gaps in [features.md](features.md) and the catalogue in [prediction-research.md](prediction-research.md). Order matters: data correctness first, then features, then validation, then models. Project skills to load are in brackets.

Legend: `[ ]` open, `[x]` done. Each task ends with "Done when".

## P0: Foundation and audit
- [x] **Retire stale `FEATURES.md`**: point it at `doc/features.md` or delete it. Done when: one source of truth.
- [x] **Audit stored data**: `hex data audit [SYMBOL...] [--json]` (`cmd/data/audit.go`): bars, range, longest gap, statement periods, low-confidence share, report pipeline, quote age, index coverage, with flags that name the fix.
- [x] **Start accumulating history now**: launchd and cron recipes in `doc/sync-schedule.md`. Still to do by hand: install one.
- [ ] **Fix `debt-equity` in `analyze`**: feed a debt input or drop the metric. [fundamental-analysis]

## Found by the first real-data run (2026-11-02, `doc/data-sources.md` section 3)
- [x] One-year windows failed when history began a few days late: windows now tolerate a start up to 7 days before the first bar.
- [x] **Index history job** (`hex sync index`): a year of daily ASPI (`chartId=1`) and S&P SL20 (`chartId=40`) levels; market comparisons now use it. `market_daily` still holds only the last 2 days, so market turnover and foreign flow fill in only as the daily sync keeps running.
- [x] Default interim reports raised from 4 to 6: each report yields only its own period, so year-on-year needs the report from a year earlier. Verified on JKH: revenue growth, margin changes and Piotroski (5 of 6) now compute.
- [ ] Bank statement extraction (SAMP: "no statement values found"); capex and debt labels (JKH).
- [ ] Round large numbers in the report (`69,385,571.4`).

## P1: Price correctness (prerequisite for every return feature)
- [x] **Corporate-action dates**: the actions job now uses `POST /api/smd` (`doc/data-sources.md`), 20 companies per request, 400 days by default or `--actions-since YYYY-MM-DD` (with `--force`) to backfill. Migration `0002` adds ex date, XR, XC, record, payment, trading, allotment and entitlement dates, proportion text, parsed ratio, price and dividend per share. Live backfill from 2018: 1,541 dividends (1,500 with an ex date), 189 scrip, 232 rights, 76 sub-divisions, 2 capitalisations. [cse-api, scrape-engine]
- [ ] **Adjustment factors**: split, bonus, rights, scrip. Store adjusted close beside raw. Confirmed the chart is raw (WAPO 102.75 to 23.60 across a 1-for-4 sub-division), so this is required. Ratios parse for 96 of 101 rights and 153 of 158 scrip texts, but **only 10 of 76 sub-divisions state a ratio** (the rest say "Refer to the Attachment" or only the title). So derive split factors from the price and volume jump at the announced ex date (round to a plausible ratio, check against the text when there is one), and keep a manual override. One event is several rows (announcement, dates, further announcement): merge them by ticker and `childAnnouncement`. [sqlite-store]
- [ ] **Holiday-aware market-open calendar** for TTLs and gaps. [cse-market-context]
- [ ] **Tests** with fixtures for each action type. [go-testing]

## P2: Technical indicators (`internal/analysis/technical.go`)
- [x] SMA / EMA (20, 50, 200 / 12, 26), price vs SMA 50 and 200, trend finding. [technical-analysis] SMA cross date not reported yet.
- [x] RSI 14, MACD 12/26/9, Bollinger %B, ATR 14 (stochastic still open).
- [x] Returns 1W/1M/3M/YTD/1Y; relative strength vs ASPI 3M/1Y. 12-1 momentum and mean-reversion z-score still open.
- [x] Realised volatility (20d, 1Y), max drawdown. Downside deviation still open.
- [x] Volume vs 20-day average. OBV and spike detection still open.
- [ ] Beta and correlation vs ASPI and S&P SL20 (rolling, from `price_bars` + `index_bars`). `quote_stats` still holds CSE's beta.
- [x] `hex analyze --section technical` (text and `--json`); minimum-history guards; golden file.
- [x] Done when: RSI, EMA and MACD matched an independent Python reference on a 120-bar series to 1e-4 (checked once, not kept as a fixture; add `testdata/indicators/` if you want it repeatable).
- Known limit: stored prices are not adjusted for splits, bonus or rights (P1), so returns and drawdown can jump on corporate actions.

## P3: Liquidity and market context
- [x] Traded-days ratio, zero-return days, Amihud illiquidity, average turnover (`internal/analysis/liquidity.go`, `--section liquidity`). Turnover is estimated as close x volume.
- [x] **Liquidity gate**: finding `liquidity` (watch below 50% traded days or LKR 250,000 average turnover). Not yet used to block scoring, since there is no score yet.
- [x] Relative strength vs ASPI (technical section) and vs S&P SL20 (market section). Sector index still open: needs a verified sector-name to index-id mapping.
- [x] Market context from `market_daily`: ASPI trend and volatility, turnover vs 20-day average, share of securities traded. Advancers/decliners breadth still open (the data has only listed and traded counts).
- [x] Foreign net flow over 20 days as % of market turnover (market-wide, not per stock).
- [ ] Sector and market benchmarks to replace absolute thresholds in `rules.go`.

## P4: Fundamental extensions
- [x] Margin and ROE trends (change in percentage points against the same period a year earlier) and growth acceleration (`internal/analysis/trends.go`). ROE change uses the period's profit on closing equity, not annualised.
- [ ] ROIC, EV/EBITDA. (Asset turnover on closing assets and FCF yield are done in `trends.go`.)
- [x] Piotroski F-score (`internal/analysis/piotroski.go`): eight of the nine tests; the ninth (no new shares) needs share-count history that is not stored.
- [ ] Altman Z, Beneish M-score, accruals ratio. Need fields the extractor does not read yet (retained earnings, receivables, SG&A, depreciation). [fundamental-analysis, pdf-extraction]
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

## P5a: Local LLMs (Ollama) and AI features
Design, rules and risks are in [llm-plan.md](llm-plan.md). Every model output is verified against its source before it is stored or printed.
- [ ] **A1 Split `internal/llm`**: move `Verify`, prompt text, field list and schema out of `claude.go` into shared files; add `Config` and `NewFallback`; `cmd/reports.go` and `cmd/sync.go` use it. No behaviour change. Done when: existing `claude_test.go` cases pass unchanged. [go-conventions, pdf-extraction]
- [ ] **A2 Ollama backend** (`internal/llm/ollama.go`): `/api/chat` with a JSON-schema `format`, temperature 0, explicit `num_ctx` and truncation check, own long-timeout client, reachability and missing-model errors, `ErrNeedsVision` for scanned PDFs. Done when: fake-server tests cover valid, invalid, truncated, down and timeout cases. [pdf-extraction, go-testing]
- [ ] **A3 Flags**: `--llm-provider claude|ollama|ollama,claude` on `hex sync` and `hex reports extract`; per-provider default model; `HEX_OLLAMA_URL`. Chain falls through to Claude below 0.7 confidence, on error, or when scanned. Done when: a chained run on a scanned fixture reaches the fake Anthropic and a confident local run does not. [add-command]
- [ ] **A4 Model eval** (`HEX_OLLAMA_EVAL=1`, not in normal tests): run `qwen3:14b`, `qwen2.5:7b`, `llama3.2:3b` over `testdata/extract/`; record recovered values and verification rejections in `llm-plan.md`. Done when: a default model is chosen from the numbers.
- [ ] **B1 `internal/narrate`**: report JSON in, `{summary, strengths, risks, watch}` out with a `ref` per claim; number check against the report with one retry. Done when: an invented number is rejected in tests. [go-testing]
- [ ] **B2 `hex analyze --narrate`** (`--narrate-provider`, `--narrate-model`, default Ollama): text block and `--json` `narrative` field, disclaimer kept, output unchanged without the flag. Done when: goldens for both modes. [terminal-output]
- [ ] **C1 Announcement parsing**: after the P1 detail-endpoint check, extract ex/record/payment dates, amount and ratio from announcement text, verify against the text, store with `source = llm`. Done when: dividend, bonus, rights and split fixtures give the expected rows. [cse-api, scrape-engine]
- [ ] **D** (later): audit opinion / going concern / outlook extraction (P5), announcement tone, embeddings for `hex search`.

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
- [ ] Optional `--narrate` (LLM explanation of already-computed numbers only). `hex analyze --narrate` is planned first as P5a B1/B2; extend it to `predict` here.
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
