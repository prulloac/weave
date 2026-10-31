package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
)

func TestDryRunIngest(t *testing.T) {
	root := copyStorageBundle(t, storageFixture("bundle-a"))
	db := filepath.Join(t.TempDir(), "index.db")

	code, stdout, stderr := invokeRaw(t, "ingest", root, "--db", db, "--strict", "--dry-run", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("dry-run exit=%d stderr=%q", code, stderr)
	}
	var report contractv1.IngestReport
	if err := json.Unmarshal(decodeEnvelope(t, stdout).Result, &report); err != nil {
		t.Fatal(err)
	}
	if !report.DryRun || report.Documents != 3 {
		t.Fatalf("unexpected report: %+v", report)
	}
	if _, err := os.Stat(db); !os.IsNotExist(err) {
		t.Fatalf("dry run must not create the database: %v", err)
	}

	code, stdout, stderr = invokeRaw(t, "ingest", root, "--db", db, "--strict", "--dry-run")
	if code != 0 || stderr != "" {
		t.Fatalf("human dry-run exit=%d stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, "dry run") {
		t.Fatalf("human dry-run missing label: %s", stdout)
	}
}

func TestDryRunSyncDoesNotWrite(t *testing.T) {
	root := copyStorageBundle(t, storageFixture("bundle-a"))
	db := filepath.Join(t.TempDir(), "index.db")
	if code, _, stderr := invokeRaw(t, "ingest", root, "--db", db, "--strict", "--json"); code != 0 {
		t.Fatalf("ingest exit=%d stderr=%q", code, stderr)
	}
	content := "---\ntype: Concept\ntitle: \"B Changed\"\nkeywords:\n  - \"changed\"\n---\n\n# B Changed\n\nChanged body.\n"
	if err := os.WriteFile(filepath.Join(root, "b.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := invokeRaw(t, "sync", root, "--db", db, "--dry-run", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("dry-run sync exit=%d stderr=%q", code, stderr)
	}
	var report contractv1.SyncReport
	if err := json.Unmarshal(decodeEnvelope(t, stdout).Result, &report); err != nil {
		t.Fatal(err)
	}
	if !report.DryRun {
		t.Fatalf("expected dry_run report: %+v", report)
	}

	code, stdout, stderr = invokeRaw(t, "show", root, "b", "--db", db, "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("show exit=%d stderr=%q", code, stderr)
	}
	var view contractv1.ShowResult
	if err := json.Unmarshal(decodeEnvelope(t, stdout).Result, &view); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(view.Body, "Changed body") {
		t.Fatal("dry-run sync wrote changes")
	}
}

func TestDryRunExport(t *testing.T) {
	root, db := exportFixture(t)
	out := filepath.Join(t.TempDir(), "export")

	code, stdout, stderr := invokeRaw(t, "export", root, "--out", out, "--format", "raw", "--dry-run", "--db", db, "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("dry-run export exit=%d stderr=%q", code, stderr)
	}
	var report contractv1.ExportReport
	if err := json.Unmarshal(decodeEnvelope(t, stdout).Result, &report); err != nil {
		t.Fatal(err)
	}
	if !report.DryRun || report.Documents != 3 || report.Encrypted != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatalf("dry run must not create the target: %v", err)
	}

	os.Unsetenv("WEAVE_KEY_EXPORT_KEY")
	code, _, stderr = invokeRaw(t, "export", root, "--out", out, "--format", "encrypted",
		"--key-id", "export-key", "--dry-run", "--db", db, "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("encrypted dry-run must not require keys: exit=%d stderr=%q", code, stderr)
	}
}

func TestNoInputFailsInsteadOfPrompting(t *testing.T) {
	root, db := exportFixture(t)
	out := filepath.Join(t.TempDir(), "export")
	code, stdout, _ := invokeRaw(t, "export", root, "--out", out, "--format", "raw",
		"--no-input", "--db", db, "--json")
	if code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
	envelope := decodeEnvelope(t, stdout)
	if envelope.Error == nil || envelope.Error.Code != "CONFIRMATION_REQUIRED" {
		t.Fatalf("expected CONFIRMATION_REQUIRED, got %+v", envelope.Error)
	}
}

func TestDryRunRejectsOtherCommands(t *testing.T) {
	root := copyStorageBundle(t, storageFixture("bundle-a"))
	code, _, _ := invokeRaw(t, "search", root, "alpha", "--no-db", "--dry-run", "--json")
	if code != 2 {
		t.Fatalf("expected exit 2 for search dry-run, got %d", code)
	}
	code, _, _ = invokeRaw(t, "sync", root, "--no-db", "--no-input", "--json")
	if code != 2 {
		t.Fatalf("expected exit 2 for sync --no-input, got %d", code)
	}
}
