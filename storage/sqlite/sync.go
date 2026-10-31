package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine/compression"
	"github.com/prulloac/weave/engine/okf"
	"github.com/prulloac/weave/engine/privacy"
)

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
			warnings, _, err := insertSyncedConcept(ctx, tx, root, concept, codec, options.Enricher, options.Provider, now)
			privacySkipped += warnings
			if err != nil {
				if errors.Is(err, privacy.ErrKeyUnavailable) && mode == "permissive" {
					privacySkipped++
					continue
				}
				return contractv1.SyncReport{}, err
			}
			added++
		case oldHash != hash:
			previous, err := loadEnrichments(ctx, tx, root, concept.ID)
			if err != nil {
				return contractv1.SyncReport{}, err
			}
			if _, err := tx.ExecContext(ctx,
				"DELETE FROM documents WHERE bundle_root = ? AND file_path = ?", root, path); err != nil {
				return contractv1.SyncReport{}, err
			}
			warnings, enriched, err := insertSyncedConcept(ctx, tx, root, concept, codec, options.Enricher, options.Provider, now)
			privacySkipped += warnings
			if err != nil {
				if errors.Is(err, privacy.ErrKeyUnavailable) && mode == "permissive" {
					privacySkipped++
					continue
				}
				return contractv1.SyncReport{}, err
			}
			if !enriched {
				for _, row := range previous {
					if err := insertEnrichment(ctx, tx, root, concept.ID, row.EnricherID, []byte(row.Data)); err != nil {
						return contractv1.SyncReport{}, err
					}
				}
			}
			updated++
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
