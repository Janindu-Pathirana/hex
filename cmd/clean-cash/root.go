package cleancash

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Janindu-Pathirana/hex/internal"
)

func CleanCacheFile(cashInfo internal.CacheConfig) error {

	path, err := os.UserCacheDir()
	if err != nil {
		return fmt.Errorf("resolve cache directory: %w", err)
	}
	path = filepath.Join(path, internal.CacheDir, cashInfo.CacheFile)

	if err := os.Remove(path); err != nil {
		return fmt.Errorf("remove cache file: %w", err)
	}

	return nil
}

func CleanAllCash() error {
	err := CleanCacheFile(internal.AvailableCompaniesCache)
	if err != nil {
		return fmt.Errorf("clean available companies cache: %w", err)
	}

	return nil
}
