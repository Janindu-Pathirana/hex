package pdfx

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/Janindu-Pathirana/hex/internal"
	"github.com/Janindu-Pathirana/hex/internal/cache"
)

// Blobs stores PDFs on disk named by their SHA-256, so a file is downloaded
// once and identical uploads share storage.
type Blobs struct {
	Dir string
}

// OpenBlobs returns the blob store in the hex cache directory.
func OpenBlobs() (*Blobs, error) {
	path, err := cache.Path(internal.PDFCache)
	if err != nil {
		return nil, err
	}
	return &Blobs{Dir: path}, nil
}

var shaPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

func (b *Blobs) path(sum string) (string, error) {
	if !shaPattern.MatchString(sum) {
		return "", fmt.Errorf("invalid pdf hash %q", sum)
	}
	return filepath.Join(b.Dir, sum+".pdf"), nil
}

// Put stores data and returns its SHA-256 (hex). Writing is atomic.
func (b *Blobs) Put(data []byte) (string, error) {
	h := sha256.Sum256(data)
	sum := hex.EncodeToString(h[:])
	path, _ := b.path(sum)
	if _, err := os.Stat(path); err == nil {
		return sum, nil
	}
	if err := os.MkdirAll(b.Dir, 0o755); err != nil {
		return "", fmt.Errorf("create pdf cache: %w", err)
	}
	tmp, err := os.CreateTemp(b.Dir, "*.tmp")
	if err != nil {
		return "", fmt.Errorf("create pdf file: %w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return "", fmt.Errorf("write pdf file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("write pdf file: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return "", fmt.Errorf("store pdf file: %w", err)
	}
	return sum, nil
}

// Get returns the stored PDF with the given hash.
func (b *Blobs) Get(sum string) ([]byte, error) {
	path, err := b.path(sum)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read pdf %s: %w", sum[:12], err)
	}
	return data, nil
}

// Has reports whether a PDF with the given hash is stored.
func (b *Blobs) Has(sum string) bool {
	path, err := b.path(sum)
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}
