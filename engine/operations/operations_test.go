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
	result := successResult(t, executor().Execute(context.Background(), contractv1.OperationRequest{NoDB: true,
		Op: contractv1.OpContract,
	}))
	info, ok := result.(contractv1.ContractInfo)
	if !ok {
		t.Fatalf("expected ContractInfo, got %T", result)
	}
	if info.Contract != operations.ContractVersion || info.CLI != operations.CLIVersion {
		t.Fatalf("unexpected versions: %+v", info)
	}
	if len(info.Operations) != 14 {
		t.Fatalf("expected 14 operations, got %d", len(info.Operations))
	}
	if len(info.Errors) == 0 {
		t.Fatal("expected error codes")
	}
}

func TestAudit(t *testing.T) {
	result := successResult(t, executor().Execute(context.Background(), contractv1.OperationRequest{NoDB: true,
		Op:     contractv1.OpAudit,
		Path:   fixture(),
		FailOn: contractv1.AuditFailOnError,
	}))
	report, ok := result.(contractv1.AuditReport)
	if !ok {
		t.Fatalf("expected AuditReport, got %T", result)
	}
	if report.FailOn != contractv1.AuditFailOnError {
		t.Fatalf("unexpected fail_on: %s", report.FailOn)
	}
	if report.Summary.Concepts != 5 {
		t.Fatalf("expected 5 concepts, got %d", report.Summary.Concepts)
	}
	found := false
	for _, finding := range report.Findings {
		if finding.Code == contractv1.AuditMissingType {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected MISSING_TYPE, got %+v", report.Findings)
	}
}

func TestRelate(t *testing.T) {
	depth := 2
	result := successResult(t, executor().Execute(context.Background(), contractv1.OperationRequest{NoDB: true,
		Op:        contractv1.OpRelate,
		Path:      filepath.Join("..", "..", "cmd/weave/testdata/relate/chain"),
		Target:    "concepts/x",
		Depth:     &depth,
		Direction: contractv1.RelateBoth,
	}))
	relate, ok := result.(contractv1.RelateResult)
	if !ok {
		t.Fatalf("expected RelateResult, got %T", result)
	}
	if relate.Root != "concepts/x" || relate.Depth != 2 || relate.Direction != contractv1.RelateBoth {
		t.Fatalf("unexpected header: %+v", relate)
	}
	if len(relate.Nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(relate.Nodes))
	}
}

func TestExplain(t *testing.T) {
	topK := 10
	result := successResult(t, executor().Execute(context.Background(), contractv1.OperationRequest{NoDB: true,
		Op:     contractv1.OpExplain,
		Path:   filepath.Join("..", "search", "testdata", "bundle"),
		Query:  "oauth",
		Target: "oauth-token-rotation",
		Mode:   contractv1.SearchHybrid,
		TopK:   &topK,
	}))
	explain, ok := result.(contractv1.ExplainResult)
	if !ok {
		t.Fatalf("expected ExplainResult, got %T", result)
	}
	if !explain.Found || explain.ID != "oauth-token-rotation" {
		t.Fatalf("unexpected explanation: %+v", explain)
	}
}

func TestIngestAndReadFromDB(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "index.db")
	root := filepath.Join("..", "..", "cmd/weave/testdata/audit/clean")

	result := successResult(t, executor().Execute(context.Background(), contractv1.OperationRequest{
		Op:     contractv1.OpIngest,
		Path:   root,
		DB:     dbPath,
		Strict: true,
	}))
	report, ok := result.(contractv1.IngestReport)
	if !ok {
		t.Fatalf("expected IngestReport, got %T", result)
	}
	if report.Documents != 2 || report.Edges != 2 || report.Mode != "strict" {
		t.Fatalf("unexpected report: %+v", report)
	}

	parsed := successResult(t, executor().Execute(context.Background(), contractv1.OperationRequest{
		Op:   contractv1.OpParse,
		Path: root,
		DB:   dbPath,
	}))
	bundle, ok := parsed.(*contractv1.Bundle)
	if !ok {
		t.Fatalf("expected Bundle, got %T", parsed)
	}
	if len(bundle.Concepts) != 2 {
		t.Fatalf("expected 2 concepts, got %d", len(bundle.Concepts))
	}
}

func TestParseAndGraph(t *testing.T) {
	exec := executor()

	bundleResult := successResult(t, exec.Execute(context.Background(), contractv1.OperationRequest{NoDB: true,
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

	graphResult := successResult(t, exec.Execute(context.Background(), contractv1.OperationRequest{NoDB: true,
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

	searchResult := successResult(t, exec.Execute(ctx, contractv1.OperationRequest{NoDB: true,
		Op: contractv1.OpSearch, Path: path, Query: "graph", Mode: contractv1.SearchHybrid,
	}))
	response, ok := searchResult.(contractv1.SearchResponse)
	if !ok {
		t.Fatalf("expected SearchResponse, got %T", searchResult)
	}
	if len(response.Hybrid) == 0 {
		t.Fatal("expected hybrid results")
	}
	if response.Hybrid[0].ID != "concepts/node-graph-engine" {
		t.Fatalf("unexpected top result: %+v", response.Hybrid[0])
	}

	listResult := successResult(t, exec.Execute(ctx, contractv1.OperationRequest{NoDB: true,
		Op: contractv1.OpList, Path: path, Types: []string{"Concept"},
	}))
	listResults, ok := listResult.([]contractv1.ListResult)
	if !ok {
		t.Fatalf("expected []ListResult, got %T", listResult)
	}
	if len(listResults) != 3 {
		t.Fatalf("expected 3 concepts, got %d", len(listResults))
	}

	showResult := successResult(t, exec.Execute(ctx, contractv1.OperationRequest{NoDB: true,
		Op: contractv1.OpShow, Path: path, Target: "concepts/okf-bundle",
	}))
	show, ok := showResult.(*contractv1.ShowResult)
	if !ok {
		t.Fatalf("expected *ShowResult, got %T", showResult)
	}
	if show.ID != "concepts/okf-bundle" {
		t.Fatalf("unexpected show id: %s", show.ID)
	}

	backlinksResult := successResult(t, exec.Execute(ctx, contractv1.OperationRequest{NoDB: true,
		Op: contractv1.OpBacklinks, Path: path, Target: "concepts/search-index",
	}))
	backlinks, ok := backlinksResult.(*contractv1.BacklinksResult)
	if !ok {
		t.Fatalf("expected *BacklinksResult, got %T", backlinksResult)
	}
	if len(backlinks.Backlinks) != 1 {
		t.Fatalf("expected 1 backlink, got %d", len(backlinks.Backlinks))
	}

	pathResult := successResult(t, exec.Execute(ctx, contractv1.OperationRequest{NoDB: true,
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

	if err := failureResult(t, exec.Execute(ctx, contractv1.OperationRequest{NoDB: true,
		Op: contractv1.OpShow, Path: path, Target: "missing",
	})); err.Code != contractv1.ErrConceptNotFound {
		t.Fatalf("expected CONCEPT_NOT_FOUND, got %s", err.Code)
	}

	if err := failureResult(t, exec.Execute(ctx, contractv1.OperationRequest{NoDB: true,
		Op: contractv1.OpPath, Path: path, From: "concepts/okf-bundle", To: "concepts/missing-type",
	})); err.Code != contractv1.ErrPathNotFound {
		t.Fatalf("expected PATH_NOT_FOUND, got %s", err.Code)
	}

	if err := failureResult(t, exec.Execute(ctx, contractv1.OperationRequest{NoDB: true,
		Op: contractv1.OpParse,
	})); err.Code != contractv1.ErrInvalidRequest {
		t.Fatalf("expected INVALID_REQUEST, got %s", err.Code)
	}

	if err := failureResult(t, exec.Execute(ctx, contractv1.OperationRequest{NoDB: true,
		Op: contractv1.OperationName("nope"), Path: path,
	})); err.Code != contractv1.ErrInvalidRequest {
		t.Fatalf("expected INVALID_REQUEST, got %s", err.Code)
	}

	if err := failureResult(t, exec.Execute(ctx, contractv1.OperationRequest{NoDB: true,
		Op: contractv1.OpParse, Path: filepath.Join("..", "does-not-exist"),
	})); err.Code != contractv1.ErrInvalidRoot {
		t.Fatalf("expected INVALID_ROOT, got %s", err.Code)
	}
}

func TestVersion(t *testing.T) {
	result := successResult(t, executor().Execute(context.Background(), contractv1.OperationRequest{NoDB: true,
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
