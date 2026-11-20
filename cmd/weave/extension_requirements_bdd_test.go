package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Behavior-level tests for manifest requires entries. See docs/bdd/extension-requirements.md.

func withCoreVersion(t *testing.T, release string) {
	t.Helper()
	previous := version
	version = release
	t.Cleanup(func() { version = previous })
}

func TestRequirementUnmetRefusesSelection(t *testing.T) {
	withCoreVersion(t, "0.10.1")
	root := extensionFixture(t)
	installExtension(t, root, "newer", "requires-newer.sh")
	db := filepath.Join(t.TempDir(), "index.db")

	code, stdout, _ := Invoke(t, "ingest", root, "--db", db, "--with-enricher", "newer", "--json")
	if code != 1 {
		t.Fatalf("expected exit 1, got %d: %s", code, stdout)
	}
	envelope := decodeEnvelope(t, stdout)
	if envelope.Error == nil || envelope.Error.Code != "EXTENSION_REQUIREMENT_UNMET" {
		t.Fatalf("expected EXTENSION_REQUIREMENT_UNMET, got %+v", envelope.Error)
	}
	if !strings.Contains(envelope.Error.Message, "newer") || !strings.Contains(envelope.Error.Message, "0.12.0") {
		t.Fatalf("message must name the extension and requirement: %q", envelope.Error.Message)
	}
	if _, err := os.Stat(db); !os.IsNotExist(err) {
		t.Fatalf("database must not be written, stat err = %v", err)
	}
}

func TestRequirementSatisfiedRuns(t *testing.T) {
	withCoreVersion(t, "0.10.1")
	root := extensionFixture(t)
	installExtension(t, root, "satisfied", "requires-satisfied.sh")
	db := filepath.Join(t.TempDir(), "index.db")

	code, stdout, stderr := Invoke(t, "ingest", root, "--db", db, "--with-enricher", "satisfied", "--json")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d stdout=%s stderr=%s", code, stdout, stderr)
	}
}

func TestRequirementUnmetRefusesRerank(t *testing.T) {
	withCoreVersion(t, "0.10.1")
	root := extensionFixture(t)
	installExtension(t, root, "newer", "requires-newer.sh")

	code, stdout, _ := Invoke(t, "search", root, "alpha", "--no-db", "--mode", "keyword", "--rerank", "newer", "--json")
	if code != 1 {
		t.Fatalf("expected exit 1, got %d: %s", code, stdout)
	}
	envelope := decodeEnvelope(t, stdout)
	if envelope.Error == nil || envelope.Error.Code != "EXTENSION_REQUIREMENT_UNMET" {
		t.Fatalf("expected EXTENSION_REQUIREMENT_UNMET, got %+v", envelope.Error)
	}
}

func TestRequirementMissingCompanionRefuses(t *testing.T) {
	withCoreVersion(t, "0.10.1")
	root := extensionFixture(t)
	installExtension(t, root, "needshelper", "requires-helper.sh")
	db := filepath.Join(t.TempDir(), "index.db")

	code, stdout, _ := Invoke(t, "ingest", root, "--db", db, "--with-enricher", "needshelper", "--json")
	if code != 1 {
		t.Fatalf("expected exit 1, got %d: %s", code, stdout)
	}
	envelope := decodeEnvelope(t, stdout)
	if envelope.Error == nil || envelope.Error.Code != "EXTENSION_REQUIREMENT_UNMET" {
		t.Fatalf("expected EXTENSION_REQUIREMENT_UNMET, got %+v", envelope.Error)
	}
	if !strings.Contains(envelope.Error.Message, "helper") {
		t.Fatalf("message must name the companion: %q", envelope.Error.Message)
	}
}

func TestRequirementUnversionedCompanionRefuses(t *testing.T) {
	withCoreVersion(t, "0.10.1")
	root := extensionFixture(t)
	installExtension(t, root, "needshelper", "requires-helper.sh")
	installExtension(t, root, "helper", "helper-unversioned.sh")
	db := filepath.Join(t.TempDir(), "index.db")

	code, stdout, _ := Invoke(t, "ingest", root, "--db", db, "--with-enricher", "needshelper", "--json")
	if code != 1 {
		t.Fatalf("expected exit 1, got %d: %s", code, stdout)
	}
	envelope := decodeEnvelope(t, stdout)
	if envelope.Error == nil || envelope.Error.Code != "EXTENSION_REQUIREMENT_UNMET" {
		t.Fatalf("expected EXTENSION_REQUIREMENT_UNMET, got %+v", envelope.Error)
	}
}

func TestRequirementVersionedCompanionRuns(t *testing.T) {
	withCoreVersion(t, "0.10.1")
	root := extensionFixture(t)
	installExtension(t, root, "needshelper", "requires-helper.sh")
	installExtension(t, root, "helper", "helper-versioned.sh")
	db := filepath.Join(t.TempDir(), "index.db")

	code, stdout, stderr := Invoke(t, "ingest", root, "--db", db, "--with-enricher", "needshelper", "--json")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d stdout=%s stderr=%s", code, stdout, stderr)
	}
}

func TestRequirementDevBuildWarns(t *testing.T) {
	withCoreVersion(t, "dev")
	root := extensionFixture(t)
	installExtension(t, root, "newer", "requires-newer.sh")
	db := filepath.Join(t.TempDir(), "index.db")

	code, stdout, stderr := Invoke(t, "ingest", root, "--db", db, "--with-enricher", "newer", "--json")
	if code != 0 {
		t.Fatalf("expected exit 0 for a development build, got %d stdout=%s stderr=%s", code, stdout, stderr)
	}
	if !strings.Contains(stderr, "cannot compare") {
		t.Fatalf("expected one warning that the requirement could not be compared, stderr=%q", stderr)
	}
}

func TestRequirementListingUnchanged(t *testing.T) {
	withCoreVersion(t, "0.10.1")
	root := extensionFixture(t)
	installExtension(t, root, "newer", "requires-newer.sh")

	code, stdout, stderr := Invoke(t, "extensions", "list", root, "--json")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d stderr=%s", code, stderr)
	}
	if !strings.Contains(stdout, `"newer"`) {
		t.Fatalf("expected newer to be listed: %s", stdout)
	}
	if strings.Contains(stdout, `"status"`) {
		t.Fatalf("listing must not carry a status field: %s", stdout)
	}
}
