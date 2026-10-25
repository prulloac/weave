package operations_test

import (
	"context"
	"path/filepath"
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine"
	"github.com/prulloac/weave/engine/operations"
)

func executor() *operations.Executor {
	return operations.New("test", engine.Options{})
}

func successResult(t *testing.T, value any) any {
	t.Helper()
	envelope, ok := value.(contractv1.SuccessEnvelope)
	if !ok {
		t.Fatalf("expected success envelope, got %T", value)
	}
	if !envelope.OK {
		t.Fatalf("expected ok=true")
	}
	return envelope.Result
}

func failureResult(t *testing.T, value any) contractv1.OperationError {
	t.Helper()
	envelope, ok := value.(contractv1.FailureEnvelope)
	if !ok {
		t.Fatalf("expected failure envelope, got %T", value)
	}
	if envelope.OK {
		t.Fatalf("expected ok=false")
	}
	return envelope.Error
}

func fixture() string {
	return filepath.Join("..", "..", "contract/v1/golden/bundles/okf-bundle")
}

func TestContractDiscovery(t *testing.T) {
	result := successResult(t, executor().Execute(context.Background(), contractv1.OperationRequest{
		Op: contractv1.OpContract,
	}))
	info, ok := result.(contractv1.ContractInfo)
	if !ok {
		t.Fatalf("expected ContractInfo, got %T", result)
	}
	if info.Contract != operations.ContractVersion || info.CLI != operations.CLIVersion {
		t.Fatalf("unexpected versions: %+v", info)
	}
	if len(info.Operations) != 9 {
		t.Fatalf("expected 9 operations, got %d", len(info.Operations))
	}
	if len(info.Errors) == 0 {
		t.Fatal("expected error codes")
	}
}

func TestParseAndGraph(t *testing.T) {
	exec := executor()

	bundleResult := successResult(t, exec.Execute(context.Background(), contractv1.OperationRequest{
		Op:   contractv1.OpParse,
		Path: fixture(),
	}))
	bundle, ok := bundleResult.(*contractv1.Bundle)
	if !ok {
		t.Fatalf("expected *Bundle, got %T", bundleResult)
	}
	if len(bundle.Concepts) != 5 {
		t.Fatalf("expected 5 concepts, got %d", len(bundle.Concepts))
	}

	graphResult := successResult(t, exec.Execute(context.Background(), contractv1.OperationRequest{
		Op:   contractv1.OpGraph,
		Path: fixture(),
	}))
	graph, ok := graphResult.(contractv1.Graph)
	if !ok {
		t.Fatalf("expected Graph, got %T", graphResult)
	}
	if len(graph.Nodes) != 5 {
		t.Fatalf("expected 5 nodes, got %d", len(graph.Nodes))
	}
	if len(graph.Edges) != 5 {
		t.Fatalf("expected 5 edges, got %d", len(graph.Edges))
	}
}

func TestNavigationOperations(t *testing.T) {
	exec := executor()
	ctx := context.Background()
	path := fixture()

	findResult := successResult(t, exec.Execute(ctx, contractv1.OperationRequest{
		Op: contractv1.OpFind, Path: path, Query: "graph",
	}))
	findResults, ok := findResult.([]contractv1.FindResult)
	if !ok {
		t.Fatalf("expected []FindResult, got %T", findResult)
	}
	if len(findResults) != 2 {
		t.Fatalf("expected 2 find results, got %d", len(findResults))
	}

	listResult := successResult(t, exec.Execute(ctx, contractv1.OperationRequest{
		Op: contractv1.OpList, Path: path, Types: []string{"Concept"},
	}))
	listResults, ok := listResult.([]contractv1.ListResult)
	if !ok {
		t.Fatalf("expected []ListResult, got %T", listResult)
	}
	if len(listResults) != 3 {
		t.Fatalf("expected 3 concepts, got %d", len(listResults))
	}

	showResult := successResult(t, exec.Execute(ctx, contractv1.OperationRequest{
		Op: contractv1.OpShow, Path: path, Target: "concepts/okf-bundle",
	}))
	show, ok := showResult.(*contractv1.ShowResult)
	if !ok {
		t.Fatalf("expected *ShowResult, got %T", showResult)
	}
	if show.ID != "concepts/okf-bundle" {
		t.Fatalf("unexpected show id: %s", show.ID)
	}

	backlinksResult := successResult(t, exec.Execute(ctx, contractv1.OperationRequest{
		Op: contractv1.OpBacklinks, Path: path, Target: "concepts/search-index",
	}))
	backlinks, ok := backlinksResult.(*contractv1.BacklinksResult)
	if !ok {
		t.Fatalf("expected *BacklinksResult, got %T", backlinksResult)
	}
	if len(backlinks.Backlinks) != 1 {
		t.Fatalf("expected 1 backlink, got %d", len(backlinks.Backlinks))
	}

	pathResult := successResult(t, exec.Execute(ctx, contractv1.OperationRequest{
		Op: contractv1.OpPath, Path: path, From: "concepts/okf-bundle", To: "tables/users",
	}))
	route, ok := pathResult.(*contractv1.PathResult)
	if !ok {
		t.Fatalf("expected *PathResult, got %T", pathResult)
	}
	if len(route.Path) != 2 {
		t.Fatalf("expected 2 hops, got %v", route.Path)
	}
}

func TestOperationErrors(t *testing.T) {
	exec := executor()
	ctx := context.Background()
	path := fixture()

	if err := failureResult(t, exec.Execute(ctx, contractv1.OperationRequest{
		Op: contractv1.OpShow, Path: path, Target: "missing",
	})); err.Code != contractv1.ErrConceptNotFound {
		t.Fatalf("expected CONCEPT_NOT_FOUND, got %s", err.Code)
	}

	if err := failureResult(t, exec.Execute(ctx, contractv1.OperationRequest{
		Op: contractv1.OpPath, Path: path, From: "concepts/okf-bundle", To: "concepts/missing-type",
	})); err.Code != contractv1.ErrPathNotFound {
		t.Fatalf("expected PATH_NOT_FOUND, got %s", err.Code)
	}

	if err := failureResult(t, exec.Execute(ctx, contractv1.OperationRequest{
		Op: contractv1.OpParse,
	})); err.Code != contractv1.ErrInvalidRequest {
		t.Fatalf("expected INVALID_REQUEST, got %s", err.Code)
	}

	if err := failureResult(t, exec.Execute(ctx, contractv1.OperationRequest{
		Op: contractv1.OperationName("nope"), Path: path,
	})); err.Code != contractv1.ErrInvalidRequest {
		t.Fatalf("expected INVALID_REQUEST, got %s", err.Code)
	}

	if err := failureResult(t, exec.Execute(ctx, contractv1.OperationRequest{
		Op: contractv1.OpParse, Path: filepath.Join("..", "does-not-exist"),
	})); err.Code != contractv1.ErrInvalidRoot {
		t.Fatalf("expected INVALID_ROOT, got %s", err.Code)
	}
}

func TestVersion(t *testing.T) {
	result := successResult(t, executor().Execute(context.Background(), contractv1.OperationRequest{
		Op: contractv1.OpVersion,
	}))
	version, ok := result.(contractv1.VersionInfo)
	if !ok {
		t.Fatalf("expected VersionInfo, got %T", result)
	}
	if version.Contract != operations.ContractVersion {
		t.Fatalf("unexpected contract: %s", version.Contract)
	}
}
