package operations

import (
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine/extensions"
)

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
	applyProjectResponse(bundle, response)

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
