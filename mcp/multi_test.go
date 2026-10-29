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

func multiFixture(t *testing.T) (string, string, []contractv1.SourceSpec) {
	t.Helper()
	base := t.TempDir()
	rootA := filepath.Join(base, "team-a")
	rootB := filepath.Join(base, "team-b")
	for _, root := range []string{rootA, rootB} {
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeDoc := func(root, name, keyword string) {
		content := "---\ntype: Concept\ntitle: \"" + name + "\"\nkeywords:\n  - \"" + keyword + "\"\n---\n\n# " + name + "\n\nbody " + name + "\n"
		if err := os.WriteFile(filepath.Join(root, name+".md"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeDoc(rootA, "common", "common")
	writeDoc(rootA, "shared", "shared")
	writeDoc(rootB, "common", "common")
	writeDoc(rootB, "shared", "shared")
	writeDoc(rootB, "only-b", "only-b")
	return rootA, rootB, []contractv1.SourceSpec{
		{Alias: "team-a", Path: rootA},
		{Alias: "b", Path: rootB},
	}
}

func multiServer(t *testing.T, rootA string, sources []contractv1.SourceSpec) *mcp.Server {
	t.Helper()
	server, err := mcp.New(operations.New("test", engine.Options{}), mcp.Options{
		Bundle:  rootA,
		Sources: sources,
	})
	if err != nil {
		t.Fatalf("server: %v", err)
	}
	return server
}

func TestMCPMultiSourceSearch(t *testing.T) {
	rootA, _, sources := multiFixture(t)
	server := multiServer(t, rootA, sources)
	value, err := server.Call(context.Background(), "weave_search", map[string]any{
		"query": "common", "mode": "keyword",
	})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	response, ok := value.(contractv1.SearchResponse)
	if !ok {
		t.Fatalf("unexpected payload: %T", value)
	}
	if len(response.Keyword) != 2 {
		t.Fatalf("expected hits from both sources, got %+v", response.Keyword)
	}
	seen := map[string]string{}
	for _, hit := range response.Keyword {
		seen[hit.ID] = hit.Source
	}
	if seen["common"] != "team-a" && seen["common"] != "b" {
		t.Fatalf("missing source annotation: %+v", seen)
	}
}

func TestMCPMultiSourceShowAmbiguityAndScope(t *testing.T) {
	rootA, _, sources := multiFixture(t)
	server := multiServer(t, rootA, sources)

	if _, err := server.Call(context.Background(), "weave_show", map[string]any{"id": "shared"}); err == nil {
		t.Fatal("ambiguous show must fail")
	} else if mcpErr, ok := err.(*mcp.Error); !ok || mcpErr.Code != contractv1.ErrInvalidRequest {
		t.Fatalf("expected INVALID_REQUEST, got %v", err)
	}

	value, err := server.Call(context.Background(), "weave_show", map[string]any{"id": "shared", "source": "b"})
	if err != nil {
		t.Fatalf("scoped show: %v", err)
	}
	view, ok := value.(*contractv1.ShowResult)
	if !ok {
		t.Fatalf("unexpected payload: %T", value)
	}
	if view.Source != "b" || view.ID != "shared" {
		t.Fatalf("unexpected scoped show: %+v", view)
	}
}

func TestMCPMultiSourceListAndAudit(t *testing.T) {
	rootA, _, sources := multiFixture(t)
	server := multiServer(t, rootA, sources)

	value, err := server.Call(context.Background(), "weave_list", map[string]any{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	listed, ok := value.([]contractv1.ListResult)
	if !ok {
		t.Fatalf("unexpected payload: %T", value)
	}
	if len(listed) != 5 {
		t.Fatalf("expected 5 entries, got %d", len(listed))
	}

	value, err = server.Call(context.Background(), "weave_audit", map[string]any{"fail_on": "none"})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	report, ok := value.(contractv1.AuditReport)
	if !ok {
		t.Fatalf("unexpected payload: %T", value)
	}
	if report.Summary.Concepts != 5 {
		t.Fatalf("audit concepts = %d, want 5", report.Summary.Concepts)
	}
	found := false
	for _, finding := range report.Findings {
		if finding.Source == "b" {
			found = true
		}
	}
	if !found {
		t.Fatal("audit findings must carry sources")
	}
}

func TestMCPMultiSourceResources(t *testing.T) {
	rootA, _, sources := multiFixture(t)
	server := multiServer(t, rootA, sources)

	content, err := server.ReadResource(context.Background(), "weave://source/b/concept/common")
	if err != nil {
		t.Fatalf("source resource: %v", err)
	}
	var view contractv1.ShowResult
	if err := json.Unmarshal([]byte(content.Text), &view); err != nil {
		t.Fatalf("decode resource: %v", err)
	}
	if view.Source != "b" || view.ID != "common" {
		t.Fatalf("unexpected source resource: %+v", view)
	}

	if _, err := server.ReadResource(context.Background(), "weave://source/missing/concept/common"); err == nil {
		t.Fatal("unknown source resource must fail")
	}
}

func TestMCPMultiSourceToolSchema(t *testing.T) {
	rootA, _, sources := multiFixture(t)
	server := multiServer(t, rootA, sources)
	for _, tool := range server.Tools() {
		properties, _ := tool.InputSchema["properties"].(map[string]any)
		if _, ok := properties["source"]; !ok {
			t.Fatalf("tool %s must accept a source argument", tool.Name)
		}
	}
}

func TestMCPMultiSourceSearchExplain(t *testing.T) {
	rootA, _, sources := multiFixture(t)
	server := multiServer(t, rootA, sources)

	value, err := server.Call(context.Background(), "weave_explain", map[string]any{
		"query": "common", "mode": "keyword", "source": "b",
	})
	if err != nil {
		t.Fatalf("explain: %v", err)
	}
	result, ok := value.(contractv1.ExplainResult)
	if !ok {
		t.Fatalf("unexpected payload: %T", value)
	}
	if result.Source != "b" {
		t.Fatalf("explain source = %q, want b", result.Source)
	}
	if result.Found && !strings.HasPrefix(result.ID, "common") {
		t.Fatalf("unexpected explain target: %+v", result)
	}
}
