package pdfx

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// Poppler extracts text with poppler's pdftotext -layout. Its output is
// monospaced, so word X positions are character columns scaled to points.
type Poppler struct {
	Path string
}

// Name implements Backend.
func (Poppler) Name() string { return "pdftotext" }

var cellSplit = regexp.MustCompile(`\S+(?: \S+)*`)

// Pages implements Backend.
func (p Poppler) Pages(ctx context.Context, data []byte, maxPages int) ([]Page, error) {
	tmp, err := os.CreateTemp("", "hex-*.pdf")
	if err != nil {
		return nil, fmt.Errorf("create temp pdf: %w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return nil, fmt.Errorf("write temp pdf: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return nil, fmt.Errorf("write temp pdf: %w", err)
	}

	args := []string{"-layout", "-enc", "UTF-8"}
	if maxPages > 0 {
		args = append(args, "-l", fmt.Sprint(maxPages+1))
	}
	var out bytes.Buffer
	cmd := exec.CommandContext(ctx, p.Path, append(args, tmp.Name(), "-")...)
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("run pdftotext: %w", err)
	}

	raw := strings.Split(out.String(), "\f")
	if len(raw) > 0 && strings.TrimSpace(raw[len(raw)-1]) == "" {
		raw = raw[:len(raw)-1]
	}
	if maxPages > 0 && len(raw) > maxPages {
		return nil, fmt.Errorf("%w: over %d", ErrTooMany, maxPages)
	}
	const charW = 5.0 // points per column, roughly an 8pt font
	pages := make([]Page, len(raw))
	for i, text := range raw {
		page := Page{N: i + 1}
		for row, line := range strings.Split(text, "\n") {
			var ws []Word
			for _, loc := range cellSplit.FindAllStringIndex(line, -1) {
				ws = append(ws, Word{X: float64(loc[0]) * charW, W: float64(loc[1]-loc[0]) * charW, Text: line[loc[0]:loc[1]]})
			}
			if len(ws) > 0 {
				page.Lines = append(page.Lines, Line{Y: float64(-row), Words: ws})
			}
		}
		pages[i] = page
	}
	return pages, nil
}
