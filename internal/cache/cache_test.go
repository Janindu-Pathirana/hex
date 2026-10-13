package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Janindu-Pathirana/hex/internal"
)

var testCfg = internal.CacheConfig{CacheFile: "test.json", CacheTTL: time.Hour}

func TestSaveLoad(t *testing.T) {
	t.Setenv(EnvDir, t.TempDir())

	var got []string
	if ok, err := Load(testCfg, &got); ok || err != nil {
		t.Fatalf("empty cache: ok=%v err=%v", ok, err)
	}
	if err := Save(testCfg, []string{"a", "b"}); err != nil {
		t.Fatal(err)
	}
	if ok, err := Load(testCfg, &got); !ok || err != nil || len(got) != 2 {
		t.Fatalf("after save: ok=%v err=%v got=%v", ok, err, got)
	}
}

func TestLoadExpired(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvDir, dir)
	if err := Save(testCfg, "x"); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(filepath.Join(dir, testCfg.CacheFile), old, old); err != nil {
		t.Fatal(err)
	}
	var v string
	if ok, err := Load(testCfg, &v); ok || err != nil {
		t.Fatalf("expired cache: ok=%v err=%v", ok, err)
	}
}

func TestLoadCorruptIsMiss(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvDir, dir)
	if err := os.WriteFile(filepath.Join(dir, testCfg.CacheFile), []byte("JKH,N0000\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var v []string
	if ok, err := Load(testCfg, &v); ok || err != nil {
		t.Fatalf("corrupt cache: ok=%v err=%v", ok, err)
	}
}

func TestRemove(t *testing.T) {
	t.Setenv(EnvDir, t.TempDir())
	if err := Remove(testCfg); err != nil {
		t.Fatalf("remove missing: %v", err)
	}
	if err := Save(testCfg, 1); err != nil {
		t.Fatal(err)
	}
	if err := Remove(testCfg); err != nil {
		t.Fatal(err)
	}
	path, _ := Path(testCfg)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("file still exists: %v", err)
	}
	if err := RemoveAll(); err != nil {
		t.Fatalf("remove all on empty dir: %v", err)
	}
}
