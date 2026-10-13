package pdfx_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Janindu-Pathirana/hex/internal/pdfx"
	"github.com/Janindu-Pathirana/hex/internal/pdfx/pdftest"
)

func TestPureExtractsPositionedWords(t *testing.T) {
	data := pdftest.Make([]pdftest.Text{
		{X: 50, Y: 800, S: "INCOME STATEMENT"},
		{X: 50, Y: 760, S: "Revenue"}, {X: 300, Y: 760, S: "1,234,567"}, {X: 400, Y: 760, S: "(98,765)"},
		{X: 50, Y: 740, S: "Profit for the period"}, {X: 300, Y: 740, S: "4,321"},
	}, []pdftest.Text{{X: 50, Y: 800, S: "page two"}})

	res, err := pdfx.Extract(context.Background(), pdfx.Pure{}, data, pdfx.DefaultLimits)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Pages) != 2 || res.Backend != "pure-go" {
		t.Fatalf("pages = %d, backend %s", len(res.Pages), res.Backend)
	}
	p := res.Pages[0]
	if len(p.Lines) != 3 {
		t.Fatalf("lines:\n%s", p.Text())
	}
	rev := p.Lines[1]
	if rev.Text() != "Revenue 1,234,567 (98,765)" {
		t.Fatalf("revenue line = %q", rev.Text())
	}
	if len(rev.Words) != 3 || rev.Words[1].X < 299 || rev.Words[1].X > 301 {
		t.Fatalf("words = %+v", rev.Words)
	}
	if !strings.Contains(p.Text(), "Profit for the period 4,321") {
		t.Fatalf("text:\n%s", p.Text())
	}
}

func TestScannedDetection(t *testing.T) {
	data := pdftest.Make([]pdftest.Text{{X: 50, Y: 50, S: "1"}}, nil)
	res, err := pdfx.Extract(context.Background(), pdfx.Pure{}, data, pdfx.DefaultLimits)
	if err != nil || !res.Scanned {
		t.Fatalf("scanned = %v, %v", res.Scanned, err)
	}
}

func TestLimits(t *testing.T) {
	data := pdftest.Make([]pdftest.Text{{X: 1, Y: 1, S: "x"}}, []pdftest.Text{{X: 1, Y: 1, S: "y"}})
	if _, err := pdfx.Extract(context.Background(), pdfx.Pure{}, data, pdfx.Limits{MaxBytes: 10}); !errors.Is(err, pdfx.ErrTooLarge) {
		t.Fatalf("size cap: %v", err)
	}
	if _, err := pdfx.Extract(context.Background(), pdfx.Pure{}, data, pdfx.Limits{MaxPages: 1}); !errors.Is(err, pdfx.ErrTooMany) {
		t.Fatalf("page cap: %v", err)
	}
	if _, err := pdfx.Extract(context.Background(), pdfx.Pure{}, []byte("%PDF-1.4 garbage"), pdfx.DefaultLimits); err == nil {
		t.Fatal("garbage should fail")
	}
}

type hang struct{}

func (hang) Name() string { return "hang" }
func (hang) Pages(ctx context.Context, _ []byte, _ int) ([]pdfx.Page, error) {
	select {} // simulates a parser stuck in a loop that ignores ctx
}

type boom struct{}

func (boom) Name() string                                            { return "boom" }
func (boom) Pages(context.Context, []byte, int) ([]pdfx.Page, error) { panic("bad xref") }

func TestTimeoutAndPanicAreErrors(t *testing.T) {
	if _, err := pdfx.Extract(context.Background(), hang{}, nil, pdfx.Limits{Timeout: 20 * time.Millisecond}); !errors.Is(err, pdfx.ErrTimeout) {
		t.Fatalf("timeout: %v", err)
	}
	if _, err := pdfx.Extract(context.Background(), boom{}, nil, pdfx.DefaultLimits); err == nil || !strings.Contains(err.Error(), "panic") {
		t.Fatalf("panic: %v", err)
	}
}

func TestBlobs(t *testing.T) {
	b := &pdfx.Blobs{Dir: t.TempDir()}
	sum, err := b.Put([]byte("%PDF-1.4 hello"))
	if err != nil || len(sum) != 64 || !b.Has(sum) {
		t.Fatalf("put = %q, %v", sum, err)
	}
	if again, _ := b.Put([]byte("%PDF-1.4 hello")); again != sum {
		t.Fatal("same content should give the same hash")
	}
	data, err := b.Get(sum)
	if err != nil || string(data) != "%PDF-1.4 hello" {
		t.Fatalf("get = %q, %v", data, err)
	}
	if _, err := b.Get("../../etc/passwd"); err == nil {
		t.Fatal("path traversal must be rejected")
	}
}

func TestLigaturesNormalised(t *testing.T) {
	data := pdftest.Make([]pdftest.Text{{X: 50, Y: 800, S: "Profit"}})
	res, _ := pdfx.Extract(context.Background(), pdfx.Pure{}, data, pdfx.DefaultLimits)
	if got := res.Pages[0].Lines[0].Text(); got != "Profit" {
		t.Fatalf("got %q", got)
	}
}
