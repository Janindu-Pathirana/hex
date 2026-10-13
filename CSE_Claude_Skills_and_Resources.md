# Claude Skills & Resources for Sri Lankan Stock Market (CSE) Analysis

*Compiled: 13 October 2026*

> **Summary:** There is no official Claude skill built specifically for the Colombo Stock Exchange (CSE). The best setup is a combination of **(1)** general equity-analysis skills, **(2)** a way to get CSE data into Claude, and **(3)** official Sri Lankan sources for fundamentals and macro context.

> ⚠️ All CSE data tools listed below are **unofficial** and depend on the public data on cse.lk. Their endpoints can change without notice, so always verify important figures on [cse.lk](https://www.cse.lk). Nothing here is financial advice.

---

## 1. Claude Skills & Plugins for Analysis

### 1.1 Anthropic Financial Services Plugins (official, free, open source)

- **Repo:** https://github.com/anthropics/financial-services-plugins
- **What it is:** A collection of reference agents, skills and data connectors for investment banking, equity research, private equity and wealth management. Works in both Claude (Cowork) and Claude Code.
- **Useful plugins:**
  | Plugin | What it does |
  |---|---|
  | `financial-analysis` | Core plugin. Shared tools and data connectors. **Install this first.** |
  | `equity-research` | Earnings analysis, initiating coverage, thesis tracking, sector overviews, stock screens, catalyst calendar, morning notes |
  | `investment-banking` | Company one-pagers, data packs, pitch decks, financial analysis |
  | `wealth-management` | Portfolio and client workflows |
- **Install in Claude Code:**
  ```bash
  claude plugin marketplace add anthropics/financial-services-plugins
  claude plugin install financial-analysis@financial-services-plugins
  claude plugin install equity-research@financial-services-plugins
  ```
- **Install in the Claude app:** Customize → Browse plugins → **+** → *Add marketplace from GitHub* → paste the repo URL above.
- **Note:** The built-in data connectors mostly cover US/global data. For CSE stocks you feed it data from Section 2.

### 1.2 Plugins in your Claude plugin catalog

- **equity-research**: earnings analysis, initiating coverage reports, research workflows
- **Data**: SQL, dataset exploration, visualizations and dashboards
- **investment-banking**: market insights, decks, financial analysis

### 1.3 tradermonty / claude-trading-skills (community, largest collection)

- **Repo:** https://github.com/tradermonty/claude-trading-skills
- **Docs:** https://tradermonty.github.io/claude-trading-skills/
- **What it is:** 70+ Claude skills for equity investors and traders: market analysis, technical charting, screeners, backtesting, portfolio management, economic calendars and strategy development.
- **Skills that carry over well to CSE** (they don't depend on US-only data):
  - `technical-analyst`: chart and technical analysis
  - `backtest-expert`: systematic strategy backtesting
  - `position-sizer`: risk-based position sizing
  - `portfolio-manager`: portfolio review and rebalancing
  - `value-dividend-screener` / `dividend-growth-pullback-screener`: the screening logic is reusable with CSE data
  - `macro-regime-detector`, `scenario-analyzer`, `data-quality-checker`
  - `trading-skills-navigator`: recommends which skill to use for your goal
- **Install:**
  - *Claude web app:* Settings → Skills → upload the ZIP from the repo's `skill-packages/` folder.
  - *Claude Code:* copy the skill folder into your skills directory, or run
    `npx skills add tradermonty/claude-trading-skills --skill <skill-name> --agent claude-code`
- **Note:** Many skills are tuned for US stocks, and some need paid API keys (FMP, FINVIZ Elite, Alpaca).

### 1.4 Other community skill collections

- https://github.com/himself65/finance-skills
- https://github.com/quant-sentiment-ai/claude-equity-research

---

## 2. Getting CSE Data into Claude

### 2.1 CSE MCP servers (connect Claude Desktop / Claude Code to live CSE data)

| Server | Features | Install |
|---|---|---|
| **Shaveen12/cse-mcp** ([GitHub](https://github.com/Shaveen12/cse-mcp), [npm](https://www.npmjs.com/package/cse-mcp)) | Company search, real-time price, change and % change | `claude mcp add cse-mcp -- npx cse-mcp` |
| **Gajarthan/cse-mcp** ([Glama](https://glama.ai/mcp/servers/Gajarthan/cse-mcp)) | Normalized stock quotes (TypeScript, local stdio) | `claude mcp add-json cse '{"type":"stdio","command":"npx","args":["-y","@gajarthan/cse-mcp"]}'` |
| **CSE MCP Intelligence** ([Glama](https://glama.ai/mcp/servers/ha9g2drjmr)) | Market scanning/filters, company profiles, financial reports, compliance info, penny-stock screening | Clone the repo and add it to `claude_desktop_config.json` |

**Claude Desktop config example** (`~/Library/Application Support/Claude/claude_desktop_config.json` on Mac):
```json
{
  "mcpServers": {
    "cse-mcp": { "command": "npx", "args": ["cse-mcp"] }
  }
}
```

### 2.2 Python client: `cse.lk`

- **PyPI:** https://pypi.org/project/cse-lk/ · **GitHub:** https://github.com/bawantha/cse.lk
- Supports all 22 CSE API endpoints, with typed data models. Covers company info, market summary, ASPI, S&P SL20, all sectors, top gainers/losers, and historical data.
  ```bash
  pip install cse.lk
  ```
  ```python
  from cse_lk import CSEClient
  client = CSEClient()
  company = client.get_company_info("LOLC.N0000")
  print(f"{company.name}: LKR {company.last_traded_price}")
  ```
- Best fit for analysis Claude runs in code: screening, charts, valuation, comparison against ASPI.

### 2.3 The underlying CSE JSON API (undocumented but stable)

The cse.lk website runs on a JSON API at `https://www.cse.lk/api/*`. Most endpoints are `POST`. Useful ones:

| Endpoint | Returns |
|---|---|
| `/api/companyInfoSummery` | Company quote and fundamentals: price, 52-week range, market cap, beta, foreign holding |
| `/api/marketSummery` | Daily turnover, volume, trades |
| `/api/aspiData` / `/api/snpData` | ASPI / S&P SL20 index values |
| `/api/aspi` / `/api/spsl` | Index constituents |
| `/api/allSectors` | 20 GICS sector indices |
| `/api/topGainers`, `/api/topLooses`, `/api/mostActiveTrades` | Daily movers |
| `/api/getContributors` | Market breadth (advancers vs decliners) |
| `/api/chartData` | Intraday time series |
| `/api/marketStatus` | Market open/closed |
| `/api/allSecurityCode` | Master list of listed securities |

Announcements, financial reports and CSE circulars are also exposed, with PDF links on `cdn.cse.lk`.
Reference: https://cloud.browserless.io/skills/cse.lk/scrape-all-data

### 2.4 Other / paid data options

- **Apify – Sri Lanka CSE Market Data** (https://apify.com/nexgendata/sri-lanka-cse-market-data): prices, gainers/losers and announcements. Pay per result, about $0.10 per record.
- **Oanor CSE API** (`api.oanor.com/cse-api`): keyless API with market summary, ASPI, quotes with fundamentals and beta, and top movers.
- **CSE-TRADE** (https://zff.dev/charithmadhuranga/CSE-TRADE): open-source Python desktop app with a screener, comparisons, market breadth and AI analysis.
- **Enterprise (expensive):** LSEG and FactSet are available as Claude connectors and include CSE coverage, but they need institutional subscriptions.
- *Not useful for CSE:* Alpha Vantage, Twelve Data, Financial Datasets and similar connectors focus on US/major markets, so check coverage before relying on them.

---

## 3. Official Sri Lankan Sources (Fundamentals & Macro)

| Source | What you get | Link |
|---|---|---|
| **Colombo Stock Exchange** | Annual and quarterly reports, corporate announcements, daily market reports, company profiles | https://www.cse.lk |
| **CBSL Data Library** | Downloadable time series for the real, monetary, fiscal, external and financial sectors (interest rates, inflation, LKR, money supply) | https://www.cbsl.lk/eresearch |
| **CBSL – Economic & Social Statistics of Sri Lanka 2026** | Long data series, including a "Money, Interest Rates and Capital Markets" section | https://www.cbsl.gov.lk/statistics/economic-and-social-statistics |
| **CBSL – Sri Lanka Socio-Economic Data 2026** | Compact booklet of key indicators | https://www.cbsl.gov.lk |
| **SEC Sri Lanka** | Regulations, unit trust data, market statistics | https://www.sec.gov.lk |
| **Local broker research** | Sector and company reports; often free with an account (e.g. CT CLSA, First Capital, Asia Securities) | Broker websites |

---

## 4. Recommended Setup

1. **Install the skills:** Anthropic's `financial-analysis` + `equity-research` plugins, plus a few tradermonty skills (technical-analyst, backtest-expert, position-sizer).
2. **Connect data:**
   - Quick live prices inside Claude Desktop: add **Shaveen12/cse-mcp**.
   - Deeper analysis: use the **`cse.lk` Python client** so Claude can pull data and compute screens, charts and valuations in code.
3. **Add macro context:** download interest-rate, inflation and FX series from the **CBSL Data Library**.
4. **Optional:** build a custom "CSE analyst" Claude skill that bundles CSE data fetching, your screening rules and your report format into one reusable workflow.

### Example prompts once set up
- "Screen CSE stocks with P/E below 8, dividend yield above 5% and market cap above LKR 10bn."
- "Compare JKH, LOLC and HAYL over the last year against ASPI."
- "Write an initiating coverage report on Commercial Bank (COMB.N0000) using its latest annual report."
- "How have CBSL policy rate changes affected the ASPI banking sector?"

---

## Sources

- [anthropics/financial-services-plugins](https://github.com/anthropics/financial-services-plugins)
- [Install financial services plugins (Claude docs)](https://claude.com/resources/tutorials/install-financial-services-plugins-for-cowork)
- [tradermonty/claude-trading-skills](https://github.com/tradermonty/claude-trading-skills)
- [Claude trading skills list](https://www.supercode.sh/en/skills/tradermonty/claude-trading-skills)
- [CSE MCP Server – Shaveen12](https://glama.ai/mcp/servers/Shaveen12/cse-mcp)
- [CSE MCP Server – Gajarthan](https://glama.ai/mcp/servers/Gajarthan/cse-mcp)
- [CSE MCP Intelligence](https://glama.ai/mcp/servers/ha9g2drjmr)
- [cse.lk Python client (PyPI)](https://pypi.org/project/cse-lk/)
- [CSE data API overview (browserless)](https://cloud.browserless.io/skills/cse.lk/scrape-all-data)
- [Apify Sri Lanka CSE data](https://apify.com/nexgendata/sri-lanka-cse-market-data)
- [Oanor CSE API](https://oanor.com/tag/aspi)
- [CSE-TRADE desktop app](https://zff.dev/charithmadhuranga/CSE-TRADE)
- [CBSL Data Library launch](https://www.cbsl.gov.lk/en/node?page=179)
- [CBSL Economic and Social Statistics 2026](https://www.cbsl.gov.lk/en/node/20814)
- [CBSL Socio-Economic Data 2026](https://www.cbsl.gov.lk/en/node/20928)
