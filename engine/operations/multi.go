package operations

import (
	"context"
	"fmt"
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
