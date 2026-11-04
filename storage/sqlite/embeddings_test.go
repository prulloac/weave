package sqlite_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
	sqlitestore "github.com/prulloac/weave/storage/sqlite"
)

func embeddingBundle(root string) *contractv1.Bundle {
	concepts := make([]contractv1.OkfConcept, 0, 3)
	validations := make([]contractv1.ConceptValidation, 0, 3)
	for _, id := range []string{"alpha", "beta", "gamma"} {
		title := id
		concepts = append(concepts, contractv1.OkfConcept{
			ID: id, Path: id + ".md", Type: "Concept", Title: &title,
			Keywords:    []string{id},
			Frontmatter: map[string]any{"type": "Concept", "keywords": []any{id}},
			Body:        "# " + id,
		})
		validations = append(validations, contractv1.ConceptValidation{
			ID: id, HasFrontmatter: true, HasType: true, HasKeywords: true,
		})
	}
	return &contractv1.Bundle{
		Root:       root,
		Concepts:   concepts,
		Validation: contractv1.Validation{Valid: true, Concepts: validations},
	}
}

func fakeEmbedder(text, _ string) (sqlitestore.Embedding, error) {
	vector := []float32{0, 0, 1}
	switch {
	case strings.Contains(text, "alpha"):
		vector = []float32{1, 0, 0}
	case strings.Contains(text, "beta"):
		vector = []float32{0, 1, 0}
	}
	return sqlitestore.Embedding{Model: "test-model", Dim: 3, Vector: vector}, nil
}

func TestIngestStoresEmbeddingsAndSemanticHits(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "index.db")
	store, err := sqlitestore.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	_, err = store.Ingest(ctx, embeddingBundle(root), sqlitestore.IngestOptions{
		Mode: "strict",
		Embedder: func(ctx context.Context, text, mediaType string) (sqlitestore.Embedding, error) {
			return fakeEmbedder(text, mediaType)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	spaces, err := store.EmbeddingSpaces(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if len(spaces) != 1 || spaces[0].Model != "test-model" || spaces[0].Dim != 3 || spaces[0].Rows != 3 {
		t.Fatalf("spaces = %+v", spaces)
	}
	hits, err := store.SemanticHits(ctx, root, "test-model", []float32{1, 0, 0}, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 3 || hits[0].DocID != "alpha" {
		t.Fatalf("hits = %+v", hits)
	}
}

func TestEmbeddingFailureKeepsDocument(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "index.db")
	store, err := sqlitestore.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	report, err := store.Ingest(ctx, embeddingBundle(root), sqlitestore.IngestOptions{
		Mode: "strict",
		Embedder: func(ctx context.Context, text, mediaType string) (sqlitestore.Embedding, error) {
			return sqlitestore.Embedding{}, errors.New("embedder down")
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Documents != 3 || report.Warnings < 3 {
		t.Fatalf("report = %+v", report)
	}
	if _, err := store.SemanticHits(ctx, root, "test-model", []float32{1, 0, 0}, 3); err == nil {
		t.Fatal("expected missing space")
	}
}

func TestRebuildAndDropEmbeddingSpace(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "index.db")
	store, err := sqlitestore.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.Ingest(ctx, embeddingBundle(root), sqlitestore.IngestOptions{
		Mode: "strict",
		Embedder: func(ctx context.Context, text, mediaType string) (sqlitestore.Embedding, error) {
			return fakeEmbedder(text, mediaType)
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.RebuildEmbeddingSpace(ctx, root, "test-model", 3); err != nil {
		t.Fatal(err)
	}
	hits, err := store.SemanticHits(ctx, root, "test-model", []float32{0, 1, 0}, 1)
	if err != nil || len(hits) != 1 || hits[0].DocID != "beta" {
		t.Fatalf("hits=%+v err=%v", hits, err)
	}
	if err := store.DropEmbeddingSpace(ctx, root, "test-model", 3, false); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SemanticHits(ctx, root, "test-model", []float32{0, 1, 0}, 1); err == nil {
		t.Fatal("expected dropped space to be missing")
	}
}
