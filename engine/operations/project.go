package operations

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
	"time"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine"
	"github.com/prulloac/weave/engine/assets"
	"github.com/prulloac/weave/engine/extensions"
)

const projectAnalysisTimeout = 120 * time.Second

func (e *Executor) projectExtension(ctx context.Context, request contractv1.OperationRequest) (extensions.Extension, extensions.Manifest, bool, error) {
	if request.Enricher == "" {
		return extensions.Extension{}, extensions.Manifest{}, false, nil
	}
	extension, err := extensions.Resolve(request.Path, request.Enricher)
	if err != nil {
		return extensions.Extension{}, extensions.Manifest{}, false, err
	}
	manifest, err := extensions.DescribeWithTimeout(ctx, extension, extensionTimeout(request))
	if err != nil {
		// A missing or malformed manifest degrades to per-document enrichment.
		return extension, extensions.Manifest{}, false, nil
	}
	return extension, manifest, manifest.SupportsProjectAnalysis(), nil
}

func projectAssetTypes(manifest extensions.Manifest) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(manifest.AssetTypes)+len(manifest.SourceExtensions))
	add := func(mediaType string) {
		if mediaType == "" || seen[mediaType] {
			return
		}
		seen[mediaType] = true
		result = append(result, mediaType)
	}
	for _, mediaType := range manifest.AssetTypes {
		add(mediaType)
	}
	for _, extension := range manifest.SourceExtensions {
		add(assets.MediaTypeForExtension(extension))
	}
	return result
}

func (e *Executor) applyProjectAnalysis(
	ctx context.Context,
	request contractv1.OperationRequest,
	extension extensions.Extension,
	eng *engine.Engine,
	scanned []contractv1.OkfConcept,
) error {
	response, err := e.runProjectAnalysis(ctx, request, extension, scanned)
	if err != nil {
		return err
	}
	applyProjectResponse(eng.Bundle(), response, scanned)
	return nil
}

func (e *Executor) runProjectAnalysis(
	ctx context.Context,
	request contractv1.OperationRequest,
	extension extensions.Extension,
	scanned []contractv1.OkfConcept,
) (extensions.ProjectAnalysisResponse, error) {
	files := make([]extensions.ProjectFile, 0, len(scanned))
	for _, concept := range scanned {
		files = append(files, extensions.ProjectFile{
			ID:          concept.ID,
			Path:        concept.Path,
			MediaType:   concept.MediaType,
			ContentHash: hashBytes(concept.BodyBytes),
		})
	}
	runner := extensions.Runner{Timeout: projectTimeout(request)}
	response, err := runner.ProjectAnalysis(ctx, extension, extensions.ProjectAnalysisRequest{
		BundleRoot: absoluteRoot(request.Path),
		Files:      files,
		Options: extensions.ProjectOptions{
			Relations: []string{"imports", "declares", "references", "calls", "inherits", "implements"},
		},
	})
	if err != nil {
		return extensions.ProjectAnalysisResponse{}, err
	}
	for _, warning := range response.Warnings {
		e.warn("project %s: %s", extension.Name, warning)
	}
	return response, nil
}

func projectTimeout(request contractv1.OperationRequest) time.Duration {
	if request.TimeoutMS > 0 {
		return time.Duration(request.TimeoutMS) * time.Millisecond
	}
	return projectAnalysisTimeout
}

func hashBytes(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func absoluteRoot(root string) string {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return root
	}
	return absolute
}

func applyProjectResponse(bundle *contractv1.Bundle, response extensions.ProjectAnalysisResponse, analyzed []contractv1.OkfConcept) {
	requested := make(map[string]bool, len(analyzed))
	for _, concept := range analyzed {
		requested[concept.ID] = true
	}
	index := make(map[string]int, len(bundle.Concepts))
	for position := range bundle.Concepts {
		index[bundle.Concepts[position].ID] = position
	}
	kept := make(map[string]bool, len(response.Documents))
	for _, document := range response.Documents {
		kept[document.ID] = true
	}
	filtered := bundle.Concepts[:0]
	for _, concept := range bundle.Concepts {
		if requested[concept.ID] && !kept[concept.ID] {
			// The project analyzer skipped this source file (ignore rules,
			// generated code or a resource limit).
			continue
		}
		filtered = append(filtered, concept)
	}
	bundle.Concepts = filtered
	index = make(map[string]int, len(bundle.Concepts))
	for position := range bundle.Concepts {
		index[bundle.Concepts[position].ID] = position
	}
	upsert := func(document extensions.ProjectDocument, docType, kind string) int {
		if position, ok := index[document.ID]; ok {
			concept := &bundle.Concepts[position]
			applyProjectMetadata(concept, document)
			return position
		}
		title := document.Title
		concept := contractv1.OkfConcept{
			ID:              document.ID,
			Path:            document.ID,
			Type:            docType,
			Title:           &title,
			Description:     stringPointer(document.Description),
			Keywords:        document.Keywords,
			Body:            document.Body,
			BodyText:        true,
			ContentKind:     kind,
			ContentMetadata: document.ContentMetadata,
			Links:           []contractv1.OkfLink{},
		}
		bundle.Concepts = append(bundle.Concepts, concept)
		position := len(bundle.Concepts) - 1
		index[document.ID] = position
		return position
	}
	for _, document := range response.Documents {
		upsert(document, "Code", "blob")
		for _, symbol := range document.SymbolDocuments {
			upsert(symbol, "CodeSymbol", "symbol")
		}
	}
	external := make(map[string]bool, len(response.ExternalDocuments))
	references := make(map[string]int, len(response.Relations))
	for _, relation := range response.Relations {
		references[relation.To]++
	}
	for _, document := range response.ExternalDocuments {
		external[document.ID] = references[document.ID] > 1
	}
	for _, document := range response.ExternalDocuments {
		if !external[document.ID] {
			continue
		}
		upsert(document, "CodeExternal", "symbol")
	}
	for _, relation := range response.Relations {
		if _, isExternal := external[relation.To]; isExternal && !external[relation.To] {
			continue
		}
		position, ok := index[relation.From]
		if !ok {
			continue
		}
		if _, ok := index[relation.To]; !ok {
			continue
		}
		kind := contractv1.RelationKind(strings.TrimSpace(relation.Kind))
		if !validProjectRelation(kind) {
			continue
		}
		bundle.Concepts[position].Relations = append(bundle.Concepts[position].Relations, contractv1.ConceptRelation{
			To:   relation.To,
			Kind: kind,
		})
	}
}

func applyProjectMetadata(concept *contractv1.OkfConcept, document extensions.ProjectDocument) {
	if document.Title != "" {
		title := document.Title
		concept.Title = &title
	}
	if document.Description != "" {
		description := document.Description
		concept.Description = &description
	}
	if len(document.Keywords) > 0 {
		concept.Keywords = append([]string(nil), document.Keywords...)
	}
	if len(document.ContentMetadata) > 0 {
		concept.ContentMetadata = document.ContentMetadata
	}
	if concept.Type == "" {
		concept.Type = "Code"
	}
}

func validProjectRelation(kind contractv1.RelationKind) bool {
	switch kind {
	case contractv1.RelationImports, contractv1.RelationDeclares, contractv1.RelationReferences,
		contractv1.RelationCalls, contractv1.RelationInherits, contractv1.RelationImplements:
		return true
	default:
		return false
	}
}

func stringPointer(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
