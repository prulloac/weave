package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
	sqlitestore "github.com/prulloac/weave/storage/sqlite"
)

// Behavior-level tests for --extension-parallelism. See docs/bdd/extension-parallelism.md.

func parallelFixture(t *testing.T, count int) string {
	t.Helper()
	root := t.TempDir()
	for index := 0; index < count; index++ {
		content := "---\ntype: Concept\ntitle: Doc\nkeywords:\n  - alpha\n---\n\n# Doc\n\nalpha\n"
		name := filepath.Join(root, fmt.Sprintf("doc-%02d.md", index))
		if err := os.WriteFile(name, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestParallelismDefaultSequential(t *testing.T) {
	root := parallelFixture(t, 3)
	installExtension(t, root, "batch", "batch-enricher.sh")
	marker := filepath.Join(t.TempDir(), "marker")
	t.Setenv("TEST_BATCH_MARKER", marker)
	if code, _, stderr := batchIngest(t, root, "batch"); code != 0 {
		t.Fatalf("ingest failed: %s", stderr)
	}
	if got := markerLines(t, marker); len(got) != 1 || got[0] != "3" {
		t.Fatalf("batches = %v, want [3]", got)
	}
}

func TestParallelismBatch(t *testing.T) {
	root := parallelFixture(t, 129)
	installExtension(t, root, "batch", "batch-enricher.sh")
	marker := filepath.Join(t.TempDir(), "marker")
	t.Setenv("TEST_BATCH_MARKER", marker)
	if code, _, stderr := batchIngest(t, root, "batch", "--extension-parallelism", "8"); code != 0 {
		t.Fatalf("ingest failed: %s", stderr)
	}
	lines := markerLines(t, marker)
	sort.Strings(lines)
	if len(lines) != 2 || lines[0] != "1" || lines[1] != "128" {
		t.Fatalf("batches = %v, want [1 128]", lines)
	}
}

func TestParallelismPerDocument(t *testing.T) {
	root := parallelFixture(t, 8)
	installExtension(t, root, "single", "batch-single.sh")
	marker := filepath.Join(t.TempDir(), "marker")
	t.Setenv("TEST_BATCH_MARKER", marker)
	if code, _, stderr := batchIngest(t, root, "single", "--extension-parallelism", "4"); code != 0 {
		t.Fatalf("ingest failed: %s", stderr)
	}
	if got := markerLines(t, marker); len(got) != 8 {
		t.Fatalf("invocations = %v, want eight", got)
	}
}

func TestParallelismMatchesSequential(t *testing.T) {
	run := func(parallelism string) map[string][]string {
		root := parallelFixture(t, 8)
		installExtension(t, root, "batch", "batch-enricher.sh")
		db := filepath.Join(t.TempDir(), "index.db")
		args := []string{"ingest", root, "--with-enricher", "batch", "--db", db, "--json"}
		if parallelism != "" {
			args = append(args, "--extension-parallelism", parallelism)
		}
		if code, stdout, stderr := Invoke(t, args...); code != 0 {
			t.Fatalf("ingest failed: %s %s", stdout, stderr)
		}
		store, err := sqlitestore.OpenExisting(db)
		if err != nil {
			t.Fatal(err)
		}
		defer store.Close()
		bundle, err := store.LoadBundle(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		keywords := map[string][]string{}
		for _, concept := range bundle.Concepts {
			keywords[concept.ID] = concept.Keywords
		}
		return keywords
	}
	sequential := run("")
	parallel := run("8")
	if len(sequential) != len(parallel) {
		t.Fatalf("bundle sizes differ: %d != %d", len(sequential), len(parallel))
	}
	for id, values := range sequential {
		if strings.Join(values, ",") != strings.Join(parallel[id], ",") {
			t.Fatalf("%s keywords differ: %v != %v", id, values, parallel[id])
		}
	}
}

func TestParallelismOrderedWarnings(t *testing.T) {
	root := parallelFixture(t, 3)
	installExtension(t, root, "warn", "parallel-warn.sh")
	code, _, stderr := Invoke(t, "ingest", root, "--with-enricher", "warn",
		"--extension-parallelism", "3", "--db", filepath.Join(t.TempDir(), "index.db"), "--json")
	if code != 0 {
		t.Fatalf("ingest failed: %s", stderr)
	}
	first := strings.Index(stderr, "warn-doc-00")
	second := strings.Index(stderr, "warn-doc-01")
	third := strings.Index(stderr, "warn-doc-02")
	if first < 0 || second < 0 || third < 0 || !(first < second && second < third) {
		t.Fatalf("warnings are not in document order:\n%s", stderr)
	}
}

func TestParallelismFailureIsolation(t *testing.T) {
	root := parallelFixture(t, 3)
	installExtension(t, root, "batch", "batch-enricher.sh")
	t.Setenv("TEST_BATCH_MODE", "item-error")
	db := filepath.Join(t.TempDir(), "index.db")
	code, stdout, stderr := Invoke(t, "ingest", root, "--with-enricher", "batch",
		"--extension-parallelism", "8", "--db", db, "--json")
	if code != 0 {
		t.Fatalf("ingest failed: %s", stderr)
	}
	envelope := decodeEnvelope(t, stdout)
	var report contractv1.IngestReport
	if err := json.Unmarshal(envelope.Result, &report); err != nil {
		t.Fatal(err)
	}
	if report.Warnings != 1 {
		t.Fatalf("warnings = %d, want 1", report.Warnings)
	}
}

func TestParallelismOverlap(t *testing.T) {
	root := parallelFixture(t, 8)
	installExtension(t, root, "overlap", "parallel-overlap.sh")
	active := filepath.Join(t.TempDir(), "active")
	maxFile := filepath.Join(t.TempDir(), "max")
	t.Setenv("TEST_ACTIVE", active)
	t.Setenv("TEST_MAX", maxFile)

	if code, _, stderr := Invoke(t, "ingest", root, "--with-enricher", "overlap",
		"--extension-parallelism", "4", "--db", filepath.Join(t.TempDir(), "index.db"), "--json"); code != 0 {
		t.Fatalf("ingest failed: %s", stderr)
	}
	data, err := os.ReadFile(maxFile)
	if err != nil {
		t.Fatal(err)
	}
	maximum := 0
	for _, value := range strings.Fields(string(data)) {
		count, err := strconv.Atoi(value)
		if err != nil {
			t.Fatal(err)
		}
		if count > maximum {
			maximum = count
		}
	}
	if maximum < 2 {
		t.Fatalf("no overlapping invocations, max concurrency = %d", maximum)
	}
}

func TestParallelismBounds(t *testing.T) {
	root := parallelFixture(t, 1)
	for _, value := range []string{"0", "9", "many"} {
		code, _, stderr := Invoke(t, "ingest", root, "--with-enricher", "single",
			"--extension-parallelism", value, "--db", filepath.Join(t.TempDir(), "index.db"), "--json")
		if code != 2 || !strings.Contains(stderr, "extension-parallelism") {
			t.Fatalf("value %q: exit=%d stderr=%q", value, code, stderr)
		}
	}
}
