package operations

import (
	"context"
	"math"
	"time"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine/extensions"
	sqlitestore "github.com/prulloac/weave/storage/sqlite"
)

func (e *Executor) searchSemantic(ctx context.Context, request contractv1.OperationRequest) any {
	if request.NoDB {
		return failure(contractv1.ErrInvalidRequest, "semantic search requires persistent storage", nil)
	}
	if request.Embedder == "" {
		return failure(contractv1.ErrInvalidRequest, "semantic search requires --embedder <name>", nil)
	}
	extension, err := extensions.Resolve(request.Path, request.Embedder)
	if err != nil {
		return failure(contractv1.ErrExtensionMissing, err.Error(), map[string]any{
			"extension": request.Embedder,
		})
	}
	timeout := time.Duration(request.TimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = extensions.DefaultTimeout
	}
	runner := extensions.Runner{Timeout: timeout}
	embedding, err := runner.Embed(ctx, extension, extensions.EmbeddingRequest{Text: request.Query})
	if err != nil {
		return failure(extensionErrorCode(err), err.Error(), map[string]any{
			"extension": request.Embedder,
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
	topK := 10
	if request.TopK != nil {
		topK = *request.TopK
	}
	if topK < 1 {
		topK = 10
	}
	if topK > 100 {
		topK = 100
	}
	hits, err := store.SemanticHits(ctx, request.Path, embedding.Model, embedding.Vector, topK)
	if err != nil {
		return failure(contractv1.ErrInvalidRequest, err.Error(), map[string]any{"path": request.Path})
	}
	bundle, err := store.LoadBundle(ctx, request.Path)
	if err != nil {
		code, message := mapStorageError(err)
		return failure(code, message, map[string]any{"path": request.Path})
	}
	concepts := make(map[string]contractv1.OkfConcept, len(bundle.Concepts))
	for _, concept := range bundle.Concepts {
		concepts[concept.ID] = concept
	}
	response := contractv1.SearchResponse{
		Query:    request.Query,
		Mode:     contractv1.SearchSemantic,
		TopK:     topK,
		Keyword:  []contractv1.KeywordHit{},
		Fuzzy:    []contractv1.FuzzyHit{},
		Hybrid:   []contractv1.HybridHit{},
		Semantic: make([]contractv1.SemanticHit, 0, len(hits)),
	}
	for _, hit := range hits {
		concept, ok := concepts[hit.DocID]
		if !ok {
			continue
		}
		title := concept.ID
		if concept.Title != nil {
			title = *concept.Title
		}
		response.Semantic = append(response.Semantic, contractv1.SemanticHit{
			ID:             concept.ID,
			Title:          title,
			Path:           concept.Path,
			Type:           concept.Type,
			Keywords:       concept.Keywords,
			MediaType:      concept.MediaType,
			SemanticScore:  roundScore(1 - hit.Distance),
			EmbeddingModel: embedding.Model,
			EmbeddingDim:   embedding.Dim,
		})
	}
	return success(response)
}

func roundScore(value float64) float64 {
	return math.Round(value*10000) / 10000
}
