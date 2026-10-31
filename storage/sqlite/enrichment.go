package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"

	contractv1 "github.com/prulloac/weave/contract/v1"
)

type EnrichmentApply struct {
	Enricher string
	Keywords []string
	Data     []byte
}

// Enricher enriches one concept before it is stored.
type Enricher func(ctx context.Context, concept contractv1.OkfConcept) (EnrichmentApply, error)

// DefaultPath returns the default user database path.

func loadEnrichments(ctx context.Context, tx *sql.Tx, root, docID string) ([]storedEnrichmentRow, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT enricher_id, data_json FROM enrichments
		WHERE bundle_root = ? AND doc_id = ?`, root, docID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	stored := make([]storedEnrichmentRow, 0)
	for rows.Next() {
		var row storedEnrichmentRow
		if err := rows.Scan(&row.EnricherID, &row.Data); err != nil {
			return nil, err
		}
		stored = append(stored, row)
	}
	return stored, rows.Err()
}

type storedEnrichmentRow struct {
	EnricherID string
	Data       string
}

func (s *Store) mergeEnrichmentKeywords(
	ctx context.Context,
	root string,
	concepts []contractv1.OkfConcept,
) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT doc_id, data_json FROM enrichments WHERE bundle_root = ?`, root)
	if err != nil {
		return err
	}
	defer rows.Close()

	type storedEnrichment struct {
		InferredKeywords []string `json:"inferred_keywords"`
	}
	byDocument := make(map[string][]string)
	for rows.Next() {
		var docID, data string
		if err := rows.Scan(&docID, &data); err != nil {
			return err
		}
		var stored storedEnrichment
		if err := json.Unmarshal([]byte(data), &stored); err != nil {
			continue
		}
		byDocument[docID] = append(byDocument[docID], stored.InferredKeywords...)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for index := range concepts {
		inferred := byDocument[concepts[index].ID]
		if len(inferred) == 0 {
			continue
		}
		seen := make(map[string]bool, len(concepts[index].Keywords))
		for _, keyword := range concepts[index].Keywords {
			seen[strings.ToLower(keyword)] = true
		}
		for _, keyword := range inferred {
			key := strings.ToLower(keyword)
			if seen[key] {
				continue
			}
			seen[key] = true
			concepts[index].Keywords = append(concepts[index].Keywords, keyword)
		}
	}
	return nil
}

// HasBundle reports whether a bundle root is indexed.
