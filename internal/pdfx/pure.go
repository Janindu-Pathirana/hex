package pdfx

import (
	"bytes"
	"context"
	"fmt"

	"github.com/ledongthuc/pdf"
)

// Pure extracts text with the pure-Go ledongthuc/pdf reader.
type Pure struct{}

// Name implements Backend.
func (Pure) Name() string { return "pure-go" }

// Pages implements Backend.
func (Pure) Pages(ctx context.Context, data []byte, maxPages int) ([]Page, error) {
	r, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("open pdf: %w", err)
	}
	n := r.NumPage()
	if maxPages > 0 && n > maxPages {
		return nil, fmt.Errorf("%w: %d", ErrTooMany, n)
	}
	pages := make([]Page, 0, n)
	for i := 1; i <= n; i++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		p := r.Page(i)
		page := Page{N: i}
		if !p.V.IsNull() {
			page.Lines = pageLines(p)
		}
		pages = append(pages, page)
	}
	return pages, nil
}

// pageLines reads one page; a malformed page yields no lines rather than
// failing the whole document.
func pageLines(p pdf.Page) (lines []Line) {
	defer func() {
		if recover() != nil {
			lines = nil
		}
	}()
	content := p.Content()
	gs := make([]glyph, 0, len(content.Text))
	for _, t := range content.Text {
		if t.X < 0 || t.Y < 0 {
			continue // drawn off the page (hidden layers)
		}
		gs = append(gs, glyph{x: t.X, y: t.Y, w: t.W, size: t.FontSize, s: t.S})
	}
	return groupLines(gs, 2)
}
