package safefile

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWithLockRemovesStaleLock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "projects.json")
	lockPath := path + ".lock"
	if err := os.WriteFile(lockPath, []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}
	staleTime := time.Now().Add(-staleLockAfter - time.Second)
	if err := os.Chtimes(lockPath, staleTime, staleTime); err != nil {
		t.Fatal(err)
	}

	called := false
	if err := WithLock(path, func() error {
		called = true
		return nil
	}); err != nil {
		t.Fatalf("WithLock() error = %v", err)
	}
	if !called {
		t.Fatal("WithLock callback was not called")
	}
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Fatalf("lock file should be removed after callback, stat err = %v", err)
	}
}
