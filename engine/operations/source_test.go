package operations

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine/extensions"
)

func writeSourceProvider(t *testing.T, root string) {
	t.Helper()
	path := filepath.Join(root, ".weave", "bin", "weave-git")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `#!/bin/sh
cat >/dev/null
printf '{"v":"weave.source/v1","type":"source_response","snapshot":{"path":"%s","source_id":"%s","commit":"8f3c2a1d4b5e6f708192a3b4c5d6e7f8091a2b3c","ref":"main","read_only":true},"repository":{"id":"_repo","name":"fixture","title":"Fixture","description":"Fixture repository","readme":"README.md"}}' "$TEST_SNAPSHOT" "$TEST_SOURCE_ID"
`
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestResolveSourcesMaterializesGitProvider(t *testing.T) {
	root := t.TempDir()
	snapshot := filepath.Join(root, "snapshot")
	if err := os.MkdirAll(snapshot, 0o755); err != nil {
		t.Fatal(err)
	}
	writeSourceProvider(t, root)
	t.Setenv("WEAVE_CACHE_DIR", root)
	t.Setenv("TEST_SNAPSHOT", snapshot)
	t.Setenv("TEST_SOURCE_ID", extensions.SingleSourceID(root))

	executor := Executor{}
	sources, err := executor.resolveSources(context.Background(), contractv1.OperationRequest{
		Path: root,
		Sources: []contractv1.SourceSpec{{
			Alias:    "core",
			Path:     root,
			Provider: "git",
			Ref:      "main",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != 1 {
		t.Fatalf("sources = %+v", sources)
	}
	source := sources[0]
	if source.Path != snapshot || source.Provider != "git" || source.Commit == "" {
		t.Fatalf("unexpected source: %+v", source)
	}
	if source.Repository.Name != "fixture" {
		t.Fatalf("unexpected repository: %+v", source.Repository)
	}
}

func TestResolveSourcesMissingGitProvider(t *testing.T) {
	root := t.TempDir()
	executor := Executor{}
	_, err := executor.resolveSources(context.Background(), contractv1.OperationRequest{
		Path: root,
		Sources: []contractv1.SourceSpec{{
			Alias:    "core",
			Path:     root,
			Provider: "git",
			Ref:      "main",
		}},
	})
	if err == nil {
		t.Fatal("expected a missing provider error")
	}
}
