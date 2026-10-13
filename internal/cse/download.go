package cse

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// ErrNotPDF is returned when a downloaded report is not a PDF file.
var ErrNotPDF = errors.New("not a pdf")

// DownloadPDF fetches a report PDF by its API path (e.g. Report.Path), relative
// to CDNBase. Files larger than max bytes fail with ErrTooLarge.
func (c *Client) DownloadPDF(ctx context.Context, path string, max int64) ([]byte, error) {
	hc := c.DownloadHTTP
	if hc == nil {
		hc = c.HTTP
	}
	path = strings.TrimPrefix(path, "/")
	body, err := c.do(ctx, hc, max, func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.CDNBase+path, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", userAgent)
		req.Header.Set("Referer", "https://www.cse.lk/")
		return req, nil
	})
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", path, err)
	}
	if !bytes.HasPrefix(bytes.TrimLeft(body, "\x00\t\r\n "), []byte("%PDF-")) {
		return nil, fmt.Errorf("download %s: %w (body: %q)", path, ErrNotPDF, snippet(body))
	}
	return body, nil
}
