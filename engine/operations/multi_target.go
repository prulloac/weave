package operations

import (
	"context"
	"strings"

	contractv1 "github.com/prulloac/weave/contract/v1"
)

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
			source := fromSource
			if !fromQualified {
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
