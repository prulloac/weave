package operations

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine/extensions"
	sqlitestore "github.com/prulloac/weave/storage/sqlite"
)

func writeSourceProvider(t *testing.T, root string) {
	t.Helper()
	path := filepath.Join(root, ".weave", "bin", "weave-git")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `#!/bin/sh
if [ "$1" = "--describe" ]; then
  printf '{"v":"weave.extension/v1","name":"git","source_providers":["git"],"document_namespaces":["_repo"]}'
  exit 0
fi
cat >/dev/null
printf '{"v":"weave.source/v1","type":"source_response","snapshot":{"path":"%s","source_id":"%s","revision":"rev-8f3c","ref":"main","read_only":true},"source_metadata":{"provider":"git","name":"fixture"},"documents":[{"id":"_repo","type":"Repository","content_kind":"blob","title":"Fixture"}],"warnings":[]}' "$TEST_SNAPSHOT" "$TEST_SOURCE_ID"
`
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func assertRelation(t *testing.T, concept contractv1.OkfConcept, to string, kind contractv1.RelationKind) {
	t.Helper()
	if !hasRelation(concept, to, kind) {
		t.Fatalf("%s missing %s -> %s: %+v", concept.ID, kind, to, concept.Relations)
	}
}

func hasRelation(concept contractv1.OkfConcept, to string, kind contractv1.RelationKind) bool {
	for _, relation := range concept.Relations {
		if relation.To == to && relation.Kind == kind {
			return true
		}
	}
	return false
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
	if source.Documents == nil || len(source.Documents) != 1 || source.Documents[0].ID != "_repo" {
		t.Fatalf("unexpected documents: %+v", source.Documents)
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

func TestResolveWriteSourcesMarksDegraded(t *testing.T) {
	cache := t.TempDir()
	workspace := t.TempDir()
	good := filepath.Join(workspace, "good")
	bad := filepath.Join(workspace, "bad")
	for _, dir := range []string{good, bad} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeSourceProvider(t, good)
	snapshot := filepath.Join(cache, "snapshot")
	if err := os.MkdirAll(snapshot, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WEAVE_CACHE_DIR", cache)
	t.Setenv("TEST_SNAPSHOT", snapshot)
	t.Setenv("TEST_SOURCE_ID", extensions.WorkspaceSourceID(workspace, "good", good))
	db := filepath.Join(t.TempDir(), "index.db")

	executor := Executor{}
	sources, err := executor.resolveWriteSources(context.Background(), contractv1.OperationRequest{
		Path:          workspace,
		WorkspaceRoot: workspace,
		DB:            db,
		Permissive:    true,
		Sources: []contractv1.SourceSpec{
			{Alias: "good", Path: good, Provider: "git", Ref: "main"},
			{Alias: "bad", Path: bad, Provider: "git", Ref: "main"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != 1 || sources[0].Alias != "good" {
		t.Fatalf("sources = %+v", sources)
	}
	store, err := sqlitestore.OpenExisting(db)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	record, ok, err := store.SourceByAlias(context.Background(), "bad")
	if err != nil || !ok {
		t.Fatalf("bad source: ok=%v err=%v", ok, err)
	}
	if record.Status != sqlitestore.SourceDegraded {
		t.Fatalf("status = %q, want degraded", record.Status)
	}
}
