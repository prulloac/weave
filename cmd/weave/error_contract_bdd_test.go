package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
)

func TestErrorContractHints(t *testing.T) {
	root := copyStorageBundle(t, storageFixture("bundle-a"))

	code, _, stderr := invokeRaw(t, "show", root, "missing", "--no-db")
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	for _, want := range []string{"CONCEPT_NOT_FOUND", "concept not found", "hint:"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr missing %q: %q", want, stderr)
		}
	}

	code, stdout, stderr := invokeRaw(t, "show", root, "missing", "--no-db", "--json")
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	var envelope contractv1.FailureEnvelope
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Error.Code != contractv1.ErrConceptNotFound {
		t.Fatalf("unexpected code: %+v", envelope.Error)
	}
	if !strings.Contains(stderr, "hint:") {
		t.Fatalf("JSON error lacks hint: %q", stderr)
	}
}

func TestEmptyListSucceeds(t *testing.T) {
	root := copyStorageBundle(t, storageFixture("bundle-a"))

	code, stdout, stderr := invokeRaw(t, "list", root, "--no-db", "--type", "Nope", "--json")
	if code != 0 {
		t.Fatalf("expected JSON empty list exit 0, got %d stderr=%q", code, stderr)
	}
	var listed []contractv1.ListResult
	if err := json.Unmarshal(decodeEnvelope(t, stdout).Result, &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed) != 0 {
		t.Fatalf("expected empty list, got %d", len(listed))
	}

	code, _, stderr = invokeRaw(t, "list", root, "--no-db", "--type", "Nope")
	if code != 0 {
		t.Fatalf("expected human empty list exit 0, got %d", code)
	}
	if !strings.Contains(stderr, "no concepts") || !strings.Contains(stderr, "hint:") {
		t.Fatalf("missing empty-list hint: %q", stderr)
	}
}

func TestUsageErrorsHaveHints(t *testing.T) {
	code, _, stderr := invokeRaw(t, "search", "only-path")
	if code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
	for _, want := range []string{"INVALID_REQUEST", "hint:"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr missing %q: %q", want, stderr)
		}
	}
}

func TestContractListsAllErrorCodes(t *testing.T) {
	code, stdout, stderr := invokeRaw(t, "contract", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("contract exit=%d stderr=%q", code, stderr)
	}
	var info contractv1.ContractInfo
	if err := json.Unmarshal(decodeEnvelope(t, stdout).Result, &info); err != nil {
		t.Fatal(err)
	}
	if len(info.Errors) != 17 {
		t.Fatalf("expected 17 error codes, got %d: %v", len(info.Errors), info.Errors)
	}
	found := false
	for _, code := range info.Errors {
		if code == contractv1.ErrBundleNotIndexed {
			found = true
		}
	}
	if !found {
		t.Fatalf("BUNDLE_NOT_INDEXED missing from contract errors: %v", info.Errors)
	}
}

func TestBundleNotIndexedHint(t *testing.T) {
	rootA := copyStorageBundle(t, storageFixture("bundle-a"))
	rootB := copyStorageBundle(t, storageFixture("bundle-b"))
	db := filepath.Join(t.TempDir(), "index.db")
	if code, _, stderr := invokeRaw(t, "ingest", rootA, "--db", db, "--strict", "--json"); code != 0 {
		t.Fatalf("ingest exit=%d stderr=%q", code, stderr)
	}

	code, _, stderr := invokeRaw(t, "show", rootB, "a", "--db", db, "--json")
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr, "BUNDLE_NOT_INDEXED") || !strings.Contains(stderr, "weave ingest") {
		t.Fatalf("missing bundle hint: %q", stderr)
	}
}
