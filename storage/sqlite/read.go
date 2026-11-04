package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	contractv1 "github.com/prulloac/weave/contract/v1"
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
		var path string
		var codec sql.NullString
		if err := rows.Scan(&path, &codec); err != nil {
			return nil, err
		}
		codecs[path] = codec.String
	}
	return codecs, rows.Err()
}

// AssetTypes returns the stored blob filter for a bundle.
func (s *Store) AssetTypes(ctx context.Context, bundleRoot string) ([]string, error) {
	root, err := normalizeRoot(bundleRoot)
	if err != nil {
		return nil, err
	}
	var encoded string
	err = s.db.QueryRowContext(ctx,
		"SELECT asset_types_json FROM bundles WHERE bundle_root = ?", root).Scan(&encoded)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var types []string
	if encoded != "" {
		if err := json.Unmarshal([]byte(encoded), &types); err != nil {
			return nil, err
		}
	}
	return types, nil
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
		       frontmatter_json, tags_json, status, links_json, content_kind,
		       media_type, content_metadata, body_raw, body_compressed, body_codec,
		       size, stored_size, content_hash, privacy_policy, privacy_provider,
		       privacy_key_id, privacy_provider_name, privacy_key_version
		FROM documents WHERE bundle_root = ? ORDER BY id`, root)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	concepts := make([]contractv1.OkfConcept, 0)
	decodeWarnings := make([]string, 0)
	for rows.Next() {
		concept, warning, err := scanConcept(ctx, rows, root)
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

func scanConcept(ctx context.Context, rows *sql.Rows, root string) (contractv1.OkfConcept, string, error) {
	var (
		concept                                            contractv1.OkfConcept
		title                                              string
		description, docType, status                       sql.NullString
		policy, provider, keyID                            sql.NullString
		providerName                                       sql.NullString
		keyVersion                                         sql.NullInt64
		bodyRaw, codec                                     sql.NullString
		kind, mediaType, metadata, contentHash             string
		size, storedSize                                   int64
		keywordsJSON, frontmatterJSON, tagsJSON, linksJSON string
		bodyCompressed                                     []byte
	)
	if err := rows.Scan(
		&concept.ID, &concept.Path, &title, &description, &docType,
		&keywordsJSON, &frontmatterJSON, &tagsJSON, &status, &linksJSON, &kind,
		&mediaType, &metadata, &bodyRaw, &bodyCompressed, &codec, &size, &storedSize,
		&contentHash, &policy, &provider, &keyID, &providerName, &keyVersion,
	); err != nil {
		return contractv1.OkfConcept{}, "", err
	}
	warning := ""
	form := ""
	switch {
	case bodyRaw.Valid:
		concept.Body = bodyRaw.String
		concept.BodyBytes = []byte(bodyRaw.String)
		concept.BodyText = true
		form = "raw"
	case codec.Valid:
		decoded, err := decodeStoredBody(ctx, root, codec.String, bodyCompressed)
		if err != nil {
			warning = "BODY_DECODE_FAILED"
		} else {
			concept.BodyBytes = decoded
			if kind != "blob" {
				concept.Body = string(decoded)
				concept.BodyText = true
			}
			form = "compressed"
		}
	default:
		warning = "BODY_DECODE_FAILED"
	}
	concept.ContentKind = kind
	concept.MediaType = mediaType
	if kind == "blob" && metadata != "" {
		if err := json.Unmarshal([]byte(metadata), &concept.ContentMetadata); err != nil {
			return contractv1.OkfConcept{}, "", err
		}
	}
	concept.BodyForm = form
	concept.BodyCodec = codec.String
	concept.Size = size
	concept.StoredSize = storedSize
	concept.ContentHash = contentHash
	concept.Title = optionalString(title)
	concept.Description = optionalString(description.String)
	concept.Type = docType.String
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
