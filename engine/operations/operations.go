package operations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine"
	"github.com/prulloac/weave/engine/audit"
	"github.com/prulloac/weave/engine/compression"
	"github.com/prulloac/weave/engine/extensions"
	"github.com/prulloac/weave/engine/graph"
	"github.com/prulloac/weave/engine/nav"
	"github.com/prulloac/weave/engine/privacy"
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

	if request.Op == contractv1.OpExtensions {
		return success(e.listExtensions(request.Path))
	}

	if request.Op == contractv1.OpExport {
		return e.export(ctx, request)
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

	if request.Op == contractv1.OpIngest || request.Op == contractv1.OpRebuild {
		return e.ingest(ctx, request)
	}
	if request.Op == contractv1.OpSync {
		return e.sync(ctx, request)
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
		if err := e.applyRerank(ctx, request, &result); err != nil {
			return failure(contractv1.ErrExtensionMissing, err.Error(), map[string]any{
				"extension": request.Rerank,
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
		result, ok, err := eng.ShowWithOptions(request.Target, engine.ShowOptions{Decrypt: request.Decrypt})
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
			Permissive: request.Permissive,
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

func (e *Executor) ingest(ctx context.Context, request contractv1.OperationRequest) any {
	if request.NoDB {
		return failure(contractv1.ErrInvalidRequest, "--no-db cannot be used with ingest or rebuild", map[string]any{
			"op": request.Op,
		})
	}
	if request.Strict && request.Permissive {
		return failure(contractv1.ErrInvalidRequest, "--strict and --permissive are mutually exclusive", map[string]any{
			"op": request.Op,
		})
	}

	permissive := !request.Strict
	mode := "strict"
	if permissive {
		mode = "permissive"
	}
	codec := request.Codec
	if codec != "" && !compression.Valid(codec) {
		return failure(contractv1.ErrInvalidRequest, "codec must be zstd, lz4 or none", map[string]any{
			"op": request.Op,
		})
	}
	enricher, err := e.newEnricher(request)
	if err != nil {
		return failure(contractv1.ErrExtensionMissing, err.Error(), map[string]any{
			"extension": request.Enricher,
		})
	}
	options := e.Options
	options.Permissive = permissive
	eng, err := engine.Open(ctx, request.Path, options)
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
		report, err = store.Rebuild(ctx, eng.Bundle(), sqlitestore.IngestOptions{Mode: mode, Codec: codec, Enricher: enricher})
	} else {
		report, err = store.Ingest(ctx, eng.Bundle(), sqlitestore.IngestOptions{Mode: mode, Codec: codec, Enricher: enricher})
	}
	if err != nil {
		if errors.Is(err, privacy.ErrKeyUnavailable) {
			return failure(contractv1.ErrPrivacyKey, err.Error(), map[string]any{
				"path": request.Path,
				"db":   dbPath,
			})
		}
		return failure(contractv1.ErrInvalidRequest, err.Error(), map[string]any{
			"path": request.Path,
			"db":   dbPath,
		})
	}
	return success(report)
}

func (e *Executor) sync(ctx context.Context, request contractv1.OperationRequest) any {
	if request.NoDB {
		return failure(contractv1.ErrInvalidRequest, "--no-db cannot be used with sync", map[string]any{
			"op": request.Op,
		})
	}
	if request.Strict && request.Permissive {
		return failure(contractv1.ErrInvalidRequest, "--strict and --permissive are mutually exclusive", map[string]any{
			"op": request.Op,
		})
	}
	dbPath, err := resolveDBPath(request.DB)
	if err != nil {
		return failure(contractv1.ErrInvalidRequest, err.Error(), nil)
	}
	store, err := sqlitestore.OpenExisting(dbPath)
	if err != nil {
		code, message := mapStorageError(err)
		return failure(code, message, map[string]any{"db": dbPath})
	}
	defer store.Close()

	mode := ""
	if request.Strict {
		mode = "strict"
	} else if request.Permissive {
		mode = "permissive"
	}
	if request.Codec != "" && !compression.Valid(request.Codec) {
		return failure(contractv1.ErrInvalidRequest, "codec must be zstd, lz4 or none", map[string]any{
			"op": request.Op,
		})
	}
	enricher, err := e.newEnricher(request)
	if err != nil {
		return failure(contractv1.ErrExtensionMissing, err.Error(), map[string]any{
			"extension": request.Enricher,
		})
	}
	report, err := store.Sync(ctx, request.Path, sqlitestore.SyncOptions{Mode: mode, Codec: request.Codec, Enricher: enricher})
	if err != nil {
		if errors.Is(err, sqlitestore.ErrBundleNotIndexed) {
			return failure(contractv1.ErrBundleNotIndexed, "bundle is not indexed; run: weave ingest "+request.Path, map[string]any{
				"path": request.Path,
			})
		}
		if errors.Is(err, privacy.ErrKeyUnavailable) {
			return failure(contractv1.ErrPrivacyKey, err.Error(), map[string]any{
				"path": request.Path,
				"db":   dbPath,
			})
		}
		code, message := mapStorageError(err)
		return failure(code, message, map[string]any{
			"path": request.Path,
			"db":   dbPath,
		})
	}
	return success(report)
}

func (e *Executor) searchBM25(ctx context.Context, request contractv1.OperationRequest) any {
	if request.NoDB {
		return failure(contractv1.ErrInvalidRequest, "bm25 requires persistent storage; remove --no-db", map[string]any{
			"mode": request.Mode,
		})
	}
	topK := 10
	if request.TopK != nil {
		topK = *request.TopK
	}
	dbPath, err := resolveDBPath(request.DB)
	if err != nil {
		return failure(contractv1.ErrInvalidRequest, err.Error(), nil)
	}
	store, err := sqlitestore.OpenExisting(dbPath)
	if err != nil {
		code, message := mapStorageError(err)
		return failure(code, message, map[string]any{"db": dbPath})
	}
	defer store.Close()

	root, err := filepath.Abs(request.Path)
	if err != nil {
		return failure(contractv1.ErrInvalidRequest, err.Error(), map[string]any{"path": request.Path})
	}
	has, err := store.HasBundle(ctx, root)
	if err != nil {
		code, message := mapStorageError(err)
		return failure(code, message, map[string]any{"path": request.Path})
	}
	if !has {
		return failure(contractv1.ErrBundleNotIndexed, "bundle is not indexed; run: weave ingest "+request.Path, map[string]any{
			"path": request.Path,
		})
	}
	hits, err := store.SearchBM25(ctx, root, request.Query, topK)
	if err != nil {
		if errors.Is(err, search.ErrInvalidTopK) {
			return failure(contractv1.ErrInvalidRequest, err.Error(), map[string]any{"top_k": topK})
		}
		return failure(contractv1.ErrInvalidRequest, err.Error(), map[string]any{"path": request.Path})
	}
	response := contractv1.SearchResponse{
		Query:   request.Query,
		Mode:    contractv1.SearchBM25,
		TopK:    topK,
		Keyword: []contractv1.KeywordHit{},
		Fuzzy:   []contractv1.FuzzyHit{},
		Hybrid:  []contractv1.HybridHit{},
		Lexical: hits,
	}
	if err := e.applyRerank(ctx, request, &response); err != nil {
		return failure(contractv1.ErrExtensionMissing, err.Error(), map[string]any{
			"extension": request.Rerank,
		})
	}
	return success(response)
}

func (e *Executor) searchHybridBM25(ctx context.Context, request contractv1.OperationRequest) any {
	topK := 10
	if request.TopK != nil {
		topK = *request.TopK
	}
	dbPath, err := resolveDBPath(request.DB)
	if err != nil {
		return failure(contractv1.ErrInvalidRequest, err.Error(), nil)
	}
	store, err := sqlitestore.OpenExisting(dbPath)
	if err != nil {
		code, message := mapStorageError(err)
		return failure(code, message, map[string]any{"db": dbPath})
	}
	defer store.Close()

	root, err := filepath.Abs(request.Path)
	if err != nil {
		return failure(contractv1.ErrInvalidRequest, err.Error(), map[string]any{"path": request.Path})
	}
	has, err := store.HasBundle(ctx, root)
	if err != nil {
		code, message := mapStorageError(err)
		return failure(code, message, map[string]any{"path": request.Path})
	}
	if !has {
		return failure(contractv1.ErrBundleNotIndexed, "bundle is not indexed; run: weave ingest "+request.Path, map[string]any{
			"path": request.Path,
		})
	}
	bundle, err := store.LoadBundle(ctx, root)
	if err != nil {
		code, message := mapStorageError(err)
		return failure(code, message, map[string]any{"path": request.Path})
	}
	eng := engine.FromBundle(bundle)
	defer eng.Close()

	fuzzyResponse, err := eng.Search(request.Query, contractv1.SearchFuzzy, topK)
	if err != nil {
		return failure(contractv1.ErrInvalidRequest, err.Error(), map[string]any{
			"query": request.Query,
			"top_k": topK,
		})
	}
	lexical, err := store.SearchBM25(ctx, root, request.Query, topK)
	if err != nil {
		if errors.Is(err, search.ErrInvalidTopK) {
			return failure(contractv1.ErrInvalidRequest, err.Error(), map[string]any{"top_k": topK})
		}
		return failure(contractv1.ErrInvalidRequest, err.Error(), map[string]any{"path": request.Path})
	}
	hybrid := search.HybridBM25(lexical, fuzzyResponse.Fuzzy, topK)
	response := contractv1.SearchResponse{
		Query:   request.Query,
		Mode:    contractv1.SearchHybrid,
		TopK:    topK,
		Keyword: []contractv1.KeywordHit{},
		Fuzzy:   fuzzyResponse.Fuzzy,
		Hybrid:  hybrid,
		Lexical: lexical,
	}
	if err := e.applyRerank(ctx, request, &response); err != nil {
		return failure(contractv1.ErrExtensionMissing, err.Error(), map[string]any{
			"extension": request.Rerank,
		})
	}
	return success(response)
}

func (e *Executor) explainBM25(ctx context.Context, request contractv1.OperationRequest) any {
	if request.NoDB {
		return failure(contractv1.ErrInvalidRequest, "bm25 explain requires persistent storage; remove --no-db", map[string]any{
			"mode": request.Mode,
		})
	}
	topK := 10
	if request.TopK != nil {
		topK = *request.TopK
	}
	dbPath, err := resolveDBPath(request.DB)
	if err != nil {
		return failure(contractv1.ErrInvalidRequest, err.Error(), nil)
	}
	store, err := sqlitestore.OpenExisting(dbPath)
	if err != nil {
		code, message := mapStorageError(err)
		return failure(code, message, map[string]any{"db": dbPath})
	}
	defer store.Close()

	root, err := filepath.Abs(request.Path)
	if err != nil {
		return failure(contractv1.ErrInvalidRequest, err.Error(), map[string]any{"path": request.Path})
	}
	has, err := store.HasBundle(ctx, root)
	if err != nil {
		code, message := mapStorageError(err)
		return failure(code, message, map[string]any{"path": request.Path})
	}
	if !has {
		return failure(contractv1.ErrBundleNotIndexed, "bundle is not indexed; run: weave ingest "+request.Path, map[string]any{
			"path": request.Path,
		})
	}
	explanation, err := store.ExplainBM25(ctx, root, request.Query, topK, request.Target)
	if err != nil {
		if errors.Is(err, search.ErrConceptNotFound) || errors.Is(err, search.ErrNoResult) || errors.Is(err, search.ErrEmptyQuery) {
			return failure(contractv1.ErrConceptNotFound, err.Error(), map[string]any{
				"query": request.Query,
				"id":    request.Target,
			})
		}
		return failure(contractv1.ErrInvalidRequest, err.Error(), map[string]any{
			"query": request.Query,
			"top_k": topK,
		})
	}
	lexical := explanation.Lexical
	return success(contractv1.ExplainResult{
		Query: request.Query,
		Mode:  contractv1.SearchBM25,
		TopK:  topK,
		ID:    explanation.Target,
		Rank:  explanation.Rank,
		Found: true,
		Keyword: contractv1.ExplainKeyword{
			MatchedKeywords: []string{},
			Tokens:          []contractv1.ExplainKeywordToken{},
		},
		Fuzzy: contractv1.ExplainFuzzy{
			MatchedFields: []contractv1.SearchField{},
			Fields:        []contractv1.ExplainFuzzyField{},
			Tokens:        []contractv1.ExplainFuzzyToken{},
		},
		Hybrid:  contractv1.ExplainHybrid{},
		Lexical: &lexical,
	})
}

func (e *Executor) explainHybridBM25(ctx context.Context, request contractv1.OperationRequest) any {
	topK := 10
	if request.TopK != nil {
		topK = *request.TopK
	}
	dbPath, err := resolveDBPath(request.DB)
	if err != nil {
		return failure(contractv1.ErrInvalidRequest, err.Error(), nil)
	}
	store, err := sqlitestore.OpenExisting(dbPath)
	if err != nil {
		code, message := mapStorageError(err)
		return failure(code, message, map[string]any{"db": dbPath})
	}
	defer store.Close()

	root, err := filepath.Abs(request.Path)
	if err != nil {
		return failure(contractv1.ErrInvalidRequest, err.Error(), map[string]any{"path": request.Path})
	}
	has, err := store.HasBundle(ctx, root)
	if err != nil {
		code, message := mapStorageError(err)
		return failure(code, message, map[string]any{"path": request.Path})
	}
	if !has {
		return failure(contractv1.ErrBundleNotIndexed, "bundle is not indexed; run: weave ingest "+request.Path, map[string]any{
			"path": request.Path,
		})
	}
	bundle, err := store.LoadBundle(ctx, root)
	if err != nil {
		code, message := mapStorageError(err)
		return failure(code, message, map[string]any{"path": request.Path})
	}
	eng := engine.FromBundle(bundle)
	defer eng.Close()

	lexical, err := store.SearchBM25(ctx, root, request.Query, topK)
	if err != nil {
		return failure(contractv1.ErrInvalidRequest, err.Error(), map[string]any{
			"query": request.Query,
			"top_k": topK,
		})
	}
	fuzzyResponse, err := eng.Search(request.Query, contractv1.SearchFuzzy, topK)
	if err != nil {
		return failure(contractv1.ErrInvalidRequest, err.Error(), map[string]any{
			"query": request.Query,
			"top_k": topK,
		})
	}
	hybrid := search.HybridBM25(lexical, fuzzyResponse.Fuzzy, topK)

	target := request.Target
	rank := 0
	if target == "" {
		if len(hybrid) == 0 {
			return failure(contractv1.ErrConceptNotFound, search.ErrNoResult.Error(), map[string]any{
				"query": request.Query,
			})
		}
		target = hybrid[0].ID
		rank = 1
	} else {
		for index, hit := range hybrid {
			if hit.ID == target {
				rank = index + 1
				break
			}
		}
	}
	lexicalExplanation, err := store.ExplainBM25(ctx, root, request.Query, topK, target)
	if err != nil {
		if errors.Is(err, search.ErrConceptNotFound) || errors.Is(err, search.ErrNoResult) || errors.Is(err, search.ErrEmptyQuery) {
			return failure(contractv1.ErrConceptNotFound, err.Error(), map[string]any{
				"query": request.Query,
				"id":    target,
			})
		}
		return failure(contractv1.ErrInvalidRequest, err.Error(), map[string]any{"query": request.Query})
	}
	fuzzyExplanation, err := eng.Explain(request.Query, contractv1.SearchFuzzy, topK, target)
	if err != nil {
		if errors.Is(err, search.ErrConceptNotFound) || errors.Is(err, search.ErrNoResult) || errors.Is(err, search.ErrEmptyQuery) {
			return failure(contractv1.ErrConceptNotFound, err.Error(), map[string]any{
				"query": request.Query,
				"id":    target,
			})
		}
		return failure(contractv1.ErrInvalidRequest, err.Error(), map[string]any{"query": request.Query})
	}
	lexicalComponent := lexicalExplanation.Lexical
	total := search.RoundScore(keywordWeightForExplain*lexicalComponent.Score + fuzzyWeightForExplain*fuzzyExplanation.Fuzzy.Score)

	return success(contractv1.ExplainResult{
		Query: request.Query,
		Mode:  contractv1.SearchHybrid,
		TopK:  topK,
		ID:    target,
		Rank:  rank,
		Found: true,
		Keyword: contractv1.ExplainKeyword{
			MatchedKeywords: []string{},
			Tokens:          []contractv1.ExplainKeywordToken{},
		},
		Fuzzy: fuzzyExplanation.Fuzzy,
		Hybrid: contractv1.ExplainHybrid{
			KeywordWeight: 0,
			LexicalWeight: keywordWeightForExplain,
			FuzzyWeight:   fuzzyWeightForExplain,
			Total:         total,
		},
		Lexical: &lexicalComponent,
	})
}

const (
	keywordWeightForExplain = 0.50
	fuzzyWeightForExplain   = 0.50
)

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

func (e *Executor) warn(format string, args ...any) {
	if e.Warn == nil {
		return
	}
	e.Warn(fmt.Sprintf(format, args...))
}

func (e *Executor) listExtensions(root string) contractv1.ExtensionList {
	discovered := extensions.Discover(root)
	result := contractv1.ExtensionList{Extensions: make([]contractv1.ExtensionInfo, 0, len(discovered))}
	for _, extension := range discovered {
		result.Extensions = append(result.Extensions, contractv1.ExtensionInfo{
			Name:   extension.Name,
			Path:   extension.Path,
			Source: extension.Source,
		})
	}
	return result
}

func extensionTimeout(request contractv1.OperationRequest) time.Duration {
	if request.TimeoutMS > 0 {
		return time.Duration(request.TimeoutMS) * time.Millisecond
	}
	return extensions.DefaultTimeout
}

func (e *Executor) newEnricher(request contractv1.OperationRequest) (sqlitestore.Enricher, error) {
	if request.Enricher == "" {
		return nil, nil
	}
	extension, err := extensions.Resolve(request.Path, request.Enricher)
	if err != nil {
		return nil, err
	}
	runner := extensions.Runner{Timeout: extensionTimeout(request)}
	return func(ctx context.Context, concept contractv1.OkfConcept) (sqlitestore.EnrichmentApply, error) {
		title := concept.ID
		if concept.Title != nil {
			title = *concept.Title
		}
		response, err := runner.Enrich(ctx, extension, extensions.EnrichmentRequest{
			DocID:       concept.ID,
			Title:       title,
			Frontmatter: concept.Frontmatter,
			Body:        privacy.RedactForAI(concept),
		})
		if err != nil {
			e.warn("extension %s: %v", extension.Name, err)
			return sqlitestore.EnrichmentApply{}, err
		}
		data, err := json.Marshal(map[string]any{
			"name":              extension.Name,
			"inferred_keywords": response.InferredKeywords,
			"response":          response.CustomMetadata,
		})
		if err != nil {
			return sqlitestore.EnrichmentApply{}, err
		}
		return sqlitestore.EnrichmentApply{
			Enricher: extension.Name,
			Keywords: mergeKeywords(concept.Keywords, response.InferredKeywords),
			Data:     data,
		}, nil
	}, nil
}

func mergeKeywords(declared, inferred []string) []string {
	merged := make([]string, 0, len(declared)+len(inferred))
	seen := make(map[string]bool, len(declared)+len(inferred))
	for _, keyword := range append(append([]string{}, declared...), inferred...) {
		trimmed := strings.TrimSpace(keyword)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if seen[key] {
			continue
		}
		seen[key] = true
		merged = append(merged, trimmed)
	}
	return merged
}

func (e *Executor) applyRerank(
	ctx context.Context,
	request contractv1.OperationRequest,
	response *contractv1.SearchResponse,
) error {
	if request.Rerank == "" {
		return nil
	}
	extension, err := extensions.Resolve(request.Path, request.Rerank)
	if err != nil {
		return err
	}
	mode := request.Mode
	if mode == "" {
		mode = contractv1.SearchHybrid
	}
	candidates := rerankCandidates(*response, mode)
	if len(candidates) == 0 {
		return nil
	}
	runner := extensions.Runner{Timeout: extensionTimeout(request)}
	scores, err := runner.Rerank(ctx, extension, extensions.RerankRequest{
		Query:      response.Query,
		Candidates: candidates,
	})
	if err != nil {
		e.warn("extension %s: %v", extension.Name, err)
		return nil
	}
	blendRerank(response, mode, scores)
	return nil
}

func rerankCandidates(response contractv1.SearchResponse, mode contractv1.SearchMode) []extensions.Candidate {
	switch mode {
	case contractv1.SearchKeyword:
		candidates := make([]extensions.Candidate, 0, len(response.Keyword))
		for _, hit := range response.Keyword {
			candidates = append(candidates, extensions.Candidate{ID: hit.ID, KeywordScore: hit.Score})
		}
		return candidates
	case contractv1.SearchFuzzy:
		candidates := make([]extensions.Candidate, 0, len(response.Fuzzy))
		for _, hit := range response.Fuzzy {
			candidates = append(candidates, extensions.Candidate{ID: hit.ID, FuzzyScore: hit.Score})
		}
		return candidates
	case contractv1.SearchBM25:
		candidates := make([]extensions.Candidate, 0, len(response.Lexical))
		for _, hit := range response.Lexical {
			candidates = append(candidates, extensions.Candidate{ID: hit.ID, LexicalScore: hit.Score})
		}
		return candidates
	default:
		candidates := make([]extensions.Candidate, 0, len(response.Hybrid))
		for _, hit := range response.Hybrid {
			candidates = append(candidates, extensions.Candidate{
				ID:           hit.ID,
				KeywordScore: hit.KeywordScore,
				LexicalScore: hit.LexicalScore,
				FuzzyScore:   hit.FuzzyScore,
			})
		}
		return candidates
	}
}

func blendRerank(
	response *contractv1.SearchResponse,
	mode contractv1.SearchMode,
	scores map[string]float64,
) {
	blend := func(core, rerank float64) float64 {
		return search.RoundScore(0.5*core + 0.5*rerank)
	}
	switch mode {
	case contractv1.SearchKeyword:
		for index := range response.Keyword {
			hit := &response.Keyword[index]
			score := scores[hit.ID]
			hit.Score = blend(hit.Score, score)
			hit.RerankScore = &score
		}
		sort.SliceStable(response.Keyword, func(i, j int) bool {
			if response.Keyword[i].Score != response.Keyword[j].Score {
				return response.Keyword[i].Score > response.Keyword[j].Score
			}
			return response.Keyword[i].ID < response.Keyword[j].ID
		})
	case contractv1.SearchFuzzy:
		for index := range response.Fuzzy {
			hit := &response.Fuzzy[index]
			score := scores[hit.ID]
			hit.Score = blend(hit.Score, score)
			hit.RerankScore = &score
		}
		sort.SliceStable(response.Fuzzy, func(i, j int) bool {
			if response.Fuzzy[i].Score != response.Fuzzy[j].Score {
				return response.Fuzzy[i].Score > response.Fuzzy[j].Score
			}
			return response.Fuzzy[i].ID < response.Fuzzy[j].ID
		})
	case contractv1.SearchBM25:
		for index := range response.Lexical {
			hit := &response.Lexical[index]
			score := scores[hit.ID]
			hit.Score = blend(hit.Score, score)
			hit.RerankScore = &score
		}
		sort.SliceStable(response.Lexical, func(i, j int) bool {
			if response.Lexical[i].Score != response.Lexical[j].Score {
				return response.Lexical[i].Score > response.Lexical[j].Score
			}
			return response.Lexical[i].ID < response.Lexical[j].ID
		})
	default:
		for index := range response.Hybrid {
			hit := &response.Hybrid[index]
			score := scores[hit.ID]
			hit.TotalScore = blend(hit.TotalScore, score)
			hit.RerankScore = &score
		}
		sort.SliceStable(response.Hybrid, func(i, j int) bool {
			if response.Hybrid[i].TotalScore != response.Hybrid[j].TotalScore {
				return response.Hybrid[i].TotalScore > response.Hybrid[j].TotalScore
			}
			return response.Hybrid[i].ID < response.Hybrid[j].ID
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
