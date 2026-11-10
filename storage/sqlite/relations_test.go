package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
)

func TestLoadBundleRestoresCodeRelations(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	root := filepath.Join(t.TempDir(), "repo")
	bundle := &contractv1.Bundle{
		Root: root,
		Concepts: []contractv1.OkfConcept{
			{
				ID: "main.go", Path: "main.go", Type: "Code", Keywords: []string{"go"},
				Body: "package main\n", BodyText: true,
				Relations: []contractv1.ConceptRelation{{To: "app.py", Kind: contractv1.RelationImports}},
			},
			{ID: "app.py", Path: "app.py", Type: "Code", Keywords: []string{"python"}, Body: "print()\n", BodyText: true},
		},
		Validation: contractv1.Validation{Valid: true},
	}
	if _, err := store.Ingest(context.Background(), bundle, IngestOptions{Mode: "strict"}); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.LoadBundle(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	var relations []contractv1.ConceptRelation
	for _, concept := range loaded.Concepts {
		if concept.ID == "main.go" {
			relations = concept.Relations
		}
	}
	if len(relations) != 1 || relations[0].To != "app.py" || relations[0].Kind != contractv1.RelationImports {
		t.Fatalf("unexpected relations: %+v", relations)
	}
}
