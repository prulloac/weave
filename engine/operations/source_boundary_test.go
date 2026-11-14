package operations

import (
	"strings"
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
)

func TestApplySyntheticSourceOutput(t *testing.T) {
	bundle := &contractv1.Bundle{Concepts: []contractv1.OkfConcept{
		{ID: "README.md", Path: "README.md", Type: "Document", ContentKind: "text"},
		{ID: "notes", Path: "notes.md", Type: "Document", ContentKind: "text"},
		{ID: "web/app.ts", Path: "web/app.ts", Type: "Document", ContentKind: "text"},
		{ID: "web/pkg/lib.ts", Path: "web/pkg/lib.ts", Type: "Document", ContentKind: "text"},
	}}
	rootScope := ""
	webScope := "web"
	packageScope := "web/pkg"
	source := &contractv1.SourceContext{
		Provider: "git", URI: "https://example.com/acme/mono.git", Ref: "main",
		Commit:             "8f3c2a1d4b5e6f708192a3b4c5d6e7f8091a2b3c",
		DocumentNamespaces: []string{"_repo", "_project/", "_package/"},
		RelationKinds:      []string{"depends_on"},
		Documents: []contractv1.SyntheticDocument{
			{ID: "_repo", Type: "Repository", ContentKind: "blob", Title: "Mono", Scope: &rootScope, Consumes: []string{"README.md"}},
			{ID: "_project/web", Type: "Project", ContentKind: "symbol", Title: "Web", Scope: &webScope},
			{ID: "_project/web/pkg", Type: "Project", ContentKind: "symbol", Title: "Pkg", Scope: &packageScope},
			{ID: "_package/npm/react", Type: "ExternalPackage", ContentKind: "symbol", Title: "react"},
		},
		Relations: []contractv1.SyntheticRelation{
			{From: "_repo", To: "_project/web", Kind: contractv1.RelationContains},
			{From: "_project/web", To: "_package/npm/react", Kind: contractv1.RelationDependsOn},
		},
	}
	if err := applySourceRepository(bundle, source); err != nil {
		t.Fatal(err)
	}
	byID := map[string]contractv1.OkfConcept{}
	for _, concept := range bundle.Concepts {
		byID[concept.ID] = concept
	}
	if _, ok := byID["README.md"]; ok {
		t.Fatal("consumed README should be removed")
	}
	for _, id := range []string{"_repo", "_project/web", "_project/web/pkg", "_package/npm/react"} {
		if byID[id].ID == "" {
			t.Fatalf("missing %s: %+v", id, bundle.Concepts)
		}
	}
	assertRelation(t, byID["_repo"], "_project/web", contractv1.RelationContains)
	assertRelation(t, byID["_project/web"], "_package/npm/react", contractv1.RelationDependsOn)
	assertRelation(t, byID["_repo"], "notes", contractv1.RelationContains)
	assertRelation(t, byID["_repo"], "web/app.ts", contractv1.RelationContains)
	assertRelation(t, byID["_project/web"], "web/app.ts", contractv1.RelationContains)
	assertRelation(t, byID["_project/web/pkg"], "web/pkg/lib.ts", contractv1.RelationContains)
	if hasRelation(byID["_package/npm/react"], "web/app.ts", contractv1.RelationContains) {
		t.Fatal("package must not contain files")
	}
}

func TestApplySyntheticSourceOutputRejectsUnknownNamespace(t *testing.T) {
	bundle := &contractv1.Bundle{}
	source := &contractv1.SourceContext{
		Provider:           "git",
		DocumentNamespaces: []string{"_project/"},
		Documents:          []contractv1.SyntheticDocument{{ID: "_custom/node", Type: "Node"}},
	}
	err := applySourceRepository(bundle, source)
	if err == nil || !strings.Contains(err.Error(), "outside declared namespaces") {
		t.Fatalf("err = %v", err)
	}
}

func TestApplySyntheticSourceOutputRejectsUnknownRelationKind(t *testing.T) {
	bundle := &contractv1.Bundle{Concepts: []contractv1.OkfConcept{
		{ID: "notes", Path: "notes.md", Type: "Document"},
	}}
	source := &contractv1.SourceContext{
		Provider:           "git",
		DocumentNamespaces: []string{"_repo"},
		Documents:          []contractv1.SyntheticDocument{{ID: "_repo", Type: "Repository"}},
		Relations: []contractv1.SyntheticRelation{
			{From: "_repo", To: "notes", Kind: contractv1.RelationKind("custom_edge")},
		},
	}
	err := applySourceRepository(bundle, source)
	if err == nil || !strings.Contains(err.Error(), "not registered") {
		t.Fatalf("err = %v", err)
	}
}

func TestApplySyntheticSourceOutputResolvesPathTargets(t *testing.T) {
	bundle := &contractv1.Bundle{Concepts: []contractv1.OkfConcept{
		{ID: "web/app.ts", Path: "web/app.ts", Type: "Document"},
	}}
	scope := "web"
	source := &contractv1.SourceContext{
		Provider:           "git",
		DocumentNamespaces: []string{"_project/"},
		Documents:          []contractv1.SyntheticDocument{{ID: "_project/web", Type: "Project", Scope: &scope}},
		Relations: []contractv1.SyntheticRelation{
			{From: "_project/web", ToPath: "web/app.ts", Kind: contractv1.RelationContains},
			{From: "_project/web", ToPath: "web/missing.ts", Kind: contractv1.RelationContains},
		},
	}
	if err := applySourceRepository(bundle, source); err != nil {
		t.Fatal(err)
	}
	byID := map[string]contractv1.OkfConcept{}
	for _, concept := range bundle.Concepts {
		byID[concept.ID] = concept
	}
	assertRelation(t, byID["_project/web"], "web/app.ts", contractv1.RelationContains)
	if len(bundle.Validation.Warnings) == 0 || !strings.Contains(strings.Join(bundle.Validation.Warnings, " "), "web/missing.ts") {
		t.Fatalf("warnings = %+v", bundle.Validation.Warnings)
	}
}
