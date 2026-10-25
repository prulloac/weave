package operations

import (
	"context"
	"errors"
	"os"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine"
	"github.com/prulloac/weave/engine/graph"
	"github.com/prulloac/weave/engine/nav"
	"github.com/prulloac/weave/engine/search"
)

const (
	ContractVersion = "weave.contract/v1"
	CLIVersion      = "weave.cli/v1"
)

var operationDescriptors = []contractv1.OperationDescriptor{
	{Op: contractv1.OpContract, Fields: []string{}, Result: "ContractInfo"},
	{Op: contractv1.OpParse, Fields: []string{"path"}, Result: "Bundle"},
	{Op: contractv1.OpGraph, Fields: []string{"path"}, Result: "Graph"},
	{Op: contractv1.OpFind, Fields: []string{"path", "query", "limit", "fields"}, Result: "FindResults"},
	{Op: contractv1.OpList, Fields: []string{"path", "types", "tags", "status", "limit"}, Result: "ListResults"},
	{Op: contractv1.OpShow, Fields: []string{"path", "target"}, Result: "ShowResult"},
	{Op: contractv1.OpBacklinks, Fields: []string{"path", "target"}, Result: "BacklinksResult"},
	{Op: contractv1.OpPath, Fields: []string{"path", "from", "to"}, Result: "PathResult"},
	{Op: contractv1.OpVersion, Fields: []string{}, Result: "VersionInfo"},
}

var operationErrors = []contractv1.ErrorCode{
	contractv1.ErrInvalidRequest,
	contractv1.ErrInvalidRoot,
	contractv1.ErrPermissionDenied,
	contractv1.ErrParseError,
	contractv1.ErrConceptNotFound,
	contractv1.ErrPathNotFound,
	contractv1.ErrInternal,
}

// OpenFunc loads an engine for a bundle path.
type OpenFunc func(ctx context.Context, path string) (*engine.Engine, error)

// Executor dispatches transport-neutral operations.
type Executor struct {
	Version string
	Open    OpenFunc
	Options engine.Options
}

// New creates an executor with the default engine loader.
func New(version string, options engine.Options) *Executor {
	return &Executor{
		Version: version,
		Options: options,
		Open: func(ctx context.Context, path string) (*engine.Engine, error) {
			return engine.Open(ctx, path, options)
		},
	}
}

// Execute runs one operation and returns a success or failure envelope.
func (e *Executor) Execute(ctx context.Context, request contractv1.OperationRequest) any {
	if request.Op == "" {
		return failure(contractv1.ErrInvalidRequest, "operation is required", nil)
	}

	if request.Op == contractv1.OpContract {
		return success(contractv1.ContractInfo{
			Contract:      ContractVersion,
			CLI:           CLIVersion,
			BinaryVersion: e.Version,
			Operations:    operationDescriptors,
			Errors:        operationErrors,
		})
	}

	if request.Op == contractv1.OpVersion {
		return success(contractv1.VersionInfo{
			BinaryVersion: e.Version,
			Contract:      ContractVersion,
			CLI:           CLIVersion,
		})
	}

	if request.Path == "" {
		return failure(contractv1.ErrInvalidRequest, "path is required for this operation", map[string]any{
			"op": request.Op,
		})
	}

	eng, err := e.Open(ctx, request.Path)
	if err != nil {
		code, message := mapOpenError(err)
		return failure(code, message, map[string]any{"path": request.Path})
	}
	defer eng.Close()

	switch request.Op {
	case contractv1.OpParse:
		return success(eng.Bundle())
	case contractv1.OpGraph:
		view := eng.Graph()
		return success(contractv1.Graph{
			Nodes:    graph.SortedNodes(view),
			Edges:    graph.SortedEdges(view),
			Dangling: view.Dangling,
		})
	case contractv1.OpFind:
		limit := 20
		if request.Limit != nil {
			limit = *request.Limit
		}
		return success(eng.Find(request.Query, search.Options{
			Limit:  limit,
			Fields: request.Fields,
		}))
	case contractv1.OpList:
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
	case contractv1.OpShow:
		result, ok := eng.Show(request.Target)
		if !ok {
			return failure(contractv1.ErrConceptNotFound, "concept not found: "+request.Target, map[string]any{
				"target": request.Target,
			})
		}
		return success(result)
	case contractv1.OpBacklinks:
		result, ok := eng.Backlinks(request.Target)
		if !ok {
			return failure(contractv1.ErrConceptNotFound, "concept not found: "+request.Target, map[string]any{
				"target": request.Target,
			})
		}
		return success(result)
	case contractv1.OpPath:
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
	default:
		return failure(contractv1.ErrInvalidRequest, "unknown operation: "+string(request.Op), map[string]any{
			"op": request.Op,
		})
	}
}

func success(result any) contractv1.SuccessEnvelope {
	return contractv1.SuccessEnvelope{
		Contract: ContractVersion,
		CLI:      CLIVersion,
		OK:       true,
		Result:   result,
	}
}

func failure(code contractv1.ErrorCode, message string, details map[string]any) contractv1.FailureEnvelope {
	return contractv1.FailureEnvelope{
		Contract: ContractVersion,
		CLI:      CLIVersion,
		OK:       false,
		Error: contractv1.OperationError{
			Code:    code,
			Message: message,
			Details: details,
		},
	}
}

func mapOpenError(err error) (contractv1.ErrorCode, string) {
	if errors.Is(err, os.ErrPermission) {
		return contractv1.ErrPermissionDenied, "permission denied: " + err.Error()
	}
	if errors.Is(err, os.ErrNotExist) {
		return contractv1.ErrInvalidRoot, "path does not exist: " + err.Error()
	}
	return contractv1.ErrInvalidRoot, "cannot open bundle: " + err.Error()
}
