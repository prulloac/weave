package operations

import (
	"context"
	"errors"
	"path/filepath"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine"
	"github.com/prulloac/weave/engine/search"
	sqlitestore "github.com/prulloac/weave/storage/sqlite"
)

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
