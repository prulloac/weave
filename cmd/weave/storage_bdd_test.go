package main

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
)

// These are behavior-level tests for the persistent storage foundation.
// Incremental sync and watch mode are covered by later BDD suites.

type bddStorageReport struct {
	DB        string `json:"db"`
	Bundle    string `json:"bundle"`
	Mode      string `json:"mode"`
	Documents int    `json:"documents"`
	Edges     int    `json:"edges"`
	Synthetic int    `json:"synthetic"`
	Warnings  int    `json:"warnings"`
	Errors    int    `json:"errors"`
}

func invokePersistent(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	return HarnessInvokePersistent(t, args...)
}

func storageHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	return home
}

func defaultStorageDB(home string) string {
	return filepath.Join(home, ".weave", "index.db")
}

func storageFixture(name string) string {
	return filepath.Join("testdata", "storage", name)
}

func copyStorageBundle(t *testing.T, source string) string {
	t.Helper()
	destination := t.TempDir()
	if err := os.CopyFS(destination, os.DirFS(source)); err != nil {
		t.Fatalf("copy bundle: %v", err)
	}
	return destination
}

func runStorageJSON(t *testing.T, command string, args ...string) (int, testEnvelope, bddStorageReport) {
	t.Helper()

	fullArgs := append([]string{command}, args...)
	code, stdout, stderr := invokePersistent(t, fullArgs...)
	if stderr != "" {
		t.Fatalf("expected empty stderr in JSON mode, got %q", stderr)
	}
	envelope := decodeEnvelope(t, stdout)
	if !envelope.OK {
		t.Fatalf("expected success envelope, got failure: %+v", envelope.Error)
	}
	var report bddStorageReport
	if err := json.Unmarshal(envelope.Result, &report); err != nil {
		t.Fatalf("decode storage report: %v\n%s", err, envelope.Result)
	}
	return code, envelope, report
}

func runParseBundle(t *testing.T, args ...string) contractv1.Bundle {
	t.Helper()

	fullArgs := append([]string{"parse"}, args...)
	code, stdout, stderr := invokePersistent(t, fullArgs...)
	if code != 0 || stderr != "" {
		t.Fatalf("parse exit=%d stderr=%q", code, stderr)
	}
	envelope := decodeEnvelope(t, stdout)
	if !envelope.OK {
		t.Fatalf("parse failed: %+v", envelope.Error)
	}
	var bundle contractv1.Bundle
	if err := json.Unmarshal(envelope.Result, &bundle); err != nil {
		t.Fatalf("decode bundle: %v", err)
	}
	return bundle
}

func runStorageSearch(t *testing.T, path string, extra ...string) contractv1.SearchResponse {
	t.Helper()

	args := []string{"search", path, "alpha", "--json"}
	args = append(args, extra...)
	code, stdout, stderr := invokePersistent(t, args...)
	if code != 0 || stderr != "" {
		t.Fatalf("search exit=%d stderr=%q", code, stderr)
	}
	envelope := decodeEnvelope(t, stdout)
	if !envelope.OK {
		t.Fatalf("search failed: %+v", envelope.Error)
	}
	var response contractv1.SearchResponse
	if err := json.Unmarshal(envelope.Result, &response); err != nil {
		t.Fatalf("decode search response: %v", err)
	}
	return response
}

func storageFailure(t *testing.T, stdout string) contractv1.OperationError {
	t.Helper()

	envelope := decodeEnvelope(t, stdout)
	if envelope.OK {
		t.Fatal("expected a failure envelope")
	}
	if envelope.Error == nil {
		t.Fatal("expected an error payload")
	}
	return *envelope.Error
}

func snapshotStorageBundle(t *testing.T, root string) map[string]string {
	t.Helper()

	files := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(relative)] = string(content)
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot bundle: %v", err)
	}
	return files
}

func TestStorageBehavior(t *testing.T) {
	t.Run("ingest creates the default database", func(t *testing.T) {
		home := storageHome(t)
		bundle := storageFixture("bundle-a")
		code, _, report := runStorageJSON(t, "ingest", bundle, "--strict", "--json")

		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
		absolute, err := filepath.Abs(bundle)
		if err != nil {
			t.Fatal(err)
		}
		if report.DB != defaultStorageDB(home) {
			t.Fatalf("db=%q, want %q", report.DB, defaultStorageDB(home))
		}
		if report.Bundle != absolute {
			t.Fatalf("bundle=%q, want %q", report.Bundle, absolute)
		}
		if report.Mode != "strict" {
			t.Fatalf("mode=%q, want strict", report.Mode)
		}
		if report.Documents != 3 || report.Edges != 3 {
			t.Fatalf("unexpected counts: %+v", report)
		}
		if _, err := os.Stat(defaultStorageDB(home)); err != nil {
			t.Fatalf("default database missing: %v", err)
		}
	})

	t.Run("repeated ingest is idempotent", func(t *testing.T) {
		storageHome(t)
		bundle := storageFixture("bundle-a")
		_, _, first := runStorageJSON(t, "ingest", bundle, "--strict", "--json")
		_, _, second := runStorageJSON(t, "ingest", bundle, "--strict", "--json")

		if first.Documents != second.Documents || first.Edges != second.Edges {
			t.Fatalf("ingest is not idempotent: %+v vs %+v", first, second)
		}
		parsed := runParseBundle(t, bundle, "--json")
		if len(parsed.Concepts) != 3 {
			t.Fatalf("expected 3 concepts, got %d", len(parsed.Concepts))
		}
	})

	t.Run("ingest does not write into the source bundle", func(t *testing.T) {
		storageHome(t)
		bundle := copyStorageBundle(t, storageFixture("bundle-a"))
		before := snapshotStorageBundle(t, bundle)

		runStorageJSON(t, "ingest", bundle, "--strict", "--json")

		after := snapshotStorageBundle(t, bundle)
		if !reflect.DeepEqual(before, after) {
			t.Fatalf("ingest modified the source bundle:\nbefore: %v\nafter:  %v", before, after)
		}
		if _, err := os.Stat(filepath.Join(bundle, ".weave")); !os.IsNotExist(err) {
			t.Fatalf("expected no .weave directory inside the source bundle, got %v", err)
		}
	})

	t.Run("read uses the database after sources are removed", func(t *testing.T) {
		storageHome(t)
		bundle := copyStorageBundle(t, storageFixture("bundle-a"))
		runStorageJSON(t, "ingest", bundle, "--strict", "--json")
		if err := os.RemoveAll(bundle); err != nil {
			t.Fatal(err)
		}

		parsed := runParseBundle(t, bundle, "--json")
		if len(parsed.Concepts) != 3 {
			t.Fatalf("expected 3 concepts from the database, got %d", len(parsed.Concepts))
		}
	})

	t.Run("--no-db uses transient in-memory parsing", func(t *testing.T) {
		home := storageHome(t)
		bundle := storageFixture("bundle-a")

		parsed := runParseBundle(t, bundle, "--no-db", "--json")
		if len(parsed.Concepts) != 3 {
			t.Fatalf("expected 3 concepts, got %d", len(parsed.Concepts))
		}
		if _, err := os.Stat(defaultStorageDB(home)); !os.IsNotExist(err) {
			t.Fatalf("expected no default database, got %v", err)
		}
	})

	t.Run("--db overrides the default path", func(t *testing.T) {
		home := storageHome(t)
		bundle := storageFixture("bundle-a")
		custom := filepath.Join(t.TempDir(), "custom.db")

		_, _, report := runStorageJSON(t, "ingest", bundle, "--strict", "--db", custom, "--json")
		if report.DB != custom {
			t.Fatalf("db=%q, want %q", report.DB, custom)
		}
		if _, err := os.Stat(custom); err != nil {
			t.Fatalf("custom database missing: %v", err)
		}
		if _, err := os.Stat(defaultStorageDB(home)); !os.IsNotExist(err) {
			t.Fatalf("expected no default database, got %v", err)
		}
		parsed := runParseBundle(t, bundle, "--db", custom, "--json")
		if len(parsed.Concepts) != 3 {
			t.Fatalf("expected 3 concepts, got %d", len(parsed.Concepts))
		}
	})

	t.Run("database-backed reads match in-memory reads", func(t *testing.T) {
		storageHome(t)
		bundle := storageFixture("bundle-a")
		runStorageJSON(t, "ingest", bundle, "--strict", "--json")

		persisted := runStorageSearch(t, bundle)
		source := runStorageSearch(t, bundle, "--no-db")
		if !reflect.DeepEqual(persisted, source) {
			t.Fatalf("search mismatch:\npersisted: %+v\nsource:    %+v", persisted, source)
		}
	})

	t.Run("two bundles coexist", func(t *testing.T) {
		storageHome(t)
		a := storageFixture("bundle-a")
		b := storageFixture("bundle-b")
		runStorageJSON(t, "ingest", a, "--strict", "--json")
		runStorageJSON(t, "ingest", b, "--strict", "--json")

		if got := len(runParseBundle(t, a, "--json").Concepts); got != 3 {
			t.Fatalf("bundle A concepts=%d, want 3", got)
		}
		if got := len(runParseBundle(t, b, "--json").Concepts); got != 2 {
			t.Fatalf("bundle B concepts=%d, want 2", got)
		}
	})

	t.Run("re-ingesting one bundle leaves the other intact", func(t *testing.T) {
		storageHome(t)
		a := storageFixture("bundle-a")
		b := storageFixture("bundle-b")
		runStorageJSON(t, "ingest", a, "--strict", "--json")
		runStorageJSON(t, "ingest", b, "--strict", "--json")
		runStorageJSON(t, "ingest", a, "--strict", "--json")

		if got := len(runParseBundle(t, b, "--json").Concepts); got != 2 {
			t.Fatalf("bundle B concepts=%d, want 2", got)
		}
	})

	t.Run("rebuild only affects its bundle", func(t *testing.T) {
		storageHome(t)
		a := storageFixture("bundle-a")
		b := storageFixture("bundle-b")
		runStorageJSON(t, "ingest", a, "--strict", "--json")
		runStorageJSON(t, "ingest", b, "--strict", "--json")

		_, _, report := runStorageJSON(t, "rebuild", a, "--strict", "--json")
		if report.Documents != 3 || report.Edges != 3 {
			t.Fatalf("unexpected rebuild report: %+v", report)
		}
		if got := len(runParseBundle(t, b, "--json").Concepts); got != 2 {
			t.Fatalf("bundle B concepts=%d, want 2", got)
		}
	})

	t.Run("missing database is a usage error", func(t *testing.T) {
		storageHome(t)
		missing := filepath.Join(t.TempDir(), "missing.db")
		code, stdout, _ := invokePersistent(t, "parse", storageFixture("bundle-a"), "--db", missing, "--json")

		if code != 2 {
			t.Fatalf("expected exit 2, got %d", code)
		}
		if failure := storageFailure(t, stdout); failure.Code != "INVALID_REQUEST" {
			t.Fatalf("expected INVALID_REQUEST, got %s", failure.Code)
		}
	})

	t.Run("unindexed bundle is reported explicitly", func(t *testing.T) {
		storageHome(t)
		runStorageJSON(t, "ingest", storageFixture("bundle-a"), "--strict", "--json")

		code, stdout, _ := invokePersistent(t, "parse", storageFixture("bundle-b"), "--json")
		if code != 1 {
			t.Fatalf("expected exit 1, got %d", code)
		}
		if failure := storageFailure(t, stdout); failure.Code != "BUNDLE_NOT_INDEXED" {
			t.Fatalf("expected BUNDLE_NOT_INDEXED, got %s", failure.Code)
		}
	})

	t.Run("--db and --no-db are mutually exclusive", func(t *testing.T) {
		storageHome(t)
		custom := filepath.Join(t.TempDir(), "custom.db")
		code, stdout, _ := invokePersistent(t, "parse", storageFixture("bundle-a"),
			"--db", custom, "--no-db", "--json")

		if code != 2 {
			t.Fatalf("expected exit 2, got %d", code)
		}
		if failure := storageFailure(t, stdout); failure.Code != "INVALID_REQUEST" {
			t.Fatalf("expected INVALID_REQUEST, got %s", failure.Code)
		}
	})

	t.Run("ingest rejects --no-db", func(t *testing.T) {
		storageHome(t)
		code, stdout, _ := invokePersistent(t, "ingest", storageFixture("bundle-a"), "--no-db", "--json")

		if code != 2 {
			t.Fatalf("expected exit 2, got %d", code)
		}
		if failure := storageFailure(t, stdout); failure.Code != "INVALID_REQUEST" {
			t.Fatalf("expected INVALID_REQUEST, got %s", failure.Code)
		}
	})

	t.Run("invalid root is rejected", func(t *testing.T) {
		storageHome(t)
		missing := filepath.Join(t.TempDir(), "missing-root")
		code, stdout, _ := invokePersistent(t, "ingest", missing, "--strict", "--json")

		if code != 2 {
			t.Fatalf("expected exit 2, got %d", code)
		}
		if failure := storageFailure(t, stdout); failure.Code != "INVALID_ROOT" {
			t.Fatalf("expected INVALID_ROOT, got %s", failure.Code)
		}
	})
}
