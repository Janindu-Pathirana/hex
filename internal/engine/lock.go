package engine

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// ErrLocked is returned when another hex sync holds the lock.
var ErrLocked = errors.New("another hex sync is running")

// LockFile is the lock's name inside the data directory.
const LockFile = "sync.lock"

// Lock takes the sync lock in dir. A lock left by a process that no longer
// exists is taken over. Call unlock when the run ends.
func Lock(dir string) (unlock func() error, err error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}
	path := filepath.Join(dir, LockFile)
	for attempt := 0; attempt < 2; attempt++ {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			_, werr := fmt.Fprintf(f, "%d\n", os.Getpid())
			if cerr := f.Close(); werr == nil {
				werr = cerr
			}
			if werr != nil {
				os.Remove(path)
				return nil, fmt.Errorf("write lock: %w", werr)
			}
			return func() error {
				if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
					return fmt.Errorf("remove lock: %w", err)
				}
				return nil
			}, nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return nil, fmt.Errorf("create lock: %w", err)
		}
		pid := lockPID(path)
		if pid > 0 && alive(pid) {
			return nil, fmt.Errorf("%w (pid %d; remove %s if it is stale)", ErrLocked, pid, path)
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("remove stale lock: %w", err)
		}
	}
	return nil, ErrLocked
}

func lockPID(path string) int {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	pid, _ := strconv.Atoi(strings.TrimSpace(string(b)))
	return pid
}

func alive(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = p.Signal(syscall.Signal(0))
	return err == nil || errors.Is(err, syscall.EPERM)
}
