package engine_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Janindu-Pathirana/hex/internal/csetest"
	"github.com/Janindu-Pathirana/hex/internal/engine"
	"github.com/Janindu-Pathirana/hex/internal/store"
)

func newEnv(t *testing.T) *engine.Env {
	t.Helper()
	db, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "hex.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return &engine.Env{Store: db, Log: io.Discard}
}

func counter(key string, ttl time.Duration, n *atomic.Int32, err error) engine.Job {
	return engine.FuncJob{K: key, Kd: "test", T: ttl, F: func(ctx context.Context, env *engine.Env) error {
		n.Add(1)
		return err
	}}
}

func TestTTLSkipAndForce(t *testing.T) {
	env := newEnv(t)
	ctx := context.Background()
	var n atomic.Int32
	stages := []engine.Stage{{counter("a", time.Hour, &n, nil)}}

	for i, opt := range []engine.Options{{}, {}, {Force: true}} {
		if _, err := engine.Run(ctx, env, stages, opt); err != nil {
			t.Fatalf("run %d: %v", i, err)
		}
	}
	if n.Load() != 2 {
		t.Fatalf("job ran %d times, want 2 (first run + forced)", n.Load())
	}

	env.Now = func() time.Time { return time.Now().Add(2 * time.Hour) }
	sum, _ := engine.Run(ctx, env, stages, engine.Options{})
	if n.Load() != 3 || sum.Count(engine.Ran) != 1 {
		t.Fatalf("expired job should run again: ran %d, summary %+v", n.Load(), sum)
	}
}

func TestFailuresAreRecordedAndRetried(t *testing.T) {
	env := newEnv(t)
	ctx := context.Background()
	var ok, bad atomic.Int32
	boom := errors.New("boom")
	stages := []engine.Stage{{counter("good", time.Hour, &ok, nil), counter("bad", time.Hour, &bad, boom)}}

	sum, err := engine.Run(ctx, env, stages, engine.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if sum.Count(engine.Ran) != 1 || sum.Count(engine.Failed) != 1 || !errors.Is(sum.Err(), boom) {
		t.Fatalf("summary = %+v", sum)
	}
	s, _, _ := env.Store.JobState(ctx, "bad")
	if s.Status != store.JobFailed || s.LastError != "boom" || s.Attempts != 1 {
		t.Fatalf("bad state = %+v", s)
	}
	engine.Run(ctx, env, stages, engine.Options{})
	if bad.Load() != 2 || ok.Load() != 1 {
		t.Fatalf("failed job should retry and fresh job skip: bad %d good %d", bad.Load(), ok.Load())
	}
	s, _, _ = env.Store.JobState(ctx, "bad")
	if s.Attempts != 2 {
		t.Fatalf("attempts = %d, want 2", s.Attempts)
	}
}

func TestPanicIsAFailure(t *testing.T) {
	env := newEnv(t)
	j := engine.FuncJob{K: "p", Kd: "test", T: time.Hour, F: func(context.Context, *engine.Env) error { panic("bad pdf") }}
	sum, _ := engine.Run(context.Background(), env, []engine.Stage{{j}}, engine.Options{})
	if sum.Count(engine.Failed) != 1 {
		t.Fatalf("summary = %+v", sum)
	}
}

func TestCancelLeavesPendingAndResumes(t *testing.T) {
	env := newEnv(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var n atomic.Int32
	var jobs engine.Stage
	for _, k := range []string{"a", "b", "c", "d", "e", "f"} {
		jobs = append(jobs, engine.FuncJob{K: k, Kd: "test", T: time.Hour, F: func(ctx context.Context, env *engine.Env) error {
			if n.Add(1) == 2 {
				cancel()
			}
			return ctx.Err()
		}})
	}
	sum, err := engine.Run(ctx, env, []engine.Stage{jobs}, engine.Options{Concurrency: 1})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
	if sum.Count(engine.Ran) != 1 || sum.Count(engine.Pending) != 5 {
		t.Fatalf("summary ran=%d pending=%d", sum.Count(engine.Ran), sum.Count(engine.Pending))
	}

	n.Store(100) // never cancels again
	sum, err = engine.Run(context.Background(), env, []engine.Stage{jobs}, engine.Options{})
	if err != nil || sum.Count(engine.Ran) != 5 || sum.Count(engine.Skipped) != 1 {
		t.Fatalf("resume: err %v ran=%d fresh=%d", err, sum.Count(engine.Ran), sum.Count(engine.Skipped))
	}
}

func TestConcurrencyCapAndStages(t *testing.T) {
	env := newEnv(t)
	var cur, peak atomic.Int32
	var firstDone atomic.Bool
	var jobs engine.Stage
	for i := 0; i < 12; i++ {
		jobs = append(jobs, engine.FuncJob{K: string(rune('a' + i)), Kd: "test", T: time.Hour, F: func(context.Context, *engine.Env) error {
			c := cur.Add(1)
			for {
				p := peak.Load()
				if c <= p || peak.CompareAndSwap(p, c) {
					break
				}
			}
			time.Sleep(5 * time.Millisecond)
			cur.Add(-1)
			return nil
		}})
	}
	var orderOK atomic.Bool
	second := engine.FuncJob{K: "z", Kd: "test", T: time.Hour, F: func(context.Context, *engine.Env) error {
		orderOK.Store(cur.Load() == 0 && firstDone.Load())
		return nil
	}}
	last := jobs[len(jobs)-1].(engine.FuncJob)
	inner := last.F
	last.F = func(ctx context.Context, env *engine.Env) error { defer firstDone.Store(true); return inner(ctx, env) }
	jobs[len(jobs)-1] = last

	if _, err := engine.Run(context.Background(), env, []engine.Stage{jobs, {second}}, engine.Options{Concurrency: 99}); err != nil {
		t.Fatal(err)
	}
	if peak.Load() > engine.MaxConcurrency {
		t.Fatalf("peak concurrency %d > %d", peak.Load(), engine.MaxConcurrency)
	}
	if !orderOK.Load() {
		t.Fatal("second stage started before the first finished")
	}
}

func TestDryRunDoesNothing(t *testing.T) {
	env := newEnv(t)
	var n atomic.Int32
	sum, _ := engine.Run(context.Background(), env, []engine.Stage{{counter("a", time.Hour, &n, nil)}}, engine.Options{DryRun: true})
	if n.Load() != 0 || sum.Count(engine.Planned) != 1 {
		t.Fatalf("dry run ran %d, summary %+v", n.Load(), sum)
	}
}

func TestLimiterSpacesRequests(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		_, _ = w.Write([]byte(`{"status":"Regular Trading"}`))
	}))
	defer srv.Close()

	c := engine.Limit(csetest.Client(srv.URL, srv.Client()), engine.NewLimiter(50, 4)) // 20ms apart
	start := time.Now()
	for i := 0; i < 6; i++ {
		if _, err := c.MarketStatus(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if el := time.Since(start); el < 90*time.Millisecond {
		t.Fatalf("6 requests at 50 rps took %v, want >= 100ms", el)
	}
	if hits.Load() != 6 {
		t.Fatalf("hits = %d", hits.Load())
	}
}

func TestLimiterRespectsCancel(t *testing.T) {
	l := engine.NewLimiter(1, 1)
	release, err := l.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := l.Acquire(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v", err)
	}
}

func TestLock(t *testing.T) {
	dir := t.TempDir()
	unlock, err := engine.Lock(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Lock(dir); !errors.Is(err, engine.ErrLocked) {
		t.Fatalf("second lock: %v", err)
	}
	if err := unlock(); err != nil {
		t.Fatal(err)
	}

	// A lock left by a dead process is taken over.
	if err := os.WriteFile(filepath.Join(dir, engine.LockFile), []byte("999999\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	unlock, err = engine.Lock(dir)
	if err != nil {
		t.Fatalf("stale lock not taken over: %v", err)
	}
	unlock()
}
