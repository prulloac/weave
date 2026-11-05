package sqlite_test

import (
	"context"
	"path/filepath"
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
	sqlitestore "github.com/prulloac/weave/storage/sqlite"
)

func TestEnrichmentAppliesBlobMetadata(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "index.db")
	store, err := sqlitestore.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	bundle := &contractv1.Bundle{
		Root: root,
		Concepts: []contractv1.OkfConcept{{
			ID: "photo.png", Path: "photo.png", Keywords: []string{},
			ContentKind: "blob", MediaType: "image/png",
			BodyBytes: []byte("fake image"), ContentMetadata: map[string]any{"v": 1},
		}},
		Validation: contractv1.Validation{Valid: true},
	}
	report, err := store.Ingest(ctx, bundle, sqlitestore.IngestOptions{
		Mode: "strict",
		Enricher: func(ctx context.Context, concept contractv1.OkfConcept) (sqlitestore.EnrichmentApply, error) {
			return sqlitestore.EnrichmentApply{
				Enricher:    "image",
				Keywords:    []string{"shop"},
				Title:       "Shop",
				Description: "A storefront",
				Text:        "Opening hours",
				TextSource:  "ocr",
				ContentMetadata: map[string]any{
					"v":          1,
					"attributes": map[string]any{"width": 10, "height": 20},
				},
				Data:     []byte(`{"name":"image"}`),
				Warnings: 2,
			}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Warnings < 2 {
		t.Fatalf("warnings = %d", report.Warnings)
	}
	loaded, err := store.LoadBundle(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	concept := loaded.Concepts[0]
	if concept.Title == nil || *concept.Title != "Shop" {
		t.Fatalf("title = %+v", concept.Title)
	}
	if concept.Description == nil || *concept.Description != "A storefront" {
		t.Fatalf("description = %+v", concept.Description)
	}
	if concept.ContentMetadata["text"] != "Opening hours" ||
		concept.ContentMetadata["text_source"] != "ocr" {
		t.Fatalf("metadata = %+v", concept.ContentMetadata)
	}
	if len(concept.Keywords) != 1 || concept.Keywords[0] != "shop" {
		t.Fatalf("keywords = %v", concept.Keywords)
	}
}
