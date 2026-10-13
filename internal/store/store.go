// Package store is hex's local SQLite database: everything the scrape engine
// collects, plus job state and raw responses for re-processing.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite" // pure-Go driver, registers "sqlite"
)

// EnvDir overrides the data directory (used by tests).
const EnvDir = "HEX_DATA_DIR"

// FileName is the database file inside the data directory.
const FileName = "hex.db"

// Dir returns the hex data directory: $HEX_DATA_DIR, or os.UserConfigDir()/hex
// (~/Library/Application Support/hex on macOS).
func Dir() (string, error) {
	if d := os.Getenv(EnvDir); d != "" {
		return d, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve data directory: %w", err)
	}
	return filepath.Join(base, "hex"), nil
}

// Path returns the default database path.
func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, FileName), nil
}

// DB is an open hex database. It uses a single connection, so SQLite never
// sees two writers; do not call DB methods from inside an Update callback.
type DB struct {
	sql *sql.DB
	now func() time.Time
}

// Open opens (creating if needed) the database at path and applies migrations.
func Open(ctx context.Context, path string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}
	dsn := "file:" + path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=synchronous(NORMAL)"
	sdb, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database %s: %w", path, err)
	}
	sdb.SetMaxOpenConns(1)
	db := &DB{sql: sdb, now: time.Now}
	if err := db.migrate(ctx); err != nil {
		sdb.Close()
		return nil, err
	}
	return db, nil
}

// OpenDefault opens the database at Path().
func OpenDefault(ctx context.Context) (*DB, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	return Open(ctx, path)
}

// Close closes the database.
func (db *DB) Close() error { return db.sql.Close() }

// SetClock replaces the clock used for updated_at stamps (tests).
func (db *DB) SetClock(now func() time.Time) { db.now = now }

// Tx is a write transaction. All write methods live on Tx.
type Tx struct {
	tx  *sql.Tx
	now time.Time
}

// Update runs fn in a transaction, committing if fn returns nil.
func (db *DB) Update(ctx context.Context, fn func(*Tx) error) (err error) {
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()
	if err := fn(&Tx{tx: tx, now: db.now()}); err != nil {
		return errors.Join(err, tx.Rollback())
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

func (t *Tx) exec(ctx context.Context, query string, args ...any) error {
	_, err := t.tx.ExecContext(ctx, query, args...)
	return err
}

func ms(t time.Time) int64 { return t.UnixMilli() }

func fromMS(v sql.NullInt64) time.Time {
	if !v.Valid {
		return time.Time{}
	}
	return time.UnixMilli(v.Int64)
}

// nullable helpers: zero values are stored as NULL.

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullInt(v int64) any {
	if v == 0 {
		return nil
	}
	return v
}

func nullTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t.UnixMilli()
}

func ptrFloat(v sql.NullFloat64) *float64 {
	if !v.Valid {
		return nil
	}
	f := v.Float64
	return &f
}
