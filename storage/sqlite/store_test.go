package sqlite_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	contractv1 "github.com/prulloac/weave/contract/v1"
	sqlitestore "github.com/prulloac/weave/storage/sqlite"
)

func TestIngestLoadBundleRoundTrip(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "index.db")
	store, err := sqlitestore.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	bundle := sampleBundle(root)
	report, err := store.Ingest(ctx, bundle, sqlitestore.IngestOptions{Mode: "strict"})
	if err != nil {
		t.Fatal(err)
	}
	if report.DB != dbPath || report.Bundle != root || report.Mode != "strict" {
		t.Fatalf("unexpected report: %+v", report)
	}
	if report.Documents != 2 || report.Edges != 1 {
		t.Fatalf("unexpected counts: %+v", report)
	}

	has, err := store.HasBundle(ctx, root)
	if err != nil || !has {
		t.Fatalf("HasBundle=%t err=%v", has, err)
	}
	loaded, err := store.LoadBundle(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Root != root || len(loaded.Concepts) != 2 || len(loaded.Index) != 2 {
		t.Fatalf("unexpected loaded bundle: %+v", loaded)
	}
	if loaded.Concepts[0].ID != "a" || loaded.Concepts[0].Title == nil || *loaded.Concepts[0].Title != "A" {
		t.Fatalf("concept a was not preserved: %+v", loaded.Concepts[0])
	}
	if loaded.Concepts[0].Body != "# A\n\nSee [B](b.md).\n" {
		t.Fatalf("body was not preserved: %q", loaded.Concepts[0].Body)
	}
	if len(loaded.Concepts[0].Links) != 1 || !loaded.Concepts[0].Links[0].ResolvesInBundle {
		t.Fatalf("links were not preserved: %+v", loaded.Concepts[0].Links)
	}
	if !loaded.Validation.Valid {
		t.Fatal("validation was not preserved")
	}
}

func TestIngestReplacesOnlyTargetBundle(t *testing.T) {
	ctx := context.Background()
	store, err := sqlitestore.Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	rootA := t.TempDir()
	rootB := t.TempDir()
	bundleA := sampleBundle(rootA)
	bundleB := sampleBundle(rootB)
	bundleB.Concepts = []contractv1.OkfConcept{{
		ID: "b", Path: "b.md", Type: "Concept", Keywords: []string{"b"}, Body: "# B",
	}}
	bundleB.Index = []contractv1.IndexEntry{{Title: "B", Target: "b.md"}}

	if _, err := store.Ingest(ctx, bundleA, sqlitestore.IngestOptions{Mode: "strict"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Ingest(ctx, bundleB, sqlitestore.IngestOptions{Mode: "strict"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Ingest(ctx, bundleA, sqlitestore.IngestOptions{Mode: "strict"}); err != nil {
		t.Fatal(err)
	}

	loadedA, err := store.LoadBundle(ctx, rootA)
	if err != nil {
		t.Fatal(err)
	}
	if len(loadedA.Concepts) != 2 {
		t.Fatalf("bundle A concepts=%d, want 2", len(loadedA.Concepts))
	}
	loadedB, err := store.LoadBundle(ctx, rootB)
	if err != nil {
		t.Fatal(err)
	}
	if len(loadedB.Concepts) != 1 {
		t.Fatalf("bundle B concepts=%d, want 1", len(loadedB.Concepts))
	}
}

func TestMissingBundle(t *testing.T) {
	ctx := context.Background()
	store, err := sqlitestore.Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	has, err := store.HasBundle(ctx, "/missing")
	if err != nil || has {
		t.Fatalf("HasBundle=%t err=%v", has, err)
	}
	if _, err := store.LoadBundle(ctx, "/missing"); !errors.Is(err, sqlitestore.ErrBundleNotIndexed) {
		t.Fatalf("expected ErrBundleNotIndexed, got %v", err)
	}
}

func TestSchemaVersionMismatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "index.db")
	store, err := sqlitestore.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec("PRAGMA user_version = 2"); err != nil {
		t.Fatal(err)
	}
	raw.Close()

	if _, err := sqlitestore.Open(path); err == nil {
		t.Fatal("expected schema version error")
	}
}

func TestDefaultPathUsesHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	path, err := sqlitestore.DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, ".weave", "index.db"); path != want {
		t.Fatalf("path=%q, want %q", path, want)
	}
}

func sampleBundle(root string) *contractv1.Bundle {
	titleA := "A"
	titleB := "B"
	resolved := "b.md"
	return &contractv1.Bundle{
		Root: root,
		Concepts: []contractv1.OkfConcept{
			{
				ID: "a", Path: "a.md", Type: "Concept", Title: &titleA,
				Keywords: []string{"alpha"}, Tags: []string{"tag"},
				Frontmatter: map[string]any{"type": "Concept", "keywords": []any{"alpha"}},
				Body:        "# A\n\nSee [B](b.md).\n",
				Links: []contractv1.OkfLink{{
					Raw: "[B](b.md)", Target: "b.md", Resolved: &resolved, ResolvesInBundle: true,
				}},
			},
			{
				ID: "b", Path: "b.md", Type: "Concept", Title: &titleB,
				Keywords:    []string{"beta"},
				Frontmatter: map[string]any{"type": "Concept", "keywords": []any{"beta"}},
				Body:        "# B",
			},
		},
		Index: []contractv1.IndexEntry{
			{Title: "A", Target: "a.md"},
			{Title: "B", Target: "b.md"},
		},
		Validation: contractv1.Validation{
			Valid: true,
			Concepts: []contractv1.ConceptValidation{
				{ID: "a", HasFrontmatter: true, HasType: true, HasKeywords: true},
				{ID: "b", HasFrontmatter: true, HasType: true, HasKeywords: true},
			},
		},
	}
}
