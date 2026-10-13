---
name: scrape-engine
description: How hex's scrape engine works and how to extend it: the Job/Stage model, TTLs, the bulk → detail → history → pdfs → fundamentals stage order, the shared rate limiter and sync lock, raw-response recording, and wiring a new data group into "hex sync". Use when adding a data source or job, changing freshness, debugging "hex sync", or touching internal/engine or internal/sources.
---

# Scrape engine

## Pieces
- `internal/engine`: `Job{Key, Kind, TTL, Run(ctx, *Env)}` (use `engine.FuncJob` for small jobs), `Stage` (jobs that may run together), `Run(ctx, env, stages, Options{Concurrency, Force, DryRun})`.
- `Env{CSE, Store, Files, Log, Now}`. `CSE` is wrapped by `engine.Limit(client, NewLimiter(rps, 4))`, so **every** request, retries and PDF downloads included, is spaced and capped. Never build another `http.Client` in a job.
- `engine.Lock(store.Dir())` stops two syncs; `engine.RecordRaw` keeps response bodies in `raw_responses`.
- `internal/sources/csejobs`: cse.lk JSON jobs + `Plan(scope, codes, now)`; `internal/sources/reportjobs`: `PDFs`, `Fundamentals`.
- `cmd/sync.go`: groups → plan → stages; `a.engineEnv(cmd)` builds the environment (also used by `hex reports extract`).

## Runner semantics
- Skip when `sync_jobs.status = done` and `last_ok_at + TTL > now`, unless `Force`. TTL 0 always runs.
- Failures are recorded (`failed`, `attempts`, `last_error`) and the run continues; failed jobs retry on the next run.
- Cancelling leaves jobs `pending`; `ResetRunning` at start fixes jobs left `running` by a crash.
- Panics become failures. Progress lines go to `env.Log` (stderr); skipped jobs are only counted.

## Stage order (later stages read what earlier ones stored)
1. bulk: `securities`, `market`, `quotes` (stores stockId for every traded security)
2. detail: `info:<code>` (stockId, shares), `profile:<code>` (year end), `reports:<code>`, `actions`
3. `history:<code>` (needs stockId)
4. `pdfs:<code>` (needs the catalogue)
5. `fundamentals:<code>` (needs text, shares, year end)

## Adding a job
1. Add the endpoint to `internal/cse` (see `cse-api`) and a fixture to `testdata/cse/` + a route in `csetest.New`.
2. Add store rows/repo (see `sqlite-store`).
3. Write a pure normaliser (API struct → store rows) with a unit test.
4. Write the job: fetch → normalise → one `env.Store.Update` with upserts. It must be idempotent. Key = `kind:target`.
5. Pick a TTL by how often the data changes (`cse-market-context` for trading hours; `csejobs.MarketOpen`).
6. Put it in the right stage in `Plan` and add a group to `syncGroups`/`syncScope` in `cmd/sync.go`.
7. Tests: run the plan against `csetest` with a temp store; assert rows, then that a second run makes zero requests.

## Rules
- Prefer one bulk call over N per-symbol calls. Keep concurrency ≤ 4 and the default 5 req/s.
- Treat `cse.ErrNoData` as "nothing to store" where that is normal (no reports, never traded), not as failure.
- Wrap errors with the job's subject: `fmt.Errorf("fetch history for %s: %w", code, err)`.
