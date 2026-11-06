package mcp_test

import (
	"context"
	"encoding/json"
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
	})
	if err != nil {
		t.Fatal(err)
	}
	return server
}

func TestWorkspaceSourcesTool(t *testing.T) {
	server := workspaceServer(t)

	found := false
	for _, tool := range server.Tools() {
		if tool.Name == "weave_sources" {
			found = true
		}
		if _, ok := tool.InputSchema["properties"].(map[string]any)["source"]; ok && tool.Name == "weave_sources" {
			t.Fatalf("weave_sources must not accept a source argument")
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
}

func TestWorkspaceSourcesResource(t *testing.T) {
	server := workspaceServer(t)

	uris := map[string]bool{}
	for _, resource := range server.Resources() {
		uris[resource.URI] = true
	}
	if !uris["weave://workspace/sources"] {
		t.Fatalf("workspace resource missing: %+v", uris)
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

	// A removed source must no longer resolve.
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
