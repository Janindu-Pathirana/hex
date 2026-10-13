// Package cse is a client for the undocumented JSON API behind cse.lk.
package cse

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Janindu-Pathirana/hex/internal/httpclient"
)

const (
	// DefaultBaseURL is the root of the cse.lk API endpoints.
	DefaultBaseURL = "https://www.cse.lk/api/"
	// DefaultSiteURL is the root of the cse.lk website (static files such as the symbol CSV).
	DefaultSiteURL = "https://www.cse.lk/"
	// CDNURL is the root that report and logo paths are relative to.
	CDNURL = "https://cdn.cse.lk/"

	userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X) AppleWebKit/537.36 (KHTML, like Gecko) Chrome Safari"
)

// ErrNoData is returned when the API answers successfully but with no content,
// which is how cse.lk reports an unknown symbol (204, or 200 with {} / []).
var ErrNoData = errors.New("no data")

// Client talks to cse.lk. The zero value is not usable; use New or Default.
type Client struct {
	BaseURL string       // API root, e.g. DefaultBaseURL; overridable in tests
	SiteURL string       // website root, e.g. DefaultSiteURL
	HTTP    *http.Client // transport
	Retries int          // extra attempts on network errors and 5xx
	Backoff time.Duration
}

// New returns a client pointed at the live site.
func New() *Client {
	return &Client{
		BaseURL: DefaultBaseURL,
		SiteURL: DefaultSiteURL,
		HTTP:    httpclient.Default,
		Retries: 2,
		Backoff: 500 * time.Millisecond,
	}
}

// Default is the client commands use unless one is injected.
var Default = New()

// post sends a form-encoded POST to endpoint and decodes the JSON response into out.
func (c *Client) post(ctx context.Context, endpoint string, form url.Values, out any) error {
	body, err := c.do(ctx, func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+endpoint, strings.NewReader(form.Encode()))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
		req.Header.Set("Accept", "application/json, text/plain, */*")
		req.Header.Set("Origin", "https://www.cse.lk")
		req.Header.Set("Referer", "https://www.cse.lk/")
		req.Header.Set("User-Agent", userAgent)
		return req, nil
	})
	if err != nil {
		return fmt.Errorf("request %s: %w", endpoint, err)
	}

	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || string(trimmed) == "{}" || string(trimmed) == "[]" || string(trimmed) == "null" {
		return fmt.Errorf("request %s: %w", endpoint, ErrNoData)
	}
	if err := json.Unmarshal(trimmed, out); err != nil {
		return fmt.Errorf("decode %s: %w (body: %q)", endpoint, err, snippet(trimmed))
	}
	return nil
}

// get fetches a file relative to SiteURL.
func (c *Client) get(ctx context.Context, path string) ([]byte, error) {
	body, err := c.do(ctx, func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.SiteURL+path, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", userAgent)
		return req, nil
	})
	if err != nil {
		return nil, fmt.Errorf("request %s: %w", path, err)
	}
	return body, nil
}

// do runs the request built by newReq, retrying network errors and 5xx responses.
func (c *Client) do(ctx context.Context, newReq func() (*http.Request, error)) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(c.Backoff * time.Duration(1<<(attempt-1))):
			}
		}

		req, err := newReq()
		if err != nil {
			return nil, err
		}
		body, retry, err := c.once(req)
		if err == nil {
			return body, nil
		}
		if !retry || ctx.Err() != nil {
			return nil, err
		}
		lastErr = err
	}
	return nil, lastErr
}

func (c *Client) once(req *http.Request) (body []byte, retry bool, err error) {
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() {
		if closeErr := resp.Body.Close(); err == nil && closeErr != nil {
			err = fmt.Errorf("close response body: %w", closeErr)
		}
	}()

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, true, fmt.Errorf("read body: %w", err)
	}
	switch {
	case resp.StatusCode == http.StatusNoContent:
		return nil, false, nil
	case resp.StatusCode >= 500:
		return nil, true, fmt.Errorf("unexpected status %s", resp.Status)
	case resp.StatusCode != http.StatusOK:
		return nil, false, fmt.Errorf("unexpected status %s", resp.Status)
	}
	return body, false, nil
}

func snippet(b []byte) string {
	const max = 200
	if len(b) > max {
		return string(b[:max]) + "…"
	}
	return string(b)
}

// AssetURL turns a path from the API (report PDF, logo) into an absolute URL.
func AssetURL(path string) string {
	if path == "" || strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	return CDNURL + strings.TrimPrefix(path, "/")
}
