package okf_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/prulloac/weave/engine/okf"
)

func TestParseCacheHitsAndInvalidates(t *testing.T) {
	root := t.TempDir()
	cacheDir := t.TempDir()
	file := filepath.Join(root, "a.md")
	if err := os.WriteFile(file, []byte("---\ntype: Concept\ntitle: A\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	run := func() (int, string) {
		hits := 0
		bundle, err := okf.ParseBundle(context.Background(), root, okf.Options{
			Cache:      true,
			CacheDir:   cacheDir,
			OnCacheHit: func(string) { hits++ },
		})
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if len(bundle.Concepts) != 1 {
			t.Fatalf("expected 1 concept, got %d", len(bundle.Concepts))
		}
		title := ""
		if bundle.Concepts[0].Title != nil {
			title = *bundle.Concepts[0].Title
		}
		return hits, title
	}

	if hits, title := run(); hits != 0 || title != "A" {
		t.Fatalf("first parse: hits=%d title=%q", hits, title)
	}
	if hits, title := run(); hits != 1 || title != "A" {
		t.Fatalf("second parse: hits=%d title=%q", hits, title)
	}

	if err := os.WriteFile(file, []byte("---\ntype: Concept\ntitle: Updated title\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if hits, title := run(); hits != 0 || title != "Updated title" {
		t.Fatalf("updated parse: hits=%d title=%q", hits, title)
	}
}

func TestParseCacheNoCache(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "a.md")
	if err := os.WriteFile(file, []byte("---\ntype: Concept\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	hits := 0
	for range 2 {
		if _, err := okf.ParseBundle(context.Background(), root, okf.Options{
			OnCacheHit: func(string) { hits++ },
		}); err != nil {
			t.Fatalf("parse: %v", err)
		}
	}
	if hits != 0 {
		t.Fatalf("expected no cache hits, got %d", hits)
	}
}
