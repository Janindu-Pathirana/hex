package engine

import (
	"context"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/Janindu-Pathirana/hex/internal/cse"
)

// Limiter spaces requests evenly at a fixed rate and caps how many are in
// flight. One Limiter is shared by every job in a run.
type Limiter struct {
	interval time.Duration
	slots    chan struct{}

	mu   sync.Mutex
	next time.Time
}

// NewLimiter allows rps requests per second with at most inFlight at once.
func NewLimiter(rps float64, inFlight int) *Limiter {
	if inFlight < 1 {
		inFlight = 1
	}
	return &Limiter{interval: time.Duration(float64(time.Second) / rps), slots: make(chan struct{}, inFlight)}
}

// Acquire waits for a rate slot and an in-flight slot. Call the returned
// release when the request is done.
func (l *Limiter) Acquire(ctx context.Context) (release func(), err error) {
	select {
	case l.slots <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	l.mu.Lock()
	now := time.Now()
	if l.next.Before(now) {
		l.next = now
	}
	wait := l.next.Sub(now)
	l.next = l.next.Add(l.interval)
	l.mu.Unlock()

	if wait > 0 {
		t := time.NewTimer(wait)
		defer t.Stop()
		select {
		case <-t.C:
		case <-ctx.Done():
			<-l.slots
			return nil, ctx.Err()
		}
	}
	return func() { <-l.slots }, nil
}

// Transport wraps next so every request (including retries) goes through l.
func (l *Limiter) Transport(next http.RoundTripper) http.RoundTripper {
	if next == nil {
		next = http.DefaultTransport
	}
	return roundTripper(func(req *http.Request) (*http.Response, error) {
		release, err := l.Acquire(req.Context())
		if err != nil {
			return nil, err
		}
		resp, err := next.RoundTrip(req)
		if err != nil {
			release()
			return nil, err
		}
		// Hold the in-flight slot until the body is read and closed.
		resp.Body = &releaseOnClose{ReadCloser: resp.Body, release: release}
		return resp, nil
	})
}

// Limit returns a copy of c whose API and download clients go through l.
func Limit(c *cse.Client, l *Limiter) *cse.Client {
	lc := *c
	lc.HTTP = limitedHTTP(c.HTTP, l)
	if c.DownloadHTTP != nil {
		lc.DownloadHTTP = limitedHTTP(c.DownloadHTTP, l)
	}
	return &lc
}

func limitedHTTP(hc *http.Client, l *Limiter) *http.Client {
	if hc == nil {
		hc = http.DefaultClient
	}
	cp := *hc
	cp.Transport = l.Transport(hc.Transport)
	return &cp
}

type roundTripper func(*http.Request) (*http.Response, error)

func (f roundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type releaseOnClose struct {
	io.ReadCloser
	release func()
	once    sync.Once
}

func (r *releaseOnClose) Close() error {
	err := r.ReadCloser.Close()
	r.once.Do(r.release)
	return err
}
