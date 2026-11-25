package operations

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine/extensions"
	"github.com/prulloac/weave/engine/privacy"
	sqlitestore "github.com/prulloac/weave/storage/sqlite"
)

type enrichmentItemResult struct {
	id       string
	apply    sqlitestore.EnrichmentApply
	warnings []string
	err      error
}

type enrichmentJob struct {
	concepts []contractv1.OkfConcept
}

func enrichmentParallelism(request contractv1.OperationRequest) (int, error) {
	if request.Parallelism == 0 {
		return 1, nil
	}
	if request.Parallelism < 1 || request.Parallelism > 8 {
		return 0, fmt.Errorf("%w: --extension-parallelism must be between 1 and 8", extensions.ErrProtocol)
	}
	return request.Parallelism, nil
}

func (e *Executor) enrichOne(
	ctx context.Context,
	bundleRoot string,
	extension extensions.Extension,
	manifest extensions.Manifest,
	runner extensions.Runner,
	concept contractv1.OkfConcept,
) (sqlitestore.EnrichmentApply, error) {
	result := e.runEnrichmentJob(ctx, bundleRoot, extension, manifest, runner, enrichmentJob{concepts: []contractv1.OkfConcept{concept}}, false)
	item := result[0]
	for _, warning := range item.warnings {
		e.warn("extension %s: %s", extension.Name, warning)
	}
	return item.apply, item.err
}

func (e *Executor) enrichmentResults(
	ctx context.Context,
	bundleRoot string,
	extension extensions.Extension,
	manifest extensions.Manifest,
	runner extensions.Runner,
	concepts []contractv1.OkfConcept,
	parallelism int,
	batch bool,
) (map[string]sqlitestore.EnrichmentApply, map[string]error) {
	jobs := make([]enrichmentJob, 0, len(concepts))
	if batch {
		for start := 0; start < len(concepts); start += extensions.MaxBatchRequests {
			end := start + extensions.MaxBatchRequests
			if end > len(concepts) {
				end = len(concepts)
			}
			jobs = append(jobs, enrichmentJob{concepts: concepts[start:end]})
		}
	} else {
		for _, concept := range concepts {
			jobs = append(jobs, enrichmentJob{concepts: []contractv1.OkfConcept{concept}})
		}
	}

	results := make([][]enrichmentItemResult, len(jobs))
	if parallelism <= 1 {
		for index, job := range jobs {
			results[index] = e.runEnrichmentJob(ctx, bundleRoot, extension, manifest, runner, job, batch)
		}
	} else {
		indexes := make(chan int)
		var group sync.WaitGroup
		for worker := 0; worker < parallelism; worker++ {
			group.Add(1)
			go func() {
				defer group.Done()
				for index := range indexes {
					results[index] = e.runEnrichmentJob(ctx, bundleRoot, extension, manifest, runner, jobs[index], batch)
				}
			}()
		}
		for index := range jobs {
			indexes <- index
		}
		close(indexes)
		group.Wait()
	}

	applies := make(map[string]sqlitestore.EnrichmentApply, len(concepts))
	errs := make(map[string]error)
	for _, jobResults := range results {
		for _, item := range jobResults {
			for _, warning := range item.warnings {
				e.warn("extension %s: %s", extension.Name, warning)
			}
			if item.err != nil {
				errs[item.id] = item.err
				continue
			}
			applies[item.id] = item.apply
		}
	}
	return applies, errs
}

func (e *Executor) runEnrichmentJob(
	ctx context.Context,
	bundleRoot string,
	extension extensions.Extension,
	manifest extensions.Manifest,
	runner extensions.Runner,
	job enrichmentJob,
	batch bool,
) []enrichmentItemResult {
	if !batch {
		concept := job.concepts[0]
		workspace, err := makeEnrichmentWorkspace()
		if err != nil {
			e := enrichmentItemResult{id: concept.ID, err: err}
			e.warnings = append(e.warnings, err.Error())
			return []enrichmentItemResult{e}
		}
		defer os.RemoveAll(workspace)
		response, err := runner.Enrich(ctx, extension, enrichmentRequest(concept, bundleRoot, workspace))
		if err != nil {
			return []enrichmentItemResult{{id: concept.ID, err: err, warnings: []string{err.Error()}}}
		}
		apply, warnings, err := e.applyEnrichmentResponse(extension, concept, response, workspace, enrichmentCacheKey(extension, manifest, concept))
		return []enrichmentItemResult{{id: concept.ID, apply: apply, warnings: warnings, err: err}}
	}

	items := make([]batchEnrichmentItem, 0, len(job.concepts))
	requests := make([]extensions.EnrichmentRequest, 0, len(job.concepts))
	for _, concept := range job.concepts {
		workspace, err := makeEnrichmentWorkspace()
		if err != nil {
			items = append(items, batchEnrichmentItem{concept: concept, err: err})
			continue
		}
		items = append(items, batchEnrichmentItem{
			concept:   concept,
			workspace: workspace,
			request:   enrichmentRequest(concept, bundleRoot, workspace),
		})
		requests = append(requests, items[len(items)-1].request)
	}
	responses, itemErrors, batchErr := runner.EnrichBatch(ctx, extension, requests)
	results := make([]enrichmentItemResult, len(items))
	responseIndex := 0
	for index, item := range items {
		results[index].id = item.concept.ID
		if item.err != nil {
			results[index].err = item.err
			results[index].warnings = []string{item.err.Error()}
			continue
		}
		if batchErr != nil {
			results[index].err = batchErr
			results[index].warnings = []string{batchErr.Error()}
			continue
		}
		if itemErrors[responseIndex] != nil {
			results[index].err = itemErrors[responseIndex]
			results[index].warnings = []string{itemErrors[responseIndex].Error()}
			responseIndex++
			continue
		}
		apply, warnings, err := e.applyEnrichmentResponse(extension, item.concept, responses[responseIndex], item.workspace, enrichmentCacheKey(extension, manifest, item.concept))
		results[index].apply = apply
		results[index].warnings = warnings
		results[index].err = err
		responseIndex++
	}
	for _, item := range items {
		if item.workspace != "" {
			os.RemoveAll(item.workspace)
		}
	}
	return results
}

type batchEnrichmentItem struct {
	concept   contractv1.OkfConcept
	workspace string
	request   extensions.EnrichmentRequest
	err       error
}

func makeEnrichmentWorkspace() (string, error) {
	workspace, err := os.MkdirTemp("", "weave-extension-*")
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Join(workspace, "files"), 0o700); err != nil {
		os.RemoveAll(workspace)
		return "", err
	}
	return workspace, nil
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
	cacheKey string,
) (sqlitestore.EnrichmentApply, []string, error) {
	warnings := make([]string, 0, len(response.Warnings)+2)
	for _, warning := range response.Warnings {
		if strings.TrimSpace(warning) != "" {
			warnings = append(warnings, warning)
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
		Warnings:        len(response.Warnings),
	}
	files, err := extensions.ValidateWorkspace(workspace)
	if err != nil {
		warnings = append(warnings, err.Error())
		apply.Warnings++
	} else if files != nil {
		for index := range files {
			content, err := os.ReadFile(files[index].SourcePath)
			if err != nil {
				warnings = append(warnings, err.Error())
				apply.Warnings++
				files = nil
				break
			}
			files[index].Content = content
		}
		if files != nil {
			apply.ExtensionData = files
			apply.ExtensionReady = true
		}
	}
	data, err := json.Marshal(map[string]any{
		"name":              extension.Name,
		"inferred_keywords": response.InferredKeywords,
		"response":          response.CustomMetadata,
		"cache_key":         cacheKey,
		"has_files":         len(apply.ExtensionData) > 0,
		"apply": map[string]any{
			"title":            apply.Title,
			"description":      apply.Description,
			"text":             apply.Text,
			"text_source":      apply.TextSource,
			"keywords":         apply.Keywords,
			"content_metadata": apply.ContentMetadata,
		},
	})
	if err != nil {
		return sqlitestore.EnrichmentApply{}, warnings, err
	}
	apply.Data = data
	return apply, warnings, nil
}

func enrichmentCacheKey(extension extensions.Extension, manifest extensions.Manifest, concept contractv1.OkfConcept) string {
	version := manifest.Version
	if strings.TrimSpace(version) == "" {
		version = "unversioned"
	}
	mediaType := concept.MediaType
	if strings.TrimSpace(mediaType) == "" {
		mediaType = "text/markdown"
	}
	payload := map[string]any{
		"enricher":    extension.Name,
		"version":     version,
		"cache_key":   manifest.CacheKey,
		"doc_id":      concept.ID,
		"doc_path":    concept.Path,
		"media_type":  mediaType,
		"title":       enrichmentTitle(concept),
		"frontmatter": concept.Frontmatter,
		"body":        privacy.RedactForAI(concept),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func enrichmentTitle(concept contractv1.OkfConcept) string {
	if concept.Title != nil {
		return *concept.Title
	}
	return concept.ID
}

type cachedEnrichmentPayload struct {
	CacheKey string `json:"cache_key"`
	HasFiles bool   `json:"has_files"`
	Apply    struct {
		Title           string         `json:"title"`
		Description     string         `json:"description"`
		Text            string         `json:"text"`
		TextSource      string         `json:"text_source"`
		Keywords        []string       `json:"keywords"`
		ContentMetadata map[string]any `json:"content_metadata"`
	} `json:"apply"`
}

func (e *Executor) cachedEnrichmentApplies(
	ctx context.Context,
	store *sqlitestore.Store,
	request contractv1.OperationRequest,
	extension extensions.Extension,
	manifest extensions.Manifest,
	concepts []contractv1.OkfConcept,
) (map[string]sqlitestore.EnrichmentApply, []contractv1.OkfConcept) {
	applies := make(map[string]sqlitestore.EnrichmentApply)
	if store == nil || request.Force {
		return applies, concepts
	}
	remaining := make([]contractv1.OkfConcept, 0, len(concepts))
	for _, concept := range concepts {
		cacheKey := enrichmentCacheKey(extension, manifest, concept)
		data, ok, err := store.EnrichmentData(ctx, request.Path, concept.ID, extension.Name)
		if err != nil || !ok {
			remaining = append(remaining, concept)
			continue
		}
		var cached cachedEnrichmentPayload
		if err := json.Unmarshal(data, &cached); err != nil || cached.CacheKey != cacheKey || cached.HasFiles || cacheKey == "" {
			remaining = append(remaining, concept)
			continue
		}
		applies[concept.ID] = sqlitestore.EnrichmentApply{
			Enricher:        extension.Name,
			Keywords:        cached.Apply.Keywords,
			Title:           cached.Apply.Title,
			Description:     cached.Apply.Description,
			Text:            cached.Apply.Text,
			TextSource:      cached.Apply.TextSource,
			ContentMetadata: cached.Apply.ContentMetadata,
			Data:            data,
		}
	}
	return applies, remaining
}
