package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	contractv1 "github.com/prulloac/weave/contract/v1"
)

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(data)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func waitForOutput(t *testing.T, buffer *syncBuffer, substring string) {
	t.Helper()
	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(buffer.String(), substring) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("output missing %q:\n%s", substring, buffer.String())
}

func TestSyncWatchRequiresDB(t *testing.T) {
	root := copyStorageBundle(t, storageFixture("bundle-a"))
	code, _, _ := invokeRaw(t, "sync", root, "--watch", "--json")
	if code != 2 {
		t.Fatalf("expected exit 2 without --db, got %d", code)
	}
	code, _, _ = invokeRaw(t, "sync", root, "--watch", "--no-db", "--json")
	if code != 2 {
		t.Fatalf("expected exit 2 with --no-db, got %d", code)
	}
}

func TestSyncWatchRejectsUnsupportedFlags(t *testing.T) {
	root := copyStorageBundle(t, storageFixture("bundle-a"))
	db := filepath.Join(t.TempDir(), "index.db")
	code, _, _ := invokeRaw(t, "sync", root, "--watch", "--with-enricher", "echo", "--db", db)
	if code != 2 {
		t.Fatalf("expected exit 2 for enricher, got %d", code)
	}
	code, _, _ = invokeRaw(t, "sync", root, "--events", "--json")
	if code != 2 {
		t.Fatalf("expected exit 2 for --events without --watch, got %d", code)
	}
}

func TestSyncWatchRequiresIndexedBundle(t *testing.T) {
	rootA := copyStorageBundle(t, storageFixture("bundle-a"))
	rootB := copyStorageBundle(t, storageFixture("bundle-b"))
	db := filepath.Join(t.TempDir(), "index.db")
	code, _, stderr := invokeRaw(t, "ingest", rootA, "--db", db, "--strict", "--json")
	if code != 0 {
		t.Fatalf("ingest exit=%d stderr=%q", code, stderr)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var stdout, stderrBuffer syncBuffer
	code = runSyncWatch(ctx, syncWatchOptions{
		Root: rootB, DB: db, JSON: true,
	}, &stdout, &stderrBuffer)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d stderr=%q", code, stderrBuffer.String())
	}
	if !strings.Contains(stdout.String(), "BUNDLE_NOT_INDEXED") {
		t.Fatalf("expected BUNDLE_NOT_INDEXED, got %s", stdout.String())
	}
}

func TestSyncWatchInitialAndUpdates(t *testing.T) {
	root := copyStorageBundle(t, storageFixture("bundle-a"))
	db := filepath.Join(t.TempDir(), "index.db")
	code, _, stderr := invokeRaw(t, "ingest", root, "--db", db, "--strict", "--json")
	if code != 0 {
		t.Fatalf("ingest exit=%d stderr=%q", code, stderr)
	}

	ctx, cancel := context.WithCancel(context.Background())
	var stdout, stderrBuffer syncBuffer
	done := make(chan int, 1)
	go func() {
		done <- runSyncWatch(ctx, syncWatchOptions{Root: root, DB: db}, &stdout, &stderrBuffer)
	}()

	waitForOutput(t, &stdout, "documents")

	content := "---\ntype: Concept\ntitle: \"B Updated\"\nkeywords:\n  - \"watched\"\n---\n\n# B Updated\n\nWatched content.\n"
	if err := os.WriteFile(filepath.Join(root, "b.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	waitForOutput(t, &stdout, "updated=1")
	cancel()
	if code := <-done; code != 0 {
		t.Fatalf("watch exit=%d stderr=%q", code, stderrBuffer.String())
	}
}

func TestSyncWatchJSONEvents(t *testing.T) {
	root := copyStorageBundle(t, storageFixture("bundle-a"))
	db := filepath.Join(t.TempDir(), "index.db")
	code, _, stderr := invokeRaw(t, "ingest", root, "--db", db, "--strict", "--json")
	if code != 0 {
		t.Fatalf("ingest exit=%d stderr=%q", code, stderr)
	}

	ctx, cancel := context.WithCancel(context.Background())
	var stdout, stderrBuffer syncBuffer
	done := make(chan int, 1)
	go func() {
		done <- runSyncWatch(ctx, syncWatchOptions{Root: root, DB: db, JSON: true, Events: true}, &stdout, &stderrBuffer)
	}()

	waitForOutput(t, &stdout, `"ok":true`)
	content := "---\ntype: Concept\ntitle: \"B Event\"\nkeywords:\n  - \"event\"\n---\n\n# B Event\n\nEvent content.\n"
	if err := os.WriteFile(filepath.Join(root, "b.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	waitForOutput(t, &stdout, `"event":"sync"`)
	cancel()
	if code := <-done; code != 0 {
		t.Fatalf("watch exit=%d stderr=%q", code, stderrBuffer.String())
	}
	if !strings.Contains(stdout.String(), `"updated":1`) {
		t.Fatalf("event missing update count:\n%s", stdout.String())
	}

	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected an initial envelope and one event, got:\n%s", stdout.String())
	}
	validateCLIJSON(t, lines[0], "sync.schema.json")
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var doc any
		if err := json.Unmarshal([]byte(line), &doc); err != nil {
			t.Fatalf("decode watch event: %v\n%s", err, line)
		}
		validateTestSchema(t, "watch-event.schema.json", doc)
		var event contractv1.WatchEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatal(err)
		}
		if event.Event != "sync" {
			t.Fatalf("unexpected watch event: %+v", event)
		}
	}
}
