package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestSchemaV9SourceProvenance(t *testing.T) {
	path := filepath.Join(t.TempDir(), "index.db")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	metadata := json.RawMessage(`{"v":1,"uri":"git+https://example.com/team/weave.git","ref_name":"main","commit_sha":"8f3c2a1d4b5e6f708192a3b4c5d6e7f8091a2b3c"}`)
	if err := store.UpsertSource(context.Background(), SourceRecord{
		Alias:      "core",
		Path:       "/cache/weave/sources/core/worktree",
		Status:     SourceActive,
		Generation: 1,
		Provider:   "git",
		Metadata:   metadata,
	}); err != nil {
		t.Fatal(err)
	}
	record, ok, err := store.SourceByAlias(context.Background(), "core")
	if err != nil || !ok {
		t.Fatalf("source: ok=%v err=%v", ok, err)
	}
	if record.Provider != "git" {
		t.Fatalf("provider = %q", record.Provider)
	}
	if string(record.Metadata) != string(metadata) {
		t.Fatalf("metadata = %s", record.Metadata)
	}
	found, ok, err := store.SourceByInput(context.Background(), "git+https://example.com/team/weave.git")
	if err != nil || !ok || found.Alias != "core" {
		t.Fatalf("source by input: ok=%v alias=%q err=%v", ok, found.Alias, err)
	}
}

func TestMigrateV8ToV9BackfillsSourceProvenance(t *testing.T) {
	path := filepath.Join(t.TempDir(), "index.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`
CREATE TABLE sources (
    alias TEXT PRIMARY KEY,
    path TEXT NOT NULL UNIQUE,
    status TEXT NOT NULL DEFAULT 'indexing',
    generation INTEGER NOT NULL DEFAULT 0,
    last_synced INTEGER NOT NULL DEFAULT 0,
    content_hash TEXT NOT NULL DEFAULT ''
);
INSERT INTO sources (alias, path, status) VALUES ('notes', '/notes', 'active');`); err != nil {
		t.Fatal(err)
	}
	if err := migrateV8toV9(db); err != nil {
		t.Fatal(err)
	}
	store := &Store{db: db}
	record, ok, err := store.SourceByAlias(context.Background(), "notes")
	if err != nil || !ok {
		t.Fatalf("source: ok=%v err=%v", ok, err)
	}
	if record.Provider != "path" {
		t.Fatalf("provider = %q, want path", record.Provider)
	}
	if string(record.Metadata) != "{}" {
		t.Fatalf("metadata = %s, want {}", record.Metadata)
	}
}
