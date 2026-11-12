package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/prulloac/weave/engine/extensions"
)

func TestWeaveGitLocalSourceIngest(t *testing.T) {
	root := weaveCodeRepo(t)
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Fixture\n\nA fixture repository.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	provider := `#!/bin/sh
cat >/dev/null
printf '{"v":"weave.source/v1","type":"source_response","snapshot":{"path":"%s","source_id":"%s","commit":"8f3c2a1d4b5e6f708192a3b4c5d6e7f8091a2b3c","ref":"HEAD","read_only":true},"repository":{"id":"_repo","name":"fixture","title":"Fixture","description":"A fixture repository.","readme":"README.md"}}' "$TEST_SNAPSHOT" "$TEST_SOURCE_ID"
`
	if err := os.WriteFile(filepath.Join(root, ".weave", "bin", "weave-git"), []byte(provider), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WEAVE_CACHE_DIR", root)
	t.Setenv("TEST_SNAPSHOT", root)
	t.Setenv("TEST_SOURCE_ID", extensions.SingleSourceID(root))

	db := filepath.Join(t.TempDir(), "index.db")
	code, stdout, stderr := Invoke(t, "ingest", root, "--git", "--with-enricher", "code", "--db", db, "--strict", "--json")
	if code != 0 {
		t.Fatalf("ingest exit=%d stderr=%q stdout=%s", code, stderr, stdout)
	}
	code, stdout, stderr = Invoke(t, "list", root, "--type", "Code", "--db", db, "--json")
	if code != 0 {
		t.Fatalf("list exit=%d stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, `"main.go"`) {
		t.Fatalf("expected main.go in code documents: %s", stdout)
	}
	code, stdout, stderr = Invoke(t, "list", root, "--type", "Repository", "--db", db, "--json")
	if code != 0 {
		t.Fatalf("repository list exit=%d stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, `"_repo"`) {
		t.Fatalf("expected _repo repository document: %s", stdout)
	}
	code, stdout, stderr = Invoke(t, "graph", root, "--relation", "contains", "--db", db, "--json")
	if code != 0 {
		t.Fatalf("graph exit=%d stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, `"_repo"`) || !strings.Contains(stdout, `"main.go"`) {
		t.Fatalf("expected contains edge from _repo: %s", stdout)
	}
}
