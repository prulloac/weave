package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/prulloac/weave/engine/extensions"
)

func writeFakeGitProvider(t *testing.T, root string) {
	t.Helper()
	provider := `#!/bin/sh
if [ "$1" = "--describe" ]; then
  printf '{"v":"weave.extension/v1","name":"git","source_providers":["git"]}'
  exit 0
fi
cat >/dev/null
printf '{"v":"weave.source/v1","type":"source_response","snapshot":{"path":"%s","source_id":"%s","commit":"8f3c2a1d4b5e6f708192a3b4c5d6e7f8091a2b3c","ref":"HEAD","read_only":true},"repository":{"id":"_repo","name":"fixture","title":"Fixture","description":"A fixture repository.","readme":"README.md"},"warnings":["fixture warning"]}' "$TEST_SNAPSHOT" "$TEST_SOURCE_ID"
`
	if err := os.WriteFile(filepath.Join(root, ".weave", "bin", "weave-git"), []byte(provider), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestWeaveGitLocalSourceIngest(t *testing.T) {
	root := weaveCodeRepo(t)
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Fixture\n\nA fixture repository.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeFakeGitProvider(t, root)
	t.Setenv("WEAVE_CACHE_DIR", root)
	t.Setenv("TEST_SNAPSHOT", root)
	t.Setenv("TEST_SOURCE_ID", extensions.SingleSourceID(root))

	db := filepath.Join(t.TempDir(), "index.db")
	code, stdout, stderr := Invoke(t, "ingest", root, "--git", "--with-enricher", "code", "--db", db, "--strict", "--json")
	if code != 0 {
		t.Fatalf("ingest exit=%d stderr=%q stdout=%s", code, stderr, stdout)
	}
	if !strings.Contains(stderr, "fixture warning") {
		t.Fatalf("expected provider warning in stderr: %q", stderr)
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
	out := filepath.Join(t.TempDir(), "export")
	code, stdout, stderr = Invoke(t, "export", root, "--out", out, "--format", "raw", "--yes", "--db", db, "--json")
	if code != 0 {
		t.Fatalf("export exit=%d stderr=%q stdout=%s", code, stderr, stdout)
	}
	if _, err := os.Stat(filepath.Join(out, "_repo")); !os.IsNotExist(err) {
		t.Fatalf("_repo must not be exported as a file: %v", err)
	}
	repositories, err := os.ReadFile(filepath.Join(out, "_sources", "repositories.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(repositories), `"_repo"`) || !strings.Contains(string(repositories), `"main.go"`) {
		t.Fatalf("unexpected repository export: %s", repositories)
	}
}

func TestWeaveGitSourceAdd(t *testing.T) {
	sourceRoot := weaveCodeRepo(t)
	writeFakeGitProvider(t, sourceRoot)
	workspaceDir := t.TempDir()
	if code, _, stderr := Invoke(t, "workspace", "init", workspaceDir, "--json"); code != 0 {
		t.Fatalf("workspace init exit=%d stderr=%q", code, stderr)
	}
	t.Setenv("WEAVE_CACHE_DIR", workspaceDir)
	t.Setenv("TEST_SNAPSHOT", sourceRoot)
	t.Setenv("TEST_SOURCE_ID", extensions.WorkspaceSourceID(workspaceDir, "core", sourceRoot))
	code, stdout, stderr := Invoke(t, "source", "add", "core", sourceRoot, "--git", "--ref", "HEAD", "--worktree", "--submodules", "--lfs", "--workspace", workspaceDir, "--json")
	if code != 0 {
		t.Fatalf("source add exit=%d stderr=%q stdout=%s", code, stderr, stdout)
	}
	file, _, err := loadWorkspaceFile(workspaceDir)
	if err != nil {
		t.Fatal(err)
	}
	source := file.Sources["core"]
	if !source.Submodules || !source.LFS || !source.Worktree || source.Ref != "HEAD" {
		t.Fatalf("workspace source = %+v", source)
	}
	code, stdout, stderr = Invoke(t, "source", "list", "--workspace", workspaceDir, "--json")
	if code != 0 {
		t.Fatalf("source list exit=%d stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, `"provider":"git"`) || !strings.Contains(stdout, `"commit":"8f3c2a1d4b5e6f708192a3b4c5d6e7f8091a2b3c"`) {
		t.Fatalf("unexpected source list: %s", stdout)
	}
}

func TestResolvedWorkspaceSourcesUseSnapshot(t *testing.T) {
	sourceRoot := weaveCodeRepo(t)
	writeFakeGitProvider(t, sourceRoot)
	snapshot := t.TempDir()
	if err := os.WriteFile(filepath.Join(snapshot, "README.md"), []byte("# Snapshot\n\nIndexed snapshot body.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(snapshot, "notes.md"), []byte("snapshot note\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	workspaceDir := t.TempDir()
	manifest := fmt.Sprintf("version: 1\nsources:\n  core:\n    git: %s\n    ref: HEAD\n    worktree: true\n", sourceRoot)
	if err := os.WriteFile(filepath.Join(workspaceDir, "weave.workspace.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WEAVE_CACHE_DIR", workspaceDir)
	t.Setenv("TEST_SNAPSHOT", snapshot)
	t.Setenv("TEST_SOURCE_ID", extensions.WorkspaceSourceID(workspaceDir, "core", sourceRoot))
	if code, _, stderr := Invoke(t, "ingest", "--workspace", workspaceDir, "--json"); code != 0 {
		t.Fatalf("workspace ingest exit=%d stderr=%q", code, stderr)
	}
	ws, err := loadWorkspace(workspaceDir)
	if err != nil {
		t.Fatal(err)
	}
	specs, provenance := resolvedWorkspaceSources(ws)
	if len(specs) != 1 || specs[0].Path != snapshot {
		t.Fatalf("specs = %+v, want snapshot %s", specs, snapshot)
	}
	value := provenance["core"]
	if value.Provider != "git" || value.URI != sourceRoot ||
		value.Commit != "8f3c2a1d4b5e6f708192a3b4c5d6e7f8091a2b3c" ||
		value.Ref != "HEAD" {
		t.Fatalf("provenance = %+v", value)
	}
}

func TestWeaveGitWorkspaceSource(t *testing.T) {
	sourceRoot := weaveCodeRepo(t)
	writeFakeGitProvider(t, sourceRoot)
	workspaceDir := t.TempDir()
	manifest := fmt.Sprintf("version: 1\nsources:\n  core:\n    git: %s\n    ref: HEAD\n    worktree: true\n", sourceRoot)
	if err := os.WriteFile(filepath.Join(workspaceDir, "weave.workspace.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WEAVE_CACHE_DIR", workspaceDir)
	t.Setenv("TEST_SNAPSHOT", sourceRoot)
	t.Setenv("TEST_SOURCE_ID", extensions.WorkspaceSourceID(workspaceDir, "core", sourceRoot))

	code, stdout, stderr := Invoke(t, "ingest", "--workspace", workspaceDir, "--with-enricher", "code", "--json")
	if code != 0 {
		t.Fatalf("workspace ingest exit=%d stderr=%q stdout=%s", code, stderr, stdout)
	}
	code, stdout, stderr = Invoke(t, "source", "list", "--workspace", workspaceDir, "--json")
	if code != 0 {
		t.Fatalf("source list exit=%d stderr=%q", code, stderr)
	}
	for _, want := range []string{`"provider":"git"`, `"uri":"` + sourceRoot + `"`, `"commit":"8f3c2a1d4b5e6f708192a3b4c5d6e7f8091a2b3c"`} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("source list missing %s: %s", want, stdout)
		}
	}
}
