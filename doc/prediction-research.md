# Features for accurate prediction: research

Goal: list every signal that can feed a prediction for CSE-listed stocks, and say what `hex` already has. Status: **have** (computed today), **data** (stored, not used), **partial**, **missing**.

## 0. Reality check

- Markets are close to efficient. Papers combining technical, fundamental and text data report about 60–66% direction accuracy at best (for example 66% index / 62% single-stock on S&P 500 weekly data), and results are not comparable across datasets and metrics.
- The CSE is small (about 280 names), thinly traded and has frequent regime breaks (2022 crisis, IMF programme). Small data punishes complex models.
- So aim for **calibrated probabilities and ranked scores with honest validation**, not price targets. Keep the "informational, not investment advice" disclaimer.

## 1. Price and volume technicals

| Feature | Status |
|---|---|
| SMA / EMA (20, 50, 200), price vs MA, MA cross | missing (data: `price_bars`) |
| RSI, stochastic, Williams %R | missing |
| MACD (line, signal, histogram) | missing |
| Bollinger bands, %B, bandwidth | missing |
| ATR, realised volatility (10/20/60d), downside deviation | missing |
| Momentum: 1/3/6/12-month return, 12-1 skip-month | missing |
| 52-week high/low distance, new-high count | data (`quote_stats`) |
| Max drawdown, time since drawdown | missing |
| OBV, VWAP, volume vs average, volume spikes | missing |
| Gaps, candlestick patterns, support/resistance | missing (low value, add last) |
| Beta vs ASPI and S&P SL20, rolling correlation | data (`quote_stats` beta) |
| Mean reversion (z-score of price vs MA, short-term reversal) | missing |

## 2. Liquidity and microstructure (matters most on the CSE)

| Feature | Status |
|---|---|
| Daily turnover, share volume, trade count | data (`quotes`, `price_bars`) |
| Traded-days ratio, zero-return days | missing |
| Amihud illiquidity (|return| / turnover) | missing |
| Price impact / tradability filter (minimum turnover) | missing |
| Foreign % holding, foreign net buying (market level) | data (`quote_stats`, `market_daily`) |
| Domestic vs foreign buy/sell flow | data (`market_daily`) |
| Board (Main / Diri Savi / watch list), trading halts | missing |
| Crossing / large-block trades | missing |
| Public float, top-holder concentration | missing (annual report) |

Illiquid names produce stale prices and fake signals. A liquidity gate should come before any model.

## 3. Fundamental features

| Feature | Status |
|---|---|
| ROE, ROA, margins, EPS, BVPS, P/E, P/B, earnings yield, PEG, Graham, dividend yield, payout, current ratio, interest cover, cash conversion, FCF, YoY growth | have |
| Margin trend (YoY change in bps), ROE trend | partial (margin finding only) |
| ROIC, asset turnover, EV/EBITDA, EV/Sales, FCF yield | missing |
| Debt/equity in `analyze` (no debt input fed) | partial |
| Piotroski F-score, Altman Z, Beneish M-score, accruals ratio | missing |
| Earnings surprise (vs prior-year same quarter; seasonal random walk), SUE | missing |
| Revenue / EPS acceleration (second derivative), growth consistency | missing |
| Quality factors: stability of ROE, low leverage, earnings volatility | missing |
| Sector-relative valuation and growth (z-score vs sector median) | missing ("no sector benchmark yet") |
| Banks: NIM, NPL ratio, provision coverage, capital adequacy (CET1, total), LDR, cost/income, CASA, ROE | missing (some inputs in field list) |
| Insurers: combined ratio, solvency (CAR), GWP growth, investment yield | missing |
| Conglomerate / segment mix | missing |
| Data confidence as a model weight | have (per metric) |

Point-in-time rule: a figure becomes usable on the **report publish date** (`reports.uploaded_at`), not the period end.

## 4. Corporate actions and events

| Feature | Status |
|---|---|
| Announcements classified (dividend, rights, split, bonus, AGM) | have |
| Ex-dividend, record, payment dates | missing (dates never filled; endpoint unverified) |
| Dividend history, growth, consistency, special dividends | partial (DPS from statements) |
| Price adjustment for splits, bonus, rights, scrip | missing (history is unadjusted: breaks returns and indicators) |
| Results-date calendar and pre-earnings drift | missing |
| Rights issues / private placements (dilution) | partial (announcements) |
| Share buybacks | missing |
| Director dealings, substantial shareholder changes | missing |
| Mergers, takeovers, mandatory offers, delistings | missing |
| Audit qualification, going concern, related-party notes | missing (LLM extraction) |

Adjusted prices are a prerequisite for any return-based feature.

## 5. Market, sector and factor context

| Feature | Status |
|---|---|
| ASPI and S&P SL20 daily values | data (`index_bars`) |
| Sector indices | data (`index_bars`) |
| Market PER / PBV / dividend yield | data (`market_daily`) |
| Relative strength vs ASPI and vs sector | missing |
| Market breadth (advancers / decliners, % above 50d MA) | missing (listed / traded counts exist) |
| Market turnover regime, volatility regime | data (`market_daily`) |
| Sector rotation / sector momentum | missing |
| Size, value, momentum, quality factor exposures | missing |
| Correlation to ASPI, idiosyncratic volatility | missing |

## 6. Macro (CBSL and others)

Local studies (monthly data, 1995–2019 samples, university journals) agree that **interest rates weigh on the ASPI** and **GDP growth is the most consistent positive driver**. Inflation and exchange-rate effects flip sign between studies, so use them as regime or interaction features, never fixed-sign rules.

| Feature | Source | Status |
|---|---|---|
| Policy rates (SDFR, SLFR), AWPLR, AWDR | CBSL | missing |
| T-bill and bond yields, yield curve slope | CBSL / CSE | missing |
| Inflation (CCPI, NCPI), core inflation | CBSL / DCS | missing |
| USD/LKR level and change | CBSL | missing |
| Gross official reserves | CBSL | missing |
| Money supply, private credit growth | CBSL | missing |
| GDP growth, fiscal deficit, debt metrics | CBSL / Treasury | missing |
| IMF review milestones, budget dates, elections | manual calendar | missing |
| Tourist arrivals, remittances, exports | SLTDA / CBSL | missing |
| Oil, tea, rubber prices (sector-specific) | external | missing |
| Real interest rate (policy minus inflation) | derived | missing |

Macro series are slow (monthly): use them for regime and sector tilt, not for day-level signals.

## 7. Sentiment and text (add last; noisy)

- CSE announcements: classification by type and tone (profit warning, dividend, rights, litigation).
- News (Daily FT, EconomyNext, Newswire): headline sentiment per symbol and per sector.
- LLM extraction from reports: management outlook, risk notes, going concern, auditor emphasis. Reuse the verify-against-source pattern from `internal/llm`.
- Reviews report that sentiment adds accuracy mainly alongside technicals, but depends on noisy data and raises overfitting risk.

## 8. Calendar and regime

Day-of-week and month-end effects, dividend season, results season, budget and election windows, bull/bear state (price vs 200d MA or HMM on returns), volatility regime (realised vol percentile), market open/close effects. Cheap to compute; test for real effect before keeping.

## 9. What makes a prediction accurate: target and validation

| Item | Why |
|---|---|
| Define the target before features | forward 5/20/60-day return, excess over ASPI, or triple-barrier (profit / stop / time) label |
| Point-in-time features | no look-ahead: publish dates, adjusted prices, data known at close |
| Survivorship-bias-free universe | keep delisted and suspended companies |
| Walk-forward splits with purging and embargo | overlapping label windows leak across train/test splits |
| Never use labels as features | triple-barrier labels use future prices by design |
| Costs and liquidity constraints | brokerage and CSE levies, spread, minimum turnover, price limits |
| Baselines | naive (no change), buy-and-hold ASPI, simple momentum, value rank |
| Calibration | Brier score, reliability curve, so "60%" means 60% |
| Rank metrics | information coefficient (rank IC), top-minus-bottom quantile spread |
| Multiple-testing control | many features and models inflate backtests; use deflated Sharpe, hold-out era, few hypotheses |
| Explainability | SHAP / feature importance so output stays auditable |
| Drift monitoring | retrain triggers when feature distributions or hit rate shift |
| Report confidence and data coverage with every prediction | reuse the existing coverage and confidence machinery |

## 10. Models, in order

1. **Transparent score**: z-scored factors (value, quality, growth, momentum, low risk) with a liquidity gate. No training; easy to explain.
2. **Regularised linear / logistic model** on the same features.
3. **Gradient boosting** (LightGBM / XGBoost) on tabular features with monotonic constraints where sensible.
4. **Regime-aware ensemble**; calibrate probabilities (isotonic / Platt).
5. LSTM / transformers only if the data volume proves sufficient. Likely not with ~280 names and short history.

## 11. Data hex lacks for all of this

1. Longer daily history (the chart endpoint returns one year per sync; keep accumulating from day one).
2. Corporate-action dates and adjustment factors.
3. Historical fundamentals as first published (point-in-time) and report publish dates.
4. CBSL macro series.
5. Delisted-company records.
6. News / announcement text.

## Sources

- [Stock price prediction using technical data and sentiment score](https://discover.unhas.ac.id/publications/stock-price-prediction-using-technical-data-and-sentiment-score)
- [S&P 500 stock price prediction using technical, fundamental and text data (arXiv 2108.10826)](https://ar5iv.labs.arxiv.org/html/2108.10826)
- [Systematic literature review: stock price prediction with machine learning](https://data.mendeley.com/datasets/w6nk65bgwp/1)
- [Impact of macroeconomic variables on stock market performance: evidence from Sri Lanka](https://journals.sjp.ac.lk/index.php/sljbe/article/view/3933)
- [Impact of macroeconomic variables on stock market performance in Sri Lanka (JFN)](https://rer.jfn.ac.lk/items/a8684153-6dec-4d30-8fd1-636cd030baf0)
- [Triple-barrier labeling and meta-labeling (Hudson & Thames)](https://hudsonthames.org/?p=3502)
- [fynance triple_barrier docs (leakage warning)](https://fynance.readthedocs.io/en/latest/generated/fynance.features.labels.triple_barrier.html)
- López de Prado, *Advances in Financial Machine Learning* (purged CV, embargo, backtest overfitting). Not fetched; cited from general knowledge.

The Sri Lankan macro findings come from abstracts and summaries only; the full papers were not read.
