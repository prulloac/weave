package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
	sqlitestore "github.com/prulloac/weave/storage/sqlite"
)

// Behavior-level tests for the batch capability. See docs/bdd/extension-batching.md.

func markerLines(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Fields(string(data))
}

func batchIngest(t *testing.T, root, enricher string, extra ...string) (int, contractv1.IngestReport, string) {
	t.Helper()
	args := []string{"ingest", root, "--with-enricher", enricher, "--db", filepath.Join(t.TempDir(), "index.db"), "--json"}
	args = append(args, extra...)
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

func TestEnrichmentBatchRequest(t *testing.T) {
	root := extensionFixture(t)
	installExtension(t, root, "batch", "batch-enricher.sh")
	marker := filepath.Join(t.TempDir(), "marker")
	t.Setenv("TEST_BATCH_MARKER", marker)

	code, report, stderr := batchIngest(t, root, "batch")
	if code != 0 {
		t.Fatalf("ingest failed: %d %s", code, stderr)
	}
	if report.Warnings != 0 {
		t.Fatalf("warnings = %d", report.Warnings)
	}
	if got := markerLines(t, marker); len(got) != 1 || got[0] != "3" {
		t.Fatalf("batch invocations = %v, want [3]", got)
	}
}

func TestEnrichmentWithoutBatchFallsBack(t *testing.T) {
	root := extensionFixture(t)
	installExtension(t, root, "single", "batch-single.sh")
	marker := filepath.Join(t.TempDir(), "marker")
	t.Setenv("TEST_BATCH_MARKER", marker)

	if code, _, stderr := batchIngest(t, root, "single"); code != 0 {
		t.Fatalf("ingest failed: %s", stderr)
	}
	if got := markerLines(t, marker); len(got) != 3 {
		t.Fatalf("per-document invocations = %v, want three", got)
	}
}

func TestEnrichmentBatchItemFailure(t *testing.T) {
	root := extensionFixture(t)
	installExtension(t, root, "batch", "batch-enricher.sh")
	t.Setenv("TEST_BATCH_MODE", "item-error")
	db := filepath.Join(t.TempDir(), "index.db")

	code, stdout, stderr := Invoke(t, "ingest", root, "--with-enricher", "batch", "--db", db, "--json")
	if code != 0 {
		t.Fatalf("ingest failed: %d %s", code, stderr)
	}
	envelope := decodeEnvelope(t, stdout)
	var report contractv1.IngestReport
	if err := json.Unmarshal(envelope.Result, &report); err != nil {
		t.Fatal(err)
	}
	if report.Warnings != 1 {
		t.Fatalf("warnings = %d, want 1", report.Warnings)
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
	if !containsKeyword(keywords["a"], "batch") || !containsKeyword(keywords["c"], "batch") {
		t.Fatalf("batch keywords missing: %+v", keywords)
	}
	if containsKeyword(keywords["b"], "batch") {
		t.Fatalf("failed item was enriched: %+v", keywords["b"])
	}
}

func TestEnrichmentBatchLimit(t *testing.T) {
	root := t.TempDir()
	for index := 0; index < 129; index++ {
		name := filepath.Join(root, fmt.Sprintf("doc-%03d.md", index))
		content := "---\ntype: Concept\ntitle: Doc\nkeywords:\n  - batch\n---\n\n# Doc\n"
		if err := os.WriteFile(name, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	installExtension(t, root, "batch", "batch-enricher.sh")
	marker := filepath.Join(t.TempDir(), "marker")
	t.Setenv("TEST_BATCH_MARKER", marker)

	if code, _, stderr := batchIngest(t, root, "batch"); code != 0 {
		t.Fatalf("ingest failed: %s", stderr)
	}
	lines := markerLines(t, marker)
	if len(lines) != 2 {
		t.Fatalf("batches = %v, want two", lines)
	}
	if lines[0] != "128" || lines[1] != "1" {
		t.Fatalf("batch sizes = %v, want [128 1]", lines)
	}
}

func TestEnrichmentBatchTimeout(t *testing.T) {
	root := extensionFixture(t)
	installExtension(t, root, "batch", "batch-enricher.sh")
	t.Setenv("TEST_BATCH_MODE", "timeout")
	code, report, stderr := batchIngest(t, root, "batch", "--timeout", "100ms")
	if code != 0 {
		t.Fatalf("ingest failed: %s", stderr)
	}
	if report.Warnings != 3 {
		t.Fatalf("warnings = %d, want 3", report.Warnings)
	}
}

func TestEnrichmentBatchMalformed(t *testing.T) {
	root := extensionFixture(t)
	installExtension(t, root, "batch", "batch-enricher.sh")
	t.Setenv("TEST_BATCH_MODE", "malformed")
	code, report, stderr := batchIngest(t, root, "batch")
	if code != 0 {
		t.Fatalf("ingest failed: %s", stderr)
	}
	if report.Warnings != 3 {
		t.Fatalf("warnings = %d, want 3", report.Warnings)
	}
}

func TestCheckBatchCapability(t *testing.T) {
	withCoreVersion(t, "0.11.0")
	root := extensionFixture(t)
	installExtension(t, root, "batch", "batch-enricher.sh")
	code, stdout, stderr := Invoke(t, "extensions", "check", "batch", root, "--json")
	if code != 0 {
		t.Fatalf("check failed: %d %s %s", code, stdout, stderr)
	}
	report := decodeCheckReport(t, stdout)
	if got := checkBy(t, report, "batch").Status; got != contractv1.ExtensionCheckPass {
		t.Fatalf("batch check = %s", got)
	}
}

func TestCheckBatchMalformed(t *testing.T) {
	withCoreVersion(t, "0.11.0")
	root := extensionFixture(t)
	installExtension(t, root, "batch", "batch-enricher.sh")
	t.Setenv("TEST_BATCH_MODE", "malformed")
	code, stdout, _ := Invoke(t, "extensions", "check", "batch", root, "--json")
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	report := decodeCheckReport(t, stdout)
	check := checkBy(t, report, "batch")
	if check.Status != contractv1.ExtensionCheckFail || check.Code != contractv1.ErrExtensionProto {
		t.Fatalf("unexpected batch check: %+v", check)
	}
}

func containsKeyword(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
