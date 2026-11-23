package sqlite_test

import (
	"context"
	"path/filepath"
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
	sqlitestore "github.com/prulloac/weave/storage/sqlite"
)

func filterStore(t *testing.T) (*sqlitestore.Store, string) {
	t.Helper()
	ctx := context.Background()
	root := t.TempDir()
	store, err := sqlitestore.Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	stable := contractv1.StatusStable
	draft := contractv1.StatusDraft
	bundle := &contractv1.Bundle{
		Root: root,
		Concepts: []contractv1.OkfConcept{
			{ID: "a", Path: "a.md", Type: "Concept", Status: &stable, Tags: []string{"storage"}, Keywords: []string{"alpha", "storage"}, Body: "# A\n\nalpha storage\n"},
			{ID: "b", Path: "b.md", Type: "Concept", Status: &draft, Tags: []string{"storage"}, Keywords: []string{"alpha"}, Body: "# B\n\nalpha\n"},
			{ID: "c", Path: "c.md", Type: "Table", Status: &stable, Tags: []string{"data"}, Keywords: []string{"alpha", "storage"}, Body: "# C\n\nalpha storage\n"},
		},
		Validation: contractv1.Validation{Valid: true},
	}
	if _, err := store.Ingest(ctx, bundle, sqlitestore.IngestOptions{Mode: "strict"}); err != nil {
		t.Fatal(err)
	}
	return store, root
}

func TestKeywordHitsWithFilters(t *testing.T) {
	store, root := filterStore(t)
	hits, err := store.KeywordHitsWithFilters(context.Background(), root, "alpha", 10,
		[]contractv1.SearchFilter{{Field: "type", Value: "Table"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].ID != "c" {
		t.Fatalf("hits = %+v, want c", hits)
	}
}

func TestKeywordHitsFiltersCombine(t *testing.T) {
	store, root := filterStore(t)
	hits, err := store.KeywordHitsWithFilters(context.Background(), root, "alpha", 10,
		[]contractv1.SearchFilter{
			{Field: "type", Value: "Concept"},
			{Field: "status", Value: "stable"},
		})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].ID != "a" {
		t.Fatalf("hits = %+v, want a", hits)
	}
}

func TestKeywordHitsFilterKeywordCaseInsensitive(t *testing.T) {
	store, root := filterStore(t)
	hits, err := store.KeywordHitsWithFilters(context.Background(), root, "alpha", 10,
		[]contractv1.SearchFilter{{Field: "keyword", Value: "STORAGE"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 2 {
		t.Fatalf("hits = %+v, want two", hits)
	}
}

func TestFuzzyHitsWithFilters(t *testing.T) {
	store, root := filterStore(t)
	hits, err := store.FuzzyHitsWithFilters(context.Background(), root, "alpha", 10,
		[]contractv1.SearchFilter{{Field: "type", Value: "Table"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].ID != "c" {
		t.Fatalf("hits = %+v, want c", hits)
	}
}

func TestSearchBM25WithFilters(t *testing.T) {
	store, root := filterStore(t)
	hits, err := store.SearchBM25WithFilters(context.Background(), root, "alpha", 10,
		[]contractv1.SearchFilter{{Field: "tag", Value: "data"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].ID != "c" {
		t.Fatalf("hits = %+v, want c", hits)
	}
}

func TestSearchFiltersUnknownField(t *testing.T) {
	store, root := filterStore(t)
	if _, err := store.KeywordHitsWithFilters(context.Background(), root, "alpha", 10,
		[]contractv1.SearchFilter{{Field: "colour", Value: "red"}}); err == nil {
		t.Fatal("expected an error")
	}
}
