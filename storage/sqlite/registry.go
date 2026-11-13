package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"strconv"
	"strings"
)

// SourceStatus is the lifecycle state of a registered source.
type SourceStatus string

const (
	SourceIndexing SourceStatus = "indexing"
	SourceActive   SourceStatus = "active"
	SourceDegraded SourceStatus = "degraded"
	SourceRemoved  SourceStatus = "removed"
)

// ErrSourceNotFound is returned when a source alias is not registered.
var ErrSourceNotFound = errors.New("source not found")

// SourceRecord is one persisted workspace source.
type SourceRecord struct {
	Alias       string
	Path        string
	Status      SourceStatus
	Generation  int
	LastSynced  int64
	ContentHash string
	Provider    string
	Metadata    json.RawMessage
}

// UpsertSource inserts or replaces a source record.
func (s *Store) UpsertSource(ctx context.Context, record SourceRecord) error {
	if record.Alias == "" || strings.TrimSpace(record.Path) == "" {
		return errors.New("source alias and path are required")
	}
	status := record.Status
	if status == "" {
		status = SourceIndexing
	}
	provider := strings.TrimSpace(record.Provider)
	if provider == "" {
		provider = "path"
	}
	metadata := record.Metadata
	if len(metadata) == 0 {
		metadata = json.RawMessage("{}")
	}
	if !json.Valid(metadata) {
		return errors.New("source metadata must be valid JSON")
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO sources (alias, path, status, generation, last_synced, content_hash, provider, metadata_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(alias) DO UPDATE SET
			path = excluded.path,
			status = excluded.status,
			generation = excluded.generation,
			last_synced = excluded.last_synced,
			content_hash = excluded.content_hash,
			provider = excluded.provider,
			metadata_json = excluded.metadata_json`,
		record.Alias, record.Path, string(status), record.Generation, record.LastSynced, record.ContentHash, provider, string(metadata))
	return err
}

// SetSourceStatus updates only the status of a registered source.
func (s *Store) SetSourceStatus(ctx context.Context, alias string, status SourceStatus) error {
	result, err := s.db.ExecContext(ctx,
		"UPDATE sources SET status = ? WHERE alias = ?", string(status), alias)
	if err != nil {
		return err
	}
	if affected, err := result.RowsAffected(); err == nil && affected == 0 {
		return ErrSourceNotFound
	}
	return nil
}

// SourceByAlias returns the source registered under alias.
func (s *Store) SourceByAlias(ctx context.Context, alias string) (SourceRecord, bool, error) {
	var record SourceRecord
	var status, provider, metadata string
	err := s.db.QueryRowContext(ctx, `
		SELECT alias, path, status, generation, last_synced, content_hash, provider, metadata_json
		FROM sources WHERE alias = ?`, alias).
		Scan(&record.Alias, &record.Path, &status, &record.Generation, &record.LastSynced, &record.ContentHash, &provider, &metadata)
	if errors.Is(err, sql.ErrNoRows) {
		return SourceRecord{}, false, nil
	}
	if err != nil {
		return SourceRecord{}, false, err
	}
	record.Status = SourceStatus(status)
	record.Provider = provider
	record.Metadata = json.RawMessage(metadata)
	return record, true, nil
}

// ListSources returns all registered sources ordered by alias.
func (s *Store) ListSources(ctx context.Context) ([]SourceRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT alias, path, status, generation, last_synced, content_hash, provider, metadata_json
		FROM sources ORDER BY alias`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := make([]SourceRecord, 0)
	for rows.Next() {
		var record SourceRecord
		var status, provider, metadata string
		if err := rows.Scan(&record.Alias, &record.Path, &status, &record.Generation,
			&record.LastSynced, &record.ContentHash, &provider, &metadata); err != nil {
			return nil, err
		}
		record.Status = SourceStatus(status)
		record.Provider = provider
		record.Metadata = json.RawMessage(metadata)
		records = append(records, record)
	}
	return records, rows.Err()
}

// SourceByInput resolves a user-supplied source path or URI to its registry
// entry. The materialized path, metadata URI and alias are all accepted.
func (s *Store) SourceByInput(ctx context.Context, input string) (SourceRecord, bool, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return SourceRecord{}, false, nil
	}
	records, err := s.ListSources(ctx)
	if err != nil {
		return SourceRecord{}, false, err
	}
	for _, record := range records {
		if record.Path == input || record.Alias == input {
			return record, true, nil
		}
		var metadata struct {
			URI string `json:"uri"`
		}
		if len(record.Metadata) > 0 && json.Unmarshal(record.Metadata, &metadata) == nil && metadata.URI == input {
			return record, true, nil
		}
	}
	return SourceRecord{}, false, nil
}

// RemoveSource marks a source removed without deleting its index rows.
func (s *Store) RemoveSource(ctx context.Context, alias string) error {
	return s.SetSourceStatus(ctx, alias, SourceRemoved)
}

// PruneSource removes the source and its bundle rows.
func (s *Store) PruneSource(ctx context.Context, alias string) error {
	record, ok, err := s.SourceByAlias(ctx, alias)
	if err != nil {
		return err
	}
	if !ok {
		return ErrSourceNotFound
	}
	if _, err := s.db.ExecContext(ctx, "DELETE FROM sources WHERE alias = ?", alias); err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, "DELETE FROM bundles WHERE bundle_root = ?", record.Path)
	return err
}

// BackfillSources derives a registry from existing bundle roots.
func (s *Store) BackfillSources(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, "SELECT bundle_root FROM bundles ORDER BY bundle_root")
	if err != nil {
		return err
	}
	roots := make([]string, 0)
	for rows.Next() {
		var root string
		if err := rows.Scan(&root); err != nil {
			rows.Close()
			return err
		}
		roots = append(roots, root)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	used := make(map[string]bool)
	for _, root := range roots {
		base := sanitizeAlias(root)
		alias := base
		for suffix := 2; used[alias]; suffix++ {
			alias = base + "-" + strconv.Itoa(suffix)
		}
		used[alias] = true
		if err := s.UpsertSource(ctx, SourceRecord{
			Alias:      alias,
			Path:       root,
			Status:     SourceActive,
			Generation: 1,
			Provider:   "path",
		}); err != nil {
			return err
		}
	}
	return nil
}

func sanitizeAlias(path string) string {
	base := strings.ToLower(filepath.Base(filepath.Clean(path)))
	var builder strings.Builder
	lastDash := false
	for _, char := range base {
		switch {
		case char >= 'a' && char <= 'z', char >= '0' && char <= '9':
			builder.WriteRune(char)
			lastDash = false
		default:
			if !lastDash && builder.Len() > 0 {
				builder.WriteByte('-')
				lastDash = true
			}
		}
	}
	alias := strings.Trim(builder.String(), "-")
	if alias == "" || alias == "contract" || alias == "weave" || alias == "workspace" {
		return "bundle"
	}
	return alias
}
