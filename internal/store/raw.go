package store

import (
	"context"
	"fmt"
	"time"
)

// RawKeep is how many raw responses are kept per (source, endpoint, key).
const RawKeep = 3

// SaveRaw stores a raw response body (gzipped) and prunes older copies.
func (t *Tx) SaveRaw(ctx context.Context, source, endpoint, key string, body []byte) error {
	gz, err := gzipBytes(body)
	if err != nil {
		return err
	}
	if err := t.exec(ctx, `INSERT INTO raw_responses (source, endpoint, key, fetched_at, body_gz) VALUES (?, ?, ?, ?, ?)`,
		source, endpoint, key, ms(t.now), gz); err != nil {
		return fmt.Errorf("save raw %s/%s: %w", endpoint, key, err)
	}
	err = t.exec(ctx, `
		DELETE FROM raw_responses WHERE source = ? AND endpoint = ? AND key = ? AND id NOT IN (
			SELECT id FROM raw_responses WHERE source = ? AND endpoint = ? AND key = ?
			ORDER BY fetched_at DESC, id DESC LIMIT ?)`,
		source, endpoint, key, source, endpoint, key, RawKeep)
	if err != nil {
		return fmt.Errorf("prune raw %s/%s: %w", endpoint, key, err)
	}
	return nil
}

// LatestRaw returns the newest stored body for a key, or sql.ErrNoRows.
func (db *DB) LatestRaw(ctx context.Context, source, endpoint, key string) ([]byte, time.Time, error) {
	var gz []byte
	var at int64
	err := db.sql.QueryRowContext(ctx, `
		SELECT body_gz, fetched_at FROM raw_responses WHERE source = ? AND endpoint = ? AND key = ?
		ORDER BY fetched_at DESC, id DESC LIMIT 1`, source, endpoint, key).Scan(&gz, &at)
	if err != nil {
		return nil, time.Time{}, err
	}
	body, err := gunzipBytes(gz)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("decompress raw %s/%s: %w", endpoint, key, err)
	}
	return body, time.UnixMilli(at), nil
}

// ClearRaw deletes every stored raw response and returns how many were removed.
func (db *DB) ClearRaw(ctx context.Context) (int64, error) {
	res, err := db.sql.ExecContext(ctx, `DELETE FROM raw_responses`)
	if err != nil {
		return 0, fmt.Errorf("clear raw responses: %w", err)
	}
	if _, err := db.sql.ExecContext(ctx, `VACUUM`); err != nil {
		return 0, fmt.Errorf("vacuum database: %w", err)
	}
	return res.RowsAffected()
}
