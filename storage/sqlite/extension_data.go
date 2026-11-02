package sqlite

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/prulloac/weave/engine/extensions"
)

// ErrExtensionDataNotFound is returned when a requested record or blob is missing.
var ErrExtensionDataNotFound = errors.New("extension data not found")

// ErrExtensionDataAmbiguous is returned when a key exists on several documents.
var ErrExtensionDataAmbiguous = errors.New("extension data key is ambiguous; use --doc")

// ExtensionDataRecord is one stored extension data reference.
type ExtensionDataRecord struct {
	Extension string
	DocID     string
	Key       string
	MediaType string
	SHA256    string
	Size      int64
	Metadata  json.RawMessage
}

// ExtensionDataFilter limits list results.
type ExtensionDataFilter struct {
	Extension string
	DocID     string
}

// ReplaceExtensionData replaces one extension's data set for a document.
// An empty slice removes the previous set.
func (s *Store) ReplaceExtensionData(
	ctx context.Context,
	root string,
	extension string,
	docID string,
	files []extensions.DataFile,
) error {
	root, err := normalizeRoot(root)
	if err != nil {
		return err
	}
	if !extensions.ValidName(extension) {
		return fmt.Errorf("invalid extension name %q", extension)
	}
	if strings.TrimSpace(docID) == "" {
		return fmt.Errorf("document id is required")
	}

	blobRoot := s.extensionDataRoot(root)
	if err := os.MkdirAll(blobRoot, 0o700); err != nil {
		return err
	}
	if err := copyExtensionDataBlobs(blobRoot, files); err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := replaceExtensionDataTx(ctx, tx, root, extension, docID, files); err != nil {
		return err
	}
	return tx.Commit()
}

func replaceExtensionDataTx(
	ctx context.Context,
	tx *sql.Tx,
	root string,
	extension string,
	docID string,
	files []extensions.DataFile,
) error {
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM extension_data
		WHERE bundle_root = ? AND extension = ? AND doc_id = ?`,
		root, extension, docID); err != nil {
		return err
	}
	now := time.Now().Unix()
	for _, file := range files {
		metadata := string(file.Metadata)
		if strings.TrimSpace(metadata) == "" {
			metadata = "{}"
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO extension_data
				(bundle_root, extension, doc_id, key, sha256, size, media_type, metadata_json, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			root, extension, docID, file.Key, file.SHA256, file.Size, file.MediaType, metadata, now); err != nil {
			return err
		}
	}
	return nil
}

// ListExtensionData returns metadata references ordered by extension, document and key.
func (s *Store) ListExtensionData(
	ctx context.Context,
	root string,
	filter ExtensionDataFilter,
) ([]ExtensionDataRecord, error) {
	root, err := normalizeRoot(root)
	if err != nil {
		return nil, err
	}
	query := `
		SELECT extension, doc_id, key, sha256, size, COALESCE(media_type, ''), metadata_json
		FROM extension_data WHERE bundle_root = ?`
	args := []any{root}
	if filter.Extension != "" {
		query += " AND extension = ?"
		args = append(args, filter.Extension)
	}
	if filter.DocID != "" {
		query += " AND doc_id = ?"
		args = append(args, filter.DocID)
	}
	query += " ORDER BY extension, doc_id, key"
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records := make([]ExtensionDataRecord, 0)
	for rows.Next() {
		var record ExtensionDataRecord
		var metadata string
		if err := rows.Scan(
			&record.Extension, &record.DocID, &record.Key, &record.SHA256,
			&record.Size, &record.MediaType, &metadata,
		); err != nil {
			return nil, err
		}
		record.Metadata = json.RawMessage(metadata)
		records = append(records, record)
	}
	return records, rows.Err()
}

// ReadExtensionData returns the bytes and metadata for one record.
func (s *Store) ReadExtensionData(
	ctx context.Context,
	root string,
	extension string,
	docID string,
	key string,
) ([]byte, ExtensionDataRecord, error) {
	root, err := normalizeRoot(root)
	if err != nil {
		return nil, ExtensionDataRecord{}, err
	}
	var record ExtensionDataRecord
	var metadata string
	err = s.db.QueryRowContext(ctx, `
		SELECT extension, doc_id, key, sha256, size, COALESCE(media_type, ''), metadata_json
		FROM extension_data
		WHERE bundle_root = ? AND extension = ? AND doc_id = ? AND key = ?`,
		root, extension, docID, key).Scan(
		&record.Extension, &record.DocID, &record.Key, &record.SHA256,
		&record.Size, &record.MediaType, &metadata,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ExtensionDataRecord{}, ErrExtensionDataNotFound
	}
	if err != nil {
		return nil, ExtensionDataRecord{}, err
	}
	record.Metadata = json.RawMessage(metadata)
	data, err := os.ReadFile(s.extensionDataBlobPath(root, record.SHA256))
	if err != nil {
		return nil, ExtensionDataRecord{}, fmt.Errorf("%w: blob %s: %v", ErrExtensionDataNotFound, record.SHA256, err)
	}
	return data, record, nil
}

// ReadExtensionDataByKey returns one record when the key is unique for the extension.
func (s *Store) ReadExtensionDataByKey(
	ctx context.Context,
	root string,
	extension string,
	key string,
) ([]byte, ExtensionDataRecord, error) {
	root, err := normalizeRoot(root)
	if err != nil {
		return nil, ExtensionDataRecord{}, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT extension, doc_id, key, sha256, size, COALESCE(media_type, ''), metadata_json
		FROM extension_data
		WHERE bundle_root = ? AND extension = ? AND key = ?`,
		root, extension, key)
	if err != nil {
		return nil, ExtensionDataRecord{}, err
	}
	defer rows.Close()
	var records []ExtensionDataRecord
	for rows.Next() {
		var record ExtensionDataRecord
		var metadata string
		if err := rows.Scan(
			&record.Extension, &record.DocID, &record.Key, &record.SHA256,
			&record.Size, &record.MediaType, &metadata,
		); err != nil {
			return nil, ExtensionDataRecord{}, err
		}
		record.Metadata = json.RawMessage(metadata)
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, ExtensionDataRecord{}, err
	}
	if len(records) == 0 {
		return nil, ExtensionDataRecord{}, ErrExtensionDataNotFound
	}
	if len(records) > 1 {
		return nil, ExtensionDataRecord{}, ErrExtensionDataAmbiguous
	}
	record := records[0]
	data, err := os.ReadFile(s.extensionDataBlobPath(root, record.SHA256))
	if err != nil {
		return nil, ExtensionDataRecord{}, fmt.Errorf("%w: blob %s: %v", ErrExtensionDataNotFound, record.SHA256, err)
	}
	return data, record, nil
}

// DeleteExtensionDataForDocument removes all extension data bound to one document.
func (s *Store) DeleteExtensionDataForDocument(ctx context.Context, root, docID string) error {
	root, err := normalizeRoot(root)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		"DELETE FROM extension_data WHERE bundle_root = ? AND doc_id = ?", root, docID)
	return err
}

// DeleteExtensionDataForBundle removes all extension data for one bundle.
func (s *Store) DeleteExtensionDataForBundle(ctx context.Context, root string) error {
	root, err := normalizeRoot(root)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		"DELETE FROM extension_data WHERE bundle_root = ?", root)
	return err
}

// GCExtensionData removes blobs that have no metadata row and returns the count.
func (s *Store) GCExtensionData(ctx context.Context, root string) (int, error) {
	return s.GCExtensionDataWithOptions(ctx, root, false)
}

// GCExtensionDataWithOptions reports or removes blobs with no metadata row.
func (s *Store) GCExtensionDataWithOptions(ctx context.Context, root string, dryRun bool) (int, error) {
	root, err := normalizeRoot(root)
	if err != nil {
		return 0, err
	}
	rows, err := s.db.QueryContext(ctx,
		"SELECT DISTINCT sha256 FROM extension_data WHERE bundle_root = ?", root)
	if err != nil {
		return 0, err
	}
	referenced := make(map[string]bool)
	for rows.Next() {
		var hash string
		if err := rows.Scan(&hash); err != nil {
			rows.Close()
			return 0, err
		}
		referenced[hash] = true
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()

	blobRoot := s.extensionDataRoot(root)
	entries, err := os.ReadDir(blobRoot)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	removed := 0
	for _, entry := range entries {
		if entry.IsDir() || referenced[entry.Name()] {
			continue
		}
		removed++
		if dryRun {
			continue
		}
		if err := os.Remove(filepath.Join(blobRoot, entry.Name())); err != nil {
			return removed, err
		}
	}
	return removed, nil
}

// ExtensionDataBlobRoot is exposed for diagnostics and tests.
func (s *Store) ExtensionDataBlobRoot(root string) (string, error) {
	normalized, err := normalizeRoot(root)
	if err != nil {
		return "", err
	}
	return s.extensionDataRoot(normalized), nil
}

func (s *Store) extensionDataRoot(root string) string {
	base := filepath.Dir(s.path)
	if s.path == ":memory:" {
		base = os.TempDir()
	}
	return filepath.Join(base, "weave-extension-data", bundleHash(root))
}

func (s *Store) extensionDataBlobPath(root, hash string) string {
	return filepath.Join(s.extensionDataRoot(root), hash)
}

func bundleHash(root string) string {
	absolute, err := filepath.Abs(root)
	if err != nil {
		absolute = root
	}
	sum := sha256.Sum256([]byte(filepath.Clean(absolute)))
	return hex.EncodeToString(sum[:])[:16]
}

func (s *Store) applyExtensionDataTx(
	ctx context.Context,
	tx *sql.Tx,
	root string,
	extension string,
	docID string,
	files []extensions.DataFile,
) error {
	if extension == "" {
		return fmt.Errorf("extension data requires an enricher identity")
	}
	if err := copyExtensionDataBlobs(s.extensionDataRoot(root), files); err != nil {
		return err
	}
	return replaceExtensionDataTx(ctx, tx, root, extension, docID, files)
}

func copyExtensionDataBlobs(blobRoot string, files []extensions.DataFile) error {
	if err := os.MkdirAll(blobRoot, 0o700); err != nil {
		return err
	}
	for _, file := range files {
		if _, err := copyExtensionBlob(blobRoot, file); err != nil {
			return err
		}
	}
	return nil
}

func copyExtensionBlob(blobRoot string, file extensions.DataFile) (string, error) {
	var source io.ReadCloser
	if file.Content != nil {
		source = io.NopCloser(bytes.NewReader(file.Content))
	} else {
		if strings.TrimSpace(file.SourcePath) == "" {
			return "", fmt.Errorf("extension data source path is required")
		}
		opened, err := os.Open(file.SourcePath)
		if err != nil {
			return "", err
		}
		source = opened
	}
	defer source.Close()
	if len(file.SHA256) != sha256.Size*2 {
		return "", fmt.Errorf("extension data hash is invalid")
	}
	temp, err := os.CreateTemp(blobRoot, ".incoming-*")
	if err != nil {
		return "", err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	hasher := sha256.New()
	if _, err := io.Copy(io.MultiWriter(temp, hasher), source); err != nil {
		temp.Close()
		return "", err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return "", err
	}
	if err := temp.Close(); err != nil {
		return "", err
	}
	hash := hex.EncodeToString(hasher.Sum(nil))
	if !strings.EqualFold(hash, file.SHA256) {
		return "", fmt.Errorf("extension data hash mismatch for %q", file.Key)
	}
	if err := os.Chmod(tempPath, 0o600); err != nil {
		return "", err
	}
	destination := filepath.Join(blobRoot, hash)
	if _, err := os.Stat(destination); err == nil {
		return hash, nil
	}
	if err := os.Rename(tempPath, destination); err != nil {
		return "", err
	}
	return hash, nil
}
