package operations

import (
	"context"
	"errors"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine"
	"github.com/prulloac/weave/engine/compression"
	"github.com/prulloac/weave/engine/graph"
	"github.com/prulloac/weave/engine/privacy"
	sqlitestore "github.com/prulloac/weave/storage/sqlite"
)

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
	if request.DryRun {
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
		bundle := eng.Bundle()
		return success(contractv1.IngestReport{
			DB:        dbPath,
			Bundle:    bundle.Root,
			Mode:      mode,
			Documents: len(bundle.Concepts),
			Edges:     len(graph.SortedEdges(eng.Graph())),
			Warnings:  len(bundle.Validation.Warnings),
			DryRun:    true,
		})
	}
	enricher, err := e.newEnricher(request)
	if err != nil {
		return failure(contractv1.ErrExtensionMissing, err.Error(), map[string]any{
			"extension": request.Enricher,
		})
	}
	provider, err := e.resolveProvider(request.Path, request.Provider)
	if err != nil {
		return failure(contractv1.ErrPrivacyKey, err.Error(), map[string]any{"provider": request.Provider})
	}
	if provider != nil {
		defer provider.Close()
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
		report, err = store.Rebuild(ctx, eng.Bundle(), sqlitestore.IngestOptions{Mode: mode, Codec: codec, Enricher: enricher, Provider: provider})
	} else {
		report, err = store.Ingest(ctx, eng.Bundle(), sqlitestore.IngestOptions{Mode: mode, Codec: codec, Enricher: enricher, Provider: provider})
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
	if request.DryRun {
		options := e.Options
		options.Permissive = request.Permissive
		eng, err := engine.Open(ctx, request.Path, options)
		if err != nil {
			code, message := mapOpenError(err)
			return failure(code, message, map[string]any{"path": request.Path})
		}
		defer eng.Close()
		bundle := eng.Bundle()
		return success(contractv1.SyncReport{
			DB:        dbPath,
			Bundle:    bundle.Root,
			Mode:      mode,
			Documents: len(bundle.Concepts),
			Warnings:  len(bundle.Validation.Warnings),
			DryRun:    true,
		})
	}
	enricher, err := e.newEnricher(request)
	if err != nil {
		return failure(contractv1.ErrExtensionMissing, err.Error(), map[string]any{
			"extension": request.Enricher,
		})
	}
	provider, err := e.resolveProvider(request.Path, request.Provider)
	if err != nil {
		return failure(contractv1.ErrPrivacyKey, err.Error(), map[string]any{"provider": request.Provider})
	}
	if provider != nil {
		defer provider.Close()
	}
	report, err := store.Sync(ctx, request.Path, sqlitestore.SyncOptions{Mode: mode, Codec: request.Codec, Enricher: enricher, Provider: provider})
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
