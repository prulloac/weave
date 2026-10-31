package operations

import (
	"context"
	"errors"
	"path/filepath"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine"
	sqlitestore "github.com/prulloac/weave/storage/sqlite"
)

const (
	ContractVersion = "weave.contract/v1"
	CLIVersion      = "weave.cli/v1"
)

var operationDescriptors = []contractv1.OperationDescriptor{
	{Op: contractv1.OpContract, Fields: []string{}, Result: "ContractInfo"},
	{Op: contractv1.OpParse, Fields: []string{"path"}, Result: "Bundle|SourceBundles"},
	{Op: contractv1.OpGraph, Fields: []string{"path"}, Result: "Graph|SourceGraphs"},
	{Op: contractv1.OpSearch, Fields: []string{"path", "query", "mode", "top_k"}, Result: "SearchResponse"},
	{Op: contractv1.OpList, Fields: []string{"path", "types", "tags", "status", "limit"}, Result: "ListResults"},
	{Op: contractv1.OpShow, Fields: []string{"path", "target"}, Result: "ShowResult"},
	{Op: contractv1.OpBacklinks, Fields: []string{"path", "target"}, Result: "BacklinksResult"},
	{Op: contractv1.OpPath, Fields: []string{"path", "from", "to"}, Result: "PathResult"},
	{Op: contractv1.OpVersion, Fields: []string{}, Result: "VersionInfo"},
	{Op: contractv1.OpAudit, Fields: []string{"path", "fail_on", "verify_keys"}, Result: "AuditReport"},
	{Op: contractv1.OpRelate, Fields: []string{"path", "target", "depth", "direction"}, Result: "RelateResult"},
	{Op: contractv1.OpExplain, Fields: []string{"path", "query", "target", "mode", "top_k"}, Result: "ExplainResult"},
	{Op: contractv1.OpIngest, Fields: []string{"path", "db", "strict", "codec"}, Result: "IngestReport"},
	{Op: contractv1.OpRebuild, Fields: []string{"path", "db", "strict", "codec"}, Result: "IngestReport"},
	{Op: contractv1.OpSync, Fields: []string{"path", "db", "strict", "permissive", "codec"}, Result: "SyncReport"},
	{Op: contractv1.OpExtensions, Fields: []string{"path"}, Result: "ExtensionList"},
	{Op: contractv1.OpExport, Fields: []string{"path", "out", "format", "key_id", "db", "force", "confirmed"}, Result: "ExportReport"},
}

var operationErrors = []contractv1.ErrorCode{
	contractv1.ErrInvalidRequest,
	contractv1.ErrInvalidRoot,
	contractv1.ErrPermissionDenied,
	contractv1.ErrParseError,
	contractv1.ErrConceptNotFound,
	contractv1.ErrPathNotFound,
	contractv1.ErrBundleNotIndexed,
	contractv1.ErrPrivacyDirective,
	contractv1.ErrPrivacyKey,
	contractv1.ErrPrivacyProvider,
	contractv1.ErrPrivacyDecrypt,
	contractv1.ErrExtensionMissing,
	contractv1.ErrExtensionTimeout,
	contractv1.ErrExtensionFailed,
	contractv1.ErrExtensionProto,
	contractv1.ErrConfirmation,
	contractv1.ErrInternal,
}

// OpenFunc loads an engine for a bundle path.
type OpenFunc func(ctx context.Context, path string) (*engine.Engine, error)

// Executor dispatches transport-neutral operations.
type Executor struct {
	Version string
	Open    OpenFunc
	Options engine.Options
	Warn    func(message string)
}

// New creates an executor with the default engine loader.
func New(version string, options engine.Options) *Executor {
	return &Executor{
		Version: version,
		Open: func(ctx context.Context, path string) (*engine.Engine, error) {
			return engine.Open(ctx, path, options)
		},
		Options: options,
	}
}

type operationHandler func(*Executor, context.Context, contractv1.OperationRequest) any
type engineHandler func(*Executor, context.Context, *engine.Engine, contractv1.OperationRequest) any

var preflightHandlers = map[contractv1.OperationName]operationHandler{
	contractv1.OpContract:   (*Executor).contractInfo,
	contractv1.OpVersion:    (*Executor).versionInfo,
	contractv1.OpExtensions: (*Executor).extensionList,
	contractv1.OpExport:     (*Executor).export,
}

var requestHandlers = map[contractv1.OperationName]operationHandler{
	contractv1.OpIngest:  (*Executor).ingest,
	contractv1.OpRebuild: (*Executor).ingest,
	contractv1.OpSync:    (*Executor).sync,
}

var engineHandlers = map[contractv1.OperationName]engineHandler{
	contractv1.OpParse:     (*Executor).readParse,
	contractv1.OpGraph:     (*Executor).readGraph,
	contractv1.OpSearch:    (*Executor).readSearch,
	contractv1.OpList:      (*Executor).readList,
	contractv1.OpShow:      (*Executor).readShow,
	contractv1.OpBacklinks: (*Executor).readBacklinks,
	contractv1.OpPath:      (*Executor).readPath,
	contractv1.OpAudit:     (*Executor).readAudit,
	contractv1.OpRelate:    (*Executor).readRelate,
	contractv1.OpExplain:   (*Executor).readExplain,
}

// Execute runs one operation and returns a success or failure envelope.
func (e *Executor) Execute(ctx context.Context, request contractv1.OperationRequest) any {
	if request.Op == "" {
		return failure(contractv1.ErrInvalidRequest, "operation is required", nil)
	}

	if handler, ok := preflightHandlers[request.Op]; ok {
		return handler(e, ctx, request)
	}

	if len(request.Sources) > 1 {
		return e.executeMulti(ctx, request)
	}
	if len(request.Sources) == 1 {
		request.Path = request.Sources[0].Path
	}
	if request.Path == "" {
		return failure(contractv1.ErrInvalidRequest, "path is required for this operation", map[string]any{
			"op": request.Op,
		})
	}

	if handler, ok := requestHandlers[request.Op]; ok {
		return handler(e, ctx, request)
	}

	if request.Op == contractv1.OpSearch && request.Mode == contractv1.SearchBM25 {
		return e.searchBM25(ctx, request)
	}
	if request.Op == contractv1.OpSearch && request.Mode == contractv1.SearchHybrid && !request.NoDB {
		return e.searchHybridBM25(ctx, request)
	}
	if request.Op == contractv1.OpExplain {
		mode := request.Mode
		if mode == "" {
			mode = contractv1.SearchHybrid
		}
		if mode == contractv1.SearchBM25 {
			return e.explainBM25(ctx, request)
		}
		if mode == contractv1.SearchHybrid && !request.NoDB {
			return e.explainHybridBM25(ctx, request)
		}
	}

	eng, cleanup, err := e.openEngine(ctx, request)
	if err != nil {
		return openFailure(request, err)
	}
	defer cleanup()

	if handler, ok := engineHandlers[request.Op]; ok {
		return handler(e, ctx, eng, request)
	}
	return failure(contractv1.ErrInvalidRequest, "unknown operation: "+string(request.Op), map[string]any{
		"op": request.Op,
	})
}

func (e *Executor) contractInfo(_ context.Context, _ contractv1.OperationRequest) any {
	return success(contractv1.ContractInfo{
		Contract:      ContractVersion,
		CLI:           CLIVersion,
		BinaryVersion: e.Version,
		Operations:    operationDescriptors,
		Errors:        operationErrors,
	})
}

func (e *Executor) versionInfo(_ context.Context, _ contractv1.OperationRequest) any {
	return success(contractv1.VersionInfo{
		BinaryVersion: e.Version,
		Contract:      ContractVersion,
		CLI:           CLIVersion,
	})
}

func (e *Executor) extensionList(_ context.Context, request contractv1.OperationRequest) any {
	return success(e.listExtensions(request.Path))
}

func openFailure(request contractv1.OperationRequest, err error) any {
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

func (e *Executor) openEngine(
	ctx context.Context,
	request contractv1.OperationRequest,
) (*engine.Engine, func(), error) {
	if request.NoDB {
		options := e.Options
		if request.Op == contractv1.OpAudit {
			options.Permissive = request.Permissive
		}
		eng, err := engine.Open(ctx, request.Path, options)
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
