// Package lockfile provides a portable, advisory single-writer lock.
package lockfile

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// ErrLocked means another live writer holds the lock.
var ErrLocked = errors.New("index is locked by another writer")

// Lock is one acquired advisory lock.
type Lock struct {
	path     string
	released bool
}

// Acquire takes the lock at path, replacing a stale lock once.
func Acquire(path string) (*Lock, error) {
	for attempt := 0; attempt < 2; attempt++ {
		file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			_, _ = fmt.Fprintf(file, "%d\n%d\n", os.Getpid(), time.Now().Unix())
			_ = file.Close()
			return &Lock{path: path}, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, err
		}
		if attempt == 0 && stale(path) {
			_ = os.Remove(path)
			continue
		}
		return nil, ErrLocked
	}
	return nil, ErrLocked
}

// Release removes the lock file.
func (l *Lock) Release() error {
	if l == nil || l.released {
		return nil
	}
	l.released = true
	return os.Remove(l.path)
}

func stale(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	pid, err := strconv.Atoi(strings.TrimSpace(lines[0]))
	if err != nil || pid <= 0 {
		return true
	}
	return !processAlive(pid)
}
