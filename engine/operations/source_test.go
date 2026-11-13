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
  printf '{"v":"weave.extension/v1","name":"git","source_providers":["git"]}'
  exit 0
fi
cat >/dev/null
printf '{"v":"weave.source/v1","type":"source_response","snapshot":{"path":"%s","source_id":"%s","commit":"8f3c2a1d4b5e6f708192a3b4c5d6e7f8091a2b3c","ref":"main","read_only":true},"repository":{"id":"_repo","name":"fixture","title":"Fixture","description":"Fixture repository","readme":"README.md"}}' "$TEST_SNAPSHOT" "$TEST_SOURCE_ID"
`
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestApplySourceRepositoryProjectsAndDependencies(t *testing.T) {
	bundle := &contractv1.Bundle{Concepts: []contractv1.OkfConcept{
		{ID: "main.go", Path: "main.go", ContentKind: "blob"},
		{ID: "web/app.ts", Path: "web/app.ts", ContentKind: "blob"},
		{ID: "web/pkg/lib.ts", Path: "web/pkg/lib.ts", ContentKind: "blob"},
	}}
	source := &contractv1.SourceContext{
		Alias: "core", Provider: "git", URI: "https://example.com/acme/mono.git",
		Ref: "main", Commit: "8f3c2a1d4b5e6f708192a3b4c5d6e7f8091a2b3c",
		SnapshotPath: "/cache/sources/src/worktree", RepositoryName: "mono",
		RepositoryTitle: "Mono", Projects: []contractv1.SourceProject{
			{Path: "", Name: "example.com/mono", Ecosystem: "go", Manifest: "go.mod", Dependencies: []contractv1.SourceDependency{
				{Name: "github.com/acme/lib", Version: "v1.2.3", Scope: "runtime"},
			}},
			{Path: "web", Name: "@acme/web", Ecosystem: "npm", Manifest: "web/package.json", Dependencies: []contractv1.SourceDependency{
				{Name: "react", Version: "^18", Scope: "runtime"},
			}},
			{Path: "web/pkg", Name: "@acme/pkg", Ecosystem: "npm", Manifest: "web/pkg/package.json", Dependencies: []contractv1.SourceDependency{
				{Name: "react", Version: "^18", Scope: "peer"},
			}},
		},
	}
	applySourceRepository(bundle, source)

	byID := map[string]contractv1.OkfConcept{}
	for _, concept := range bundle.Concepts {
		byID[concept.ID] = concept
	}
	for _, id := range []string{"_repo", "_project", "_project/web", "_project/web/pkg"} {
		if byID[id].ID == "" {
			t.Fatalf("missing %s: %+v", id, bundle.Concepts)
		}
	}
	if byID["_repo"].Type != "Repository" || byID["_project/web"].Type != "Project" {
		t.Fatalf("unexpected types: %+v", byID)
	}
	reactID := "_package/npm/react"
	libID := "_package/go/github-com-acme-lib"
	if byID[reactID].Type != "ExternalPackage" || byID[libID].Type != "ExternalPackage" {
		t.Fatalf("missing packages: %+v", byID)
	}
	versions := byID[reactID].ContentMetadata["versions"].([]string)
	scopes := byID[reactID].ContentMetadata["scopes"].([]string)
	if len(versions) != 1 || versions[0] != "^18" || len(scopes) != 2 || scopes[0] != "peer" || scopes[1] != "runtime" {
		t.Fatalf("react metadata = %+v", byID[reactID].ContentMetadata)
	}

	assertRelation(t, byID["_repo"], "_project/web", contractv1.RelationContains)
	assertRelation(t, byID["_project"], "main.go", contractv1.RelationContains)
	assertRelation(t, byID["_project/web"], "web/app.ts", contractv1.RelationContains)
	assertRelation(t, byID["_project/web/pkg"], "web/pkg/lib.ts", contractv1.RelationContains)
	if hasRelation(byID["_project/web"], "web/pkg/lib.ts", contractv1.RelationContains) {
		t.Fatal("nested file should belong to the deepest project")
	}
	assertRelation(t, byID["_project/web"], reactID, contractv1.RelationDependsOn)
	assertRelation(t, byID["_project/web/pkg"], reactID, contractv1.RelationDependsOn)
	assertRelation(t, byID["_project"], libID, contractv1.RelationDependsOn)
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
