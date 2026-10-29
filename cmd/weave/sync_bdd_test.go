package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	contractv1 "github.com/prulloac/weave/contract/v1"
)

// Behavior tests for incremental sync. Watch mode and extension enrichment are
// covered by later suites.

type bddSyncReport struct {
	DB        string `json:"db"`
	Bundle    string `json:"bundle"`
	Mode      string `json:"mode"`
	Documents int    `json:"documents"`
	Added     int    `json:"added"`
	Updated   int    `json:"updated"`
	Removed   int    `json:"removed"`
	Skipped   int    `json:"skipped"`
	Warnings  int    `json:"warnings"`
	Errors    int    `json:"errors"`
}

func runSyncJSON(t *testing.T, args ...string) (int, testEnvelope, bddSyncReport) {
	t.Helper()

	fullArgs := append([]string{"sync"}, args...)
	code, stdout, stderr := invokePersistent(t, fullArgs...)
	if stderr != "" {
		t.Fatalf("expected empty stderr in JSON mode, got %q", stderr)
	}
	envelope := decodeEnvelope(t, stdout)
	if !envelope.OK {
		t.Fatalf("expected success envelope, got failure: %+v", envelope.Error)
	}
	var report bddSyncReport
	if err := json.Unmarshal(envelope.Result, &report); err != nil {
		t.Fatalf("decode sync report: %v\n%s", err, envelope.Result)
	}
	return code, envelope, report
}

func TestIncrementalSyncBehavior(t *testing.T) {
	t.Run("no changes reports every document as skipped", func(t *testing.T) {
		storageHome(t)
		bundle := copyStorageBundle(t, storageFixture("bundle-a"))
		runStorageJSON(t, "ingest", bundle, "--strict", "--json")

		code, _, report := runSyncJSON(t, bundle, "--strict", "--json")
		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
		if report.Documents != 3 || report.Added != 0 || report.Updated != 0 ||
			report.Removed != 0 || report.Skipped != 3 {
			t.Fatalf("unexpected report: %+v", report)
		}
	})

	t.Run("touching a file without changing content skips it", func(t *testing.T) {
		storageHome(t)
		bundle := copyStorageBundle(t, storageFixture("bundle-a"))
		runStorageJSON(t, "ingest", bundle, "--strict", "--json")

		later := time.Now().Add(time.Hour)
		if err := os.Chtimes(filepath.Join(bundle, "b.md"), later, later); err != nil {
			t.Fatal(err)
		}
		_, _, report := runSyncJSON(t, bundle, "--strict", "--json")
		if report.Updated != 0 || report.Skipped != 3 {
			t.Fatalf("unexpected report: %+v", report)
		}
	})

	t.Run("changed content updates exactly one document", func(t *testing.T) {
		storageHome(t)
		bundle := copyStorageBundle(t, storageFixture("bundle-a"))
		runStorageJSON(t, "ingest", bundle, "--strict", "--json")

		writeSyncFile(t, filepath.Join(bundle, "b.md"), `---
type: Concept
title: "B Updated"
keywords:
  - "beta2"
---

# B Updated

Updated content for sync.
`)
		_, _, report := runSyncJSON(t, bundle, "--strict", "--json")
		if report.Updated != 1 || report.Skipped != 2 || report.Documents != 3 {
			t.Fatalf("unexpected report: %+v", report)
		}

		parsed := runParseBundle(t, bundle, "--json")
		concept := findConcept(t, parsed, "b")
		if concept.Title == nil || *concept.Title != "B Updated" ||
			!strings.Contains(concept.Body, "Updated content for sync.") {
			t.Fatalf("update was not persisted: %+v", concept)
		}
		search := permissiveSearch(t, bundle, "beta2", "--mode", "keyword")
		if !hasKeywordID(search, "b") {
			t.Fatalf("updated keyword missing from search: %+v", search)
		}
	})

	t.Run("a new file is added", func(t *testing.T) {
		storageHome(t)
		bundle := copyStorageBundle(t, storageFixture("bundle-a"))
		runStorageJSON(t, "ingest", bundle, "--strict", "--json")

		writeSyncFile(t, filepath.Join(bundle, "d.md"), `---
type: Concept
title: "D"
keywords:
  - "delta"
---

# D

Added during sync.
`)
		_, _, report := runSyncJSON(t, bundle, "--strict", "--json")
		if report.Added != 1 || report.Skipped != 3 || report.Documents != 4 {
			t.Fatalf("unexpected report: %+v", report)
		}
		if got := len(runParseBundle(t, bundle, "--json").Concepts); got != 4 {
			t.Fatalf("concepts=%d, want 4", got)
		}
	})

	t.Run("a deleted file is removed", func(t *testing.T) {
		storageHome(t)
		bundle := copyStorageBundle(t, storageFixture("bundle-a"))
		runStorageJSON(t, "ingest", bundle, "--strict", "--json")

		if err := os.Remove(filepath.Join(bundle, "c.md")); err != nil {
			t.Fatal(err)
		}
		_, _, report := runSyncJSON(t, bundle, "--strict", "--json")
		if report.Removed != 1 || report.Skipped != 2 || report.Documents != 2 {
			t.Fatalf("unexpected report: %+v", report)
		}
		if got := len(runParseBundle(t, bundle, "--json").Concepts); got != 2 {
			t.Fatalf("concepts=%d, want 2", got)
		}
	})

	t.Run("a rename is an addition plus a removal", func(t *testing.T) {
		storageHome(t)
		bundle := copyStorageBundle(t, storageFixture("bundle-a"))
		runStorageJSON(t, "ingest", bundle, "--strict", "--json")

		if err := os.Rename(filepath.Join(bundle, "b.md"), filepath.Join(bundle, "d.md")); err != nil {
			t.Fatal(err)
		}
		_, _, report := runSyncJSON(t, bundle, "--strict", "--json")
		if report.Added != 1 || report.Removed != 1 || report.Skipped != 2 {
			t.Fatalf("unexpected report: %+v", report)
		}
		parsed := runParseBundle(t, bundle, "--json")
		if !hasConceptID(parsed, "d") || hasConceptID(parsed, "b") {
			t.Fatalf("rename was not reflected: %+v", parsed.Concepts)
		}
	})

	t.Run("no-op sync leaves source files untouched", func(t *testing.T) {
		storageHome(t)
		bundle := copyStorageBundle(t, storageFixture("bundle-a"))
		runStorageJSON(t, "ingest", bundle, "--strict", "--json")
		before := snapshotStorageBundle(t, bundle)

		runSyncJSON(t, bundle, "--strict", "--json")

		after := snapshotStorageBundle(t, bundle)
		if !reflect.DeepEqual(before, after) {
			t.Fatalf("sync modified source files:\nbefore: %v\nafter:  %v", before, after)
		}
	})

	t.Run("deleting a link target leaves the source link dangling", func(t *testing.T) {
		storageHome(t)
		bundle := copyStorageBundle(t, storageFixture("bundle-a"))
		runStorageJSON(t, "ingest", bundle, "--strict", "--json")

		if err := os.Remove(filepath.Join(bundle, "c.md")); err != nil {
			t.Fatal(err)
		}
		runSyncJSON(t, bundle, "--strict", "--json")

		parsed := runParseBundle(t, bundle, "--json")
		if hasConceptID(parsed, "c") || !hasConceptID(parsed, "b") {
			t.Fatalf("unexpected concepts after delete: %+v", parsed.Concepts)
		}

		code, report := runAuditFor(t, bundle, "--json")
		if code != 1 {
			t.Fatalf("expected audit exit 1, got %d", code)
		}
		if !hasAuditCode(report, "BROKEN_LINK", "error") {
			t.Fatalf("expected BROKEN_LINK, got %+v", report.Findings)
		}
	})

	t.Run("sync requires an indexed bundle", func(t *testing.T) {
		storageHome(t)
		runStorageJSON(t, "ingest", storageFixture("bundle-a"), "--strict", "--json")

		code, stdout, _ := invokePersistent(t, "sync", storageFixture("bundle-b"), "--json")
		if code != 1 {
			t.Fatalf("expected exit 1, got %d", code)
		}
		if failure := storageFailure(t, stdout); failure.Code != "BUNDLE_NOT_INDEXED" {
			t.Fatalf("expected BUNDLE_NOT_INDEXED, got %s", failure.Code)
		}
	})

	t.Run("sync requires an existing database", func(t *testing.T) {
		storageHome(t)
		missing := filepath.Join(t.TempDir(), "missing.db")
		code, stdout, _ := invokePersistent(t, "sync", storageFixture("bundle-a"), "--db", missing, "--json")

		if code != 2 {
			t.Fatalf("expected exit 2, got %d", code)
		}
		if failure := storageFailure(t, stdout); failure.Code != "INVALID_REQUEST" {
			t.Fatalf("expected INVALID_REQUEST, got %s", failure.Code)
		}
	})

	t.Run("sync rejects --no-db", func(t *testing.T) {
		storageHome(t)
		code, stdout, _ := invokePersistent(t, "sync", storageFixture("bundle-a"), "--no-db", "--json")

		if code != 2 {
			t.Fatalf("expected exit 2, got %d", code)
		}
		if failure := storageFailure(t, stdout); failure.Code != "INVALID_REQUEST" {
			t.Fatalf("expected INVALID_REQUEST, got %s", failure.Code)
		}
	})

	t.Run("sync preserves the stored permissive mode", func(t *testing.T) {
		storageHome(t)
		bundle := copyStorageBundle(t, permissiveFixture())
		runStorageJSON(t, "ingest", bundle, "--permissive", "--json")

		_, _, report := runSyncJSON(t, bundle, "--json")
		if report.Mode != "permissive" || report.Documents != 6 || report.Skipped != 6 {
			t.Fatalf("unexpected report: %+v", report)
		}
	})
}

func writeSyncFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func runAuditFor(t *testing.T, path string, extra ...string) (int, bddAuditReport) {
	t.Helper()

	args := []string{"audit", path}
	args = append(args, extra...)
	code, stdout, stderr := invokePersistent(t, args...)
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	envelope := decodeEnvelope(t, stdout)
	if !envelope.OK {
		t.Fatalf("expected audit report, got failure: %+v", envelope.Error)
	}
	var report bddAuditReport
	if err := json.Unmarshal(envelope.Result, &report); err != nil {
		t.Fatal(err)
	}
	return code, report
}

func hasConceptID(bundle contractv1.Bundle, id string) bool {
	for _, concept := range bundle.Concepts {
		if concept.ID == id {
			return true
		}
	}
	return false
}

func hasKeywordID(response contractv1.SearchResponse, id string) bool {
	for _, hit := range response.Keyword {
		if hit.ID == id {
			return true
		}
	}
	return false
}
