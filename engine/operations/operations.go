package operations

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine"
	"github.com/prulloac/weave/engine/audit"
	"github.com/prulloac/weave/engine/graph"
	"github.com/prulloac/weave/engine/nav"
	"github.com/prulloac/weave/engine/relate"
	"github.com/prulloac/weave/engine/search"
	sqlitestore "github.com/prulloac/weave/storage/sqlite"
)

const (
	ContractVersion = "weave.contract/v1"
	CLIVersion      = "weave.cli/v1"
)

var operationDescriptors = []contractv1.OperationDescriptor{
	{Op: contractv1.OpContract, Fields: []string{}, Result: "ContractInfo"},
	{Op: contractv1.OpParse, Fields: []string{"path"}, Result: "Bundle"},
	{Op: contractv1.OpGraph, Fields: []string{"path"}, Result: "Graph"},
	{Op: contractv1.OpSearch, Fields: []string{"path", "query", "mode", "top_k"}, Result: "SearchResponse"},
	{Op: contractv1.OpList, Fields: []string{"path", "types", "tags", "status", "limit"}, Result: "ListResults"},
	{Op: contractv1.OpShow, Fields: []string{"path", "target"}, Result: "ShowResult"},
	{Op: contractv1.OpBacklinks, Fields: []string{"path", "target"}, Result: "BacklinksResult"},
	{Op: contractv1.OpPath, Fields: []string{"path", "from", "to"}, Result: "PathResult"},
	{Op: contractv1.OpVersion, Fields: []string{}, Result: "VersionInfo"},
	{Op: contractv1.OpAudit, Fields: []string{"path", "fail_on", "verify_keys"}, Result: "AuditReport"},
	{Op: contractv1.OpRelate, Fields: []string{"path", "target", "depth", "direction"}, Result: "RelateResult"},
	{Op: contractv1.OpExplain, Fields: []string{"path", "query", "target", "mode", "top_k"}, Result: "ExplainResult"},
	{Op: contractv1.OpIngest, Fields: []string{"path", "db", "strict"}, Result: "IngestReport"},
	{Op: contractv1.OpRebuild, Fields: []string{"path", "db", "strict"}, Result: "IngestReport"},
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

	if request.Op == contractv1.OpIngest || request.Op == contractv1.OpRebuild {
		return e.ingest(ctx, request)
	}

	eng, cleanup, err := e.openEngine(ctx, request)
	if err != nil {
		switch {
		case errors.Is(err, sqlitestore.ErrBundleNotIndexed):
			return failure(contractv1.ErrBundleNotIndexed, "bundle is not indexed; run: weave ingest "+request.Path, map[string]any{
				"path": request.Path,
			})
		case request.NoDB:
			code, message := mapOpenError(err)
			return failure(code, message, map[string]any{"path": request.Path})
		default:
			code, message := mapStorageError(err)
			return failure(code, message, map[string]any{"path": request.Path})
		}
	}
	defer cleanup()

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
	case contractv1.OpSearch:
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
		return success(result)
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
	case contractv1.OpAudit:
		failOn := request.FailOn
		if failOn == "" {
			failOn = contractv1.AuditFailOnError
		}
		return success(audit.Build(eng.Bundle(), eng.Graph(), audit.Options{
			FailOn:     failOn,
			VerifyKeys: request.VerifyKeys,
		}))
	case contractv1.OpRelate:
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
	case contractv1.OpExplain:
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
	default:
		return failure(contractv1.ErrInvalidRequest, "unknown operation: "+string(request.Op), map[string]any{
			"op": request.Op,
		})
	}
}

func (e *Executor) openEngine(
	ctx context.Context,
	request contractv1.OperationRequest,
) (*engine.Engine, func(), error) {
	if request.NoDB {
		eng, err := e.Open(ctx, request.Path)
		if err != nil {
			return nil, func() {}, err
		}
		return eng, func() { _ = eng.Close() }, nil
	}

	dbPath, err := resolveDBPath(request.DB)
	if err != nil {
		return nil, func() {}, err
	}
	store, err := sqlitestore.OpenExisting(dbPath)
	if err != nil {
		return nil, func() {}, err
	}
	closeStore := func() { _ = store.Close() }

	root, err := filepath.Abs(request.Path)
	if err != nil {
		closeStore()
		return nil, func() {}, err
	}
	has, err := store.HasBundle(ctx, root)
	if err != nil {
		closeStore()
		return nil, func() {}, err
	}
	if !has {
		closeStore()
		return nil, func() {}, sqlitestore.ErrBundleNotIndexed
	}
	bundle, err := store.LoadBundle(ctx, root)
	if err != nil {
		closeStore()
		return nil, func() {}, err
	}
	return engine.FromBundle(bundle), closeStore, nil
}

func (e *Executor) ingest(ctx context.Context, request contractv1.OperationRequest) any {
	if request.NoDB {
		return failure(contractv1.ErrInvalidRequest, "--no-db cannot be used with ingest or rebuild", map[string]any{
			"op": request.Op,
		})
	}

	eng, err := e.Open(ctx, request.Path)
	if err != nil {
		code, message := mapOpenError(err)
		return failure(code, message, map[string]any{"path": request.Path})
	}
	defer eng.Close()

	dbPath, err := resolveDBPath(request.DB)
	if err != nil {
		return failure(contractv1.ErrInvalidRequest, err.Error(), nil)
	}
	store, err := sqlitestore.Open(dbPath)
	if err != nil {
		code, message := mapStorageError(err)
		return failure(code, message, map[string]any{"db": dbPath})
	}
	defer store.Close()

	var report contractv1.IngestReport
	if request.Op == contractv1.OpRebuild {
		report, err = store.Rebuild(ctx, eng.Bundle())
	} else {
		report, err = store.Ingest(ctx, eng.Bundle())
	}
	if err != nil {
		return failure(contractv1.ErrInvalidRequest, err.Error(), map[string]any{
			"path": request.Path,
			"db":   dbPath,
		})
	}
	return success(report)
}

func resolveDBPath(path string) (string, error) {
	if path == "" {
		return sqlitestore.DefaultPath()
	}
	return sqlitestore.ExpandPath(path)
}

func mapStorageError(err error) (contractv1.ErrorCode, string) {
	if errors.Is(err, os.ErrPermission) {
		return contractv1.ErrPermissionDenied, "permission denied: " + err.Error()
	}
	if errors.Is(err, os.ErrNotExist) {
		return contractv1.ErrInvalidRequest, "database does not exist: " + err.Error()
	}
	return contractv1.ErrInvalidRequest, "cannot open database: " + err.Error()
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
