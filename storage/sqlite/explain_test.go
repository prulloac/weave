package sqlite_test

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine"
	sqlitestore "github.com/prulloac/weave/storage/sqlite"
)

func TestExplainFuzzyMatchesInMemory(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	store, err := sqlitestore.Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	bundle := sampleBundle(root)
	if _, err := store.Ingest(ctx, bundle, sqlitestore.IngestOptions{Mode: "strict"}); err != nil {
		t.Fatal(err)
	}
	eng := engine.FromBundle(bundle)
	want, err := eng.Explain("alpha", contractv1.SearchFuzzy, 10, "a")
	if err != nil {
		t.Fatal(err)
	}
	got, err := store.ExplainFuzzy(ctx, root, "alpha", "a")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want.Fuzzy) {
		t.Fatalf("fuzzy evidence mismatch\ngot:  %+v\nwant: %+v", got, want.Fuzzy)
	}
}
