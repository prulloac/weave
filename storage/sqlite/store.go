// Package sqlite implements Weave's persistent bundle storage.
package sqlite

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine/privacy"
)

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
	Mode     string // strict | permissive
	Codec    string // zstd | lz4 | none
	Enricher Enricher
	Provider privacy.Provider
}

// SyncOptions controls incremental synchronization.
type SyncOptions struct {
	Mode     string // empty keeps the stored mode
	Codec    string // zstd | lz4 | none | empty inherits per row
	Enricher Enricher
	Provider privacy.Provider
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
		if err := migrateV2toV3(db); err != nil {
			db.Close()
			return nil, err
		}
		if _, err := db.Exec(fmt.Sprintf("PRAGMA user_version = %d", schemaVersion)); err != nil {
			db.Close()
			return nil, err
		}
	case 2:
		if err := migrateV2toV3(db); err != nil {
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
