package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
)

// ArtifactRecord is one extension-owned export artifact stored with a bundle.
type ArtifactRecord struct {
	Path      string
	MediaType string
	Content   []byte
}

func replaceArtifactsTx(ctx context.Context, tx *sql.Tx, root string, records []ArtifactRecord) error {
	if _, err := tx.ExecContext(ctx, "DELETE FROM extension_artifacts WHERE bundle_root = ?", root); err != nil {
		return err
	}
	for _, record := range records {
		path := strings.TrimSpace(record.Path)
		if path == "" || filepath.IsAbs(path) || strings.Contains(path, "\\") {
			return fmt.Errorf("invalid artifact path %q", record.Path)
		}
		clean := filepath.ToSlash(filepath.Clean(path))
		if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
			return fmt.Errorf("invalid artifact path %q", record.Path)
		}
		mediaType := strings.TrimSpace(record.MediaType)
		if mediaType == "" {
			mediaType = "application/json"
		}
		content := record.Content
		if len(content) == 0 {
			content = []byte("null")
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO extension_artifacts (bundle_root, path, media_type, content_json)
			VALUES (?, ?, ?, ?)`,
			root, clean, mediaType, string(content)); err != nil {
			return err
		}
	}
	return nil
}

// ListArtifacts returns the persisted extension artifacts for a bundle.
func (s *Store) ListArtifacts(ctx context.Context, root string) ([]ArtifactRecord, error) {
	root, err := normalizeRoot(root)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT path, media_type, content_json
		FROM extension_artifacts
		WHERE bundle_root = ?
		ORDER BY path`, root)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]ArtifactRecord, 0)
	for rows.Next() {
		var record ArtifactRecord
		var content string
		if err := rows.Scan(&record.Path, &record.MediaType, &content); err != nil {
			return nil, err
		}
		record.Content = []byte(content)
		result = append(result, record)
	}
	return result, rows.Err()
}

func replaceRelationDefaultsTx(ctx context.Context, tx *sql.Tx, root string, kinds []string) error {
	if _, err := tx.ExecContext(ctx, "DELETE FROM extension_relations WHERE bundle_root = ?", root); err != nil {
		return err
	}
	for _, kind := range kinds {
		kind = strings.TrimSpace(kind)
		if kind == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO extension_relations (bundle_root, kind, is_default, extension)
			VALUES (?, ?, 1, '')`, root, kind); err != nil {
			return err
		}
	}
	return nil
}

// ListRelationDefaults returns active extension default traversal kinds for a bundle.
func (s *Store) ListRelationDefaults(ctx context.Context, root string) ([]string, error) {
	root, err := normalizeRoot(root)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT kind FROM extension_relations
		WHERE bundle_root = ? AND is_default = 1
		ORDER BY kind`, root)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]string, 0)
	for rows.Next() {
		var kind string
		if err := rows.Scan(&kind); err != nil {
			return nil, err
		}
		result = append(result, kind)
	}
	return result, rows.Err()
}
