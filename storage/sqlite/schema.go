package sqlite

import (
	"context"
	"database/sql"
	"fmt"
)

const schemaVersion = 7

var migrations = []struct {
	version int
	apply   func(*sql.DB) error
}{
	{2, migrateV1toV2},
	{3, migrateV2toV3},
	{4, migrateV3toV4},
	{5, migrateV4toV5},
	{6, migrateV5toV6},
	{7, migrateV6toV7},
}

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

func migrateV3toV4(db *sql.DB) error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS search_tokens (
    bundle_root TEXT NOT NULL,
    field TEXT NOT NULL,
    token TEXT NOT NULL,
    postings BLOB NOT NULL,
    PRIMARY KEY (bundle_root, field, token)
);`)
	return err
}

func migrateV4toV5(db *sql.DB) error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS search_vocabulary (
    bundle_root TEXT NOT NULL,
    field TEXT NOT NULL,
    token TEXT NOT NULL,
    PRIMARY KEY (bundle_root, field, token)
);`)
	return err
}

func migrateV5toV6(db *sql.DB) error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS extension_data (
    bundle_root TEXT NOT NULL,
    extension TEXT NOT NULL,
    doc_id TEXT NOT NULL,
    key TEXT NOT NULL,
    sha256 TEXT NOT NULL,
    size INTEGER NOT NULL,
    media_type TEXT,
    metadata_json TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    PRIMARY KEY (bundle_root, extension, doc_id, key)
);
CREATE INDEX IF NOT EXISTS idx_extension_data_doc
    ON extension_data(bundle_root, doc_id);
CREATE INDEX IF NOT EXISTS idx_extension_data_blob
    ON extension_data(bundle_root, sha256);`)
	return err
}

type v6Document struct {
	bundleRoot, id, filePath, contentHash, title string
	description, docType, status                 sql.NullString
	keywords, frontmatter, tags, links           string
	body                                         []byte
	codec                                        string
	policy, provider, keyID, providerName        sql.NullString
	keyVersion, redacted                         sql.NullInt64
	spans                                        sql.NullString
	updatedAt                                    int64
}

func migrateV6toV7(db *sql.DB) error {
	if _, err := db.Exec("PRAGMA foreign_keys=OFF"); err != nil {
		return err
	}
	defer func() { _, _ = db.Exec("PRAGMA foreign_keys=ON") }()

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var assetColumn int
	if err := tx.QueryRow(
		"SELECT COUNT(*) FROM pragma_table_info('bundles') WHERE name = 'asset_types_json'",
	).Scan(&assetColumn); err != nil {
		return err
	}
	if assetColumn == 0 {
		if _, err := tx.Exec(
			"ALTER TABLE bundles ADD COLUMN asset_types_json TEXT NOT NULL DEFAULT '[]'"); err != nil {
			return err
		}
	}

	rows, err := tx.Query(`
		SELECT bundle_root, id, file_path, content_hash, title, description, doc_type,
		       keywords_json, frontmatter_json, tags_json, status, links_json,
		       body_compressed, body_codec, privacy_policy, privacy_provider,
		       privacy_key_id, privacy_provider_name, privacy_key_version, redacted,
		       privacy_spans_json, updated_at
		FROM documents`)
	if err != nil {
		return err
	}
	documents := make([]v6Document, 0)
	for rows.Next() {
		var document v6Document
		if err := rows.Scan(
			&document.bundleRoot, &document.id, &document.filePath, &document.contentHash,
			&document.title, &document.description, &document.docType, &document.keywords,
			&document.frontmatter, &document.tags, &document.status, &document.links,
			&document.body, &document.codec, &document.policy, &document.provider,
			&document.keyID, &document.providerName, &document.keyVersion, &document.redacted,
			&document.spans, &document.updatedAt,
		); err != nil {
			rows.Close()
			return err
		}
		documents = append(documents, document)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	if _, err := tx.Exec(`
CREATE TABLE documents_v7 (
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
    content_kind TEXT NOT NULL DEFAULT 'document',
    media_type TEXT NOT NULL DEFAULT 'text/markdown',
    content_metadata TEXT NOT NULL DEFAULT '{}',
    body_raw TEXT,
    body_compressed BLOB,
    body_codec TEXT,
    size INTEGER NOT NULL DEFAULT 0,
    stored_size INTEGER NOT NULL DEFAULT 0,
    privacy_policy TEXT DEFAULT 'none',
    privacy_provider TEXT DEFAULT '',
    privacy_key_id TEXT DEFAULT '',
    privacy_provider_name TEXT DEFAULT '',
    privacy_key_version INTEGER DEFAULT 0,
    redacted INTEGER DEFAULT 0,
    privacy_spans_json TEXT DEFAULT '[]',
    updated_at INTEGER NOT NULL,
    CHECK ((body_raw IS NULL) != (body_compressed IS NULL)),
    CHECK ((body_raw IS NOT NULL AND body_codec IS NULL)
        OR (body_compressed IS NOT NULL AND body_codec IS NOT NULL)),
    PRIMARY KEY (bundle_root, id),
    UNIQUE (bundle_root, file_path),
    FOREIGN KEY (bundle_root) REFERENCES bundles(bundle_root) ON DELETE CASCADE
);`); err != nil {
		return err
	}

	for _, document := range documents {
		decoded, err := decodeStoredBody(context.Background(), document.bundleRoot, document.codec, document.body)
		if err != nil {
			return fmt.Errorf("migrate v6 document %s: %w", document.id, err)
		}
		if _, err := tx.Exec(`
			INSERT INTO documents_v7 (
				bundle_root, id, file_path, content_hash, title, description, doc_type,
				keywords_json, frontmatter_json, tags_json, status, links_json,
				content_kind, media_type, content_metadata, body_raw, body_compressed,
				body_codec, size, stored_size, privacy_policy, privacy_provider,
				privacy_key_id, privacy_provider_name, privacy_key_version, redacted,
				privacy_spans_json, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'document', 'text/markdown', '{"v":1}',
			          ?, NULL, NULL, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			document.bundleRoot, document.id, document.filePath, document.contentHash,
			document.title, document.description, document.docType, document.keywords,
			document.frontmatter, document.tags, document.status, document.links,
			string(decoded), len(decoded), len(decoded), document.policy, document.provider,
			document.keyID, document.providerName, document.keyVersion, document.redacted,
			document.spans, document.updatedAt); err != nil {
			return err
		}
	}

	if _, err := tx.Exec("DROP TABLE documents"); err != nil {
		return err
	}
	if _, err := tx.Exec("ALTER TABLE documents_v7 RENAME TO documents"); err != nil {
		return err
	}
	if err := createEmbeddingTables(tx); err != nil {
		return err
	}

	roots, err := bundleRoots(tx)
	if err != nil {
		return err
	}
	for _, root := range roots {
		if err := populateFTS(context.Background(), tx, root); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func createEmbeddingTables(tx *sql.Tx) error {
	_, err := tx.Exec(`
CREATE TABLE IF NOT EXISTS document_embeddings (
    bundle_root TEXT NOT NULL,
    doc_id TEXT NOT NULL,
    model TEXT NOT NULL,
    dim INTEGER NOT NULL,
    vector BLOB NOT NULL,
    content_hash TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    PRIMARY KEY (bundle_root, doc_id, model, dim),
    FOREIGN KEY (bundle_root, doc_id)
        REFERENCES documents(bundle_root, id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS embedding_indexes (
    model TEXT NOT NULL,
    dim INTEGER NOT NULL,
    metric TEXT NOT NULL,
    element_type TEXT NOT NULL,
    table_name TEXT NOT NULL,
    active INTEGER NOT NULL DEFAULT 1,
    created_at INTEGER NOT NULL,
    PRIMARY KEY (model, dim, metric, element_type)
);`)
	return err
}

func bundleRoots(tx *sql.Tx) ([]string, error) {
	rows, err := tx.Query("SELECT bundle_root FROM bundles ORDER BY bundle_root")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	roots := make([]string, 0)
	for rows.Next() {
		var root string
		if err := rows.Scan(&root); err != nil {
			return nil, err
		}
		roots = append(roots, root)
	}
	return roots, rows.Err()
}

func createSchema(db *sql.DB) error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS bundles (
    bundle_root TEXT PRIMARY KEY,
    mode TEXT NOT NULL DEFAULT 'strict',
    okf_version TEXT,
    index_json TEXT NOT NULL,
    validation_json TEXT NOT NULL,
    asset_types_json TEXT NOT NULL DEFAULT '[]',
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
    content_kind TEXT NOT NULL DEFAULT 'document',
    media_type TEXT NOT NULL DEFAULT 'text/markdown',
    content_metadata TEXT NOT NULL DEFAULT '{}',
    body_raw TEXT,
    body_compressed BLOB,
    body_codec TEXT,
    size INTEGER NOT NULL DEFAULT 0,
    stored_size INTEGER NOT NULL DEFAULT 0,
    privacy_policy TEXT DEFAULT 'none',
    privacy_provider TEXT DEFAULT '',
    privacy_key_id TEXT DEFAULT '',
    privacy_provider_name TEXT DEFAULT '',
    privacy_key_version INTEGER DEFAULT 0,
    redacted INTEGER DEFAULT 0,
    privacy_spans_json TEXT DEFAULT '[]',
    updated_at INTEGER NOT NULL,
    CHECK ((body_raw IS NULL) != (body_compressed IS NULL)),
    CHECK ((body_raw IS NOT NULL AND body_codec IS NULL)
        OR (body_compressed IS NOT NULL AND body_codec IS NOT NULL)),
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
CREATE TABLE IF NOT EXISTS search_tokens (
    bundle_root TEXT NOT NULL,
    field TEXT NOT NULL,
    token TEXT NOT NULL,
    postings BLOB NOT NULL,
    PRIMARY KEY (bundle_root, field, token)
);
CREATE TABLE IF NOT EXISTS search_vocabulary (
    bundle_root TEXT NOT NULL,
    field TEXT NOT NULL,
    token TEXT NOT NULL,
    PRIMARY KEY (bundle_root, field, token)
);
CREATE TABLE IF NOT EXISTS extension_data (
    bundle_root TEXT NOT NULL,
    extension TEXT NOT NULL,
    doc_id TEXT NOT NULL,
    key TEXT NOT NULL,
    sha256 TEXT NOT NULL,
    size INTEGER NOT NULL,
    media_type TEXT,
    metadata_json TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    PRIMARY KEY (bundle_root, extension, doc_id, key)
);
CREATE INDEX IF NOT EXISTS idx_extension_data_doc
    ON extension_data(bundle_root, doc_id);
CREATE INDEX IF NOT EXISTS idx_extension_data_blob
    ON extension_data(bundle_root, sha256);
CREATE TABLE IF NOT EXISTS enrichments (
    bundle_root TEXT NOT NULL,
    doc_id TEXT NOT NULL,
    enricher_id TEXT NOT NULL,
    data_json TEXT NOT NULL,
    PRIMARY KEY (bundle_root, doc_id, enricher_id),
    FOREIGN KEY (bundle_root, doc_id)
        REFERENCES documents(bundle_root, id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS document_embeddings (
    bundle_root TEXT NOT NULL,
    doc_id TEXT NOT NULL,
    model TEXT NOT NULL,
    dim INTEGER NOT NULL,
    vector BLOB NOT NULL,
    content_hash TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    PRIMARY KEY (bundle_root, doc_id, model, dim),
    FOREIGN KEY (bundle_root, doc_id)
        REFERENCES documents(bundle_root, id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS embedding_indexes (
    model TEXT NOT NULL,
    dim INTEGER NOT NULL,
    metric TEXT NOT NULL,
    element_type TEXT NOT NULL,
    table_name TEXT NOT NULL,
    active INTEGER NOT NULL DEFAULT 1,
    created_at INTEGER NOT NULL,
    PRIMARY KEY (model, dim, metric, element_type)
);
`)
	return err
}
