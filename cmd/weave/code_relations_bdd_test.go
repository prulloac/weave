package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func relationFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	a := "---\ntype: Concept\nkeywords: [alpha]\ntitle: Alpha\n---\n\n[B](b.md)\n"
	b := "---\ntype: Concept\nkeywords: [beta]\ntitle: Beta\nrelated: [a.md]\n---\n\nBeta.\n"
	if err := os.WriteFile(filepath.Join(root, "a.md"), []byte(a), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "b.md"), []byte(b), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestGraphRelationFilter(t *testing.T) {
	root := relationFixture(t)
	code, stdout, stderr := Invoke(t, "graph", root, "--no-db", "--relation", "link", "--json")
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
	var envelope struct {
		OK     bool `json:"ok"`
		Result struct {
			Edges []struct {
				Source string `json:"source"`
			} `json:"edges"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope.Result.Edges) != 1 || envelope.Result.Edges[0].Source != "link" {
		t.Fatalf("unexpected edges: %+v", envelope.Result.Edges)
	}
}

func TestPathRelationFilter(t *testing.T) {
	root := relationFixture(t)
	code, _, stderr := Invoke(t, "path", root, "b", "a", "--no-db", "--relation", "link", "--json")
	if code != 1 {
		t.Fatalf("expected no link path, exit=%d stderr=%q", code, stderr)
	}
	code, stdout, stderr := Invoke(t, "path", root, "b", "a", "--no-db", "--relation", "related", "--json")
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
	if !json.Valid([]byte(stdout)) {
		t.Fatalf("invalid json: %s", stdout)
	}
}

func TestRelationKindValidation(t *testing.T) {
	root := relationFixture(t)
	if code, _, _ := Invoke(t, "relate", root, "a", "--no-db", "--relation", "nope", "--json"); code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
}
