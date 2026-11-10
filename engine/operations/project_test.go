package operations

import (
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine/extensions"
)

func TestApplyProjectResponseSkipsOmittedFiles(t *testing.T) {
	bundle := &contractv1.Bundle{Concepts: []contractv1.OkfConcept{
		{ID: "a.go", Path: "a.go", Type: "Code", ContentKind: "blob"},
		{ID: "b.gen.go", Path: "b.gen.go", Type: "Code", ContentKind: "blob"},
		{ID: "index.md", Path: "index.md", Type: "Index"},
	}}
	response := extensions.ProjectAnalysisResponse{
		Documents: []extensions.ProjectDocument{{ID: "a.go"}},
	}
	applyProjectResponse(bundle, response, []contractv1.OkfConcept{
		{ID: "a.go", Path: "a.go"},
		{ID: "b.gen.go", Path: "b.gen.go"},
	})
	ids := map[string]bool{}
	for _, concept := range bundle.Concepts {
		ids[concept.ID] = true
	}
	if !ids["a.go"] || !ids["index.md"] {
		t.Fatalf("expected retained documents, got %+v", bundle.Concepts)
	}
	if ids["b.gen.go"] {
		t.Fatal("omitted code file must be removed")
	}
}

func TestApplyProjectExternalThreshold(t *testing.T) {
	bundle := &contractv1.Bundle{Concepts: []contractv1.OkfConcept{
		{ID: "a.go", Path: "a.go", Type: "Code"},
		{ID: "b.go", Path: "b.go", Type: "Code"},
	}}
	response := extensions.ProjectAnalysisResponse{
		Documents: []extensions.ProjectDocument{{ID: "a.go"}, {ID: "b.go"}},
		ExternalDocuments: []extensions.ProjectDocument{
			{ID: "_code/external/go/fmt"},
			{ID: "_code/external/go/os"},
		},
		Relations: []extensions.ProjectRelation{
			{From: "a.go", To: "_code/external/go/fmt", Kind: "imports"},
			{From: "a.go", To: "_code/external/go/os", Kind: "imports"},
			{From: "b.go", To: "_code/external/go/os", Kind: "imports"},
		},
	}
	applyProjectResponse(bundle, response, bundle.Concepts)

	ids := map[string]bool{}
	for _, concept := range bundle.Concepts {
		ids[concept.ID] = true
	}
	if ids["_code/external/go/fmt"] {
		t.Fatal("single-reference external target must not become a document")
	}
	if !ids["_code/external/go/os"] {
		t.Fatal("repeated external target must become a document")
	}
	for _, concept := range bundle.Concepts {
		for _, relation := range concept.Relations {
			if relation.To == "_code/external/go/fmt" {
				t.Fatal("single-reference external relation must be dropped")
			}
		}
	}
}
