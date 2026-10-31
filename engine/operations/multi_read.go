package operations

import (
	"context"
	"sort"

	contractv1 "github.com/prulloac/weave/contract/v1"
)

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
