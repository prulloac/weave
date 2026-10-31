package sqlite

import "database/sql"

const schemaVersion = 3

func userVersion(db *sql.DB) (int, error) {
	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return 0, err
	}
	return version, nil
}

func migrateV1toV2(db *sql.DB) error {
	for _, statement := range []string{
		"ALTER TABLE documents ADD COLUMN privacy_provider TEXT DEFAULT ''",
		"ALTER TABLE documents ADD COLUMN privacy_key_id TEXT DEFAULT ''",
		"ALTER TABLE documents ADD COLUMN redacted INTEGER DEFAULT 0",
		"ALTER TABLE documents ADD COLUMN privacy_spans_json TEXT DEFAULT '[]'",
	} {
		if _, err := db.Exec(statement); err != nil {
			return err
		}
	}
	return nil
}

func migrateV2toV3(db *sql.DB) error {
	for _, statement := range []string{
		"ALTER TABLE documents ADD COLUMN privacy_provider_name TEXT DEFAULT ''",
		"ALTER TABLE documents ADD COLUMN privacy_key_version INTEGER DEFAULT 0",
	} {
		if _, err := db.Exec(statement); err != nil {
			return err
		}
	}
	return nil
}

func createSchema(db *sql.DB) error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS bundles (
    bundle_root TEXT PRIMARY KEY,
    mode TEXT NOT NULL DEFAULT 'strict',
    okf_version TEXT,
    index_json TEXT NOT NULL,
    validation_json TEXT NOT NULL,
    updated_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS documents (
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
CREATE TABLE IF NOT EXISTS edges (
    bundle_root TEXT NOT NULL,
    source_id TEXT NOT NULL,
    target_id TEXT NOT NULL,
    edge_type TEXT NOT NULL,
    dangling INTEGER DEFAULT 0,
    PRIMARY KEY (bundle_root, source_id, target_id, edge_type),
    FOREIGN KEY (bundle_root, source_id)
        REFERENCES documents(bundle_root, id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_edges_bundle_target
    ON edges(bundle_root, target_id);
CREATE VIRTUAL TABLE IF NOT EXISTS fts_documents USING fts5 (
    bundle_root UNINDEXED,
    id UNINDEXED,
    title,
    keywords,
    description,
    body,
    tokenize = 'porter unicode61'
);
CREATE VIRTUAL TABLE IF NOT EXISTS fts_trigrams USING fts5 (
    bundle_root UNINDEXED,
    id UNINDEXED,
    field UNINDEXED,
    token,
    tokenize = 'trigram'
);
CREATE TABLE IF NOT EXISTS enrichments (
    bundle_root TEXT NOT NULL,
    doc_id TEXT NOT NULL,
    enricher_id TEXT NOT NULL,
    data_json TEXT NOT NULL,
    PRIMARY KEY (bundle_root, doc_id, enricher_id),
    FOREIGN KEY (bundle_root, doc_id)
        REFERENCES documents(bundle_root, id) ON DELETE CASCADE
);
`)
	return err
}
