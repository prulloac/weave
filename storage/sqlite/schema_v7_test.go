package sqlite_test

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/prulloac/weave/engine/compression"
	sqlitestore "github.com/prulloac/weave/storage/sqlite"
)

const schemaV6 = `
CREATE TABLE bundles (
    bundle_root TEXT PRIMARY KEY,
    mode TEXT NOT NULL DEFAULT 'strict',
    okf_version TEXT,
    index_json TEXT NOT NULL,
    validation_json TEXT NOT NULL,
    updated_at INTEGER NOT NULL
);
CREATE TABLE documents (
    bundle_root TEXT NOT NULL,
    id TEXT NOT NULL,
    file_path TEXT NOT NULL,
    content_hash TEXT NOT NULL,
    title TEXT NOT NULL,
    description TEXT,
    doc_type TEXT,
    keywords_json TEXT NOT NULL,
    frontmatter_json TEXT NOT NULL,
    tags_json TEXT NOT NULL,
    status TEXT,
    links_json TEXT NOT NULL,
    body_compressed BLOB NOT NULL,
    body_codec TEXT NOT NULL DEFAULT 'none',
    privacy_policy TEXT DEFAULT 'none',
    privacy_provider TEXT DEFAULT '',
    privacy_key_id TEXT DEFAULT '',
    privacy_provider_name TEXT DEFAULT '',
    privacy_key_version INTEGER DEFAULT 0,
    redacted INTEGER DEFAULT 0,
    privacy_spans_json TEXT DEFAULT '[]',
    updated_at INTEGER NOT NULL,
    PRIMARY KEY (bundle_root, id),
    UNIQUE (bundle_root, file_path),
    FOREIGN KEY (bundle_root) REFERENCES bundles(bundle_root) ON DELETE CASCADE
);
CREATE VIRTUAL TABLE fts_documents USING fts5 (
    bundle_root UNINDEXED,
    id UNINDEXED,
    title,
    keywords,
    description,
    body,
    tokenize = 'porter unicode61'
);
CREATE VIRTUAL TABLE fts_trigrams USING fts5 (
    bundle_root UNINDEXED,
    id UNINDEXED,
    field UNINDEXED,
    token,
    tokenize = 'trigram'
);
`

func TestMigrateV6ToV7(t *testing.T) {
	path := filepath.Join(t.TempDir(), "index.db")
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(schemaV6); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`
		INSERT INTO bundles (bundle_root, mode, index_json, validation_json, updated_at)
		VALUES ('/notes', 'strict', '[]', '{}', 1)`); err != nil {
		t.Fatal(err)
	}
	plain := []byte("plain body")
	long := []byte("compressible body that is longer than the one kilobyte threshold; " +
		"compressible body that is longer than the one kilobyte threshold; " +
		"compressible body that is longer than the one kilobyte threshold; " +
		"compressible body that is longer than the one kilobyte threshold; " +
		"compressible body that is longer than the one kilobyte threshold; " +
		"compressible body that is longer than the one kilobyte threshold; " +
		"compressible body that is longer than the one kilobyte threshold; " +
		"compressible body that is longer than the one kilobyte threshold; " +
		"compressible body that is longer than the one kilobyte threshold; " +
		"compressible body that is longer than the one kilobyte threshold; " +
		"compressible body that is longer than the one kilobyte threshold; " +
		"compressible body that is longer than the one kilobyte threshold; " +
		"compressible body that is longer than the one kilobyte threshold.")
	encoded, codec, err := compression.Encode(compression.CodecZstd, long, false)
	if err != nil {
		t.Fatal(err)
	}
	for id, body := range map[string]struct {
		blob  []byte
		codec string
	}{
		"plain": {blob: plain, codec: "none"},
		"long":  {blob: encoded, codec: string(codec)},
	} {
		if _, err := raw.Exec(`
			INSERT INTO documents (
				bundle_root, id, file_path, content_hash, title, keywords_json,
				frontmatter_json, tags_json, links_json, body_compressed, body_codec, updated_at
			) VALUES (?, ?, ?, ?, ?, '[]', '{}', '[]', '[]', ?, ?, 1)`,
			"/notes", id, id+".md", id+"-hash", id, body.blob, body.codec); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := raw.Exec("PRAGMA user_version = 6"); err != nil {
		t.Fatal(err)
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}

	store, err := sqlitestore.Open(path)
	if err != nil {
		t.Fatalf("migrate v6: %v", err)
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
	var assetTypes string
	if err := db.QueryRow("SELECT asset_types_json FROM bundles").Scan(&assetTypes); err != nil {
		t.Fatalf("asset_types_json: %v", err)
	}
	if assetTypes != "[]" {
		t.Fatalf("asset_types_json = %q, want []", assetTypes)
	}
	for _, id := range []string{"plain", "long"} {
		var (
			bodyRaw, bodyCompressed, bodyCodec, kind, media, metadata sql.NullString
			size, storedSize                                          int
		)
		if err := db.QueryRow(`
			SELECT body_raw, body_compressed, body_codec, content_kind, media_type,
			       content_metadata, size, stored_size
			FROM documents WHERE id = ?`, id).Scan(
			&bodyRaw, &bodyCompressed, &bodyCodec, &kind, &media, &metadata, &size, &storedSize); err != nil {
			t.Fatalf("row %s: %v", id, err)
		}
		if bodyRaw.String == "" || bodyCompressed.Valid || bodyCodec.Valid {
			t.Fatalf("row %s was not split: raw=%q compressed=%v codec=%v",
				id, bodyRaw.String, bodyCompressed.Valid, bodyCodec.Valid)
		}
		if kind.String != "document" || media.String != "text/markdown" || metadata.String != `{"v":1}` {
			t.Fatalf("row %s metadata = %q/%q/%q", id, kind.String, media.String, metadata.String)
		}
		if size != len(bodyRaw.String) || storedSize != len(bodyRaw.String) {
			t.Fatalf("row %s sizes = %d/%d, want %d", id, size, storedSize, len(bodyRaw.String))
		}
	}
	for _, table := range []string{"document_embeddings", "embedding_indexes"} {
		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil {
			t.Fatalf("%s missing: %v", table, err)
		}
	}
}

func TestMigrateV6RequiresLZ4Codec(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	path := filepath.Join(t.TempDir(), "index.db")
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(schemaV6); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`
		INSERT INTO bundles (bundle_root, mode, index_json, validation_json, updated_at)
		VALUES ('/notes', 'strict', '[]', '{}', 1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`
		INSERT INTO documents (
			bundle_root, id, file_path, content_hash, title, keywords_json,
			frontmatter_json, tags_json, links_json, body_compressed, body_codec, updated_at
		) VALUES ('/notes', 'legacy', 'legacy.md', 'hash', 'Legacy', '[]', '{}', '[]', '[]',
		          x'00', 'lz4', 1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec("PRAGMA user_version = 6"); err != nil {
		t.Fatal(err)
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := sqlitestore.Open(path); err == nil {
		t.Fatal("expected missing lz4 codec to abort migration")
	} else if !strings.Contains(err.Error(), "CODEC_UNAVAILABLE") &&
		!strings.Contains(err.Error(), "codec unavailable") {
		t.Fatalf("unexpected error: %v", err)
	}
	db := storeTestDB(t, path)
	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != 6 {
		t.Fatalf("user_version = %d, want 6", version)
	}
}

func TestV7BodyConstraints(t *testing.T) {
	path := filepath.Join(t.TempDir(), "index.db")
	store, err := sqlitestore.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	db := storeTestDB(t, path)
	if _, err := db.Exec(`
		INSERT INTO bundles (bundle_root, mode, index_json, validation_json, asset_types_json, updated_at)
		VALUES ('/notes', 'strict', '[]', '{}', '[]', 1)`); err != nil {
		t.Fatal(err)
	}

	base := `INSERT INTO documents (
		bundle_root, id, file_path, content_hash, title, keywords_json,
		frontmatter_json, tags_json, links_json, body_raw, body_compressed,
		body_codec, size, stored_size, updated_at
	) VALUES ('/notes', ?, ?, 'hash', 'title', '[]', '{}', '[]', '[]', ?, ?, ?, 0, 0, 1)`
	if _, err := db.Exec(base, "both", "both.md", "raw", []byte("compressed"), "zstd"); err == nil {
		t.Fatal("expected both bodies to fail")
	}
	if _, err := db.Exec(base, "neither", "neither.md", nil, nil, nil); err == nil {
		t.Fatal("expected neither body to fail")
	}
	if _, err := db.Exec(base, "raw-codec", "raw-codec.md", "raw", nil, "zstd"); err == nil {
		t.Fatal("expected body_codec on raw body to fail")
	}
	if _, err := db.Exec(base, "ok", "ok.md", "raw", nil, nil); err != nil {
		t.Fatalf("valid raw row rejected: %v", err)
	}
}
