# Plan: local LLMs (Ollama) and AI features

Status: planned, nothing built. Tasks are tracked in [tasks.md](tasks.md) (P5a). Output stays informational, not investment advice.

## Why

`internal/llm` only talks to Claude, which costs money per report and needs `ANTHROPIC_API_KEY`. A local Ollama server removes the cost and keeps report text on the machine. The design already makes a weaker model safe: `llm.Verify` rejects every figure that is not printed on the page the model cites, so a small model can only return fewer figures, never wrong ones. The same "check the model against the source" rule applies to every feature below.

Models on this machine (checked 2026-11-02): `qwen3:14b` (thinking), `qwen2.5:7b`, `llama3.2:3b`, `nomic-embed-text`, `mxbai-embed-large`. None reads images.

## Rules that apply to every feature

1. **Opt-in.** No LLM call without a flag. `hex analyze` stays store-only by default.
2. **Verify before store or print.** Extracted figures must appear in the source text; narrated numbers must appear in the report. Unverified output is dropped and reported as an issue.
3. **Deterministic settings.** Temperature 0, fixed prompt version, schema-constrained output.
4. **Record provenance.** Model name, prompt version and (for Ollama) model digest go with each run. Stored values keep `source = llm`.
5. **Tests never call a real model.** Fake Ollama via `httptest`; Anthropic stays behind `a.llmOpts`.
6. **No new cse.lk traffic** from these features; they read the store.

## Phase A: Ollama backend for report extraction

Goal: `hex sync fundamentals --llm --llm-provider ollama` and `hex reports extract` work with a local model.

### Design

- Split `internal/llm/claude.go`:
  - `verify.go`: `Verify`, `printedOn`, `recorded`, the system prompt, the field list and JSON schema (moved, unchanged).
  - `prompt.go`: builds the statement-pages text prompt (the text branch of `Claude.Extract`) so both backends send identical input.
  - `claude.go`: Anthropic call only.
  - `ollama.go`: `Ollama` implementing `reportjobs.Fallback`.
  - `llm.go`: `Config{Provider, Model, URL, Max}` and `NewFallback(Config)`, used by `cmd/reports.go` and `cmd/sync.go` in place of the direct `llm.New` calls.
- `Ollama.Extract`: `POST {URL}/api/chat`, `stream: false`, `format` = the `record_statements` schema, `options.temperature = 0`, `think: false` for thinking models, `options.num_ctx` set explicitly.
  - **Context size is the main trap.** Ollama's default context silently truncates long prompts. Set `num_ctx` from the prompt size (cap 32768 for qwen), and fail with a clear error when `prompt_eval_count` shows truncation.
  - **Own HTTP client** with a long timeout (minutes). `httpclient.Default` is 30s and meant for cse.lk, so it is not used here.
  - Reachability check up front: `GET /api/tags`, and if the model is missing say `ollama pull <model>`.
- Scanned PDFs: `Ollama` returns `ErrNeedsVision`. No installed model reads images; a vision model (`qwen2.5vl`) or OCR via poppler/tesseract is a later option.
- **Chain**: `--llm-provider ollama,claude` tries local first and asks Claude only when the local result scores below 0.7, fails, or the report is scanned. Cheapest useful default.
- Flags and env:
  - `--llm-provider claude|ollama|ollama,claude` (default `claude`, so nothing changes for existing use).
  - `--llm-model` default depends on provider (`claude-opus-5-5`, `qwen3:14b`).
  - `HEX_OLLAMA_URL` (default `http://localhost:11434`).
  - `hex sync` also gets `--llm-provider`.

### Tests

- Fake Ollama server: valid output, schema-violating output, truncated context, model missing, server down, timeout.
- Existing `claude_test.go` cases move to `verify_test.go`; behaviour must not change.
- Opt-in eval (`HEX_OLLAMA_EVAL=1 go test ./internal/llm -run Eval`): runs the models over `testdata/extract/` pages and prints recovered-values vs expected and the count of verification rejections per model. Not part of normal `go test`.

### Done when

- On the extract fixtures, a local model recovers a stated share of the expected values (set after the first eval run) with zero stored values that fail `Verify`.
- A chained run falls through to Claude on a scanned fixture (fake Anthropic) and does not call Claude when Ollama is confident.
- `go vet ./... && go test -race -cover ./...` passes with no network.

## Phase B: `hex analyze --narrate`

Goal: a short plain-language reading of the numbers already in the report. Already listed in the roadmap; Ollama is the default provider because the input holds no secrets and reruns are free.

### Design

- New package `internal/narrate`, separate from extraction. Input is the `analysis.Report` after `--section` filtering, serialised to compact JSON.
- Output schema: `{summary, strengths[], risks[], watch[]}`. Each item names the metric key or finding id it rests on (`ref`).
- System prompt: explain only, use only figures from the input, give no price targets or predictions, no buy/sell wording.
- **Number check**: pull every number out of the narration, normalise (`12.3%`, `1,234`, `LKR 1.2bn`) and require a match in the report within display rounding. One retry with the failures listed; if it still fails, print the report without a narrative and say why.
- Presentation: text output gets a "Narrative (AI-generated from the figures above)" block before the disclaimer; `--json` gets a `narrative` field with `model` and `prompt_version`. The informational disclaimer stays.
- Flags on `hex analyze`: `--narrate`, `--narrate-provider ollama|claude` (default `ollama`), `--narrate-model`.
- Note for docs: `analyze` is store-only today; `--narrate` is the one opt-in network call (to localhost for Ollama).

### Tests

- Fake model returns a good narration, one with an invented number, one with a missing `ref`; golden files for text and `--json`.
- Number-extraction unit tests for the formats above, including negatives and brackets.

### Done when

- Narration of a fixture report passes the number check; an injected wrong number is rejected.
- Without `--narrate`, output is byte-identical to today (existing goldens unchanged).

## Phase C: announcements to structured corporate actions

Feeds P1 (corporate-action dates and adjustment factors). Announcements are stored and classified by category, but dates need the unverified detail endpoint.

- First verify the detail endpoint (task in P1). Use the model only for what the structured data lacks.
- Extract `ex_date`, `record_date`, `payment_date`, amount per share, ratio, action type from announcement text. Verify that each date and amount appears in the text (date written in any of the formats CSE uses) before storing.
- Store with `source = llm` and the announcement id; manual values win. Prefer the deterministic parse where the text is regular, and use the model only on the rest.
- Done when a fixture set of dividend, bonus, rights and split announcements yields the expected rows, and an altered date in the model output is rejected.

## Phase D: later

- Audit opinion, going-concern, outlook from annual reports (P5): same verify-against-page pattern.
- Announcement tone classifier (P5, last).
- Embeddings (`nomic-embed-text`) for `hex search` across announcements and report text: needs a migration `embeddings(kind, id, model, dim, vec BLOB)`. Only when there is a query that keyword search cannot answer.

## Order and dependencies

A (provider split, then Ollama) -> B (reuses `Config`/`NewFallback` provider setup) -> C (after the P1 endpoint check) -> D.

A and B touch different files (`internal/llm`, `cmd/reports.go`, `cmd/sync.go` versus `internal/narrate`, `cmd/analyze.go`), so B can start once the `Config` type from A exists.

## Risks

| Risk | Mitigation |
|---|---|
| Silent prompt truncation by Ollama | Set `num_ctx`, check `prompt_eval_count`, fail loudly |
| Small model misreads columns | `Verify` plus rules-first order; chain to Claude below 0.7 |
| Slow generation blocks `hex sync` | Long timeout, sync concurrency stays at the engine limit, `--llm-max` caps reports per run |
| Different results per model or version | Record model name, digest and prompt version in the run |
| Narration invents a number or gives advice | Number check, `ref` per claim, fixed prompt, disclaimer |
| User's Ollama not running | Up-front reachability check with the `ollama pull` hint |

## Decisions to confirm

1. Default provider for `--llm` stays `claude` (no behaviour change), and for `--narrate` is `ollama`.
2. Chain order `ollama,claude` is opt-in, not the default.
3. Ollama URL via `HEX_OLLAMA_URL` only (no config file yet).
