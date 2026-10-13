// Package pdftest builds small text PDFs for tests.
package pdftest

import (
	"bytes"
	"fmt"
	"strings"
)

// Text is a string drawn at (X, Y) in points from the bottom-left corner.
type Text struct {
	X, Y float64
	S    string
}

// Make returns a valid PDF with one page per element of pages, drawing each
// Text in 9pt Helvetica on an A4 page.
func Make(pages ...[]Text) []byte {
	var objs []string
	n := len(pages)
	// 1 catalog, 2 pages, 3 font, then per page: page object, content stream.
	kids := make([]string, n)
	for i := range pages {
		kids[i] = fmt.Sprintf("%d 0 R", 4+2*i)
	}
	objs = append(objs,
		"<< /Type /Catalog /Pages 2 0 R >>",
		fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(kids, " "), n),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>",
	)
	for i, texts := range pages {
		var c strings.Builder
		for _, t := range texts {
			s := strings.NewReplacer(`\`, `\\`, "(", `\(`, ")", `\)`).Replace(t.S)
			fmt.Fprintf(&c, "BT /F1 9 Tf %.2f %.2f Td (%s) Tj ET\n", t.X, t.Y, s)
		}
		objs = append(objs,
			fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F1 3 0 R >> >> /Contents %d 0 R >>", 5+2*i),
			fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", c.Len(), c.String()),
		)
	}

	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objs))
	for i, o := range objs {
		offsets[i] = b.Len()
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, o)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(objs)+1)
	for _, off := range offsets {
		fmt.Fprintf(&b, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objs)+1, xref)
	return b.Bytes()
}
