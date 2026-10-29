package watch_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine"
	"github.com/prulloac/weave/engine/okf"
	"github.com/prulloac/weave/engine/operations"
	"github.com/prulloac/weave/mcp"
	sqlitestore "github.com/prulloac/weave/storage/sqlite"
	"github.com/prulloac/weave/watch"
)

func watchFixture() string {
	return filepath.Join("..", "cmd/weave/testdata/storage/bundle-a")
}

func copyBundle(t *testing.T) string {
	t.Helper()
	destination := t.TempDir()
	if err := os.CopyFS(destination, os.DirFS(watchFixture())); err != nil {
		t.Fatal(err)
	}
	return destination
}

func openStore(t *testing.T) (*sqlitestore.Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "index.db")
	store, err := sqlitestore.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	return store, path
}

func ingestBundle(t *testing.T, store *sqlitestore.Store, root string) {
	t.Helper()
	bundle, err := okf.ParseBundle(context.Background(), root, okf.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Ingest(context.Background(), bundle, sqlitestore.IngestOptions{Mode: "strict"}); err != nil {
		t.Fatal(err)
	}
}

func TestWatchInitialSync(t *testing.T) {
	root := copyBundle(t)
	store, _ := openStore(t)
	defer store.Close()
	ingestBundle(t, store, root)

	synced := make(chan contractv1.SyncReport, 1)
	watcher, err := watch.New(watch.Options{
		Root:     root,
		Store:    store,
		Interval: 20 * time.Millisecond,
		OnSync: func(report contractv1.SyncReport) {
			select {
			case synced <- report:
			default:
			}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- watcher.Run(ctx) }()

	select {
	case report := <-synced:
		if report.Documents != 3 {
			t.Fatalf("unexpected initial sync: %+v", report)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("initial sync did not run")
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("watcher shutdown: %v", err)
	}
}

func TestWatchUpdatesAndDeletes(t *testing.T) {
	root := copyBundle(t)
	store, _ := openStore(t)
	defer store.Close()
	ingestBundle(t, store, root)

	var mu sync.Mutex
	syncs := 0
	watcher, err := watch.New(watch.Options{
		Root:     root,
		Store:    store,
		Interval: 20 * time.Millisecond,
		OnSync: func(report contractv1.SyncReport) {
			mu.Lock()
			syncs++
			mu.Unlock()
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go watcher.Run(ctx)
	waitFor(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return syncs >= 1
	})

	writeFile(t, filepath.Join(root, "b.md"), `---
type: Concept
title: "B Updated"
keywords:
  - "watched"
---

# B Updated

Watched content.
`)
	waitFor(t, func() bool {
		bundle, err := store.LoadBundle(context.Background(), root)
		return err == nil && strings.Contains(conceptBody(bundle, "b"), "Watched content")
	})

	if err := os.Remove(filepath.Join(root, "c.md")); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool {
		bundle, err := store.LoadBundle(context.Background(), root)
		return err == nil && !hasConcept(bundle, "c")
	})
}

func TestWatchCoalescesBurstsAndIgnoresGit(t *testing.T) {
	root := copyBundle(t)
	store, _ := openStore(t)
	defer store.Close()
	ingestBundle(t, store, root)

	var mu sync.Mutex
	syncs := 0
	watcher, err := watch.New(watch.Options{
		Root:     root,
		Store:    store,
		Interval: 20 * time.Millisecond,
		OnSync: func(report contractv1.SyncReport) {
			mu.Lock()
			syncs++
			mu.Unlock()
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go watcher.Run(ctx)
	waitFor(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return syncs >= 1
	})

	for _, name := range []string{"b.md", "c.md"} {
		writeFile(t, filepath.Join(root, name), `---
type: Concept
title: "Burst"
keywords:
  - "burst"
---

# Burst

Burst edit.
`)
	}
	waitFor(t, func() bool {
		bundle, err := store.LoadBundle(context.Background(), root)
		return err == nil && strings.Contains(conceptBody(bundle, "b"), "Burst edit")
	})

	mu.Lock()
	before := syncs
	mu.Unlock()
	writeFile(t, filepath.Join(root, ".git", "ignored.md"), "# Ignored\n")
	time.Sleep(100 * time.Millisecond)
	mu.Lock()
	after := syncs
	mu.Unlock()
	if after != before {
		t.Fatalf("ignored .git file triggered sync: %d -> %d", before, after)
	}
}

func TestWatchMCPReload(t *testing.T) {
	root := copyBundle(t)
	store, dbPath := openStore(t)
	defer store.Close()
	ingestBundle(t, store, root)

	watcher, err := watch.New(watch.Options{
		Root:     root,
		Store:    store,
		Interval: 20 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go watcher.Run(ctx)

	executor := operations.New("test", engine.Options{})
	server, err := mcp.New(executor, mcp.Options{Bundle: root, DB: dbPath})
	if err != nil {
		t.Fatal(err)
	}

	writeFile(t, filepath.Join(root, "b.md"), `---
type: Concept
title: "B Watched"
keywords:
  - "watched"
---

# B Watched

Watch reload content.
`)
	waitFor(t, func() bool {
		result, err := server.Call(context.Background(), "weave_search", map[string]any{
			"query": "watched", "mode": "keyword",
		})
		if err != nil {
			return false
		}
		response, ok := result.(contractv1.SearchResponse)
		return ok && len(response.Keyword) > 0
	})
}

func TestWatchRequiresIndexedBundle(t *testing.T) {
	root := copyBundle(t)
	store, _ := openStore(t)
	defer store.Close()

	watcher, err := watch.New(watch.Options{Root: root, Store: store, Interval: 20 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	err = watcher.Run(context.Background())
	if !errors.Is(err, sqlitestore.ErrBundleNotIndexed) {
		t.Fatalf("expected ErrBundleNotIndexed, got %v", err)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func waitFor(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("condition not met before timeout")
}

func conceptBody(bundle *contractv1.Bundle, id string) string {
	for _, concept := range bundle.Concepts {
		if concept.ID == id {
			return concept.Body
		}
	}
	return ""
}

func hasConcept(bundle *contractv1.Bundle, id string) bool {
	for _, concept := range bundle.Concepts {
		if concept.ID == id {
			return true
		}
	}
	return false
}
