package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Job statuses.
const (
	JobPending = "pending"
	JobRunning = "running"
	JobDone    = "done"
	JobFailed  = "failed"
)

// JobState is the persisted state of one engine job.
type JobState struct {
	Key       string
	Kind      string
	Status    string
	Attempts  int
	LastError string
	LastRunAt time.Time
	LastOKAt  time.Time
	NextDueAt time.Time
}

// PutJobState inserts or replaces a job's state.
func (t *Tx) PutJobState(ctx context.Context, s JobState) error {
	err := t.exec(ctx, `
		INSERT INTO sync_jobs (key, kind, status, attempts, last_error, last_run_at, last_ok_at, next_due_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (key) DO UPDATE SET kind = excluded.kind, status = excluded.status,
			attempts = excluded.attempts, last_error = excluded.last_error, last_run_at = excluded.last_run_at,
			last_ok_at = excluded.last_ok_at, next_due_at = excluded.next_due_at`,
		s.Key, s.Kind, s.Status, s.Attempts, nullStr(s.LastError), nullTime(s.LastRunAt),
		nullTime(s.LastOKAt), nullTime(s.NextDueAt))
	if err != nil {
		return fmt.Errorf("put job state %s: %w", s.Key, err)
	}
	return nil
}

func scanJob(sc interface{ Scan(...any) error }) (JobState, error) {
	var s JobState
	var lastErr sql.NullString
	var run, ok, due sql.NullInt64
	if err := sc.Scan(&s.Key, &s.Kind, &s.Status, &s.Attempts, &lastErr, &run, &ok, &due); err != nil {
		return JobState{}, err
	}
	s.LastError, s.LastRunAt, s.LastOKAt, s.NextDueAt = lastErr.String, fromMS(run), fromMS(ok), fromMS(due)
	return s, nil
}

// JobState returns a job's state; found is false when the job never ran.
func (db *DB) JobState(ctx context.Context, key string) (s JobState, found bool, err error) {
	s, err = scanJob(db.sql.QueryRowContext(ctx, `
		SELECT key, kind, status, attempts, last_error, last_run_at, last_ok_at, next_due_at
		FROM sync_jobs WHERE key = ?`, key))
	if errors.Is(err, sql.ErrNoRows) {
		return JobState{}, false, nil
	}
	if err != nil {
		return JobState{}, false, fmt.Errorf("read job state %s: %w", key, err)
	}
	return s, true, nil
}

// JobStates returns every job's state ordered by kind and key.
func (db *DB) JobStates(ctx context.Context) ([]JobState, error) {
	rows, err := db.sql.QueryContext(ctx, `
		SELECT key, kind, status, attempts, last_error, last_run_at, last_ok_at, next_due_at
		FROM sync_jobs ORDER BY kind, key`)
	if err != nil {
		return nil, fmt.Errorf("query job states: %w", err)
	}
	defer rows.Close()
	var out []JobState
	for rows.Next() {
		s, err := scanJob(rows)
		if err != nil {
			return nil, fmt.Errorf("scan job state: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// ResetRunning marks jobs left "running" by an interrupted run as pending.
func (db *DB) ResetRunning(ctx context.Context) error {
	if _, err := db.sql.ExecContext(ctx, `UPDATE sync_jobs SET status = ? WHERE status = ?`, JobPending, JobRunning); err != nil {
		return fmt.Errorf("reset running jobs: %w", err)
	}
	return nil
}
