---
name: sqlite-store
description: Conventions for hex's local SQLite database (internal/store): where it lives, adding migrations, writing typed repos with idempotent upserts, the single-writer Update transaction rule, value precedence for fundamentals, and testing with HEX_DATA_DIR. Use when adding or changing a table, column, query or repo, or when debugging database errors.
---

# SQLite store

- Driver: `modernc.org/sqlite` (pure Go, no cgo). Path: `$HEX_DATA_DIR/hex.db` or `os.UserConfigDir()/hex/hex.db`. Pragmas: WAL, `busy_timeout(5000)`, `foreign_keys(1)`.
- `SetMaxOpenConns(1)`: one connection, so writes never contend. **Never call a `*DB` method inside an `Update` callback** (it deadlocks); read first, then write.
- Reads are methods on `*DB`; writes are methods on `*Tx`, reached through `db.Update(ctx, func(tx *store.Tx) error {...})`, which commits on nil and rolls back on error or panic.

## Migrations
- Add `internal/store/migrations/NNNN_description.sql` (next number). Never edit an applied migration.
- Each file runs in one transaction and is recorded in `schema_version`.
- Prefer additive changes (`ALTER TABLE ... ADD COLUMN`, new tables, indexes). For a column change, create the new table, copy, drop, rename in one migration.
- Update `TestMigrateIsIdempotent` if the latest version changes.

## Schema conventions
- Times: Unix milliseconds `INTEGER`. Dates: `TEXT 'YYYY-MM-DD'` in Asia/Colombo (`output.Date`).
- Natural keys as primary keys (`code`, `(code, date)`, CSE ids), so writes are `INSERT ... ON CONFLICT DO UPDATE`: re-running a sync gives the same rows.
- Optional values: store NULL, not 0 (`nullStr`/`nullInt`/`nullFloat`/`nullTime` helpers; `*float64` in structs for nullable numbers).
- History tables only grow (`MergeBars` updates existing dates and never deletes).
- Fundamentals key: `(code, period_end, period_type, scope, field, source)`; `Fundamentals(code, all=false)` returns manual first, then highest confidence.
- Compressed blobs (page text, raw responses) go through `gzipBytes`/`gunzipBytes`.

## Tests
- `store.Open(ctx, filepath.Join(t.TempDir(), "hex.db"))`, or `t.Setenv(store.EnvDir, t.TempDir())` for command tests.
- Test round-trips, idempotent upserts and precedence; `db.SetClock` fixes timestamps.
