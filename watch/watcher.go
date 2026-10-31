// Package watch keeps a persistent bundle current with polling snapshots.
package watch

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine/privacy"
	sqlitestore "github.com/prulloac/weave/storage/sqlite"
)

const defaultInterval = 500 * time.Millisecond

// Options configures a polling watcher.
type Options struct {
	Root     string
	Store    *sqlitestore.Store
	Mode     string
	Codec    string
	Provider privacy.Provider
	Interval time.Duration
	OnSync   func(contractv1.SyncReport)
	OnError  func(error)
}

// Watcher polls a bundle and syncs changed snapshots.
type Watcher struct {
	root     string
	store    *sqlitestore.Store
	mode     string
	codec    string
	provider privacy.Provider
	interval time.Duration
	onSync   func(contractv1.SyncReport)
	onError  func(error)
}

// New creates a polling watcher.
func New(options Options) (*Watcher, error) {
	if strings.TrimSpace(options.Root) == "" {
		return nil, fmt.Errorf("root is required")
	}
	if options.Store == nil {
		return nil, fmt.Errorf("store is required")
	}
	if options.Interval <= 0 {
		options.Interval = defaultInterval
	}
	return &Watcher{
		root:     options.Root,
		store:    options.Store,
		mode:     options.Mode,
		codec:    options.Codec,
		provider: options.Provider,
		interval: options.Interval,
		onSync:   options.OnSync,
		onError:  options.OnError,
	}, nil
}

// Run performs the initial sync and then polls until the context is cancelled.
func (w *Watcher) Run(ctx context.Context) error {
	if err := w.sync(ctx); err != nil {
		return err
	}
	snapshot, err := snapshotBundle(w.root)
	if err != nil {
		return err
	}

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			next, err := snapshotBundle(w.root)
			if err != nil {
				w.reportError(err)
				continue
			}
			if equalSnapshots(snapshot, next) {
				continue
			}
			if err := w.sync(ctx); err != nil {
				w.reportError(err)
				continue
			}
			snapshot = next
		}
	}
}

func (w *Watcher) sync(ctx context.Context) error {
	report, err := w.store.Sync(ctx, w.root, sqlitestore.SyncOptions{Mode: w.mode, Codec: w.codec, Provider: w.provider})
	if err != nil {
		return err
	}
	if w.onSync != nil {
		w.onSync(report)
	}
	return nil
}

func (w *Watcher) reportError(err error) {
	if w.onError != nil {
		w.onError(err)
	}
}

func snapshotBundle(root string) (map[string]string, error) {
	snapshot := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if path != root && ignoredDir(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if ignoredFile(entry.Name()) {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		snapshot[filepath.ToSlash(relative)] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		return nil, err
	}
	return snapshot, nil
}

func ignoredDir(name string) bool {
	switch name {
	case ".git", "node_modules", ".weave":
		return true
	default:
		return strings.HasPrefix(name, ".") && name != "."
	}
}

func ignoredFile(name string) bool {
	if !strings.HasSuffix(strings.ToLower(name), ".md") {
		return true
	}
	if strings.HasPrefix(name, ".") {
		return true
	}
	return strings.HasSuffix(name, "~") ||
		strings.HasSuffix(name, ".tmp") ||
		strings.HasSuffix(name, ".swp")
}

func equalSnapshots(left, right map[string]string) bool {
	if len(left) != len(right) {
		return false
	}
	for path, hash := range left {
		if right[path] != hash {
			return false
		}
	}
	return true
}
