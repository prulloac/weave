package operations

import (
	"context"
	"fmt"
	"sort"
	"strings"

	contractv1 "github.com/prulloac/weave/contract/v1"
)

type resolvedSource struct {
	Alias string
	Path  string
}

func (e *Executor) executeMulti(
	ctx context.Context,
	request contractv1.OperationRequest,
) any {
	sources, err := resolveSources(request.Sources)
	if err != nil {
		return failure(contractv1.ErrInvalidRequest, err.Error(), nil)
	}

	switch request.Op {
	case contractv1.OpSearch:
		return e.multiSearch(ctx, request, sources)
	case contractv1.OpList:
		return e.multiList(ctx, request, sources)
	case contractv1.OpAudit:
		return e.multiAudit(ctx, request, sources)
	case contractv1.OpParse:
		return e.multiParse(ctx, request, sources)
	case contractv1.OpGraph:
		return e.multiGraph(ctx, request, sources)
	case contractv1.OpShow, contractv1.OpBacklinks, contractv1.OpRelate, contractv1.OpPath:
		return e.multiTarget(ctx, request, sources)
	case contractv1.OpExplain:
		return e.multiExplain(ctx, request, sources)
	case contractv1.OpIngest, contractv1.OpRebuild, contractv1.OpSync:
		return e.multiWrite(ctx, request, sources)
	default:
		return failure(contractv1.ErrInvalidRequest, "--source is not supported for this operation", map[string]any{
			"op": request.Op,
		})
	}
}

func resolveSources(specs []contractv1.SourceSpec) ([]resolvedSource, error) {
	aliases := make(map[string]bool)
	paths := make(map[string]bool)
	sources := make([]resolvedSource, 0, len(specs))
	for _, spec := range specs {
		alias := strings.TrimSpace(spec.Alias)
		path := strings.TrimSpace(spec.Path)
		if alias == "" || path == "" {
			return nil, fmt.Errorf("source alias and path are required")
		}
		if !validSourceAlias(alias) {
			return nil, fmt.Errorf("invalid source alias %q", alias)
		}
		if aliases[alias] {
			return nil, fmt.Errorf("duplicate source alias %q", alias)
		}
		if paths[path] {
			return nil, fmt.Errorf("duplicate source path %q", path)
		}
		aliases[alias] = true
		paths[path] = true
		sources = append(sources, resolvedSource{Alias: alias, Path: path})
	}
	return sources, nil
}

func validSourceAlias(alias string) bool {
	if alias == "" {
		return false
	}
	for index, char := range alias {
		switch {
		case char >= 'a' && char <= 'z':
		case char >= '0' && char <= '9':
		case char == '-' && index > 0:
		default:
			return false
		}
	}
	return true
}

func singleSourceRequest(request contractv1.OperationRequest, source resolvedSource) contractv1.OperationRequest {
	request.Path = source.Path
	request.Sources = nil
	return request
}

func unwrap(value any) (any, *contractv1.FailureEnvelope) {
	switch envelope := value.(type) {
	case contractv1.SuccessEnvelope:
		return envelope.Result, nil
	case contractv1.FailureEnvelope:
		return nil, &envelope
	default:
		return nil, &contractv1.FailureEnvelope{
			OK: false,
			Error: contractv1.OperationError{
				Code:    contractv1.ErrInternal,
				Message: "internal operation error",
			},
		}
	}
}

func (e *Executor) multiSearch(
	ctx context.Context,
	request contractv1.OperationRequest,
	sources []resolvedSource,
) any {
	merged := contractv1.SearchResponse{
		Query:   request.Query,
		Mode:    request.Mode,
		Keyword: []contractv1.KeywordHit{},
		Fuzzy:   []contractv1.FuzzyHit{},
		Hybrid:  []contractv1.HybridHit{},
		Lexical: []contractv1.LexicalHit{},
	}
	if merged.Mode == "" {
		merged.Mode = contractv1.SearchHybrid
	}
	for _, source := range sources {
		value, failed := unwrap(e.Execute(ctx, singleSourceRequest(request, source)))
		if failed != nil {
			return *failed
		}
		response, ok := value.(contractv1.SearchResponse)
		if !ok {
			return failure(contractv1.ErrInternal, "unexpected search payload", nil)
		}
		merged.TopK = response.TopK
		for _, hit := range response.Keyword {
			hit.Source = source.Alias
			merged.Keyword = append(merged.Keyword, hit)
		}
		for _, hit := range response.Fuzzy {
			hit.Source = source.Alias
			merged.Fuzzy = append(merged.Fuzzy, hit)
		}
		for _, hit := range response.Hybrid {
			hit.Source = source.Alias
			merged.Hybrid = append(merged.Hybrid, hit)
		}
		for _, hit := range response.Lexical {
			hit.Source = source.Alias
			merged.Lexical = append(merged.Lexical, hit)
		}
	}
	topK := merged.TopK
	if topK == 0 {
		topK = 10
	}
	sort.SliceStable(merged.Keyword, func(i, j int) bool {
		if merged.Keyword[i].Score != merged.Keyword[j].Score {
			return merged.Keyword[i].Score > merged.Keyword[j].Score
		}
		if merged.Keyword[i].Source != merged.Keyword[j].Source {
			return merged.Keyword[i].Source < merged.Keyword[j].Source
		}
		return merged.Keyword[i].ID < merged.Keyword[j].ID
	})
	sort.SliceStable(merged.Fuzzy, func(i, j int) bool {
		if merged.Fuzzy[i].Score != merged.Fuzzy[j].Score {
			return merged.Fuzzy[i].Score > merged.Fuzzy[j].Score
		}
		if merged.Fuzzy[i].Source != merged.Fuzzy[j].Source {
			return merged.Fuzzy[i].Source < merged.Fuzzy[j].Source
		}
		return merged.Fuzzy[i].ID < merged.Fuzzy[j].ID
	})
	sort.SliceStable(merged.Hybrid, func(i, j int) bool {
		if merged.Hybrid[i].TotalScore != merged.Hybrid[j].TotalScore {
			return merged.Hybrid[i].TotalScore > merged.Hybrid[j].TotalScore
		}
		if merged.Hybrid[i].Source != merged.Hybrid[j].Source {
			return merged.Hybrid[i].Source < merged.Hybrid[j].Source
		}
		return merged.Hybrid[i].ID < merged.Hybrid[j].ID
	})
	sort.SliceStable(merged.Lexical, func(i, j int) bool {
		if merged.Lexical[i].Score != merged.Lexical[j].Score {
			return merged.Lexical[i].Score > merged.Lexical[j].Score
		}
		if merged.Lexical[i].Source != merged.Lexical[j].Source {
			return merged.Lexical[i].Source < merged.Lexical[j].Source
		}
		return merged.Lexical[i].ID < merged.Lexical[j].ID
	})
	merged.Keyword = limitHits(merged.Keyword, topK)
	merged.Fuzzy = limitHits(merged.Fuzzy, topK)
	merged.Hybrid = limitHits(merged.Hybrid, topK)
	merged.Lexical = limitHits(merged.Lexical, topK)
	merged.TopK = topK
	return success(merged)
}

func limitHits[T any](hits []T, topK int) []T {
	if len(hits) > topK {
		return hits[:topK]
	}
	return hits
}

func (e *Executor) multiList(
	ctx context.Context,
	request contractv1.OperationRequest,
	sources []resolvedSource,
) any {
	merged := make([]contractv1.ListResult, 0)
	for _, source := range sources {
		value, failed := unwrap(e.Execute(ctx, singleSourceRequest(request, source)))
		if failed != nil {
			return *failed
		}
		entries, ok := value.([]contractv1.ListResult)
		if !ok {
			return failure(contractv1.ErrInternal, "unexpected list payload", nil)
		}
		for _, entry := range entries {
			entry.Source = source.Alias
			merged = append(merged, entry)
		}
	}
	sort.SliceStable(merged, func(i, j int) bool {
		if merged[i].Source != merged[j].Source {
			return merged[i].Source < merged[j].Source
		}
		return merged[i].ID < merged[j].ID
	})
	limit := 20
	if request.Limit != nil {
		limit = *request.Limit
	}
	if limit > 0 && len(merged) > limit {
		merged = merged[:limit]
	}
	return success(merged)
}

func (e *Executor) multiAudit(
	ctx context.Context,
	request contractv1.OperationRequest,
	sources []resolvedSource,
) any {
	report := contractv1.AuditReport{
		Path:     request.Path,
		FailOn:   request.FailOn,
		OK:       true,
		Findings: []contractv1.AuditFinding{},
	}
	if report.FailOn == "" {
		report.FailOn = contractv1.AuditFailOnError
	}
	for _, source := range sources {
		value, failed := unwrap(e.Execute(ctx, singleSourceRequest(request, source)))
		if failed != nil {
			return *failed
		}
		part, ok := value.(contractv1.AuditReport)
		if !ok {
			return failure(contractv1.ErrInternal, "unexpected audit payload", nil)
		}
		report.Summary.Concepts += part.Summary.Concepts
		report.Summary.Edges += part.Summary.Edges
		report.Summary.Orphans += part.Summary.Orphans
		report.Summary.BrokenLinks += part.Summary.BrokenLinks
		report.Summary.MissingKeywords += part.Summary.MissingKeywords
		report.Summary.MissingIndexTargets += part.Summary.MissingIndexTargets
		report.Summary.UnindexedConcepts += part.Summary.UnindexedConcepts
		report.Summary.Valid = report.Summary.Valid && part.Summary.Valid
		report.OK = report.OK && part.OK
		for _, finding := range part.Findings {
			finding.Source = source.Alias
			report.Findings = append(report.Findings, finding)
		}
	}
	sort.SliceStable(report.Findings, func(i, j int) bool {
		if report.Findings[i].Source != report.Findings[j].Source {
			return report.Findings[i].Source < report.Findings[j].Source
		}
		if report.Findings[i].Code != report.Findings[j].Code {
			return report.Findings[i].Code < report.Findings[j].Code
		}
		return report.Findings[i].Concept < report.Findings[j].Concept
	})
	return success(report)
}

func (e *Executor) multiParse(
	ctx context.Context,
	request contractv1.OperationRequest,
	sources []resolvedSource,
) any {
	result := contractv1.SourceBundles{Sources: make([]contractv1.SourceBundle, 0, len(sources))}
	for _, source := range sources {
		value, failed := unwrap(e.Execute(ctx, singleSourceRequest(request, source)))
		if failed != nil {
			return *failed
		}
		bundle, ok := value.(*contractv1.Bundle)
		if !ok {
			return failure(contractv1.ErrInternal, "unexpected parse payload", nil)
		}
		result.Sources = append(result.Sources, contractv1.SourceBundle{Source: source.Alias, Bundle: bundle})
	}
	return success(result)
}

func (e *Executor) multiGraph(
	ctx context.Context,
	request contractv1.OperationRequest,
	sources []resolvedSource,
) any {
	result := contractv1.SourceGraphs{Sources: make([]contractv1.SourceGraph, 0, len(sources))}
	for _, source := range sources {
		value, failed := unwrap(e.Execute(ctx, singleSourceRequest(request, source)))
		if failed != nil {
			return *failed
		}
		view, ok := value.(contractv1.Graph)
		if !ok {
			return failure(contractv1.ErrInternal, "unexpected graph payload", nil)
		}
		for index := range view.Nodes {
			view.Nodes[index].Source = source.Alias
		}
		result.Sources = append(result.Sources, contractv1.SourceGraph{Source: source.Alias, Graph: view})
	}
	return success(result)
}

func splitTarget(target string, sources []resolvedSource) (resolvedSource, string, bool) {
	index := strings.Index(target, ":")
	if index <= 0 {
		return resolvedSource{}, target, false
	}
	alias := target[:index]
	for _, source := range sources {
		if source.Alias == alias {
			return source, target[index+1:], true
		}
	}
	return resolvedSource{}, target, false
}

func (e *Executor) multiTarget(
	ctx context.Context,
	request contractv1.OperationRequest,
	sources []resolvedSource,
) any {
	fromSource, fromTarget, fromQualified := splitTarget(request.From, sources)
	toSource, toTarget, toQualified := splitTarget(request.To, sources)
	if request.Op == contractv1.OpPath && fromQualified && toQualified && fromSource.Alias != toSource.Alias {
		return failure(contractv1.ErrInvalidRequest, "path endpoints must belong to the same source", map[string]any{
			"from": request.From,
			"to":   request.To,
		})
	}
	if request.Op == contractv1.OpPath {
		sub := singleSourceRequest(request, sources[0])
		if fromQualified {
			sub.From = fromTarget
		}
		if toQualified {
			sub.To = toTarget
		}
		if fromQualified || toQualified {
			source := sources[0]
			if fromQualified {
				source = fromSource
			} else {
				source = toSource
			}
			sub = singleSourceRequest(request, source)
			sub.From = fromTarget
			sub.To = toTarget
			value, failed := unwrap(e.Execute(ctx, sub))
			if failed != nil {
				return *failed
			}
			return success(annotateSource(value, source.Alias))
		}
	}

	if source, target, ok := splitTarget(request.Target, sources); ok {
		sub := singleSourceRequest(request, source)
		sub.Target = target
		value, failed := unwrap(e.Execute(ctx, sub))
		if failed != nil {
			return *failed
		}
		return success(annotateSource(value, source.Alias))
	}

	successes := make([]any, 0, 1)
	var lastFailure *contractv1.FailureEnvelope
	for _, source := range sources {
		value, failed := unwrap(e.Execute(ctx, singleSourceRequest(request, source)))
		if failed != nil {
			lastFailure = failed
			continue
		}
		successes = append(successes, annotateSource(value, source.Alias))
	}
	switch len(successes) {
	case 1:
		return success(successes[0])
	case 0:
		if request.Op == contractv1.OpPath {
			fromAlias, fromFound := e.findTargetSource(ctx, request, sources, request.From)
			toAlias, toFound := e.findTargetSource(ctx, request, sources, request.To)
			if fromFound && toFound && fromAlias != toAlias {
				return failure(contractv1.ErrInvalidRequest, "path endpoints must belong to the same source", map[string]any{
					"from": request.From,
					"to":   request.To,
				})
			}
		}
		if lastFailure != nil {
			return *lastFailure
		}
		return failure(contractv1.ErrConceptNotFound, "concept not found: "+request.Target, map[string]any{"target": request.Target})
	default:
		return failure(contractv1.ErrInvalidRequest, "target is ambiguous; qualify it with a source alias", map[string]any{
			"target": request.Target,
		})
	}
}

func (e *Executor) findTargetSource(
	ctx context.Context,
	request contractv1.OperationRequest,
	sources []resolvedSource,
	target string,
) (string, bool) {
	if source, _, ok := splitTarget(target, sources); ok {
		return source.Alias, true
	}
	found := ""
	for _, source := range sources {
		sub := singleSourceRequest(request, source)
		sub.Op = contractv1.OpShow
		sub.Target = target
		sub.From = ""
		sub.To = ""
		value, failed := unwrap(e.Execute(ctx, sub))
		if failed != nil {
			continue
		}
		if value == nil {
			continue
		}
		if found != "" {
			return "", false
		}
		found = source.Alias
	}
	return found, found != ""
}

func annotateSource(value any, alias string) any {
	switch typed := value.(type) {
	case *contractv1.ShowResult:
		typed.Source = alias
		return typed
	case *contractv1.BacklinksResult:
		return typed
	case *contractv1.PathResult:
		typed.Source = alias
		return typed
	case *contractv1.RelateResult:
		typed.Source = alias
		for index := range typed.Nodes {
			typed.Nodes[index].Source = alias
		}
		return typed
	default:
		return value
	}
}

func (e *Executor) multiExplain(
	ctx context.Context,
	request contractv1.OperationRequest,
	sources []resolvedSource,
) any {
	if source, target, ok := splitTarget(request.Target, sources); ok {
		sub := singleSourceRequest(request, source)
		sub.Target = target
		value, failed := unwrap(e.Execute(ctx, sub))
		if failed != nil {
			return *failed
		}
		if result, ok := value.(contractv1.ExplainResult); ok {
			result.Source = source.Alias
			return success(result)
		}
		return success(value)
	}

	searchRequest := singleSourceRequest(request, sources[0])
	searchRequest.Op = contractv1.OpSearch
	searchRequest.Sources = request.Sources
	searchRequest.Target = ""
	merged := e.multiSearch(ctx, searchRequest, sources)
	value, failed := unwrap(merged)
	if failed != nil {
		return *failed
	}
	response, ok := value.(contractv1.SearchResponse)
	if !ok {
		return failure(contractv1.ErrInternal, "unexpected search payload", nil)
	}
	hitID, hitSource := topHit(response)
	if hitID == "" {
		return failure(contractv1.ErrConceptNotFound, "no result for query", map[string]any{"query": request.Query})
	}
	sub := singleSourceRequest(request, sourceByAlias(sources, hitSource))
	sub.Target = hitID
	value, failed = unwrap(e.Execute(ctx, sub))
	if failed != nil {
		return *failed
	}
	if result, ok := value.(contractv1.ExplainResult); ok {
		result.Source = hitSource
		return success(result)
	}
	return success(value)
}

func topHit(response contractv1.SearchResponse) (string, string) {
	switch response.Mode {
	case contractv1.SearchKeyword:
		if len(response.Keyword) > 0 {
			return response.Keyword[0].ID, response.Keyword[0].Source
		}
	case contractv1.SearchFuzzy:
		if len(response.Fuzzy) > 0 {
			return response.Fuzzy[0].ID, response.Fuzzy[0].Source
		}
	case contractv1.SearchBM25:
		if len(response.Lexical) > 0 {
			return response.Lexical[0].ID, response.Lexical[0].Source
		}
	default:
		if len(response.Hybrid) > 0 {
			return response.Hybrid[0].ID, response.Hybrid[0].Source
		}
	}
	return "", ""
}

func sourceByAlias(sources []resolvedSource, alias string) resolvedSource {
	for _, source := range sources {
		if source.Alias == alias {
			return source
		}
	}
	return sources[0]
}

func (e *Executor) multiWrite(
	ctx context.Context,
	request contractv1.OperationRequest,
	sources []resolvedSource,
) any {
	reports := make([]contractv1.SourceReport, 0, len(sources))
	var aggregate any
	for _, source := range sources {
		value, failed := unwrap(e.Execute(ctx, singleSourceRequest(request, source)))
		if failed != nil {
			return *failed
		}
		switch report := value.(type) {
		case contractv1.IngestReport:
			report.Sources = nil
			reports = append(reports, contractv1.SourceReport{
				Source:    source.Alias,
				Documents: report.Documents,
				Warnings:  report.Warnings,
				Errors:    report.Errors,
			})
			if aggregate == nil {
				report.Bundle = request.Path
				aggregate = report
			}
		case contractv1.SyncReport:
			report.Sources = nil
			reports = append(reports, contractv1.SourceReport{
				Source:    source.Alias,
				Documents: report.Documents,
				Added:     report.Added,
				Updated:   report.Updated,
				Removed:   report.Removed,
				Skipped:   report.Skipped,
				Warnings:  report.Warnings,
				Errors:    report.Errors,
			})
			if aggregate == nil {
				report.Bundle = request.Path
				aggregate = report
			}
		default:
			return failure(contractv1.ErrInternal, "unexpected write payload", nil)
		}
	}

	switch report := aggregate.(type) {
	case contractv1.IngestReport:
		report.Documents = 0
		report.Edges = 0
		report.Synthetic = 0
		report.Warnings = 0
		report.Errors = 0
		for _, part := range reports {
			report.Documents += part.Documents
			report.Warnings += part.Warnings
			report.Errors += part.Errors
		}
		report.Sources = reports
		return success(report)
	case contractv1.SyncReport:
		report.Documents = 0
		report.Added = 0
		report.Updated = 0
		report.Removed = 0
		report.Skipped = 0
		report.Warnings = 0
		report.Errors = 0
		for _, part := range reports {
			report.Documents += part.Documents
			report.Added += part.Added
			report.Updated += part.Updated
			report.Removed += part.Removed
			report.Skipped += part.Skipped
			report.Warnings += part.Warnings
			report.Errors += part.Errors
		}
		report.Sources = reports
		return success(report)
	default:
		return failure(contractv1.ErrInternal, "unexpected write payload", nil)
	}
}
