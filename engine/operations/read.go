package operations

import (
	"context"
	"errors"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine"
	"github.com/prulloac/weave/engine/audit"
	"github.com/prulloac/weave/engine/graph"
	"github.com/prulloac/weave/engine/nav"
	"github.com/prulloac/weave/engine/privacy"
	"github.com/prulloac/weave/engine/relate"
	"github.com/prulloac/weave/engine/search"
)

func (e *Executor) readParse(_ context.Context, eng *engine.Engine, _ contractv1.OperationRequest) any {
	return success(eng.Bundle())
}

func (e *Executor) readGraph(_ context.Context, eng *engine.Engine, _ contractv1.OperationRequest) any {
	view := eng.Graph()
	return success(contractv1.Graph{
		Nodes:    graph.SortedNodes(view),
		Edges:    graph.SortedEdges(view),
		Dangling: view.Dangling,
	})
}

func (e *Executor) readSearch(ctx context.Context, eng *engine.Engine, request contractv1.OperationRequest) any {
	topK := 10
	if request.TopK != nil {
		topK = *request.TopK
	}
	mode := request.Mode
	if mode == "" {
		mode = contractv1.SearchHybrid
	}
	result, err := eng.Search(request.Query, mode, topK)
	if err != nil {
		return failure(contractv1.ErrInvalidRequest, err.Error(), map[string]any{
			"query": request.Query,
			"mode":  mode,
			"top_k": topK,
		})
	}
	if err := e.applyRerank(ctx, request, &result); err != nil {
		return failure(contractv1.ErrExtensionMissing, err.Error(), map[string]any{
			"extension": request.Rerank,
		})
	}
	return success(result)
}

func (e *Executor) readList(_ context.Context, eng *engine.Engine, request contractv1.OperationRequest) any {
	limit := 20
	if request.Limit != nil {
		limit = *request.Limit
	}
	return success(eng.List(nav.ListFilters{
		Types:  request.Types,
		Tags:   request.Tags,
		Status: request.Status,
		Limit:  limit,
	}))
}

func (e *Executor) readShow(_ context.Context, eng *engine.Engine, request contractv1.OperationRequest) any {
	result, ok, err := eng.ShowWithOptions(request.Target, engine.ShowOptions{
		Decrypt:  request.Decrypt,
		Provider: request.Provider,
	})
	if err != nil {
		switch {
		case errors.Is(err, privacy.ErrKeyUnavailable):
			return failure(contractv1.ErrPrivacyKey, err.Error(), map[string]any{"target": request.Target})
		case errors.Is(err, privacy.ErrDecrypt):
			return failure(contractv1.ErrPrivacyDecrypt, err.Error(), map[string]any{"target": request.Target})
		default:
			return failure(contractv1.ErrInternal, err.Error(), map[string]any{"target": request.Target})
		}
	}
	if !ok {
		return failure(contractv1.ErrConceptNotFound, "concept not found: "+request.Target, map[string]any{
			"target": request.Target,
		})
	}
	return success(result)
}

func (e *Executor) readBacklinks(_ context.Context, eng *engine.Engine, request contractv1.OperationRequest) any {
	result, ok := eng.Backlinks(request.Target)
	if !ok {
		return failure(contractv1.ErrConceptNotFound, "concept not found: "+request.Target, map[string]any{
			"target": request.Target,
		})
	}
	return success(result)
}

func (e *Executor) readPath(_ context.Context, eng *engine.Engine, request contractv1.OperationRequest) any {
	result, ok := eng.Path(request.From, request.To)
	if !ok {
		return failure(contractv1.ErrConceptNotFound, "concept not found", map[string]any{
			"from": request.From,
			"to":   request.To,
		})
	}
	if result.Path == nil {
		return failure(contractv1.ErrPathNotFound, "no route between concepts", map[string]any{
			"from": result.From,
			"to":   result.To,
		})
	}
	return success(result)
}

func (e *Executor) readAudit(_ context.Context, eng *engine.Engine, request contractv1.OperationRequest) any {
	failOn := request.FailOn
	if failOn == "" {
		failOn = contractv1.AuditFailOnError
	}
	provider, err := e.resolveProvider(request.Path, request.Provider)
	if request.Provider == "" && !request.VerifyKeys {
		provider, err = nil, nil
	}
	if err != nil {
		return failure(contractv1.ErrPrivacyKey, err.Error(), map[string]any{"provider": request.Provider})
	}
	if provider != nil {
		defer provider.Close()
	}
	return success(audit.Build(eng.Bundle(), eng.Graph(), audit.Options{
		FailOn:     failOn,
		VerifyKeys: request.VerifyKeys,
		Permissive: request.Permissive,
		Provider:   provider,
	}))
}

func (e *Executor) readRelate(_ context.Context, eng *engine.Engine, request contractv1.OperationRequest) any {
	id, ok := nav.NormalizeTarget(eng.Bundle(), request.Target)
	if !ok {
		return failure(contractv1.ErrConceptNotFound, "concept not found: "+request.Target, map[string]any{
			"target": request.Target,
		})
	}
	depth := 1
	if request.Depth != nil {
		depth = *request.Depth
	}
	direction := request.Direction
	if direction == "" {
		direction = contractv1.RelateBoth
	}
	result, err := relate.Build(eng.Graph(), id, direction, depth)
	if err != nil {
		switch {
		case errors.Is(err, relate.ErrInvalidDepth), errors.Is(err, relate.ErrInvalidDirection):
			return failure(contractv1.ErrInvalidRequest, err.Error(), map[string]any{
				"target":    id,
				"depth":     depth,
				"direction": direction,
			})
		case errors.Is(err, relate.ErrRootNotFound):
			return failure(contractv1.ErrConceptNotFound, "concept not found: "+request.Target, map[string]any{
				"target": request.Target,
			})
		default:
			return failure(contractv1.ErrInternal, err.Error(), map[string]any{"target": id})
		}
	}
	return success(result)
}

func (e *Executor) readExplain(_ context.Context, eng *engine.Engine, request contractv1.OperationRequest) any {
	topK := 10
	if request.TopK != nil {
		topK = *request.TopK
	}
	mode := request.Mode
	if mode == "" {
		mode = contractv1.SearchHybrid
	}
	result, err := eng.Explain(request.Query, mode, topK, request.Target)
	if err != nil {
		switch {
		case errors.Is(err, search.ErrConceptNotFound), errors.Is(err, search.ErrNoResult), errors.Is(err, search.ErrEmptyQuery):
			return failure(contractv1.ErrConceptNotFound, err.Error(), map[string]any{
				"query": request.Query,
				"id":    request.Target,
			})
		default:
			return failure(contractv1.ErrInvalidRequest, err.Error(), map[string]any{
				"query": request.Query,
				"mode":  mode,
				"top_k": topK,
			})
		}
	}
	return success(result)
}
