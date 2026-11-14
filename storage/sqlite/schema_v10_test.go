package sqlite

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
)

func TestSchemaV10Artifacts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "index.db")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	var version int
	if err := store.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != 10 {
		t.Fatalf("user_version = %d, want 10", version)
	}

	root := filepath.Join(t.TempDir(), "bundle")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Ingest(context.Background(), &contractv1.Bundle{Root: root}, IngestOptions{
		Artifacts: []ArtifactRecord{{
			Path: "_fixture/hello.json", MediaType: "application/json", Content: []byte(`{"hello":"world"}`),
		}},
		RelationDefaults: []string{"custom_edge"},
	}); err != nil {
		t.Fatal(err)
	}
	records, err := store.ListArtifacts(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].Path != "_fixture/hello.json" ||
		string(records[0].Content) != `{"hello":"world"}` {
		t.Fatalf("artifacts = %+v", records)
	}
	defaults, err := store.ListRelationDefaults(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(defaults) != 1 || defaults[0] != "custom_edge" {
		t.Fatalf("defaults = %+v", defaults)
	}
}
