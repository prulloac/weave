package lockfile

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestAcquireExcludesSecondWriter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "index.db.lock")
	first, err := Acquire(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Acquire(path); !errors.Is(err, ErrLocked) {
		t.Fatalf("second acquire err = %v, want ErrLocked", err)
	}
	if err := first.Release(); err != nil {
		t.Fatal(err)
	}
	second, err := Acquire(path)
	if err != nil {
		t.Fatalf("acquire after release: %v", err)
	}
	_ = second.Release()
}

func TestStaleLockIsReplaced(t *testing.T) {
	path := filepath.Join(t.TempDir(), "index.db.lock")
	if err := os.WriteFile(path, []byte("999999\n1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	lock, err := Acquire(path)
	if err != nil {
		t.Fatalf("stale lock not replaced: %v", err)
	}
	_ = lock.Release()
}
