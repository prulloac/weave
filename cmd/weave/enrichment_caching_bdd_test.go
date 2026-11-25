package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
	sqlitestore "github.com/prulloac/weave/storage/sqlite"
)

// Behavior-level tests for enrichment caching. See docs/bdd/enrichment-caching.md.

func cacheFixture(t *testing.T) (string, string, string) {
	t.Helper()
	root := extensionFixture(t)
	installExtension(t, root, "cache", "cache-enricher.sh")
	marker := filepath.Join(t.TempDir(), "marker")
	t.Setenv("TEST_CACHE_MARKER", marker)
	return root, marker, filepath.Join(t.TempDir(), "index.db")
}

func cacheIngest(t *testing.T, root, db string, extra ...string) (int, contractv1.IngestReport, string) {
	t.Helper()
	args := append([]string{"ingest", root, "--with-enricher", "cache", "--db", db, "--json"}, extra...)
	code, stdout, stderr := Invoke(t, args...)
	if code != 0 {
		return code, contractv1.IngestReport{}, stderr
	}
	envelope := decodeEnvelope(t, stdout)
	var report contractv1.IngestReport
	if err := json.Unmarshal(envelope.Result, &report); err != nil {
		t.Fatal(err)
	}
	return code, report, stderr
}

func cacheKeywords(t *testing.T, db, root string) map[string][]string {
	t.Helper()
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

func TestEnrichmentCacheReuse(t *testing.T) {
	root, marker, db := cacheFixture(t)
	if code, _, stderr := cacheIngest(t, root, db); code != 0 {
		t.Fatalf("first ingest failed: %s", stderr)
	}
	first := cacheKeywords(t, db, root)
	if lines := markerLines(t, marker); len(lines) != 3 {
		t.Fatalf("first invocations = %v", lines)
	}
	if code, report, stderr := cacheIngest(t, root, db); code != 0 {
		t.Fatalf("second ingest failed: %s", stderr)
	} else if report.Warnings != 0 {
		t.Fatalf("reuse added warnings: %+v", report)
	}
	if lines := markerLines(t, marker); len(lines) != 3 {
		t.Fatalf("cached documents invoked the extension: %v", lines)
	}
	second := cacheKeywords(t, db, root)
	for id, values := range first {
		if strings.Join(values, ",") != strings.Join(second[id], ",") {
			t.Fatalf("%s keywords changed: %v != %v", id, values, second[id])
		}
	}
}

func TestEnrichmentCacheInvalidation(t *testing.T) {
	root, marker, db := cacheFixture(t)
	if code, _, stderr := cacheIngest(t, root, db); code != 0 {
		t.Fatalf("first ingest failed: %s", stderr)
	}
	path := filepath.Join(root, "a.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, []byte("\nchanged body\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := cacheIngest(t, root, db); code != 0 {
		t.Fatalf("second ingest failed: %s", stderr)
	}
	if lines := markerLines(t, marker); len(lines) != 4 {
		t.Fatalf("changed document should be the only invocation: %v", lines)
	}
}

func TestEnrichmentCacheKey(t *testing.T) {
	root, marker, db := cacheFixture(t)
	t.Setenv("TEST_CACHE_KEY", "one")
	if code, _, stderr := cacheIngest(t, root, db); code != 0 {
		t.Fatalf("first ingest failed: %s", stderr)
	}
	t.Setenv("TEST_CACHE_KEY", "two")
	if code, _, stderr := cacheIngest(t, root, db); code != 0 {
		t.Fatalf("second ingest failed: %s", stderr)
	}
	if lines := markerLines(t, marker); len(lines) != 6 {
		t.Fatalf("cache_key change did not invalidate: %v", lines)
	}
}

func TestEnrichmentCacheForce(t *testing.T) {
	root, marker, db := cacheFixture(t)
	if code, _, stderr := cacheIngest(t, root, db); code != 0 {
		t.Fatalf("first ingest failed: %s", stderr)
	}
	if code, _, stderr := cacheIngest(t, root, db, "--force"); code != 0 {
		t.Fatalf("force ingest failed: %s", stderr)
	}
	if lines := markerLines(t, marker); len(lines) != 6 {
		t.Fatalf("--force did not invoke every document: %v", lines)
	}
}

func TestEnrichmentCacheSkipsFiles(t *testing.T) {
	root, marker, db := cacheFixture(t)
	t.Setenv("TEST_CACHE_FILES", "1")
	if code, _, stderr := cacheIngest(t, root, db); code != 0 {
		t.Fatalf("first ingest failed: %s", stderr)
	}
	if code, _, stderr := cacheIngest(t, root, db); code != 0 {
		t.Fatalf("second ingest failed: %s", stderr)
	}
	if lines := markerLines(t, marker); len(lines) != 6 {
		t.Fatalf("file-producing enrichment was reused: %v", lines)
	}
}

func TestEnrichmentCacheFailure(t *testing.T) {
	root, marker, db := cacheFixture(t)
	if code, _, stderr := cacheIngest(t, root, db); code != 0 {
		t.Fatalf("first ingest failed: %s", stderr)
	}
	t.Setenv("TEST_CACHE_FAIL", "1")
	path := filepath.Join(root, "a.md")
	data, _ := os.ReadFile(path)
	if err := os.WriteFile(path, append(data, []byte("\nchanged\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	code, report, stderr := cacheIngest(t, root, db)
	if code != 0 {
		t.Fatalf("failed ingest should keep core behavior: %s", stderr)
	}
	if report.Warnings == 0 {
		t.Fatal("expected a warning for the failed enrichment")
	}
	t.Setenv("TEST_CACHE_FAIL", "")
	if code, _, stderr := cacheIngest(t, root, db); code != 0 {
		t.Fatalf("third ingest failed: %s", stderr)
	}
	if lines := markerLines(t, marker); len(lines) != 5 {
		t.Fatalf("failed response was cached: %v", lines)
	}
}

func TestEnrichmentCacheForceRequiresEnricher(t *testing.T) {
	root := extensionFixture(t)
	code, _, stderr := Invoke(t, "ingest", root, "--force", "--db", filepath.Join(t.TempDir(), "index.db"), "--json")
	if code != 2 || !strings.Contains(stderr, "--force requires --with-enricher") {
		t.Fatalf("expected a usage error, got %d %q", code, stderr)
	}
}

func TestEnrichmentCacheSync(t *testing.T) {
	root, marker, db := cacheFixture(t)
	if code, _, stderr := cacheIngest(t, root, db); code != 0 {
		t.Fatalf("ingest failed: %s", stderr)
	}
	path := filepath.Join(root, "a.md")
	data, _ := os.ReadFile(path)
	if err := os.WriteFile(path, append(data, []byte("\nchanged\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := Invoke(t, "sync", root, "--with-enricher", "cache", "--db", db, "--json")
	if code != 0 {
		t.Fatalf("sync failed: %s %s", stdout, stderr)
	}
	if lines := markerLines(t, marker); len(lines) != 4 {
		t.Fatalf("sync should enrich only the changed document: %v", lines)
	}
}
