---
name: pdf-extraction
description: How hex reads CSE financial-report PDFs into fundamentals: pdfx text backends and safety caps, statement page and column detection in internal/extract, the label dictionary and units, accounting-identity confidence, golden fixtures from real reports, and the opt-in Claude fallback with its verification rules. Use when extraction misses or misreads a value, when adding a field or sector, when a new report layout appears, or when touching internal/pdfx, internal/extract or internal/llm.
---

# Reading report PDFs

## Pipeline
1. `reportjobs.PDFs`: `cse.Client.DownloadPDF` (size cap, `%PDF` check) → `pdfx.Blobs` (by SHA-256) → `pdfx.Extract` (backend `Pure`, or `Poppler` when `pdftotext` exists; 50 MB / 1000 pages / 60 s, `recover`) → page JSON in `report_pages`. Under 40 chars/page = scanned.
2. `reportjobs.Fundamentals` → `extract.Extract(pages, Meta{PeriodEnd, Annual, Shares, FYEnd})` → `fundamentals` (source `pdf`) + an `extraction_runs` row. Below `LowConfidence` (0.7) the run is flagged, and the `Fallback` (Claude) is tried when `--llm` is set.

## How extract reads a statement
- **Sections** start at title lines anchored at line start (`titleLead` + "income statement", "statement of profit or loss", "statement of financial position", "statement of cash flows"; titles may wrap onto two lines). Comprehensive-income-only and changes-in-equity statements are skipped; sentences that mention a statement are rejected by `titleNoise`.
- **Header** = lines before the first amount. Dated columns come from full dates (`30.06.2026`, `30/06/2026`, `30-Jun-26`, `30th June 2026`) or years (`2026`, `2026/27`). `%`, change, variance and note columns are ignored.
- **Blocks**: columns split into current/prior runs; scope words (Group/Company/Bank) and durations (quarter/03 months → 3M, six months → 6M, nine → 9M, year → 12M, "for the period" → year-to-date from `FYEnd`) map to blocks in order when there is one caption per block, else to the nearest caption.
- **Current column** = latest date per block, kept if not after the report date; for the same scope+duration the most recent wins. Period end = the column's date, or the report's when the header gives only years.
- **Rows**: figures are assigned to the nearest column (centre or right edge). Lines without figures become captions: a label that matches nothing is retried as caption + label, and an unlabeled total as "total " + caption.
- **Fields** (`labels.go`): anchored regexes per statement; the pattern index is a priority ("Total revenue" beats "Turnover"). Expense fields are stored as positive magnitudes. Per-share fields are not unit-scaled.
- **Units**: Rs.'000 / thousands → ×1e3, millions → ×1e6, from the header, then the section, then any statement page; assumed ×1e3 with an issue otherwise.
- **Confidence**: per scope+period, 0.5 + 0.15 per identity that holds − 0.25 per failure (fields in a failed check −0.2 more). Identities: GP = revenue − COS; PAT = PBT − tax (− deferred tax); NII = interest income − expense; assets = equity + liabilities (or = equity and liabilities); parent equity ≤ total; EPS ≈ parent profit / shares; NAV ≈ parent equity / shares.

## Fixing a misread
1. Get the report into a scratch database: `HEX_DATA_DIR=<tmp> hex sync fundamentals --symbols SYM`, then `hex reports extract SYM --show-pages`.
2. Look at the words and x positions of the page (decode `report_pages.text_gz`) to see whether the problem is the title, header, column assignment or the label.
3. Change the narrowest thing (a pattern, a regex) and add a unit test in `parse_test.go`.
4. Run `go test ./internal/extract`: `TestGoldenReports` must still pass for every fixture.

## Golden fixtures
- `testdata/extract/<CODE>_<reportID>.json` = statement pages + `Meta`; `.want.json` = expected values keyed `scope|ptype|date|field`.
- Generate from a scratch DB: `GEN_GOLDEN=1 SPIKE_DB=<db> go test -run TestGenGolden ./internal/extract` (add the report id to the list). **Check the key fields against the PDF by hand before committing**: the generator writes what the extractor currently reads.
- Cover a new layout or sector with a fixture whenever you fix one.

## Claude fallback (`internal/llm`)
- Opt-in (`--llm`), `claude-opus-5-5` by default, `--llm-max` reports per run. Load the `claude-api` skill before changing SDK calls or the model.
- Sends only candidate statement pages (text) or the cached PDF (scanned, ≤30 MB). Output through the strict `record_statements` tool with `tool_choice: auto` (forced tool use is rejected by current models); refusal fallback enabled.
- `Verify` keeps a figure only if the field is known, the raw text parses, the date is valid and not after the report date, and (text reports) the raw figure is printed on the cited page. Then `extract.Finish` scores it. Scanned results are capped at 0.6.
- Never log or store the API key. Tests use a fake server via `option.WithBaseURL`.
