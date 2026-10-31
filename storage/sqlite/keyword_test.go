package sqlite_test

import (
	"context"
	"path/filepath"
	"testing"

	sqlitestore "github.com/prulloac/weave/storage/sqlite"
)

func TestKeywordHitsMetadataOnly(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	store, err := sqlitestore.Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.Ingest(ctx, sampleBundle(root), sqlitestore.IngestOptions{Mode: "strict"}); err != nil {
		t.Fatal(err)
	}

	hits, err := store.KeywordHits(ctx, root, "alpha", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].ID != "a" || hits[0].Score != 1 {
		t.Fatalf("unexpected exact hits: %+v", hits)
	}
	if len(hits[0].MatchedKeywords) != 1 || hits[0].MatchedKeywords[0] != "alpha" {
		t.Fatalf("unexpected matched keywords: %+v", hits[0])
	}

	hits, err = store.KeywordHits(ctx, root, "alp", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].ID != "a" || hits[0].Score != 0.75 {
		t.Fatalf("unexpected prefix hits: %+v", hits)
	}

	hits, err = store.KeywordHits(ctx, root, "alpha beta", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 0 {
		t.Fatalf("AND semantics expected no hits, got %+v", hits)
	}

	hits, err = store.KeywordHits(ctx, root, "beta", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].ID != "b" {
		t.Fatalf("unexpected beta hits: %+v", hits)
	}
}
