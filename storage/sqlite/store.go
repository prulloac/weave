// Package sqlite implements Weave's persistent bundle storage.
package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine/graph"
)

const schemaVersion = 1

var (
	// ErrBundleNotIndexed is returned when a bundle is absent from the database.
	ErrBundleNotIndexed = errors.New("bundle not indexed")
)

// Store is a bundle-scoped SQLite index.
type Store struct {
	db   *sql.DB
	path string
}

// IngestOptions controls how a bundle is stored.
type IngestOptions struct {
	Mode string // strict | permissive
}

// DefaultPath returns the default user database path.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".weave", "index.db"), nil
}

// ExpandPath expands a leading ~ to the user home directory.
func ExpandPath(path string) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if path == "~" {
			return home, nil
		}
		return filepath.Join(home, path[2:]), nil
	}
	return path, nil
}

// Open opens or creates the database and verifies the schema version.
func Open(path string) (*Store, error) {
	if path == "" {
		return nil, fmt.Errorf("database path is required")
	}
	if path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return nil, err
		}
	}

	db, err := openConnection(path)
	if err != nil {
		return nil, err
	}

	version, err := userVersion(db)
	if err != nil {
		db.Close()
		return nil, err
	}
	switch version {
	case 0:
		if err := createSchema(db); err != nil {
			db.Close()
			return nil, err
		}
		if _, err := db.Exec(fmt.Sprintf("PRAGMA user_version = %d", schemaVersion)); err != nil {
			db.Close()
			return nil, err
		}
	case schemaVersion:
	default:
		db.Close()
		return nil, fmt.Errorf("unsupported database schema version %d", version)
	}

	return &Store{db: db, path: path}, nil
}

// OpenExisting opens an existing database without creating one.
func OpenExisting(path string) (*Store, error) {
	if path == "" {
		return nil, fmt.Errorf("database path is required")
	}
	if path != ":memory:" {
		if _, err := os.Stat(path); err != nil {
			return nil, err
		}
	}
	db, err := openConnection(path)
	if err != nil {
		return nil, err
	}
	version, err := userVersion(db)
	if err != nil {
		db.Close()
		return nil, err
	}
	if version != schemaVersion {
		db.Close()
		return nil, fmt.Errorf("unsupported database schema version %d", version)
	}
	return &Store{db: db, path: path}, nil
}

func openConnection(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)

	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
	} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, err
		}
	}
	return db, nil
}

// Close closes the database.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// Ingest replaces one bundle's stored rows in a single transaction.
func (s *Store) Ingest(
	ctx context.Context,
	bundle *contractv1.Bundle,
	options IngestOptions,
) (contractv1.IngestReport, error) {
	return s.writeBundle(ctx, bundle, options)
}

// Rebuild re-ingests one bundle without touching other bundles.
func (s *Store) Rebuild(
	ctx context.Context,
	bundle *contractv1.Bundle,
	options IngestOptions,
) (contractv1.IngestReport, error) {
	return s.writeBundle(ctx, bundle, options)
}

// HasBundle reports whether a bundle root is indexed.
func (s *Store) HasBundle(ctx context.Context, bundleRoot string) (bool, error) {
	root, err := normalizeRoot(bundleRoot)
	if err != nil {
		return false, err
	}
	var found int
	err = s.db.QueryRowContext(ctx,
		"SELECT 1 FROM bundles WHERE bundle_root = ?", root).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// LoadBundle reconstructs a bundle from the database.
func (s *Store) LoadBundle(ctx context.Context, bundleRoot string) (*contractv1.Bundle, error) {
	root, err := normalizeRoot(bundleRoot)
	if err != nil {
		return nil, err
	}

	var version sql.NullString
	var mode string
	var indexJSON, validationJSON string
	err = s.db.QueryRowContext(ctx,
		"SELECT okf_version, mode, index_json, validation_json FROM bundles WHERE bundle_root = ?",
		root).Scan(&version, &mode, &indexJSON, &validationJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrBundleNotIndexed
	}
	if err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, file_path, title, description, doc_type, keywords_json,
		       frontmatter_json, tags_json, status, links_json, body_compressed
		FROM documents WHERE bundle_root = ? ORDER BY id`, root)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	concepts := make([]contractv1.OkfConcept, 0)
	for rows.Next() {
		concept, err := scanConcept(rows)
		if err != nil {
			return nil, err
		}
		concepts = append(concepts, concept)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var index []contractv1.IndexEntry
	if err := json.Unmarshal([]byte(indexJSON), &index); err != nil {
		return nil, err
	}
	var validation contractv1.Validation
	if err := json.Unmarshal([]byte(validationJSON), &validation); err != nil {
		return nil, err
	}
	validation.Permissive = mode == "permissive"

	bundle := &contractv1.Bundle{
		Root:       root,
		Concepts:   concepts,
		Index:      index,
		Validation: validation,
	}
	if version.Valid && version.String != "" {
		value := contractv1.OkfVersion(version.String)
		bundle.OkfVersion = &value
	}
	return bundle, nil
}

func (s *Store) writeBundle(
	ctx context.Context,
	bundle *contractv1.Bundle,
	options IngestOptions,
) (contractv1.IngestReport, error) {
	if bundle == nil {
		return contractv1.IngestReport{}, fmt.Errorf("bundle is required")
	}
	mode := options.Mode
	if mode == "" {
		mode = "strict"
	}
	if mode != "strict" && mode != "permissive" {
		return contractv1.IngestReport{}, fmt.Errorf("mode must be strict or permissive")
	}
	root, err := normalizeRoot(bundle.Root)
	if err != nil {
		return contractv1.IngestReport{}, err
	}

	indexJSON, err := json.Marshal(bundle.Index)
	if err != nil {
		return contractv1.IngestReport{}, err
	}
	validationJSON, err := json.Marshal(bundle.Validation)
	if err != nil {
		return contractv1.IngestReport{}, err
	}
	version := ""
	if bundle.OkfVersion != nil {
		version = string(*bundle.OkfVersion)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return contractv1.IngestReport{}, err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, "DELETE FROM bundles WHERE bundle_root = ?", root); err != nil {
		return contractv1.IngestReport{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO bundles (bundle_root, mode, okf_version, index_json, validation_json, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		root, mode, nullableString(version), string(indexJSON), string(validationJSON), time.Now().Unix()); err != nil {
		return contractv1.IngestReport{}, err
	}

	now := time.Now().Unix()
	for _, concept := range bundle.Concepts {
		if err := insertConcept(ctx, tx, root, concept, now); err != nil {
			return contractv1.IngestReport{}, err
		}
	}

	edges, err := collectEdges(bundle)
	if err != nil {
		return contractv1.IngestReport{}, err
	}
	for _, edge := range edges {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO edges (bundle_root, source_id, target_id, edge_type, dangling)
			VALUES (?, ?, ?, ?, ?)`,
			root, edge.from, edge.to, edge.kind, boolInt(edge.dangling)); err != nil {
			return contractv1.IngestReport{}, err
		}
	}

	if err := tx.Commit(); err != nil {
		return contractv1.IngestReport{}, err
	}

	return contractv1.IngestReport{
		DB:        s.path,
		Bundle:    root,
		Mode:      mode,
		Documents: len(bundle.Concepts),
		Edges:     len(edges),
		Synthetic: syntheticCount(bundle, mode),
		Warnings:  warningCount(bundle),
		Errors:    errorCount(bundle, mode),
	}, nil
}

func insertConcept(ctx context.Context, tx *sql.Tx, root string, concept contractv1.OkfConcept, now int64) error {
	frontmatter, err := json.Marshal(concept.Frontmatter)
	if err != nil {
		return err
	}
	keywords, err := json.Marshal(concept.Keywords)
	if err != nil {
		return err
	}
	tags, err := json.Marshal(concept.Tags)
	if err != nil {
		return err
	}
	links, err := json.Marshal(concept.Links)
	if err != nil {
		return err
	}

	title := ""
	if concept.Title != nil {
		title = *concept.Title
	}
	var description any
	if concept.Description != nil {
		description = *concept.Description
	}
	var status any
	if concept.Status != nil {
		status = string(*concept.Status)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO documents (
			bundle_root, id, file_path, content_hash, title, description, doc_type,
			keywords_json, frontmatter_json, tags_json, status, links_json,
			body_compressed, body_codec, privacy_policy, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		root, concept.ID, concept.Path, contentHash(root, concept), title, description, concept.Type,
		string(keywords), string(frontmatter), string(tags), status, string(links),
		[]byte(concept.Body), "none", "none", now)
	return err
}

type storedEdge struct {
	from     string
	to       string
	kind     string
	dangling bool
}

func collectEdges(bundle *contractv1.Bundle) ([]storedEdge, error) {
	view := graph.Build(bundle)
	edges := make([]storedEdge, 0, len(view.Outgoing))
	seen := make(map[string]bool)
	add := func(edge storedEdge) {
		key := edge.from + "\x00" + edge.to + "\x00" + edge.kind
		if seen[key] {
			return
		}
		seen[key] = true
		edges = append(edges, edge)
	}
	for _, concept := range bundle.Concepts {
		for _, link := range concept.Links {
			target := link.Target
			if link.Resolved != nil {
				target = strings.TrimSuffix(*link.Resolved, ".md")
			}
			add(storedEdge{
				from:     concept.ID,
				to:       target,
				kind:     "link",
				dangling: !link.ResolvesInBundle,
			})
		}
		for _, entry := range graph.RelatedEntries(concept.Frontmatter["related"]) {
			if entry == "" {
				continue
			}
			target, found := view.ResolveRelated(entry, concept.ID)
			if !found {
				target = entry
			}
			add(storedEdge{
				from:     concept.ID,
				to:       target,
				kind:     "related",
				dangling: !found,
			})
		}
	}
	return edges, nil
}

func scanConcept(rows *sql.Rows) (contractv1.OkfConcept, error) {
	var (
		concept                                            contractv1.OkfConcept
		title                                              string
		description, docType, status                       sql.NullString
		keywordsJSON, frontmatterJSON, tagsJSON, linksJSON string
		body                                               []byte
	)
	if err := rows.Scan(
		&concept.ID, &concept.Path, &title, &description, &docType,
		&keywordsJSON, &frontmatterJSON, &tagsJSON, &status, &linksJSON, &body,
	); err != nil {
		return contractv1.OkfConcept{}, err
	}
	concept.Title = optionalString(title)
	concept.Description = optionalString(description.String)
	concept.Type = docType.String
	concept.Body = string(body)
	if err := json.Unmarshal([]byte(keywordsJSON), &concept.Keywords); err != nil {
		return contractv1.OkfConcept{}, err
	}
	if err := json.Unmarshal([]byte(frontmatterJSON), &concept.Frontmatter); err != nil {
		return contractv1.OkfConcept{}, err
	}
	if err := json.Unmarshal([]byte(tagsJSON), &concept.Tags); err != nil {
		return contractv1.OkfConcept{}, err
	}
	if err := json.Unmarshal([]byte(linksJSON), &concept.Links); err != nil {
		return contractv1.OkfConcept{}, err
	}
	if status.Valid && status.String != "" {
		value := contractv1.ConceptStatus(status.String)
		concept.Status = &value
	}
	return concept, nil
}

func contentHash(root string, concept contractv1.OkfConcept) string {
	path := filepath.Join(root, filepath.FromSlash(concept.Path))
	if data, err := os.ReadFile(path); err == nil {
		sum := sha256.Sum256(data)
		return hex.EncodeToString(sum[:])
	}
	sum := sha256.Sum256([]byte(concept.ID + "\x00" + concept.Body))
	return hex.EncodeToString(sum[:])
}

func normalizeRoot(root string) (string, error) {
	if strings.TrimSpace(root) == "" {
		return "", fmt.Errorf("bundle root is required")
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	return filepath.Clean(absolute), nil
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func syntheticCount(bundle *contractv1.Bundle, mode string) int {
	if mode != "permissive" {
		return 0
	}
	count := 0
	for _, validation := range bundle.Validation.Concepts {
		if !validation.HasFrontmatter || !validation.HasType || !validation.HasKeywords {
			count++
		}
	}
	return count
}

func warningCount(bundle *contractv1.Bundle) int {
	count := len(bundle.Validation.Warnings)
	for _, validation := range bundle.Validation.Concepts {
		count += len(validation.Warnings)
	}
	return count
}

func errorCount(bundle *contractv1.Bundle, mode string) int {
	if mode == "permissive" {
		return 0
	}
	count := 0
	for _, validation := range bundle.Validation.Concepts {
		if !validation.HasFrontmatter || !validation.HasType || !validation.HasKeywords {
			count++
		}
	}
	return count
}

func userVersion(db *sql.DB) (int, error) {
	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return 0, err
	}
	return version, nil
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
    description,
    keywords,
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
