package operations

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine/extensions"
	"github.com/prulloac/weave/engine/privacy"
	"github.com/prulloac/weave/engine/search"
	sqlitestore "github.com/prulloac/weave/storage/sqlite"
)

func (e *Executor) resolveProvider(root, name string) (privacy.Provider, error) {
	if root == "" {
		return nil, nil
	}
	return privacy.ResolveProvider(root, name)
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
