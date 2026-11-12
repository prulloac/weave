package sqlite_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	sqlitestore "github.com/prulloac/weave/storage/sqlite"
)

func TestSchemaV8HasSources(t *testing.T) {
	path := filepath.Join(t.TempDir(), "index.db")
	store, err := sqlitestore.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	db := storeTestDB(t, path)
	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != 9 {
		t.Fatalf("user_version = %d, want 9", version)
	}
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM sources").Scan(&count); err != nil {
		t.Fatalf("sources table missing: %v", err)
	}
}

func insertBundle(t *testing.T, db *sql.DB, root string) {
	t.Helper()
	if _, err := db.Exec(`
		INSERT INTO bundles (bundle_root, mode, index_json, validation_json, asset_types_json, updated_at)
		VALUES (?, 'strict', '[]', '{}', '[]', 1)`, root); err != nil {
		t.Fatal(err)
	}
}

func TestBackfillSourcesDerivesAliases(t *testing.T) {
	path := filepath.Join(t.TempDir(), "index.db")
	store, err := sqlitestore.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	db := storeTestDB(t, path)
	insertBundle(t, db, "/home/pabl1nsky/notes")
	insertBundle(t, db, "/home/pabl1nsky/work/notes")

	if err := store.BackfillSources(context.Background()); err != nil {
		t.Fatal(err)
	}
	records, err := store.ListSources(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 {
		t.Fatalf("sources = %d, want 2", len(records))
	}
	aliases := map[string]string{}
	for _, record := range records {
		if record.Status != sqlitestore.SourceActive {
			t.Fatalf("status = %s, want active", record.Status)
		}
		aliases[record.Alias] = record.Path
	}
	if aliases["notes"] != "/home/pabl1nsky/notes" {
		t.Fatalf("aliases = %v", aliases)
	}
	if aliases["notes-2"] != "/home/pabl1nsky/work/notes" {
		t.Fatalf("aliases = %v", aliases)
	}
}

func TestRegistryLifecycle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "index.db")
	store, err := sqlitestore.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()
	if err := store.UpsertSource(ctx, sqlitestore.SourceRecord{
		Alias: "docs", Path: "/home/docs", Status: sqlitestore.SourceIndexing,
	}); err != nil {
		t.Fatal(err)
	}
	record, ok, err := store.SourceByAlias(ctx, "docs")
	if err != nil || !ok {
		t.Fatalf("source by alias: ok=%v err=%v", ok, err)
	}
	if record.Status != sqlitestore.SourceIndexing {
		t.Fatalf("status = %s, want indexing", record.Status)
	}

	if err := store.SetSourceStatus(ctx, "docs", sqlitestore.SourceActive); err != nil {
		t.Fatal(err)
	}
	records, err := store.ListSources(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].Status != sqlitestore.SourceActive {
		t.Fatalf("records = %+v", records)
	}

	if err := store.RemoveSource(ctx, "docs"); err != nil {
		t.Fatal(err)
	}
	if record, ok, _ := store.SourceByAlias(ctx, "docs"); !ok || record.Status != sqlitestore.SourceRemoved {
		t.Fatalf("remove: record = %+v ok=%v", record, ok)
	}

	if err := store.PruneSource(ctx, "docs"); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := store.SourceByAlias(ctx, "docs"); ok {
		t.Fatal("expected pruned source to be gone")
	}
}

func TestSourceNotFound(t *testing.T) {
	path := filepath.Join(t.TempDir(), "index.db")
	store, err := sqlitestore.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if err := store.SetSourceStatus(context.Background(), "missing", sqlitestore.SourceActive); err != sqlitestore.ErrSourceNotFound {
		t.Fatalf("err = %v, want ErrSourceNotFound", err)
	}
}
