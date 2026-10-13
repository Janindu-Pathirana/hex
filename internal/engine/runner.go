package engine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/Janindu-Pathirana/hex/internal/store"
)

// MaxConcurrency is the hard cap on concurrent jobs (cse.lk is a public site).
const MaxConcurrency = 4

// Options control a run.
type Options struct {
	Concurrency int  // jobs at once, 1..MaxConcurrency (default MaxConcurrency)
	Force       bool // ignore TTLs and run everything
	DryRun      bool // report what would run without running it
}

// Outcome of one job in a run.
const (
	Ran     = "ok"
	Skipped = "fresh"
	Failed  = "failed"
	Planned = "planned" // dry run
	Pending = "pending" // cancelled before or while running
)

// Result is what happened to one job.
type Result struct {
	Key      string
	Kind     string
	Outcome  string
	Err      error
	Duration time.Duration
}

// Summary totals a run.
type Summary struct {
	Results []Result
}

// Count returns how many jobs had the given outcome.
func (s Summary) Count(outcome string) int {
	n := 0
	for _, r := range s.Results {
		if r.Outcome == outcome {
			n++
		}
	}
	return n
}

// Err returns the job failures joined, or nil.
func (s Summary) Err() error {
	var errs []error
	for _, r := range s.Results {
		if r.Outcome == Failed {
			errs = append(errs, fmt.Errorf("%s: %w", r.Key, r.Err))
		}
	}
	return errors.Join(errs...)
}

// Run executes stages in order. Within a stage up to Concurrency jobs run at
// once. Failures are recorded and do not stop the run; cancelling ctx stops
// scheduling and leaves unfinished jobs pending, so the next run resumes.
func Run(ctx context.Context, env *Env, stages []Stage, opt Options) (Summary, error) {
	if opt.Concurrency < 1 || opt.Concurrency > MaxConcurrency {
		opt.Concurrency = MaxConcurrency
	}
	if env.Now == nil {
		env.Now = time.Now
	}
	if env.Log == nil {
		env.Log = io.Discard
	}
	if err := env.Store.ResetRunning(ctx); err != nil {
		return Summary{}, err
	}

	total := 0
	for _, st := range stages {
		total += len(st)
	}
	r := &runner{env: env, opt: opt, total: total}
	for _, st := range stages {
		r.stage(ctx, st)
	}
	return Summary{Results: r.results}, ctx.Err()
}

type runner struct {
	env   *Env
	opt   Options
	total int

	mu      sync.Mutex
	done    int
	results []Result
}

func (r *runner) stage(ctx context.Context, jobs Stage) {
	sem := make(chan struct{}, r.opt.Concurrency)
	var wg sync.WaitGroup
	for _, j := range jobs {
		if ctx.Err() != nil {
			r.record(Result{Key: j.Key(), Kind: j.Kind(), Outcome: Pending})
			continue
		}
		fresh, err := r.fresh(ctx, j)
		if err != nil {
			r.record(Result{Key: j.Key(), Kind: j.Kind(), Outcome: Failed, Err: err})
			continue
		}
		if fresh {
			r.record(Result{Key: j.Key(), Kind: j.Kind(), Outcome: Skipped})
			continue
		}
		if r.opt.DryRun {
			r.record(Result{Key: j.Key(), Kind: j.Kind(), Outcome: Planned})
			continue
		}
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			r.record(Result{Key: j.Key(), Kind: j.Kind(), Outcome: Pending})
			continue
		}
		wg.Add(1)
		go func(j Job) {
			defer wg.Done()
			defer func() { <-sem }()
			r.record(r.run(ctx, j))
		}(j)
	}
	wg.Wait()
}

func (r *runner) fresh(ctx context.Context, j Job) (bool, error) {
	if r.opt.Force {
		return false, nil
	}
	s, found, err := r.env.Store.JobState(ctx, j.Key())
	if err != nil || !found {
		return false, err
	}
	return s.Status == store.JobDone && !s.LastOKAt.IsZero() && r.env.Now().Before(s.LastOKAt.Add(j.TTL())), nil
}

func (r *runner) run(ctx context.Context, j Job) (res Result) {
	res = Result{Key: j.Key(), Kind: j.Kind()}
	prev, _, err := r.env.Store.JobState(ctx, j.Key())
	if err != nil {
		res.Outcome, res.Err = Failed, err
		return res
	}
	state := prev
	state.Key, state.Kind = j.Key(), j.Kind()
	state.Status, state.Attempts, state.LastRunAt = store.JobRunning, prev.Attempts+1, r.env.Now()
	if err := r.put(ctx, state); err != nil {
		res.Outcome, res.Err = Failed, err
		return res
	}

	start := time.Now()
	err = safeRun(ctx, j, r.env)
	res.Duration = time.Since(start)

	// Record the final state even if ctx was cancelled mid-job.
	bg := context.WithoutCancel(ctx)
	switch {
	case err == nil:
		now := r.env.Now()
		state.Status, state.Attempts, state.LastError = store.JobDone, 0, ""
		state.LastOKAt, state.NextDueAt = now, now.Add(j.TTL())
		res.Outcome = Ran
	case ctx.Err() != nil:
		state.Status = store.JobPending
		res.Outcome, res.Err = Pending, err
	default:
		state.Status, state.LastError = store.JobFailed, err.Error()
		res.Outcome, res.Err = Failed, err
	}
	if perr := r.put(bg, state); perr != nil && res.Err == nil {
		res.Outcome, res.Err = Failed, perr
	}
	return res
}

func (r *runner) put(ctx context.Context, s store.JobState) error {
	return r.env.Store.Update(ctx, func(tx *store.Tx) error { return tx.PutJobState(ctx, s) })
}

func safeRun(ctx context.Context, j Job, env *Env) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("panic: %v", p)
		}
	}()
	return j.Run(ctx, env)
}

func (r *runner) record(res Result) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.done++
	r.results = append(r.results, res)
	switch res.Outcome {
	case Skipped:
		return // counted in the summary, not printed one by one
	case Failed, Pending:
		fmt.Fprintf(r.env.Log, "[%d/%d] %s %s: %v\n", r.done, r.total, res.Key, res.Outcome, res.Err)
	case Planned:
		fmt.Fprintf(r.env.Log, "[%d/%d] %s would run\n", r.done, r.total, res.Key)
	default:
		fmt.Fprintf(r.env.Log, "[%d/%d] %s %s %.1fs\n", r.done, r.total, res.Key, res.Outcome, res.Duration.Seconds())
	}
}
