package operations

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine/extensions"
)

func writeProjectAnalyzer(t *testing.T, root, manifest string) {
	t.Helper()
	path := filepath.Join(root, ".weave", "bin", "weave-code")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "#!/bin/sh\nif [ \"$1\" = \"--describe\" ]; then\nprintf '%s' '" + manifest + "'\nexit 0\nfi\ncat >/dev/null\nprintf '%s' '{\"v\":\"weave.cli/v1\",\"documents\":[]}'\n"
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	trustOperationsExtension(t, root, "code")
}

func TestProjectExtensionRefusesUnversionedAnalyzer(t *testing.T) {
	previous := extensions.CoreVersion
	extensions.CoreVersion = "0.11.0"
	t.Cleanup(func() { extensions.CoreVersion = previous })

	root := t.TempDir()
	writeProjectAnalyzer(t, root, `{"v":"weave.extension/v1","name":"code","analysis_modes":["project"]}`)
	executor := Executor{}
	_, _, _, err := executor.projectExtension(context.Background(), contractv1.OperationRequest{
		Path:     root,
		Enricher: "code",
	})
	if !errors.Is(err, extensions.ErrRequirementUnmet) {
		t.Fatalf("expected ErrRequirementUnmet for an unversioned analyzer, got %v", err)
	}
}

func TestProjectExtensionAcceptsVersionedAnalyzer(t *testing.T) {
	previous := extensions.CoreVersion
	extensions.CoreVersion = "0.11.0"
	t.Cleanup(func() { extensions.CoreVersion = previous })

	root := t.TempDir()
	writeProjectAnalyzer(t, root, `{"v":"weave.extension/v1","name":"code","version":"0.11.0","analysis_modes":["project"]}`)
	executor := Executor{}
	_, _, project, err := executor.projectExtension(context.Background(), contractv1.OperationRequest{
		Path:     root,
		Enricher: "code",
	})
	if err != nil {
		t.Fatalf("versioned analyzer refused: %v", err)
	}
	if !project {
		t.Fatal("expected project analysis mode")
	}
}

func TestApplyProjectResponseUsesDeclaredTypes(t *testing.T) {
	bundle := &contractv1.Bundle{Concepts: []contractv1.OkfConcept{
		{ID: "a.custom", Path: "a.custom", Type: "Document", ContentKind: "text"},
	}}
	response := extensions.ProjectAnalysisResponse{
		Documents: []extensions.ProjectDocument{{
			ID: "a.custom", Type: "CustomDocument", ContentKind: "blob", Title: "Custom",
		}},
	}
	if err := applyProjectResponse(bundle, response, bundle.Concepts[:1]); err != nil {
		t.Fatal(err)
	}
	for _, concept := range bundle.Concepts {
		if concept.ID == "a.custom" {
			if concept.Type != "CustomDocument" || concept.ContentKind != "blob" {
				t.Fatalf("concept = %+v", concept)
			}
			return
		}
	}
	t.Fatal("a.custom missing")
}

func TestApplyProjectResponseRequiresType(t *testing.T) {
	bundle := &contractv1.Bundle{}
	response := extensions.ProjectAnalysisResponse{
		Documents: []extensions.ProjectDocument{{ID: "_code/external/go/fmt"}},
	}
	if err := applyProjectResponse(bundle, response, nil); err == nil {
		t.Fatal("expected a missing type error")
	}
}

func TestApplyProjectResponseSkipsOmittedFiles(t *testing.T) {
	bundle := &contractv1.Bundle{Concepts: []contractv1.OkfConcept{
		{ID: "a.go", Path: "a.go", Type: "Code", ContentKind: "blob"},
		{ID: "b.gen.go", Path: "b.gen.go", Type: "Code", ContentKind: "blob"},
		{ID: "index.md", Path: "index.md", Type: "Index"},
	}}
	response := extensions.ProjectAnalysisResponse{
		Documents: []extensions.ProjectDocument{{ID: "a.go", Type: "Code", ContentKind: "blob"}},
	}
	if err := applyProjectResponse(bundle, response, []contractv1.OkfConcept{
		{ID: "a.go", Path: "a.go"},
		{ID: "b.gen.go", Path: "b.gen.go"},
	}); err != nil {
		t.Fatal(err)
	}
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
		Documents: []extensions.ProjectDocument{
			{ID: "a.go", Type: "Code", ContentKind: "blob"},
			{ID: "b.go", Type: "Code", ContentKind: "blob"},
		},
		ExternalDocuments: []extensions.ProjectDocument{
			{ID: "_code/external/go/fmt", Type: "CodeExternal", ContentKind: "symbol"},
			{ID: "_code/external/go/os", Type: "CodeExternal", ContentKind: "symbol"},
		},
		Relations: []extensions.ProjectRelation{
			{From: "a.go", To: "_code/external/go/fmt", Kind: "imports"},
			{From: "a.go", To: "_code/external/go/os", Kind: "imports"},
			{From: "b.go", To: "_code/external/go/os", Kind: "imports"},
		},
	}
	if err := applyProjectResponse(bundle, response, bundle.Concepts); err != nil {
		t.Fatal(err)
	}

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
