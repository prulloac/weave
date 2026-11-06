package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeNote(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\ntype: Note\ntitle: " + name + "\nkeywords:\n  - note\n  - " + strings.ToLower(name) + "\n---\n\n# " + name + "\n"
	if err := os.WriteFile(filepath.Join(dir, name+".md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestWorkspaceInitShowPath(t *testing.T) {
	dir := t.TempDir()

	code, stdout, stderr := Invoke(t, "workspace", "init", dir)
	if code != 0 {
		t.Fatalf("init exit=%d stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, "weave.workspace.yaml") {
		t.Fatalf("init stdout = %q", stdout)
	}
	if _, err := os.Stat(filepath.Join(dir, "weave.workspace.yaml")); err != nil {
		t.Fatalf("manifest missing: %v", err)
	}

	code, stdout, stderr = Invoke(t, "workspace", "show", "--workspace", dir, "--json")
	if code != 0 {
		t.Fatalf("show exit=%d stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, `"index"`) {
		t.Fatalf("show json = %q", stdout)
	}

	code, stdout, stderr = Invoke(t, "workspace", "path", "--workspace", dir)
	if code != 0 {
		t.Fatalf("path exit=%d stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, "weave.workspace.yaml") {
		t.Fatalf("path stdout = %q", stdout)
	}
}

func TestWorkspaceMissingManifestFails(t *testing.T) {
	dir := t.TempDir()
	code, _, stderr := Invoke(t, "workspace", "show", "--workspace", dir)
	if code != 2 || !strings.Contains(stderr, "WORKSPACE_NOT_FOUND") {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
}

func TestSourceAddListShowRemove(t *testing.T) {
	dir := t.TempDir()
	notes := filepath.Join(t.TempDir(), "notes")
	writeNote(t, notes, "Alpha")

	if code, _, stderr := Invoke(t, "workspace", "init", dir); code != 0 {
		t.Fatalf("init: %s", stderr)
	}
	code, stdout, stderr := Invoke(t, "source", "add", "notes", notes, "--workspace", dir)
	if code != 0 {
		t.Fatalf("add exit=%d stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, "notes") {
		t.Fatalf("add stdout = %q", stdout)
	}

	code, stdout, stderr = Invoke(t, "source", "list", "--workspace", dir, "--json")
	if code != 0 {
		t.Fatalf("list exit=%d stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, `"alias":"notes"`) || !strings.Contains(stdout, `"status":"active"`) {
		t.Fatalf("list json = %q", stdout)
	}

	code, stdout, stderr = Invoke(t, "source", "show", "notes", "--workspace", dir, "--json")
	if code != 0 {
		t.Fatalf("show exit=%d stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, `"alias":"notes"`) {
		t.Fatalf("show json = %q", stdout)
	}

	code, _, stderr = Invoke(t, "source", "remove", "notes", "--workspace", dir)
	if code != 0 {
		t.Fatalf("remove exit=%d stderr=%q", code, stderr)
	}
	code, stdout, stderr = Invoke(t, "source", "list", "--workspace", dir, "--json")
	if code != 0 {
		t.Fatalf("list after remove exit=%d stderr=%q", code, stderr)
	}
	if strings.Contains(stdout, `"status":"active"`) {
		t.Fatalf("removed source still active: %q", stdout)
	}
}

func TestSourceAddRejectsDuplicates(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(t.TempDir(), "a")
	b := filepath.Join(t.TempDir(), "b")
	writeNote(t, a, "One")
	writeNote(t, b, "Two")

	Invoke(t, "workspace", "init", dir)
	Invoke(t, "source", "add", "docs", a, "--workspace", dir)

	code, _, stderr := Invoke(t, "source", "add", "docs", b, "--workspace", dir)
	if code != 2 || !strings.Contains(stderr, "WORKSPACE_INVALID") {
		t.Fatalf("duplicate alias exit=%d stderr=%q", code, stderr)
	}

	code, _, stderr = Invoke(t, "source", "add", "other", a, "--workspace", dir)
	if code != 2 || !strings.Contains(stderr, "WORKSPACE_INVALID") {
		t.Fatalf("duplicate path exit=%d stderr=%q", code, stderr)
	}
}

func TestSourceAddRejectsReservedAlias(t *testing.T) {
	dir := t.TempDir()
	notes := filepath.Join(t.TempDir(), "notes")
	writeNote(t, notes, "One")
	Invoke(t, "workspace", "init", dir)

	for _, alias := range []string{"contract", "weave", "workspace", "Bad-Alias"} {
		code, _, stderr := Invoke(t, "source", "add", alias, notes, "--workspace", dir)
		if code != 2 || !strings.Contains(stderr, "INVALID_REQUEST") {
			t.Fatalf("alias %q exit=%d stderr=%q", alias, code, stderr)
		}
	}
}

func TestSourceRemoveUnknownAlias(t *testing.T) {
	dir := t.TempDir()
	Invoke(t, "workspace", "init", dir)
	code, _, stderr := Invoke(t, "source", "remove", "missing", "--workspace", dir)
	if code != 1 || !strings.Contains(stderr, "SOURCE_NOT_FOUND") {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
}
