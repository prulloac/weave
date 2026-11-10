package graph

import (
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
)

func relationBundle() *contractv1.Bundle {
	return &contractv1.Bundle{
		Root: "/repo",
		Concepts: []contractv1.OkfConcept{
			{
				ID: "src/main.go", Path: "src/main.go", Type: "Code", Keywords: []string{"go"},
				Relations: []contractv1.ConceptRelation{
					{To: "src/server.go", Kind: contractv1.RelationImports},
					{To: "src/main.go#main", Kind: contractv1.RelationDeclares},
					{To: "src/server.go#Run", Kind: contractv1.RelationCalls},
				},
			},
			{
				ID: "src/server.go", Path: "src/server.go", Type: "Code", Keywords: []string{"go"},
				Relations: []contractv1.ConceptRelation{
					{To: "src/server.go#Run", Kind: contractv1.RelationDeclares},
				},
			},
			{ID: "src/main.go#main", Path: "src/main.go#main", Type: "CodeSymbol", Keywords: []string{"main"}},
			{ID: "src/server.go#Run", Path: "src/server.go#Run", Type: "CodeSymbol", Keywords: []string{"Run"}},
		},
	}
}

func TestBuildAddsExtensionRelations(t *testing.T) {
	view := Build(relationBundle())
	edges := SortedEdges(view)
	if len(edges) != 4 {
		t.Fatalf("expected 4 edges, got %+v", edges)
	}
	want := map[string]bool{
		"src/main.go|src/server.go|imports":        true,
		"src/main.go|src/main.go#main|declares":    true,
		"src/main.go|src/server.go#Run|calls":      true,
		"src/server.go#Run|calls":                  false,
		"src/server.go|src/server.go#Run|declares": true,
	}
	for _, edge := range edges {
		key := edge.From + "|" + edge.To + "|" + edge.Source
		if _, ok := want[key]; !ok {
			t.Fatalf("unexpected edge %s", key)
		}
	}
}

func TestTraversalDefaultsExcludeCalls(t *testing.T) {
	view := Build(&contractv1.Bundle{
		Root: "/repo",
		Concepts: []contractv1.OkfConcept{
			{
				ID: "main", Path: "main.go", Type: "Code", Keywords: []string{"go"},
				Relations: []contractv1.ConceptRelation{{To: "run", Kind: contractv1.RelationCalls}},
			},
			{ID: "run", Path: "run.go", Type: "Code", Keywords: []string{"go"}},
		},
	})
	if path := FindPath(view, "main", "run"); path != nil {
		t.Fatalf("default traversal must not follow calls, got %v", path)
	}
	path := FindPathWithOptions(view, "main", "run", TraversalOptions{
		Relations: []contractv1.RelationKind{contractv1.RelationCalls},
	})
	if len(path) != 2 || path[0] != "main" || path[1] != "run" {
		t.Fatalf("unexpected call path: %v", path)
	}
}

func TestFilterEdges(t *testing.T) {
	edges := SortedEdges(Build(relationBundle()))
	filtered := FilterEdges(edges, []contractv1.RelationKind{contractv1.RelationImports})
	if len(filtered) != 1 || filtered[0].Source != "imports" {
		t.Fatalf("unexpected filtered edges: %+v", filtered)
	}
	if len(FilterEdges(edges, nil)) != len(edges) {
		t.Fatal("empty filter must keep every edge")
	}
}
