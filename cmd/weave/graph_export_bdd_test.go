package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Behavior-level tests for `weave graph --format`. See docs/bdd/graph-export.md.

func danglingBundle(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	content := "---\ntype: Concept\ntitle: A\nkeywords:\n  - alpha\nrelated:\n  - concepts/missing.md\n---\n\n# A\n"
	if err := os.WriteFile(filepath.Join(root, "a.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestGraphExportDOT(t *testing.T) {
	code, stdout, stderr := InvokeRead(t, "graph", fixturePath(), "--format", "dot")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d stderr=%s", code, stderr)
	}
	if !strings.HasPrefix(stdout, "digraph weave {") || !strings.Contains(stdout, "->") {
		t.Fatalf("unexpected DOT output:\n%s", stdout)
	}
}

func TestGraphExportMermaid(t *testing.T) {
	code, stdout, stderr := InvokeRead(t, "graph", fixturePath(), "--format", "mermaid")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d stderr=%s", code, stderr)
	}
	if !strings.HasPrefix(stdout, "flowchart LR") {
		t.Fatalf("unexpected Mermaid output:\n%s", stdout)
	}
}

func TestGraphExportGraphML(t *testing.T) {
	code, stdout, stderr := InvokeRead(t, "graph", fixturePath(), "--format", "graphml")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d stderr=%s", code, stderr)
	}
	if !strings.Contains(stdout, "<graphml") || !strings.Contains(stdout, "<node id=") {
		t.Fatalf("unexpected GraphML output:\n%s", stdout)
	}
}

func graphFilterBundle(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	a := "---\ntype: Concept\ntitle: A\nkeywords:\n  - alpha\nrelated:\n  - b.md\n---\n\n# A\n\nSee [B](b.md).\n"
	b := "---\ntype: Concept\ntitle: B\nkeywords:\n  - alpha\n---\n\n# B\n"
	if err := os.WriteFile(filepath.Join(root, "a.md"), []byte(a), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "b.md"), []byte(b), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestGraphExportRelationFilter(t *testing.T) {
	root := graphFilterBundle(t)
	code, stdout, stderr := InvokeRead(t, "graph", root, "--relation", "related", "--format", "dot")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d stderr=%s", code, stderr)
	}
	if strings.Contains(stdout, `[label="link"]`) {
		t.Fatalf("relation filter did not apply:\n%s", stdout)
	}
	if !strings.Contains(stdout, `[label="related"]`) {
		t.Fatalf("related edge missing:\n%s", stdout)
	}
}

func TestGraphExportDeterministic(t *testing.T) {
	firstCode, first, firstStderr := InvokeRead(t, "graph", fixturePath(), "--format", "dot")
	secondCode, second, secondStderr := InvokeRead(t, "graph", fixturePath(), "--format", "dot")
	if firstCode != 0 || secondCode != 0 {
		t.Fatalf("expected exit 0, got %d/%d stderr=%s/%s", firstCode, secondCode, firstStderr, secondStderr)
	}
	if first != second {
		t.Fatal("graph export is not byte-identical across runs")
	}
}

func TestGraphExportFormatWithJSON(t *testing.T) {
	code, _, stderr := InvokeRead(t, "graph", fixturePath(), "--format", "dot", "--json")
	if code != 2 || !strings.Contains(stderr, "mutually exclusive") {
		t.Fatalf("expected a usage error, got %d %q", code, stderr)
	}
}

func TestGraphExportUnknownFormat(t *testing.T) {
	code, _, stderr := InvokeRead(t, "graph", fixturePath(), "--format", "svg")
	if code != 2 || !strings.Contains(stderr, "dot, mermaid or graphml") {
		t.Fatalf("expected a usage error, got %d %q", code, stderr)
	}
}

func TestGraphExportOutFile(t *testing.T) {
	out := filepath.Join(t.TempDir(), "graph.dot")
	code, stdout, stderr := InvokeRead(t, "graph", fixturePath(), "--format", "dot", "--out", out)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d stderr=%s", code, stderr)
	}
	if stdout != "" {
		t.Fatalf("stdout must stay empty with --out, got %q", stdout)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), "digraph weave {") {
		t.Fatalf("unexpected file content:\n%s", data)
	}
}

func TestGraphExportNoPartialFile(t *testing.T) {
	out := filepath.Join(t.TempDir(), "graph.svg")
	code, _, _ := InvokeRead(t, "graph", fixturePath(), "--format", "svg", "--out", out)
	if code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatalf("no file may be written on failure, stat err = %v", err)
	}
}

func TestGraphExportDangling(t *testing.T) {
	root := danglingBundle(t)
	code, stdout, stderr := InvokeRead(t, "graph", root, "--format", "dot")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d stderr=%s", code, stderr)
	}
	if strings.Contains(stdout, "missing.md") {
		t.Fatalf("dangling target must be omitted:\n%s", stdout)
	}
	if !strings.Contains(stderr, "dangling") {
		t.Fatalf("expected one dangling warning, got %q", stderr)
	}
}
