// Package pdfx turns report PDFs into positioned text: pages of lines of
// words with x coordinates, so tables can be rebuilt. It also stores PDFs by
// content hash and guards against hostile files.
package pdfx

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"
)

// Word is a run of text on a line, starting at X (PDF points) with width W.
type Word struct {
	X    float64 `json:"x"`
	W    float64 `json:"w"`
	Text string  `json:"t"`
}

// Line is the words sharing a baseline, left to right.
type Line struct {
	Y     float64 `json:"y"`
	Words []Word  `json:"w"`
}

// Page is one page's lines, top to bottom.
type Page struct {
	N     int    `json:"n"` // 1-based
	Lines []Line `json:"lines"`
}

// Text renders the page as plain text, one line per row.
func (p Page) Text() string {
	var b strings.Builder
	for _, l := range p.Lines {
		b.WriteString(l.Text())
		b.WriteByte('\n')
	}
	return b.String()
}

// Text renders a line with single spaces between words.
func (l Line) Text() string {
	parts := make([]string, len(l.Words))
	for i, w := range l.Words {
		parts[i] = w.Text
	}
	return strings.Join(parts, " ")
}

// Chars counts the non-space characters on the page.
func (p Page) Chars() int {
	n := 0
	for _, l := range p.Lines {
		for _, w := range l.Words {
			n += len(strings.ReplaceAll(w.Text, " ", ""))
		}
	}
	return n
}

// Backend extracts positioned text from a PDF.
type Backend interface {
	Name() string
	Pages(ctx context.Context, data []byte, maxPages int) ([]Page, error)
}

// Limits bound the work done on one (untrusted) PDF.
type Limits struct {
	MaxBytes int64
	MaxPages int
	Timeout  time.Duration
}

// DefaultLimits are the caps used by the engine.
var DefaultLimits = Limits{MaxBytes: 50 << 20, MaxPages: 1000, Timeout: 60 * time.Second}

// Errors returned by Extract.
var (
	ErrTooLarge  = errors.New("pdf too large")
	ErrTooMany   = errors.New("pdf has too many pages")
	ErrTimeout   = errors.New("pdf text extraction timed out")
	ErrNoBackend = errors.New("no pdf text backend")
)

// scannedChars is the average characters per page below which a PDF is
// treated as scanned images rather than text.
const scannedChars = 40

// Result is the outcome of extracting a PDF.
type Result struct {
	Backend string
	Pages   []Page
	Scanned bool
}

// Extract runs backend on data within lim. A panic or hang inside the PDF
// parser becomes an error instead of taking down the process.
func Extract(ctx context.Context, b Backend, data []byte, lim Limits) (Result, error) {
	if b == nil {
		return Result{}, ErrNoBackend
	}
	if lim.MaxBytes > 0 && int64(len(data)) > lim.MaxBytes {
		return Result{}, fmt.Errorf("%w: %d bytes", ErrTooLarge, len(data))
	}
	if lim.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, lim.Timeout)
		defer cancel()
	}

	type out struct {
		pages []Page
		err   error
	}
	ch := make(chan out, 1)
	go func() {
		defer func() {
			if p := recover(); p != nil {
				ch <- out{err: fmt.Errorf("parse pdf: panic: %v", p)}
			}
		}()
		pages, err := b.Pages(ctx, data, lim.MaxPages)
		ch <- out{pages, err}
	}()

	select {
	case o := <-ch:
		if o.err != nil {
			return Result{}, o.err
		}
		total := 0
		for _, p := range o.pages {
			total += p.Chars()
		}
		scanned := len(o.pages) > 0 && total/len(o.pages) < scannedChars
		return Result{Backend: b.Name(), Pages: o.pages, Scanned: scanned}, nil
	case <-ctx.Done():
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return Result{}, ErrTimeout
		}
		return Result{}, ctx.Err()
	}
}

// DefaultBackend is poppler's pdftotext when it is installed (better column
// layout), otherwise the pure-Go reader.
func DefaultBackend() Backend {
	if path, err := exec.LookPath("pdftotext"); err == nil {
		return Poppler{Path: path}
	}
	return Pure{}
}

// glyph is one positioned text fragment from a backend, before grouping.
type glyph struct {
	x, y, w, size float64
	s             string
}

// groupLines clusters glyphs into lines by baseline (within tol points) and
// merges glyphs into words when the gap between them is small relative to
// the font size.
func groupLines(gs []glyph, tol float64) []Line {
	if len(gs) == 0 {
		return nil
	}
	sort.SliceStable(gs, func(i, j int) bool {
		if d := gs[i].y - gs[j].y; d > tol || d < -tol {
			return gs[i].y > gs[j].y // PDF y grows upwards: top of page first
		}
		return gs[i].x < gs[j].x
	})

	var lines []Line
	var cur []glyph
	flush := func() {
		if len(cur) == 0 {
			return
		}
		sort.SliceStable(cur, func(i, j int) bool { return cur[i].x < cur[j].x })
		lines = append(lines, Line{Y: cur[0].y, Words: words(cur)})
		cur = nil
	}
	for _, g := range gs {
		if len(cur) > 0 && abs(g.y-cur[0].y) > tol {
			flush()
		}
		cur = append(cur, g)
	}
	flush()
	return lines
}

// words joins glyphs into words. A word ends at a space glyph that follows the
// previous glyph, or where the gap to the next glyph exceeds a fifth of the
// font size. Space glyphs drawn on top of text (filler some PDFs emit between
// digits) are ignored.
func words(gs []glyph) []Word {
	var out []Word
	var b strings.Builder
	var start, end float64
	emit := func() {
		if t := normalize(b.String()); t != "" {
			out = append(out, Word{X: start, W: end - start, Text: t})
		}
		b.Reset()
	}
	for _, g := range gs {
		if strings.TrimSpace(g.s) == "" {
			if g.s == " " && b.Len() > 0 && g.x >= end-0.1 {
				emit()
			}
			continue
		}
		size := g.size
		if size <= 0 {
			size = 8
		}
		if b.Len() > 0 && g.x-end > 0.2*size {
			emit()
		}
		if b.Len() == 0 {
			start = g.x
		}
		b.WriteString(g.s)
		if e := g.x + g.w; e > end || b.Len() == len(g.s) {
			end = e
		}
	}
	emit()
	return out
}

var ligatures = strings.NewReplacer("\ufb00", "ff", "\ufb01", "fi", "\ufb02", "fl", "\ufb03", "ffi", "\ufb04", "ffl",
	"\u2019", "'", "\u2018", "'", "\u201c", `"`, "\u201d", `"`, "\u2013", "-", "\u2014", "-", "\u00a0", " ")

// normalize expands ligatures and folds typographic punctuation to ASCII.
func normalize(s string) string { return strings.TrimSpace(ligatures.Replace(s)) }

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}
