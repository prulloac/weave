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
	"github.com/prulloac/weave/engine/compression"
	"github.com/prulloac/weave/engine/graph"
	"github.com/prulloac/weave/engine/okf"
	"github.com/prulloac/weave/engine/privacy"
	"github.com/prulloac/weave/engine/search"
)

const schemaVersion = 2

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
	Mode  string // strict | permissive
	Codec string // zstd | lz4 | none
}

// SyncOptions controls incremental synchronization.
type SyncOptions struct {
	Mode  string // empty keeps the stored mode
	Codec string // zstd | lz4 | none | empty inherits per row
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
	case 1:
		if err := migrateV1toV2(db); err != nil {
			db.Close()
			return nil, err
		}
		if _, err := db.Exec(fmt.Sprintf("PRAGMA user_version = %d", schemaVersion)); err != nil {
			db.Close()
			return nil, err
		}
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

// Sync updates one indexed bundle from the filesystem using content hashes.
func (s *Store) Sync(
	ctx context.Context,
	root string,
	options SyncOptions,
) (contractv1.SyncReport, error) {
	root, err := normalizeRoot(root)
	if err != nil {
		return contractv1.SyncReport{}, err
	}
	storedMode, err := s.bundleMode(ctx, root)
	if err != nil {
		return contractv1.SyncReport{}, err
	}
	mode := options.Mode
	if mode == "" {
		mode = storedMode
	}
	if mode != "strict" && mode != "permissive" {
		return contractv1.SyncReport{}, fmt.Errorf("mode must be strict or permissive")
	}

	bundle, err := okf.ParseBundle(ctx, root, okf.Options{Permissive: mode == "permissive"})
	if err != nil {
		return contractv1.SyncReport{}, err
	}

	current := make(map[string]contractv1.OkfConcept, len(bundle.Concepts))
	for _, concept := range bundle.Concepts {
		current[concept.Path] = concept
	}

	existing, err := s.documentHashes(ctx, root)
	if err != nil {
		return contractv1.SyncReport{}, err
	}
	codecs, err := s.documentCodecs(ctx, root)
	if err != nil {
		return contractv1.SyncReport{}, err
	}
	if options.Codec != "" && !compression.Valid(options.Codec) {
		return contractv1.SyncReport{}, fmt.Errorf("codec must be zstd, lz4 or none")
	}

	added, updated, removed, skipped := 0, 0, 0, 0
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return contractv1.SyncReport{}, err
	}
	defer tx.Rollback()

	indexJSON, err := json.Marshal(bundle.Index)
	if err != nil {
		return contractv1.SyncReport{}, err
	}
	validationJSON, err := json.Marshal(bundle.Validation)
	if err != nil {
		return contractv1.SyncReport{}, err
	}
	version := ""
	if bundle.OkfVersion != nil {
		version = string(*bundle.OkfVersion)
	}
	if _, err := tx.ExecContext(ctx,
		"UPDATE bundles SET mode = ?, okf_version = ?, index_json = ?, validation_json = ?, updated_at = ? WHERE bundle_root = ?",
		mode, nullableString(version), string(indexJSON), string(validationJSON), time.Now().Unix(), root); err != nil {
		return contractv1.SyncReport{}, err
	}

	now := time.Now().Unix()
	privacySkipped := 0
	for path, concept := range current {
		hash := contentHash(root, concept)
		oldHash, exists := existing[path]
		codec := options.Codec
		if codec == "" {
			if stored, ok := codecs[path]; ok && stored != "" {
				codec = stored
			} else {
				codec = string(compression.CodecZstd)
			}
		}
		switch {
		case !exists:
			inserted, err := insertSyncedConcept(ctx, tx, root, concept, codec, now)
			if err != nil {
				if errors.Is(err, privacy.ErrKeyUnavailable) && mode == "permissive" {
					privacySkipped++
					continue
				}
				return contractv1.SyncReport{}, err
			}
			if inserted {
				added++
			}
		case oldHash != hash:
			if _, err := tx.ExecContext(ctx,
				"DELETE FROM documents WHERE bundle_root = ? AND file_path = ?", root, path); err != nil {
				return contractv1.SyncReport{}, err
			}
			inserted, err := insertSyncedConcept(ctx, tx, root, concept, codec, now)
			if err != nil {
				if errors.Is(err, privacy.ErrKeyUnavailable) && mode == "permissive" {
					privacySkipped++
					continue
				}
				return contractv1.SyncReport{}, err
			}
			if inserted {
				updated++
			}
		default:
			skipped++
		}
	}
	for path := range existing {
		if _, ok := current[path]; ok {
			continue
		}
		if _, err := tx.ExecContext(ctx,
			"DELETE FROM documents WHERE bundle_root = ? AND file_path = ?", root, path); err != nil {
			return contractv1.SyncReport{}, err
		}
		removed++
	}

	if _, err := tx.ExecContext(ctx, "DELETE FROM edges WHERE bundle_root = ?", root); err != nil {
		return contractv1.SyncReport{}, err
	}
	edges, err := collectEdges(bundle)
	if err != nil {
		return contractv1.SyncReport{}, err
	}
	for _, edge := range edges {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO edges (bundle_root, source_id, target_id, edge_type, dangling)
			VALUES (?, ?, ?, ?, ?)`,
			root, edge.from, edge.to, edge.kind, boolInt(edge.dangling)); err != nil {
			return contractv1.SyncReport{}, err
		}
	}
	if err := populateFTS(ctx, tx, root); err != nil {
		return contractv1.SyncReport{}, err
	}

	if err := tx.Commit(); err != nil {
		return contractv1.SyncReport{}, err
	}

	return contractv1.SyncReport{
		DB:        s.path,
		Bundle:    root,
		Mode:      mode,
		Documents: len(current),
		Added:     added,
		Updated:   updated,
		Removed:   removed,
		Skipped:   skipped,
		Warnings:  warningCount(bundle) + privacySkipped,
		Errors:    errorCount(bundle, mode),
	}, nil
}

func (s *Store) bundleMode(ctx context.Context, root string) (string, error) {
	var mode string
	err := s.db.QueryRowContext(ctx,
		"SELECT mode FROM bundles WHERE bundle_root = ?", root).Scan(&mode)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrBundleNotIndexed
	}
	if err != nil {
		return "", err
	}
	return mode, nil
}

func (s *Store) documentHashes(ctx context.Context, root string) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT file_path, content_hash FROM documents WHERE bundle_root = ?", root)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	hashes := make(map[string]string)
	for rows.Next() {
		var path, hash string
		if err := rows.Scan(&path, &hash); err != nil {
			return nil, err
		}
		hashes[path] = hash
	}
	return hashes, rows.Err()
}

func (s *Store) documentCodecs(ctx context.Context, root string) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT file_path, body_codec FROM documents WHERE bundle_root = ?", root)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	codecs := make(map[string]string)
	for rows.Next() {
		var path, codec string
		if err := rows.Scan(&path, &codec); err != nil {
			return nil, err
		}
		codecs[path] = codec
	}
	return codecs, rows.Err()
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
		       frontmatter_json, tags_json, status, links_json, body_compressed,
		       privacy_policy, privacy_provider, privacy_key_id, body_codec
		FROM documents WHERE bundle_root = ? ORDER BY id`, root)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	concepts := make([]contractv1.OkfConcept, 0)
	decodeWarnings := make([]string, 0)
	for rows.Next() {
		concept, warning, err := scanConcept(rows)
		if err != nil {
			return nil, err
		}
		if warning != "" {
			decodeWarnings = append(decodeWarnings, warning)
		}
		concepts = append(concepts, concept)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	ids := make(map[string]bool, len(concepts))
	for _, concept := range concepts {
		ids[concept.ID] = true
	}
	for conceptIndex := range concepts {
		for linkIndex := range concepts[conceptIndex].Links {
			link := &concepts[conceptIndex].Links[linkIndex]
			if link.Resolved == nil {
				link.ResolvesInBundle = false
				continue
			}
			target := strings.TrimSuffix(*link.Resolved, ".md")
			link.ResolvesInBundle = ids[target]
		}
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
	validation.Warnings = append(validation.Warnings, decodeWarnings...)

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
	codec := options.Codec
	if codec == "" {
		codec = string(compression.CodecZstd)
	}
	if !compression.Valid(codec) {
		return contractv1.IngestReport{}, fmt.Errorf("codec must be zstd, lz4 or none")
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
	prepared, skipped, err := prepareConcepts(root, bundle.Concepts, mode == "permissive", codec)
	if err != nil {
		return contractv1.IngestReport{}, err
	}
	for _, item := range prepared {
		if err := insertConcept(ctx, tx, root, item, now); err != nil {
			return contractv1.IngestReport{}, err
		}
	}

	edges, err := collectEdges(&contractv1.Bundle{Root: bundle.Root, Concepts: conceptsOf(prepared)})
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
	if err := populateFTS(ctx, tx, root); err != nil {
		return contractv1.IngestReport{}, err
	}

	if err := tx.Commit(); err != nil {
		return contractv1.IngestReport{}, err
	}

	return contractv1.IngestReport{
		DB:        s.path,
		Bundle:    root,
		Mode:      mode,
		Documents: len(prepared),
		Edges:     len(edges),
		Synthetic: syntheticCount(bundle, mode),
		Warnings:  warningCount(bundle) + skipped,
		Errors:    errorCount(bundle, mode),
	}, nil
}

func populateFTS(ctx context.Context, tx *sql.Tx, root string) error {
	if _, err := tx.ExecContext(ctx, "DELETE FROM fts_documents WHERE bundle_root = ?", root); err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT id, title, keywords_json, description, body_compressed, body_codec, privacy_policy
		FROM documents WHERE bundle_root = ? ORDER BY id`, root)
	if err != nil {
		return err
	}
	type ftsRow struct {
		id, title, keywords, body, policy, codec string
		description                              sql.NullString
	}
	entries := make([]ftsRow, 0)
	for rows.Next() {
		var entry ftsRow
		if err := rows.Scan(&entry.id, &entry.title, &entry.keywords, &entry.description,
			&entry.body, &entry.codec, &entry.policy); err != nil {
			rows.Close()
			return err
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	for _, entry := range entries {
		if contractv1.PrivacyPolicy(entry.policy) == contractv1.PrivacyEncrypt {
			continue
		}
		decoded, err := compression.Decode(compression.Codec(entry.codec), []byte(entry.body))
		if err != nil {
			decoded = nil
		}
		var keywords []string
		if err := json.Unmarshal([]byte(entry.keywords), &keywords); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO fts_documents (bundle_root, id, title, keywords, description, body)
			VALUES (?, ?, ?, ?, ?, ?)`,
			root, entry.id, entry.title, strings.Join(keywords, " "),
			entry.description.String, search.StripMarkdown(string(decoded)))
		if err != nil {
			return err
		}
	}
	return nil
}

// SearchBM25 returns lexical hits for a bundle.
func (s *Store) SearchBM25(
	ctx context.Context,
	root string,
	query string,
	topK int,
) ([]contractv1.LexicalHit, error) {
	root, err := normalizeRoot(root)
	if err != nil {
		return nil, err
	}
	if topK == 0 {
		topK = 10
	}
	if topK < 1 || topK > 100 {
		return nil, search.ErrInvalidTopK
	}
	match := buildFTSQuery(query)
	aliases := privacy.ExpandQuery(query, s.privacyRefs(ctx, root), func(id string) ([]byte, error) {
		return privacy.ResolveKey(root, id)
	})
	if len(aliases) > 0 {
		if match == "" {
			match = strings.Join(aliases, " OR ")
		} else {
			match = "(" + match + ") OR (" + strings.Join(aliases, " OR ") + ")"
		}
	}
	if match == "" {
		return []contractv1.LexicalHit{}, nil
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT d.id, d.title, d.file_path, d.doc_type,
		       -bm25(fts_documents, 0.0, 0.0, 10.0, 8.0, 4.0, 1.0) AS raw
		FROM fts_documents
		JOIN documents d
		  ON d.bundle_root = fts_documents.bundle_root
		 AND d.id = fts_documents.id
		WHERE fts_documents.bundle_root = ?
		  AND fts_documents MATCH ?
		ORDER BY raw DESC, d.id ASC
		LIMIT ?`, root, match, topK)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	hits := make([]contractv1.LexicalHit, 0)
	maxScore := 0.0
	for rows.Next() {
		var hit contractv1.LexicalHit
		var docType sql.NullString
		if err := rows.Scan(&hit.ID, &hit.Title, &hit.Path, &docType, &hit.RawScore); err != nil {
			return nil, err
		}
		hit.Type = docType.String
		if hit.RawScore > maxScore {
			maxScore = hit.RawScore
		}
		hits = append(hits, hit)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if maxScore > 0 {
		for index := range hits {
			hits[index].Score = search.RoundScore(hits[index].RawScore / maxScore)
		}
	}
	return hits, nil
}

// BM25Explanation is one document's lexical evidence.
type BM25Explanation struct {
	Target  string
	Rank    int
	Lexical contractv1.ExplainLexical
}

// ExplainBM25 explains one document's BM25 score.
func (s *Store) ExplainBM25(
	ctx context.Context,
	root string,
	query string,
	topK int,
	target string,
) (BM25Explanation, error) {
	root, err := normalizeRoot(root)
	if err != nil {
		return BM25Explanation{}, err
	}
	positives, negatives := splitFTSQuery(query)
	aliases := privacy.ExpandQuery(query, s.privacyRefs(ctx, root), func(id string) ([]byte, error) {
		return privacy.ResolveKey(root, id)
	})
	for _, alias := range aliases {
		positives = append(positives, quoteFTSTerm(alias))
	}
	if len(positives) == 0 {
		return BM25Explanation{}, search.ErrEmptyQuery
	}
	match := strings.Join(positives, " OR ")
	if len(positives) > 1 {
		match = "(" + match + ")"
	}
	if len(negatives) > 0 {
		match += " NOT (" + strings.Join(negatives, " OR ") + ")"
	}
	hits, err := s.SearchBM25(ctx, root, query, topK)
	if err != nil {
		return BM25Explanation{}, err
	}
	rank := 0
	if target == "" {
		if len(hits) == 0 {
			return BM25Explanation{}, search.ErrNoResult
		}
		target = hits[0].ID
		rank = 1
	} else {
		exists, err := s.HasConcept(ctx, root, target)
		if err != nil {
			return BM25Explanation{}, err
		}
		if !exists {
			return BM25Explanation{}, search.ErrConceptNotFound
		}
		for index, hit := range hits {
			if hit.ID == target {
				rank = index + 1
				break
			}
		}
	}

	maxRaw := 0.0
	raw := 0.0
	for _, hit := range hits {
		if hit.RawScore > maxRaw {
			maxRaw = hit.RawScore
		}
		if hit.ID == target {
			raw = hit.RawScore
		}
	}
	if raw == 0 {
		raw, err = s.bm25Raw(ctx, root, match, target)
		if err != nil {
			return BM25Explanation{}, err
		}
	}
	score := 0.0
	if maxRaw > 0 {
		score = search.RoundScore(raw / maxRaw)
	}

	matchedFields := make([]contractv1.SearchField, 0)
	for _, field := range []contractv1.SearchField{
		contractv1.FieldTitle,
		contractv1.FieldKeywords,
		contractv1.FieldDescription,
		contractv1.FieldBody,
	} {
		expression := string(field) + ":(" + strings.Join(positives, " OR ") + ")"
		if len(negatives) > 0 {
			expression += " NOT (" + strings.Join(negatives, " OR ") + ")"
		}
		matched, err := s.ftsMatches(ctx, root, expression, target)
		if err != nil {
			return BM25Explanation{}, err
		}
		if matched {
			matchedFields = append(matchedFields, field)
		}
	}

	tokens := make([]contractv1.ExplainLexicalToken, 0, len(positives))
	for _, term := range positives {
		contribution, err := s.bm25Raw(ctx, root, term, target)
		if err != nil {
			return BM25Explanation{}, err
		}
		tokens = append(tokens, contractv1.ExplainLexicalToken{
			QueryToken:   strings.Trim(term, "\""),
			Matched:      contribution > 0,
			Contribution: contribution,
		})
	}

	return BM25Explanation{
		Target: target,
		Rank:   rank,
		Lexical: contractv1.ExplainLexical{
			RawScore:      raw,
			Score:         score,
			MatchedFields: matchedFields,
			Tokens:        tokens,
		},
	}, nil
}

func (s *Store) privacyRefs(ctx context.Context, root string) []privacy.KeyRef {
	rows, err := s.db.QueryContext(ctx, `
		SELECT privacy_policy, privacy_provider, privacy_key_id, privacy_spans_json
		FROM documents
		WHERE bundle_root = ? AND privacy_policy IN ('tokenize', 'hash', 'mixed')`, root)
	if err != nil {
		return nil
	}
	defer rows.Close()
	refs := make([]privacy.KeyRef, 0)
	seen := make(map[string]bool)
	add := func(policy contractv1.PrivacyPolicy, provider, keyID string) {
		if keyID == "" || (policy != contractv1.PrivacyTokenize && policy != contractv1.PrivacyHash) {
			return
		}
		key := string(policy) + "\x00" + provider + "\x00" + keyID
		if seen[key] {
			return
		}
		seen[key] = true
		refs = append(refs, privacy.KeyRef{Policy: policy, Provider: provider, KeyID: keyID})
	}
	for rows.Next() {
		var policy, provider, keyID, spansJSON string
		if err := rows.Scan(&policy, &provider, &keyID, &spansJSON); err != nil {
			return nil
		}
		stored := contractv1.PrivacyPolicy(policy)
		if stored == contractv1.PrivacyMixed {
			var spans []privacy.Span
			if err := json.Unmarshal([]byte(spansJSON), &spans); err == nil {
				for _, span := range spans {
					add(span.Policy, span.Provider, span.KeyID)
				}
			}
			continue
		}
		add(stored, provider, keyID)
	}
	return refs
}

// HasConcept reports whether a concept exists in a bundle.
func (s *Store) HasConcept(ctx context.Context, root, id string) (bool, error) {
	var found int
	err := s.db.QueryRowContext(ctx,
		"SELECT 1 FROM documents WHERE bundle_root = ? AND id = ?", root, id).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *Store) bm25Raw(ctx context.Context, root, match, id string) (float64, error) {
	var raw float64
	err := s.db.QueryRowContext(ctx, `
		SELECT -bm25(fts_documents, 0.0, 0.0, 10.0, 8.0, 4.0, 1.0)
		FROM fts_documents
		JOIN documents d
		  ON d.bundle_root = fts_documents.bundle_root
		 AND d.id = fts_documents.id
		WHERE fts_documents.bundle_root = ?
		  AND fts_documents MATCH ?
		  AND d.id = ?`, root, match, id).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return raw, nil
}

func (s *Store) ftsMatches(ctx context.Context, root, match, id string) (bool, error) {
	var found int
	err := s.db.QueryRowContext(ctx, `
		SELECT 1
		FROM fts_documents
		JOIN documents d
		  ON d.bundle_root = fts_documents.bundle_root
		 AND d.id = fts_documents.id
		WHERE fts_documents.bundle_root = ?
		  AND fts_documents MATCH ?
		  AND d.id = ?
		LIMIT 1`, root, match, id).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func buildFTSQuery(query string) string {
	positives, negatives := splitFTSQuery(query)
	if len(positives) == 0 {
		return ""
	}
	expression := strings.Join(positives, " OR ")
	if len(positives) > 1 {
		expression = "(" + expression + ")"
	}
	if len(negatives) > 0 {
		expression += " NOT (" + strings.Join(negatives, " OR ") + ")"
	}
	return expression
}

func splitFTSQuery(query string) ([]string, []string) {
	query = strings.TrimSpace(query)
	positives := make([]string, 0)
	negatives := make([]string, 0)
	index := 0
	for index < len(query) {
		for index < len(query) && (query[index] == ' ' || query[index] == '\t') {
			index++
		}
		if index >= len(query) {
			break
		}
		negative := false
		if query[index] == '-' {
			negative = true
			index++
			if index >= len(query) {
				break
			}
		}
		start := index
		if query[index] == '"' {
			index++
			for index < len(query) && query[index] != '"' {
				index++
			}
			if index < len(query) {
				index++
			}
		} else {
			for index < len(query) && query[index] != ' ' && query[index] != '\t' {
				index++
			}
		}
		term := strings.TrimSpace(query[start:index])
		if term == "" || term == "-\"\"" {
			continue
		}
		term = quoteFTSTerm(term)
		if negative {
			negatives = append(negatives, term)
		} else {
			positives = append(positives, term)
		}
	}
	return positives, negatives
}

func quoteFTSTerm(term string) string {
	if strings.HasPrefix(term, "\"") {
		return term
	}
	// FTS5 treats punctuation such as "-" as an operator; quote literal terms
	// so hyphenated identifiers match as phrases.
	return `"` + strings.ReplaceAll(term, `"`, `""`) + `"`
}

func insertConcept(ctx context.Context, tx *sql.Tx, root string, item preparedConcept, now int64) error {
	concept := item.concept
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
	spans, err := json.Marshal(item.spans)
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

	policy := "none"
	provider := ""
	keyID := ""
	if item.directive != nil {
		policy = string(item.directive.Policy)
		provider = item.directive.Provider
		keyID = item.directive.KeyID
		if policy == "mixed" && keyID == "" && len(item.spans) > 0 {
			keyID = item.spans[0].KeyID
		}
	}
	encrypted := item.directive != nil && item.directive.Policy == contractv1.PrivacyEncrypt
	blob, storedCodec, err := compression.Encode(compression.Codec(item.codec), []byte(item.body), encrypted)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO documents (
			bundle_root, id, file_path, content_hash, title, description, doc_type,
			keywords_json, frontmatter_json, tags_json, status, links_json,
			body_compressed, body_codec, privacy_policy, privacy_provider,
			privacy_key_id, redacted, privacy_spans_json, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		root, concept.ID, concept.Path, contentHash(root, concept), title, description, concept.Type,
		string(keywords), string(frontmatter), string(tags), status, string(links),
		blob, string(storedCodec), policy, provider, keyID, boolInt(item.redacted), string(spans), now)
	return err
}

func insertSyncedConcept(
	ctx context.Context,
	tx *sql.Tx,
	root string,
	concept contractv1.OkfConcept,
	codec string,
	now int64,
) (bool, error) {
	result, err := privacy.Apply(root, concept)
	if err != nil {
		return false, err
	}
	if err := insertConcept(ctx, tx, root, preparedConcept{
		concept:   concept,
		body:      result.Body,
		codec:     codec,
		directive: result.Directive,
		redacted:  result.Redacted,
		spans:     result.Spans,
	}, now); err != nil {
		return false, err
	}
	return true, nil
}

type preparedConcept struct {
	concept   contractv1.OkfConcept
	body      string
	codec     string
	directive *contractv1.PrivacyDirective
	redacted  bool
	spans     []privacy.Span
}

func prepareConcepts(
	root string,
	concepts []contractv1.OkfConcept,
	permissive bool,
	codec string,
) ([]preparedConcept, int, error) {
	prepared := make([]preparedConcept, 0, len(concepts))
	skipped := 0
	for _, concept := range concepts {
		result, err := privacy.Apply(root, concept)
		if err != nil {
			if errors.Is(err, privacy.ErrKeyUnavailable) && permissive {
				skipped++
				continue
			}
			return nil, 0, err
		}
		prepared = append(prepared, preparedConcept{
			concept:   concept,
			body:      result.Body,
			codec:     codec,
			directive: result.Directive,
			redacted:  result.Redacted,
			spans:     result.Spans,
		})
	}
	return prepared, skipped, nil
}

func conceptsOf(prepared []preparedConcept) []contractv1.OkfConcept {
	concepts := make([]contractv1.OkfConcept, 0, len(prepared))
	for _, item := range prepared {
		concepts = append(concepts, item.concept)
	}
	return concepts
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

func scanConcept(rows *sql.Rows) (contractv1.OkfConcept, string, error) {
	var (
		concept                                            contractv1.OkfConcept
		title                                              string
		description, docType, status                       sql.NullString
		policy, provider, keyID                            sql.NullString
		codec                                              string
		keywordsJSON, frontmatterJSON, tagsJSON, linksJSON string
		body                                               []byte
	)
	if err := rows.Scan(
		&concept.ID, &concept.Path, &title, &description, &docType,
		&keywordsJSON, &frontmatterJSON, &tagsJSON, &status, &linksJSON, &body,
		&policy, &provider, &keyID, &codec,
	); err != nil {
		return contractv1.OkfConcept{}, "", err
	}
	decoded, err := compression.Decode(compression.Codec(codec), body)
	warning := ""
	if err != nil {
		decoded = nil
		warning = "BODY_DECODE_FAILED"
	}
	concept.Title = optionalString(title)
	concept.Description = optionalString(description.String)
	concept.Type = docType.String
	concept.Body = string(decoded)
	if policy.Valid && policy.String != "" && policy.String != "none" {
		concept.Privacy = &contractv1.PrivacyDirective{
			Policy:   contractv1.PrivacyPolicy(policy.String),
			Provider: provider.String,
			KeyID:    keyID.String,
		}
	}
	if err := json.Unmarshal([]byte(keywordsJSON), &concept.Keywords); err != nil {
		return contractv1.OkfConcept{}, "", err
	}
	if err := json.Unmarshal([]byte(frontmatterJSON), &concept.Frontmatter); err != nil {
		return contractv1.OkfConcept{}, "", err
	}
	if err := json.Unmarshal([]byte(tagsJSON), &concept.Tags); err != nil {
		return contractv1.OkfConcept{}, "", err
	}
	if err := json.Unmarshal([]byte(linksJSON), &concept.Links); err != nil {
		return contractv1.OkfConcept{}, "", err
	}
	if status.Valid && status.String != "" {
		value := contractv1.ConceptStatus(status.String)
		concept.Status = &value
	}
	return concept, warning, nil
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
