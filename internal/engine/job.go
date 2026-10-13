// Package engine runs scrape jobs: it skips fresh work, bounds concurrency,
// rate-limits every outbound request and records job state for resuming.
package engine

import (
	"context"
	"io"
	"time"

	"github.com/Janindu-Pathirana/hex/internal/cse"
	"github.com/Janindu-Pathirana/hex/internal/store"
)

// Job is one unit of scrape work, such as fetching one company's history.
type Job interface {
	// Key is unique across all jobs, e.g. "history:JKH.N0000".
	Key() string
	// Kind groups jobs for status output, e.g. "history".
	Kind() string
	// TTL is how long a successful run stays fresh.
	TTL() time.Duration
	// Run does the work. It must be safe to repeat (idempotent).
	Run(ctx context.Context, env *Env) error
}

// Stage is a set of jobs that may run concurrently. Stages run in order, so a
// later stage can rely on data an earlier one stored.
type Stage []Job

// Env is what jobs get to work with.
type Env struct {
	CSE   *cse.Client // rate-limited; see Limit
	Store *store.DB
	Files Files     // PDF blob store; nil when PDFs are not in scope
	Log   io.Writer // progress and warnings (stderr)
	Now   func() time.Time
	// Extra carries optional settings for specific jobs (e.g. LLM options).
	Extra map[string]any
}

// Files stores report PDFs by content hash.
type Files interface {
	Put(data []byte) (sha256 string, err error)
	Get(sha256 string) ([]byte, error)
	Has(sha256 string) bool
}

// FuncJob adapts a function into a Job.
type FuncJob struct {
	K, Kd string
	T     time.Duration
	F     func(ctx context.Context, env *Env) error
}

func (j FuncJob) Key() string                             { return j.K }
func (j FuncJob) Kind() string                            { return j.Kd }
func (j FuncJob) TTL() time.Duration                      { return j.T }
func (j FuncJob) Run(ctx context.Context, env *Env) error { return j.F(ctx, env) }
