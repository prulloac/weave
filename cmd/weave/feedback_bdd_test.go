package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestQuietSuppressesSuccessAndWarnings(t *testing.T) {
	root := fixturePath()
	code, stdout, stderr := InvokeRead(t, "list", root, "--quiet")
	if code != 0 || stdout != "" || stderr != "" {
		t.Fatalf("quiet list exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}

	code, stdout, stderr = InvokeRead(t, "search", root, "zzzz", "--quiet")
	if code != 1 || stdout != "" || stderr != "" {
		t.Fatalf("quiet no-match exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}

	code, stdout, stderr = InvokeRead(t, "show", root, "missing", "--quiet")
	if code != 1 || stdout != "" || !strings.Contains(stderr, "concept not found") {
		t.Fatalf("quiet error exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestQuietKeepsJSON(t *testing.T) {
	code, stdout, stderr := InvokeRead(t, "version", "--quiet", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("quiet json exit=%d stderr=%q", code, stderr)
	}
	validateCLIJSON(t, stdout, "cli.schema.json")
}

func TestQuietVerboseConflict(t *testing.T) {
	code, _, _ := InvokeRead(t, "list", fixturePath(), "--quiet", "--verbose")
	if code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
}

func TestVerboseShowsPathsAndTiming(t *testing.T) {
	root := fixturePath()
	code, _, stderr := InvokeRead(t, "list", root, "--verbose")
	if code != 0 {
		t.Fatalf("verbose list exit=%d stderr=%q", code, stderr)
	}
	for _, want := range []string{
		"weave: bundle: " + root,
		"weave: db: disabled (--no-db)",
		"weave: cache: true",
		"weave: elapsed:",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("verbose stderr missing %q:\n%s", want, stderr)
		}
	}

	rootA, rootB := multiBundleFixture(t)
	code, _, stderr = Invoke(t, "search", rootA, "common", "--source", "b="+rootB, "--no-db", "--verbose", "--json")
	if code != 0 {
		t.Fatalf("verbose multi search exit=%d stderr=%q", code, stderr)
	}
	if !strings.Contains(stderr, "weave: source b: "+rootB) {
		t.Fatalf("verbose sources missing: %q", stderr)
	}

	storageRoot := copyStorageBundle(t, storageFixture("bundle-a"))
	db := filepath.Join(t.TempDir(), "index.db")
	code, _, stderr = Invoke(t, "ingest", storageRoot, "--db", db, "--strict", "--verbose")
	if code != 0 {
		t.Fatalf("verbose ingest exit=%d stderr=%q", code, stderr)
	}
	if !strings.Contains(stderr, "weave: db: "+db) {
		t.Fatalf("verbose db missing: %q", stderr)
	}
}

func TestASCIINoUnicodeArrows(t *testing.T) {
	root := fixturePath()
	code, stdout, stderr := InvokeRead(t, "path", root, "concepts/okf-bundle", "concepts/search-index", "--ascii")
	if code != 0 || stderr != "" {
		t.Fatalf("ascii path exit=%d stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, "->") || strings.Contains(stdout, "→") {
		t.Fatalf("ascii path output = %q", stdout)
	}

	code, stdout, stderr = InvokeRead(t, "audit", root, "--ascii", "--fail-on", "none")
	if code != 0 || stderr != "" {
		t.Fatalf("ascii audit exit=%d stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, " concepts | ") || strings.Contains(stdout, "·") {
		t.Fatalf("ascii audit output = %q", stdout)
	}
}

func TestNoColorLeavesOutputPlain(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	code, stdout, stderr := InvokeRead(t, "list", fixturePath())
	if code != 0 || stderr != "" {
		t.Fatalf("NO_COLOR list exit=%d stderr=%q", code, stderr)
	}
	if strings.Contains(stdout, "\x1b[") {
		t.Fatalf("unexpected ANSI escape: %q", stdout)
	}
}
