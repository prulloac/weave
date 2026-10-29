package mcp_test

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine"
	"github.com/prulloac/weave/engine/operations"
	"github.com/prulloac/weave/mcp"
)

func fixture() string {
	return "../contract/v1/golden/bundles/okf-bundle"
}

func auditFixture() string {
	return "../cmd/weave/testdata/audit/graph-issues"
}

func newServer(t *testing.T, bundle string) (*mcp.Server, *operations.Executor) {
	t.Helper()
	executor := operations.New("test", engine.Options{})
	server, err := mcp.New(executor, mcp.Options{Bundle: bundle})
	if err != nil {
		t.Fatal(err)
	}
	return server, executor
}

func directResult(t *testing.T, executor *operations.Executor, request contractv1.OperationRequest) any {
	t.Helper()
	envelope, ok := executor.Execute(context.Background(), request).(contractv1.SuccessEnvelope)
	if !ok {
		t.Fatalf("expected success envelope for %s", request.Op)
	}
	return envelope.Result
}

func toolError(t *testing.T, err error) *mcp.Error {
	t.Helper()
	var failure *mcp.Error
	if !errors.As(err, &failure) {
		t.Fatalf("expected *mcp.Error, got %T: %v", err, err)
	}
	return failure
}

func TestMCPToolDiscovery(t *testing.T) {
	server, _ := newServer(t, fixture())
	tools := server.Tools()

	names := make([]string, len(tools))
	for index, tool := range tools {
		names[index] = tool.Name
	}
	want := []string{
		"weave_audit", "weave_backlinks", "weave_contract", "weave_explain",
		"weave_path", "weave_relate", "weave_search", "weave_show",
	}
	slices.Sort(names)
	if !slices.Equal(names, want) {
		t.Fatalf("tools=%v, want %v", names, want)
	}

	for _, tool := range tools {
		if len(tool.InputSchema) == 0 {
			t.Fatalf("%s has no input schema", tool.Name)
		}
		if _, ok := tool.InputSchema["properties"].(map[string]any)["path"]; ok {
			t.Fatalf("%s must not accept a bundle path", tool.Name)
		}
	}

	assertRequired(t, tools, "weave_search", "query")
	assertRequired(t, tools, "weave_explain", "query")
	assertRequired(t, tools, "weave_relate", "id")
	assertRequired(t, tools, "weave_show", "id")
	assertRequired(t, tools, "weave_backlinks", "id")
	assertRequired(t, tools, "weave_path", "from", "to")
}

func assertRequired(t *testing.T, tools []mcp.ToolDescriptor, name string, fields ...string) {
	t.Helper()
	for _, tool := range tools {
		if tool.Name != name {
			continue
		}
		required, _ := tool.InputSchema["required"].([]any)
		values := make([]string, 0, len(required))
		for _, value := range required {
			if text, ok := value.(string); ok {
				values = append(values, text)
			}
		}
		for _, field := range fields {
			if !slices.Contains(values, field) {
				t.Fatalf("%s required=%v, missing %s", name, values, field)
			}
		}
		return
	}
	t.Fatalf("tool %s not found", name)
}

func TestMCPToolDispatchMatchesOperations(t *testing.T) {
	bundle := fixture()
	server, executor := newServer(t, bundle)
	ctx := context.Background()

	tests := []struct {
		name    string
		args    map[string]any
		request contractv1.OperationRequest
	}{
		{
			name: "weave_search",
			args: map[string]any{"query": "graph"},
			request: contractv1.OperationRequest{
				Op: contractv1.OpSearch, Path: bundle, Query: "graph", NoDB: true,
			},
		},
		{
			name: "weave_explain",
			args: map[string]any{"query": "graph"},
			request: contractv1.OperationRequest{
				Op: contractv1.OpExplain, Path: bundle, Query: "graph", NoDB: true,
			},
		},
		{
			name: "weave_relate",
			args: map[string]any{"id": "concepts/okf-bundle"},
			request: contractv1.OperationRequest{
				Op: contractv1.OpRelate, Path: bundle, Target: "concepts/okf-bundle", NoDB: true,
			},
		},
		{
			name: "weave_audit",
			args: map[string]any{},
			request: contractv1.OperationRequest{
				Op: contractv1.OpAudit, Path: bundle, NoDB: true,
			},
		},
		{
			name: "weave_show",
			args: map[string]any{"id": "concepts/okf-bundle"},
			request: contractv1.OperationRequest{
				Op: contractv1.OpShow, Path: bundle, Target: "concepts/okf-bundle", NoDB: true,
			},
		},
		{
			name: "weave_backlinks",
			args: map[string]any{"id": "concepts/okf-bundle"},
			request: contractv1.OperationRequest{
				Op: contractv1.OpBacklinks, Path: bundle, Target: "concepts/okf-bundle", NoDB: true,
			},
		},
		{
			name: "weave_path",
			args: map[string]any{"from": "concepts/okf-bundle", "to": "tables/users"},
			request: contractv1.OperationRequest{
				Op: contractv1.OpPath, Path: bundle,
				From: "concepts/okf-bundle", To: "tables/users", NoDB: true,
			},
		},
		{
			name:    "weave_contract",
			args:    map[string]any{},
			request: contractv1.OperationRequest{Op: contractv1.OpContract},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			result, err := server.Call(ctx, test.name, test.args)
			if err != nil {
				t.Fatalf("call failed: %v", err)
			}
			want := directResult(t, executor, test.request)
			if !reflect.DeepEqual(result, want) {
				t.Fatalf("result mismatch:\n got: %+v\nwant: %+v", result, want)
			}
		})
	}
}

func TestMCPAuditFindingsAreSuccessful(t *testing.T) {
	server, _ := newServer(t, auditFixture())
	result, err := server.Call(context.Background(), "weave_audit", map[string]any{})
	if err != nil {
		t.Fatalf("audit call failed: %v", err)
	}
	report, ok := result.(contractv1.AuditReport)
	if !ok {
		t.Fatalf("expected AuditReport, got %T", result)
	}
	if len(report.Findings) == 0 {
		t.Fatal("expected audit findings")
	}
}

func TestMCPValidationAndErrors(t *testing.T) {
	server, _ := newServer(t, fixture())
	ctx := context.Background()

	tests := []struct {
		name string
		tool string
		args map[string]any
		code contractv1.ErrorCode
	}{
		{name: "unknown tool", tool: "weave_unknown", args: map[string]any{}, code: contractv1.ErrInvalidRequest},
		{name: "missing query", tool: "weave_search", args: map[string]any{}, code: contractv1.ErrInvalidRequest},
		{name: "wrong top-k type", tool: "weave_search", args: map[string]any{"query": "graph", "top_k": "ten"}, code: contractv1.ErrInvalidRequest},
		{name: "invalid mode", tool: "weave_search", args: map[string]any{"query": "graph", "mode": "semantic"}, code: contractv1.ErrInvalidRequest},
		{name: "unknown concept", tool: "weave_show", args: map[string]any{"id": "missing"}, code: contractv1.ErrConceptNotFound},
		{name: "invalid relate depth", tool: "weave_relate", args: map[string]any{"id": "concepts/okf-bundle", "depth": float64(6)}, code: contractv1.ErrInvalidRequest},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			_, err := server.Call(ctx, test.tool, test.args)
			if err == nil {
				t.Fatal("expected an error")
			}
			failure := toolError(t, err)
			if failure.Code != test.code {
				t.Fatalf("code=%s, want %s", failure.Code, test.code)
			}
		})
	}
}

func TestMCPDefaultTopK(t *testing.T) {
	server, _ := newServer(t, fixture())
	result, err := server.Call(context.Background(), "weave_search", map[string]any{"query": "graph"})
	if err != nil {
		t.Fatal(err)
	}
	response, ok := result.(contractv1.SearchResponse)
	if !ok {
		t.Fatalf("expected SearchResponse, got %T", result)
	}
	if response.TopK != 10 {
		t.Fatalf("top_k=%d, want 10", response.TopK)
	}
}
