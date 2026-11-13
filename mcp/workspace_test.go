package mcp_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine"
	"github.com/prulloac/weave/engine/operations"
	"github.com/prulloac/weave/mcp"
)

func workspaceServer(t *testing.T) *mcp.Server {
	t.Helper()
	executor := operations.New("test", engine.Options{})
	server, err := mcp.New(executor, mcp.Options{
		Bundle:    fixture(),
		Workspace: true,
		Sources: []contractv1.SourceSpec{
			{Alias: "docs", Path: fixture()},
			{Alias: "notes", Path: fixture()},
		},
		SourceStatuses: map[string]string{"docs": "active", "notes": "indexing"},
		SourceProvenance: map[string]mcp.SourceProvenance{
			"docs": {
				Provider: "git",
				URI:      "git+https://example.com/team/docs.git",
				Ref:      "main",
				Commit:   "8f3c2a1d4b5e6f708192a3b4c5d6e7f8091a2b3c",
				Host:     "example.com",
				Owner:    "team",
				Name:     "docs",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return server
}

func emptyBundle(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	content := "---\ntype: Note\ntitle: One\nkeywords:\n  - one\n---\n\n# One\n"
	if err := os.WriteFile(filepath.Join(root, "one.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestWorkspaceSourcesTool(t *testing.T) {
	server := workspaceServer(t)

	found := false
	for _, tool := range server.Tools() {
		if tool.Name == "weave_sources" {
			found = true
		}
	}
	if !found {
		t.Fatal("weave_sources tool missing")
	}

	value, err := server.Call(context.Background(), "weave_sources", map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	items := value.([]map[string]any)
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2", len(items))
	}
	if items[0]["alias"] != "docs" || items[0]["status"] != "active" {
		t.Fatalf("items[0] = %+v", items[0])
	}
	if items[1]["alias"] != "notes" || items[1]["status"] != "indexing" {
		t.Fatalf("items[1] = %+v", items[1])
	}
	for _, item := range items {
		if _, ok := item["concepts"]; !ok {
			t.Fatalf("source missing concepts: %+v", item)
		}
		if _, ok := item["valid"]; !ok {
			t.Fatalf("source missing valid: %+v", item)
		}
	}
	if items[0]["provider"] != "git" || items[0]["uri"] != "git+https://example.com/team/docs.git" ||
		items[0]["ref"] != "main" || items[0]["commit"] != "8f3c2a1d4b5e6f708192a3b4c5d6e7f8091a2b3c" ||
		items[0]["host"] != "example.com" || items[0]["owner"] != "team" || items[0]["name"] != "docs" {
		t.Fatalf("source provenance missing: %+v", items[0])
	}
	if _, ok := items[1]["provider"]; ok {
		t.Fatalf("notes should not have git provenance: %+v", items[1])
	}
}

func TestWorkspaceSourcesResource(t *testing.T) {
	server := workspaceServer(t)

	uris := map[string]bool{}
	for _, resource := range server.Resources() {
		uris[resource.URI] = true
	}
	if !uris["weave://workspace/sources"] || !uris["weave://workspace/concepts"] ||
		!uris["weave://workspace/stats"] {
		t.Fatalf("workspace resources missing: %+v", uris)
	}

	content, err := server.ReadResource(context.Background(), "weave://workspace/sources")
	if err != nil {
		t.Fatal(err)
	}
	var items []map[string]any
	if err := json.Unmarshal([]byte(content.Text), &items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0]["alias"] != "docs" {
		t.Fatalf("items = %+v", items)
	}
}

func TestWorkspaceConceptsResource(t *testing.T) {
	server := workspaceServer(t)
	content, err := server.ReadResource(context.Background(), "weave://workspace/concepts")
	if err != nil {
		t.Fatal(err)
	}
	var concepts []contractv1.ListResult
	if err := json.Unmarshal([]byte(content.Text), &concepts); err != nil {
		t.Fatal(err)
	}
	if len(concepts) == 0 {
		t.Fatal("expected workspace concepts")
	}
	for _, concept := range concepts {
		if concept.Source == "" {
			t.Fatalf("concept missing source: %+v", concept)
		}
	}
}

func TestWorkspaceStatsResource(t *testing.T) {
	server := workspaceServer(t)
	content, err := server.ReadResource(context.Background(), "weave://workspace/stats")
	if err != nil {
		t.Fatal(err)
	}
	var stats map[string]any
	if err := json.Unmarshal([]byte(content.Text), &stats); err != nil {
		t.Fatal(err)
	}
	if stats["concepts"].(float64) == 0 {
		t.Fatalf("stats = %+v", stats)
	}
	if _, ok := stats["per_source"]; !ok {
		t.Fatalf("stats missing per_source: %+v", stats)
	}
}

func TestWorkspaceRejectsBundleResources(t *testing.T) {
	server := workspaceServer(t)
	_, err := server.ReadResource(context.Background(), "weave://bundle/index")
	if err == nil {
		t.Fatal("expected workspace bundle resource to fail")
	}
	if !strings.Contains(err.Error(), "workspace mode") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWorkspaceSetSources(t *testing.T) {
	server := workspaceServer(t)

	if err := server.SetSources([]contractv1.SourceSpec{
		{Alias: "work", Path: fixture()},
	}, map[string]string{"work": "active"}); err != nil {
		t.Fatal(err)
	}

	value, err := server.Call(context.Background(), "weave_sources", map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	items := value.([]map[string]any)
	if len(items) != 1 || items[0]["alias"] != "work" {
		t.Fatalf("items after SetSources = %+v", items)
	}

	if _, err := server.Call(context.Background(), "weave_show", map[string]any{
		"id": "docs:missing", "source": "docs",
	}); err == nil {
		t.Fatal("expected removed source to be unknown")
	}
}

func TestWorkspaceSetSourcesRejectsDuplicates(t *testing.T) {
	server := workspaceServer(t)
	err := server.SetSources([]contractv1.SourceSpec{
		{Alias: "dup", Path: fixture()},
		{Alias: "dup", Path: fixture()},
	}, map[string]string{})
	if err == nil {
		t.Fatal("expected duplicate alias to fail")
	}
}

func TestEmptyBundleIndexIsArray(t *testing.T) {
	executor := operations.New("test", engine.Options{})
	server, err := mcp.New(executor, mcp.Options{Bundle: emptyBundle(t)})
	if err != nil {
		t.Fatal(err)
	}
	content, err := server.ReadResource(context.Background(), "weave://bundle/index")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(content.Text) == "null" {
		t.Fatalf("empty index encoded as null: %s", content.Text)
	}
	var entries []contractv1.IndexEntry
	if err := json.Unmarshal([]byte(content.Text), &entries); err != nil {
		t.Fatalf("decode index: %v (%s)", err, content.Text)
	}
	if len(entries) != 0 {
		t.Fatalf("entries = %+v", entries)
	}
}
