package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/prulloac/weave/engine/extensions"
)

func writeGenericProvider(t *testing.T, root string) {
	t.Helper()
	provider := `#!/bin/sh
if [ "$1" = "--describe" ]; then
  printf '%s' '{"v":"weave.extension/v1","name":"git","source_providers":["git"],"document_namespaces":["_repo","_project/"],"artifact_namespaces":["_fixture/"],"relation_kinds":[{"name":"custom_edge","default":true}]}'
  exit 0
fi
if [ -n "${TEST_REQUEST:-}" ]; then
  cat > "$TEST_REQUEST"
else
  cat >/dev/null
fi
printf '{"v":"weave.source/v1","type":"source_response","snapshot":{"path":"%s","source_id":"%s","revision":"rev-42","ref":"HEAD","read_only":true},"source_metadata":{"provider":"git","uri":"%s","ref":"HEAD","revision":"rev-42","host_kind":"generic"},"documents":[{"id":"_repo","type":"Repository","content_kind":"blob","title":"Generated","scope":"","consumes":["README.md"]},{"id":"_project/core","type":"Project","content_kind":"symbol","title":"Core","scope":""}],"relations":[{"from":"_repo","to":"_project/core","kind":"contains"},{"from":"_project/core","to_path":"notes.md","kind":"custom_edge"}],"artifacts":[{"path":"_fixture/hello.json","media_type":"application/json","content":{"hello":"world"}}]}' "$TEST_SNAPSHOT" "$TEST_SOURCE_ID" "$TEST_SNAPSHOT"
`
	if err := os.WriteFile(filepath.Join(root, ".weave", "bin", "weave-git"), []byte(provider), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestGenericProviderDescriptors(t *testing.T) {
	root := weaveCodeRepo(t)
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Generated\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "notes.md"), []byte("---\ntype: Note\ntitle: Notes\nkeywords:\n  - notes\n---\n\n# Notes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeGenericProvider(t, root)
	requestPath := filepath.Join(t.TempDir(), "request.json")
	t.Setenv("TEST_SNAPSHOT", root)
	t.Setenv("TEST_SOURCE_ID", extensions.SingleSourceID(root))
	t.Setenv("TEST_REQUEST", requestPath)
	t.Setenv("WEAVE_CACHE_DIR", root)

	db := filepath.Join(t.TempDir(), "index.db")
	code, stdout, stderr := Invoke(t, "ingest", root, "--git", "--provider-option", "custom=true", "--db", db, "--strict", "--json")
	if code != 0 {
		t.Fatalf("ingest exit=%d stderr=%q stdout=%s", code, stderr, stdout)
	}
	request, err := os.ReadFile(requestPath)
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Options struct {
			ProviderOptions map[string]any `json:"provider_options"`
		} `json:"options"`
	}
	if err := json.Unmarshal(request, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Options.ProviderOptions["custom"] != "true" {
		t.Fatalf("provider options = %+v", payload.Options.ProviderOptions)
	}

	code, stdout, stderr = Invoke(t, "list", root, "--type", "Repository", "--db", db, "--json")
	if code != 0 || !strings.Contains(stdout, `"_repo"`) {
		t.Fatalf("repository list exit=%d stderr=%q stdout=%s", code, stderr, stdout)
	}
	code, stdout, stderr = Invoke(t, "list", root, "--type", "Project", "--db", db, "--json")
	if code != 0 || !strings.Contains(stdout, `"_project/core"`) {
		t.Fatalf("project list exit=%d stderr=%q stdout=%s", code, stderr, stdout)
	}
	code, stdout, stderr = Invoke(t, "graph", root, "--relation", "custom_edge", "--db", db, "--json")
	if code != 0 || !strings.Contains(stdout, `"notes"`) {
		t.Fatalf("graph exit=%d stderr=%q stdout=%s", code, stderr, stdout)
	}
	code, stdout, stderr = Invoke(t, "relate", root, "_project/core", "--db", db, "--json")
	if code != 0 || !strings.Contains(stdout, `"notes"`) {
		t.Fatalf("default relation traversal exit=%d stderr=%q stdout=%s", code, stderr, stdout)
	}
	out := filepath.Join(t.TempDir(), "export")
	code, stdout, stderr = Invoke(t, "export", root, "--out", out, "--format", "raw", "--yes", "--db", db, "--json")
	if code != 0 {
		t.Fatalf("export exit=%d stderr=%q stdout=%s", code, stderr, stdout)
	}
	artifact, err := os.ReadFile(filepath.Join(out, "_fixture", "hello.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(artifact), `"hello":"world"`) {
		t.Fatalf("artifact = %s", artifact)
	}
}

func TestProjectAnalyzerArtifact(t *testing.T) {
	root := weaveCodeRepo(t)
	provider := `#!/bin/sh
if [ "$1" = "--describe" ]; then
  printf '%s' '{"v":"weave.extension/v1","name":"code","source_extensions":[".go"],"analysis_modes":["project"],"artifact_namespaces":["_code/"]}'
  exit 0
fi
cat >/dev/null
printf '%s' '{"v":"weave.cli/v1","type":"project_analysis_response","documents":[],"artifacts":[{"path":"_code/symbols.json","media_type":"application/json","content":{"v":"custom-project-artifact"}}]}'
`
	if err := os.WriteFile(filepath.Join(root, ".weave", "bin", "weave-code"), []byte(provider), 0o755); err != nil {
		t.Fatal(err)
	}
	db := filepath.Join(t.TempDir(), "index.db")
	code, stdout, stderr := Invoke(t, "ingest", root, "--with-enricher", "code", "--db", db, "--strict", "--json")
	if code != 0 {
		t.Fatalf("ingest exit=%d stderr=%q stdout=%s", code, stderr, stdout)
	}
	out := filepath.Join(t.TempDir(), "export")
	code, stdout, stderr = Invoke(t, "export", root, "--out", out, "--format", "raw", "--yes", "--db", db, "--json")
	if code != 0 {
		t.Fatalf("export exit=%d stderr=%q stdout=%s", code, stderr, stdout)
	}
	artifact, err := os.ReadFile(filepath.Join(out, "_code", "symbols.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(artifact), "custom-project-artifact") {
		t.Fatalf("artifact = %s", artifact)
	}
}

func TestGenericSourceMetadata(t *testing.T) {
	sourceRoot := weaveCodeRepo(t)
	if err := os.WriteFile(filepath.Join(sourceRoot, "README.md"), []byte("# Generated\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeGenericProvider(t, sourceRoot)
	workspaceDir := t.TempDir()
	manifest := "version: 1\nsources:\n  core:\n    git: " + sourceRoot + "\n    provider_options:\n      custom: true\n"
	if err := os.WriteFile(filepath.Join(workspaceDir, "weave.workspace.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WEAVE_CACHE_DIR", sourceRoot)
	t.Setenv("TEST_SNAPSHOT", sourceRoot)
	t.Setenv("TEST_SOURCE_ID", extensions.WorkspaceSourceID(workspaceDir, "core", sourceRoot))

	if code, _, stderr := Invoke(t, "ingest", "--workspace", workspaceDir, "--json"); code != 0 {
		t.Fatalf("workspace ingest exit=%d stderr=%q", code, stderr)
	}
	code, stdout, stderr := Invoke(t, "source", "list", "--workspace", workspaceDir, "--json")
	if code != 0 {
		t.Fatalf("source list exit=%d stderr=%q", code, stderr)
	}
	for _, want := range []string{`"provider":"git"`, `"commit":"rev-42"`, `"host_kind":"generic"`, `"revision":"rev-42"`} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("source list missing %s: %s", want, stdout)
		}
	}
}
