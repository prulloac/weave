package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine/compression"
)

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
		       privacy_policy, privacy_provider, privacy_key_id, privacy_provider_name,
		       privacy_key_version, body_codec
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
	if err := s.mergeEnrichmentKeywords(ctx, root, concepts); err != nil {
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

func scanConcept(rows *sql.Rows) (contractv1.OkfConcept, string, error) {
	var (
		concept                                            contractv1.OkfConcept
		title                                              string
		description, docType, status                       sql.NullString
		policy, provider, keyID                            sql.NullString
		providerName                                       sql.NullString
		keyVersion                                         sql.NullInt64
		codec                                              string
		keywordsJSON, frontmatterJSON, tagsJSON, linksJSON string
		body                                               []byte
	)
	if err := rows.Scan(
		&concept.ID, &concept.Path, &title, &description, &docType,
		&keywordsJSON, &frontmatterJSON, &tagsJSON, &status, &linksJSON, &body,
		&policy, &provider, &keyID, &providerName, &keyVersion, &codec,
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
			Policy:       contractv1.PrivacyPolicy(policy.String),
			Provider:     provider.String,
			KeyID:        keyID.String,
			ProviderName: providerName.String,
			KeyVersion:   int(keyVersion.Int64),
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
