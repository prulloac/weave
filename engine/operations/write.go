package operations

import (
	"context"
	"errors"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine"
	"github.com/prulloac/weave/engine/assets"
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
	projectExt, projectManifest, projectMode, err := e.projectExtension(ctx, request)
	if err != nil {
		return failure(extensionErrorCode(err), err.Error(), map[string]any{
			"extension": request.Enricher,
		})
	}
	var enricher sqlitestore.Enricher
	if !projectMode {
		enricher, err = e.newEnricher(request)
		if err != nil {
			return failure(extensionErrorCode(err), err.Error(), map[string]any{
				"extension": request.Enricher,
			})
		}
	}
	embedder, err := e.newEmbedder(request)
	if err != nil {
		return failure(extensionErrorCode(err), err.Error(), map[string]any{
			"extension": request.Embedder,
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

	assetTypes := request.AssetTypes
	if projectMode && len(assetTypes) == 0 {
		assetTypes = projectAssetTypes(projectManifest)
	}
	if len(assetTypes) == 0 && !request.NoAssets && !projectMode {
		stored, err := store.AssetTypes(ctx, request.Path)
		if err != nil {
			return failure(contractv1.ErrInvalidRequest, err.Error(), map[string]any{"path": request.Path})
		}
		assetTypes = stored
	}
	if request.NoAssets {
		assetTypes = nil
		projectMode = false
	}
	if request.Assets && len(assetTypes) == 0 && !projectMode {
		return failure(contractv1.ErrAssetTypesRequired, "asset scanning requires --asset-types", map[string]any{
			"path": request.Path,
		})
	}
	scannedConcepts := []contractv1.OkfConcept{}
	if len(assetTypes) > 0 {
		scanned, err := assets.Scan(ctx, request.Path, assets.Filter{Types: assetTypes})
		if err != nil {
			return failure(contractv1.ErrInvalidRequest, err.Error(), map[string]any{"path": request.Path})
		}
		scannedConcepts = scanned.Concepts
		eng.Bundle().Concepts = append(eng.Bundle().Concepts, scanned.Concepts...)
		eng.Bundle().Validation.Warnings = append(eng.Bundle().Validation.Warnings, scanned.Warnings...)
	}
	if projectMode {
		if err := e.applyProjectAnalysis(ctx, request, projectExt, eng, scannedConcepts); err != nil {
			if request.Permissive {
				e.warn("project %s: %v", projectExt.Name, err)
			} else {
				return failure(extensionErrorCode(err), err.Error(), map[string]any{
					"extension": request.Enricher,
					"path":      request.Path,
				})
			}
		}
	}

	var report contractv1.IngestReport
	if request.Op == contractv1.OpRebuild {
		report, err = store.Rebuild(ctx, eng.Bundle(), sqlitestore.IngestOptions{Mode: mode, Codec: codec, CodecSet: request.CodecSet, AssetTypes: assetTypes, Enricher: enricher, Embedder: embedder, Provider: provider})
	} else {
		report, err = store.Ingest(ctx, eng.Bundle(), sqlitestore.IngestOptions{Mode: mode, Codec: codec, CodecSet: request.CodecSet, AssetTypes: assetTypes, Enricher: enricher, Embedder: embedder, Provider: provider})
	}
	if err != nil {
		if errors.Is(err, privacy.ErrKeyUnavailable) {
			return failure(contractv1.ErrPrivacyKey, err.Error(), map[string]any{
				"path": request.Path,
				"db":   dbPath,
			})
		}
		if code, ok := codecErrorCode(err); ok {
			return failure(code, err.Error(), map[string]any{
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
	projectExt, projectManifest, projectMode, err := e.projectExtension(ctx, request)
	if err != nil {
		return failure(extensionErrorCode(err), err.Error(), map[string]any{
			"extension": request.Enricher,
		})
	}
	if projectMode {
		has, err := store.HasBundle(ctx, request.Path)
		if err != nil {
			code, message := mapStorageError(err)
			return failure(code, message, map[string]any{"path": request.Path})
		}
		if !has {
			return failure(contractv1.ErrBundleNotIndexed, "bundle is not indexed; run: weave ingest "+request.Path, map[string]any{
				"path": request.Path,
			})
		}
	}
	var enricher sqlitestore.Enricher
	if !projectMode {
		enricher, err = e.newEnricher(request)
		if err != nil {
			return failure(extensionErrorCode(err), err.Error(), map[string]any{
				"extension": request.Enricher,
			})
		}
	}
	embedder, err := e.newEmbedder(request)
	if err != nil {
		return failure(extensionErrorCode(err), err.Error(), map[string]any{
			"extension": request.Embedder,
		})
	}
	provider, err := e.resolveProvider(request.Path, request.Provider)
	if err != nil {
		return failure(contractv1.ErrPrivacyKey, err.Error(), map[string]any{"provider": request.Provider})
	}
	if provider != nil {
		defer provider.Close()
	}
	assetTypes := request.AssetTypes
	if projectMode && len(assetTypes) == 0 {
		assetTypes = projectAssetTypes(projectManifest)
	}
	if len(assetTypes) == 0 && !request.NoAssets {
		stored, err := store.AssetTypes(ctx, request.Path)
		if err != nil {
			return failure(contractv1.ErrInvalidRequest, err.Error(), map[string]any{"path": request.Path})
		}
		assetTypes = stored
	}
	if request.NoAssets {
		assetTypes = nil
		projectMode = false
	}
	if request.Assets && len(assetTypes) == 0 && !projectMode {
		return failure(contractv1.ErrAssetTypesRequired, "asset scanning requires --asset-types", map[string]any{
			"path": request.Path,
		})
	}
	projectConcepts := []contractv1.OkfConcept{}
	if projectMode && len(assetTypes) > 0 {
		scanned, err := assets.Scan(ctx, request.Path, assets.Filter{Types: assetTypes})
		if err != nil {
			return failure(contractv1.ErrInvalidRequest, err.Error(), map[string]any{"path": request.Path})
		}
		response, err := e.runProjectAnalysis(ctx, request, projectExt, scanned.Concepts)
		if err != nil {
			if request.Permissive {
				e.warn("project %s: %v", projectExt.Name, err)
				projectConcepts = scanned.Concepts
			} else {
				return failure(extensionErrorCode(err), err.Error(), map[string]any{
					"extension": request.Enricher,
					"path":      request.Path,
				})
			}
		} else {
			bundle := &contractv1.Bundle{Root: absoluteRoot(request.Path), Concepts: scanned.Concepts}
			applyProjectResponse(bundle, response)
			projectConcepts = bundle.Concepts
		}
	}
	report, err := store.Sync(ctx, request.Path, sqlitestore.SyncOptions{
		Mode:            mode,
		Codec:           request.Codec,
		CodecSet:        request.CodecSet,
		AssetTypes:      assetTypes,
		NoAssets:        request.NoAssets,
		Enricher:        enricher,
		Embedder:        embedder,
		Provider:        provider,
		ProjectConcepts: projectConcepts,
	})
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
		if code, ok := codecErrorCode(err); ok {
			return failure(code, err.Error(), map[string]any{
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
