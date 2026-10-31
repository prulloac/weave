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

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine/compression"
	"github.com/prulloac/weave/engine/graph"
	"github.com/prulloac/weave/engine/privacy"
)

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
	prepared, skipped, err := prepareConcepts(ctx, root, bundle.Concepts, mode == "permissive", codec, options.Enricher, options.Provider)
	if err != nil {
		return contractv1.IngestReport{}, err
	}
	for _, item := range prepared {
		if err := insertConcept(ctx, tx, root, item, now); err != nil {
			return contractv1.IngestReport{}, err
		}
		if item.enricherID != "" && len(item.enrichment) > 0 {
			if err := insertEnrichment(ctx, tx, root, item.concept.ID, item.enricherID, item.enrichment); err != nil {
				return contractv1.IngestReport{}, err
			}
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
			privacy_key_id, privacy_provider_name, privacy_key_version, redacted,
			privacy_spans_json, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		root, concept.ID, concept.Path, contentHash(root, concept), title, description, concept.Type,
		string(keywords), string(frontmatter), string(tags), status, string(links),
		blob, string(storedCodec), policy, provider, keyID, item.providerName, item.keyVersion,
		boolInt(item.redacted), string(spans), now)
	return err
}

func insertSyncedConcept(
	ctx context.Context,
	tx *sql.Tx,
	root string,
	concept contractv1.OkfConcept,
	codec string,
	enricher Enricher,
	provider privacy.Provider,
	now int64,
) (int, bool, error) {
	result, err := privacy.ApplyWith(ctx, root, concept, provider)
	if err != nil {
		return 0, false, err
	}
	item := preparedConcept{
		concept:      concept,
		body:         result.Body,
		codec:        codec,
		directive:    result.Directive,
		redacted:     result.Redacted,
		spans:        result.Spans,
		providerName: result.ProviderName,
		keyVersion:   result.KeyVersion,
	}
	warnings := 0
	if enricher != nil {
		apply, err := enricher(ctx, concept)
		if err != nil {
			warnings++
		} else {
			item.concept.Keywords = apply.Keywords
			item.enricherID = apply.Enricher
			item.enrichment = apply.Data
		}
	}
	if err := insertConcept(ctx, tx, root, item, now); err != nil {
		return warnings, false, err
	}
	if item.enricherID != "" && len(item.enrichment) > 0 {
		if err := insertEnrichment(ctx, tx, root, item.concept.ID, item.enricherID, item.enrichment); err != nil {
			return warnings, false, err
		}
		return warnings, true, nil
	}
	return warnings, false, nil
}

func insertEnrichment(ctx context.Context, tx *sql.Tx, root, docID, enricherID string, data []byte) error {
	_, err := tx.ExecContext(ctx, `
		INSERT OR REPLACE INTO enrichments (bundle_root, doc_id, enricher_id, data_json)
		VALUES (?, ?, ?, ?)`, root, docID, enricherID, string(data))
	return err
}

type preparedConcept struct {
	concept      contractv1.OkfConcept
	body         string
	codec        string
	directive    *contractv1.PrivacyDirective
	redacted     bool
	spans        []privacy.Span
	providerName string
	keyVersion   int
	enricherID   string
	enrichment   []byte
}

func prepareConcepts(
	ctx context.Context,
	root string,
	concepts []contractv1.OkfConcept,
	permissive bool,
	codec string,
	enricher Enricher,
	provider privacy.Provider,
) ([]preparedConcept, int, error) {
	prepared := make([]preparedConcept, 0, len(concepts))
	warnings := 0
	for _, concept := range concepts {
		result, err := privacy.ApplyWith(ctx, root, concept, provider)
		if err != nil {
			if errors.Is(err, privacy.ErrKeyUnavailable) && permissive {
				warnings++
				continue
			}
			return nil, 0, err
		}
		item := preparedConcept{
			concept:      concept,
			body:         result.Body,
			codec:        codec,
			directive:    result.Directive,
			redacted:     result.Redacted,
			spans:        result.Spans,
			providerName: result.ProviderName,
			keyVersion:   result.KeyVersion,
		}
		if enricher != nil {
			apply, err := enricher(ctx, concept)
			if err != nil {
				warnings++
			} else {
				item.concept.Keywords = apply.Keywords
				item.enricherID = apply.Enricher
				item.enrichment = apply.Data
			}
		}
		prepared = append(prepared, item)
	}
	return prepared, warnings, nil
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

func contentHash(root string, concept contractv1.OkfConcept) string {
	path := filepath.Join(root, filepath.FromSlash(concept.Path))
	if data, err := os.ReadFile(path); err == nil {
		sum := sha256.Sum256(data)
		return hex.EncodeToString(sum[:])
	}
	sum := sha256.Sum256([]byte(concept.ID + "\x00" + concept.Body))
	return hex.EncodeToString(sum[:])
}
