// Package cache stores fetched data as JSON files under the user cache directory.
package cache

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/Janindu-Pathirana/hex/internal"
)

// EnvDir overrides the cache root (used by tests).
const EnvDir = "HEX_CACHE_DIR"

var now = time.Now

// Dir returns the hex cache directory: $HEX_CACHE_DIR, or os.UserCacheDir()/hex.
func Dir() (string, error) {
	if d := os.Getenv(EnvDir); d != "" {
		return d, nil
	}
	base, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("resolve cache directory: %w", err)
	}
	return filepath.Join(base, internal.CacheDir), nil
}

// Path returns the file path for cfg.
func Path(cfg internal.CacheConfig) (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, cfg.CacheFile), nil
}

// Load decodes the cached JSON for cfg into v. ok is false when the file is
// missing or older than cfg.CacheTTL.
func Load(cfg internal.CacheConfig, v any) (ok bool, err error) {
	path, err := Path(cfg)
	if err != nil {
		return false, err
	}
	info, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("stat cache %s: %w", cfg.CacheFile, err)
	}
	if now().Sub(info.ModTime()) > cfg.CacheTTL {
		return false, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("read cache %s: %w", cfg.CacheFile, err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		// A corrupt or old-format cache is a miss, not a failure.
		return false, nil
	}
	return true, nil
}

// Save writes v as JSON for cfg, atomically (temp file + rename).
func Save(cfg internal.CacheConfig, v any) error {
	path, err := Path(cfg)
	if err != nil {
		return err
	}
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("encode cache %s: %w", cfg.CacheFile, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create cache directory: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), cfg.CacheFile+".*.tmp")
	if err != nil {
		return fmt.Errorf("create cache %s: %w", cfg.CacheFile, err)
	}
	defer os.Remove(tmp.Name()) // no-op after a successful rename

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write cache %s: %w", cfg.CacheFile, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("write cache %s: %w", cfg.CacheFile, err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("replace cache %s: %w", cfg.CacheFile, err)
	}
	return nil
}

// Remove deletes the cache file (or directory) for cfg. A missing file is not
// an error.
func Remove(cfg internal.CacheConfig) error {
	path, err := Path(cfg)
	if err != nil {
		return err
	}
	if cfg.Dir {
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("remove cache %s: %w", cfg.CacheFile, err)
		}
		return nil
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remove cache %s: %w", cfg.CacheFile, err)
	}
	return nil
}

// RemoveAll deletes every cache in internal.AllCaches.
func RemoveAll() error {
	var errs []error
	for _, cfg := range internal.AllCaches {
		if err := Remove(cfg); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
