package operations

import (
	"context"
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
	runner extensions.Runner,
	concept contractv1.OkfConcept,
) (sqlitestore.EnrichmentApply, error) {
	result := e.runEnrichmentJob(ctx, bundleRoot, extension, runner, enrichmentJob{concepts: []contractv1.OkfConcept{concept}}, false)
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
			results[index] = e.runEnrichmentJob(ctx, bundleRoot, extension, runner, job, batch)
		}
	} else {
		indexes := make(chan int)
		var group sync.WaitGroup
		for worker := 0; worker < parallelism; worker++ {
			group.Add(1)
			go func() {
				defer group.Done()
				for index := range indexes {
					results[index] = e.runEnrichmentJob(ctx, bundleRoot, extension, runner, jobs[index], batch)
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
		apply, warnings, err := e.applyEnrichmentResponse(extension, concept, response, workspace)
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
		apply, warnings, err := e.applyEnrichmentResponse(extension, item.concept, responses[responseIndex], item.workspace)
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
) (sqlitestore.EnrichmentApply, []string, error) {
	data, err := json.Marshal(map[string]any{
		"name":              extension.Name,
		"inferred_keywords": response.InferredKeywords,
		"response":          response.CustomMetadata,
	})
	if err != nil {
		return sqlitestore.EnrichmentApply{}, nil, err
	}
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
		Data:            data,
		Warnings:        len(response.Warnings),
	}
	files, err := extensions.ValidateWorkspace(workspace)
	if err != nil {
		warnings = append(warnings, err.Error())
		apply.Warnings++
		return apply, warnings, nil
	}
	if files != nil {
		for index := range files {
			content, err := os.ReadFile(files[index].SourcePath)
			if err != nil {
				warnings = append(warnings, err.Error())
				apply.Warnings++
				return apply, warnings, nil
			}
			files[index].Content = content
		}
		apply.ExtensionData = files
		apply.ExtensionReady = true
	}
	return apply, warnings, nil
}
