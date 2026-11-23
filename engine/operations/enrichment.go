package operations

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine/extensions"
	"github.com/prulloac/weave/engine/privacy"
	sqlitestore "github.com/prulloac/weave/storage/sqlite"
)

type batchEnrichmentItem struct {
	concept   contractv1.OkfConcept
	workspace string
	request   extensions.EnrichmentRequest
}

func (e *Executor) enrichOne(
	ctx context.Context,
	bundleRoot string,
	extension extensions.Extension,
	runner extensions.Runner,
	concept contractv1.OkfConcept,
) (sqlitestore.EnrichmentApply, error) {
	workspace, err := os.MkdirTemp("", "weave-extension-*")
	if err != nil {
		e.warn("extension %s: %v", extension.Name, err)
		return sqlitestore.EnrichmentApply{}, err
	}
	defer os.RemoveAll(workspace)
	if err := os.MkdirAll(filepath.Join(workspace, "files"), 0o700); err != nil {
		e.warn("extension %s: %v", extension.Name, err)
		return sqlitestore.EnrichmentApply{}, err
	}
	response, err := runner.Enrich(ctx, extension, enrichmentRequest(concept, bundleRoot, workspace))
	if err != nil {
		e.warn("extension %s: %v", extension.Name, err)
		return sqlitestore.EnrichmentApply{}, err
	}
	return e.applyEnrichmentResponse(extension, concept, response, workspace)
}

func (e *Executor) enrichBatch(
	ctx context.Context,
	bundleRoot string,
	extension extensions.Extension,
	runner extensions.Runner,
	concepts []contractv1.OkfConcept,
) (map[string]sqlitestore.EnrichmentApply, map[string]error) {
	applies := make(map[string]sqlitestore.EnrichmentApply, len(concepts))
	errs := make(map[string]error)
	for start := 0; start < len(concepts); start += extensions.MaxBatchRequests {
		end := start + extensions.MaxBatchRequests
		if end > len(concepts) {
			end = len(concepts)
		}
		items := make([]batchEnrichmentItem, 0, end-start)
		requests := make([]extensions.EnrichmentRequest, 0, end-start)
		for _, concept := range concepts[start:end] {
			workspace, err := os.MkdirTemp("", "weave-extension-*")
			if err != nil {
				errs[concept.ID] = err
				e.warn("extension %s: %v", extension.Name, err)
				continue
			}
			if err := os.MkdirAll(filepath.Join(workspace, "files"), 0o700); err != nil {
				os.RemoveAll(workspace)
				errs[concept.ID] = err
				e.warn("extension %s: %v", extension.Name, err)
				continue
			}
			item := batchEnrichmentItem{
				concept:   concept,
				workspace: workspace,
				request:   enrichmentRequest(concept, bundleRoot, workspace),
			}
			items = append(items, item)
			requests = append(requests, item.request)
		}
		responses, itemErrors, batchErr := runner.EnrichBatch(ctx, extension, requests)
		for index, item := range items {
			if batchErr != nil {
				errs[item.concept.ID] = batchErr
				e.warn("extension %s: %v", extension.Name, batchErr)
				continue
			}
			if itemErrors[index] != nil {
				errs[item.concept.ID] = itemErrors[index]
				e.warn("extension %s: %v", extension.Name, itemErrors[index])
				continue
			}
			apply, err := e.applyEnrichmentResponse(extension, item.concept, responses[index], item.workspace)
			if err != nil {
				errs[item.concept.ID] = err
				continue
			}
			applies[item.concept.ID] = apply
		}
		for _, item := range items {
			os.RemoveAll(item.workspace)
		}
	}
	return applies, errs
}

func enrichmentRequest(concept contractv1.OkfConcept, bundleRoot, workspace string) extensions.EnrichmentRequest {
	title := concept.ID
	if concept.Title != nil {
		title = *concept.Title
	}
	if absolute, err := filepath.Abs(bundleRoot); err == nil {
		bundleRoot = absolute
	}
	return extensions.EnrichmentRequest{
		DocID:       concept.ID,
		DocPath:     concept.Path,
		BundleRoot:  bundleRoot,
		Workspace:   workspace,
		Title:       title,
		Frontmatter: concept.Frontmatter,
		Body:        privacy.RedactForAI(concept),
	}
}

func (e *Executor) applyEnrichmentResponse(
	extension extensions.Extension,
	concept contractv1.OkfConcept,
	response extensions.EnrichmentResponse,
	workspace string,
) (sqlitestore.EnrichmentApply, error) {
	data, err := json.Marshal(map[string]any{
		"name":              extension.Name,
		"inferred_keywords": response.InferredKeywords,
		"response":          response.CustomMetadata,
	})
	if err != nil {
		return sqlitestore.EnrichmentApply{}, err
	}
	for _, warning := range response.Warnings {
		if strings.TrimSpace(warning) != "" {
			e.warn("extension %s: %s", extension.Name, warning)
		}
	}
	apply := sqlitestore.EnrichmentApply{
		Enricher:        extension.Name,
		Keywords:        mergeKeywords(concept.Keywords, response.InferredKeywords),
		Title:           response.InferredTitle,
		Description:     response.InferredDescription,
		Text:            response.InferredText,
		TextSource:      response.InferredTextSource,
		ContentMetadata: response.InferredContentMetadata,
		Data:            data,
		Warnings:        len(response.Warnings),
	}
	files, err := extensions.ValidateWorkspace(workspace)
	if err != nil {
		e.warn("extension %s: %v", extension.Name, err)
		apply.Warnings++
		return apply, nil
	}
	if files != nil {
		for index := range files {
			content, err := os.ReadFile(files[index].SourcePath)
			if err != nil {
				e.warn("extension %s: %v", extension.Name, err)
				apply.Warnings++
				return apply, nil
			}
			files[index].Content = content
		}
		apply.ExtensionData = files
		apply.ExtensionReady = true
	}
	return apply, nil
}
