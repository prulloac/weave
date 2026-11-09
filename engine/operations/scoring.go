package operations

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine"
	"github.com/prulloac/weave/engine/extensions"
	"github.com/prulloac/weave/engine/search"
	sqlitestore "github.com/prulloac/weave/storage/sqlite"
)

const (
	scorerDefaultWeight  = 0.5
	scorerDefaultK       = 100
	scorerDefaultTimeout = 10 * time.Second
	maxScorerKeywords    = 64
)

// searchLimit is the number of core candidates to retrieve for one search.
func searchLimit(request contractv1.OperationRequest, topK int) int {
	if request.Scorer == "" {
		return topK
	}
	if request.CandidateK != nil {
		return *request.CandidateK
	}
	if topK > scorerDefaultK {
		return topK
	}
	return scorerDefaultK
}

func scorerWeight(request contractv1.OperationRequest) float64 {
	if request.ScorerWeight == nil {
		return scorerDefaultWeight
	}
	return *request.ScorerWeight
}

func scorerCandidateK(request contractv1.OperationRequest) int {
	if request.CandidateK != nil {
		return *request.CandidateK
	}
	return scorerDefaultK
}

func scorerTimeout(request contractv1.OperationRequest) time.Duration {
	if request.TimeoutMS > 0 {
		return time.Duration(request.TimeoutMS) * time.Millisecond
	}
	return scorerDefaultTimeout
}

// applyScorerFromStore loads the bundle and applies the scorer for a persistent search.
func (e *Executor) applyScorerFromStore(
	ctx context.Context,
	request contractv1.OperationRequest,
	store *sqlitestore.Store,
	response *contractv1.SearchResponse,
) error {
	if request.Scorer == "" {
		return nil
	}
	root, err := filepath.Abs(request.Path)
	if err != nil {
		return err
	}
	bundle, err := store.LoadBundle(ctx, root)
	if err != nil {
		return err
	}
	eng := engine.FromBundle(bundle)
	defer eng.Close()
	return e.applyScorer(ctx, request, eng, response)
}

// applyScorer scores and reorders the selected hit list. Missing extensions are
// returned as errors; runtime failures fall back to the core result.
func (e *Executor) applyScorer(
	ctx context.Context,
	request contractv1.OperationRequest,
	eng *engine.Engine,
	response *contractv1.SearchResponse,
) error {
	if request.Scorer == "" {
		return nil
	}
	if request.Rerank != "" {
		return fmt.Errorf("scorer and rerank are mutually exclusive")
	}
	extension, err := extensions.Resolve(request.Path, request.Scorer)
	if err != nil {
		return err
	}
	if scorerWeight(request) == 0 {
		return nil
	}
	mode := request.Mode
	if mode == "" {
		mode = contractv1.SearchHybrid
	}
	candidates := scorerCandidates(*response, mode, eng, request.Query)
	if len(candidates) == 0 {
		return nil
	}
	runner := extensions.Runner{Timeout: scorerTimeout(request)}
	scores, err := runner.Score(ctx, extension, extensions.ScorerRequest{
		Query:      response.Query,
		Mode:       string(mode),
		Candidates: candidates,
	})
	if err != nil {
		e.warn("scorer %s: %v", extension.Name, err)
		return nil
	}
	blendScorer(response, mode, scores, scorerWeight(request))
	response.Scorer = &contractv1.ScorerInfo{
		Name:       extension.Name,
		Weight:     scorerWeight(request),
		CandidateK: len(candidates),
	}
	if limit := responseTopK(request); limit > 0 {
		truncateResponse(response, limit)
	}
	return nil
}

func responseTopK(request contractv1.OperationRequest) int {
	if request.TopK == nil {
		return 10
	}
	return *request.TopK
}

func scorerCandidates(
	response contractv1.SearchResponse,
	mode contractv1.SearchMode,
	eng *engine.Engine,
	query string,
) []extensions.ScorerCandidate {
	concepts := make(map[string]contractv1.OkfConcept)
	if eng != nil {
		for _, concept := range eng.Bundle().Concepts {
			concepts[concept.ID] = concept
		}
	}
	terms := search.Tokenize(query)
	build := func(id, path, title, docType string, snippet *string, scores extensions.ScorerScores) extensions.ScorerCandidate {
		concept := concepts[id]
		candidate := extensions.ScorerCandidate{
			ID:       id,
			Path:     path,
			Title:    title,
			Type:     docType,
			Keywords: boundKeywords(concept.Keywords),
			Scores:   scores,
		}
		if concept.Description != nil {
			candidate.Description = *concept.Description
		}
		if concept.Status != nil {
			candidate.Status = string(*concept.Status)
		}
		if candidate.Type == "" {
			candidate.Type = concept.Type
		}
		if candidate.Title == "" && concept.Title != nil {
			candidate.Title = *concept.Title
		}
		candidate.Snippet = snippet
		if candidate.Snippet == nil && strings.TrimSpace(concept.Body) != "" {
			candidate.Snippet = search.Snippet(concept.Body, terms)
		}
		return candidate
	}

	switch mode {
	case contractv1.SearchKeyword:
		candidates := make([]extensions.ScorerCandidate, 0, len(response.Keyword))
		for _, hit := range response.Keyword {
			candidates = append(candidates, build(hit.ID, hit.Path, hit.Title, hit.Type, nil, extensions.ScorerScores{
				Keyword: hit.Score,
				Core:    hit.Score,
			}))
		}
		return candidates
	case contractv1.SearchFuzzy:
		candidates := make([]extensions.ScorerCandidate, 0, len(response.Fuzzy))
		for _, hit := range response.Fuzzy {
			candidates = append(candidates, build(hit.ID, hit.Path, hit.Title, hit.Type, hit.Snippet, extensions.ScorerScores{
				Fuzzy: hit.Score,
				Core:  hit.Score,
			}))
		}
		return candidates
	case contractv1.SearchBM25:
		candidates := make([]extensions.ScorerCandidate, 0, len(response.Lexical))
		for _, hit := range response.Lexical {
			candidates = append(candidates, build(hit.ID, hit.Path, hit.Title, hit.Type, nil, extensions.ScorerScores{
				Lexical: hit.Score,
				Core:    hit.Score,
			}))
		}
		return candidates
	default:
		candidates := make([]extensions.ScorerCandidate, 0, len(response.Hybrid))
		for _, hit := range response.Hybrid {
			candidates = append(candidates, build(hit.ID, hit.Path, hit.Title, hit.Type, hit.Snippet, extensions.ScorerScores{
				Keyword: hit.KeywordScore,
				Lexical: hit.LexicalScore,
				Fuzzy:   hit.FuzzyScore,
				Core:    hit.TotalScore,
			}))
		}
		return candidates
	}
}

func boundKeywords(keywords []string) []string {
	if len(keywords) == 0 {
		return nil
	}
	if len(keywords) > maxScorerKeywords {
		keywords = keywords[:maxScorerKeywords]
	}
	return append([]string(nil), keywords...)
}

func blendScorer(
	response *contractv1.SearchResponse,
	mode contractv1.SearchMode,
	scores map[string]float64,
	weight float64,
) {
	blend := func(core, scorer float64) float64 {
		return search.RoundScore((1-weight)*core + weight*scorer)
	}
	set := func(scorer float64) *float64 {
		value := scorer
		return &value
	}
	switch mode {
	case contractv1.SearchKeyword:
		for index := range response.Keyword {
			hit := &response.Keyword[index]
			scorer := scores[hit.ID]
			hit.Score = blend(hit.Score, scorer)
			hit.ScorerScore = set(scorer)
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
			scorer := scores[hit.ID]
			hit.Score = blend(hit.Score, scorer)
			hit.ScorerScore = set(scorer)
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
			scorer := scores[hit.ID]
			hit.Score = blend(hit.Score, scorer)
			hit.ScorerScore = set(scorer)
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
			scorer := scores[hit.ID]
			hit.TotalScore = blend(hit.TotalScore, scorer)
			hit.ScorerScore = set(scorer)
		}
		sort.SliceStable(response.Hybrid, func(i, j int) bool {
			if response.Hybrid[i].TotalScore != response.Hybrid[j].TotalScore {
				return response.Hybrid[i].TotalScore > response.Hybrid[j].TotalScore
			}
			return response.Hybrid[i].ID < response.Hybrid[j].ID
		})
	}
}

// executeOperation breaks the initialization cycle between Execute, the engine
// handler table and the scorer explain helper.
var executeOperation func(*Executor, context.Context, contractv1.OperationRequest) any

func init() {
	executeOperation = (*Executor).Execute
}

// applyScorerExplain adds scorer evidence to an explanation by running the
// scorer over the same candidate set.
func (e *Executor) applyScorerExplain(
	ctx context.Context,
	request contractv1.OperationRequest,
	result *contractv1.ExplainResult,
) (any, bool) {
	if request.Scorer == "" {
		return nil, false
	}
	limit := scorerCandidateK(request)
	nested := request
	nested.Op = contractv1.OpSearch
	nested.Target = ""
	nested.TopK = &limit
	nested.CandidateK = &limit
	envelope := executeOperation(e, ctx, nested)
	successEnvelope, ok := envelope.(contractv1.SuccessEnvelope)
	if !ok {
		return envelope, true
	}
	response, ok := successEnvelope.Result.(contractv1.SearchResponse)
	if !ok || response.Scorer == nil {
		return nil, false
	}
	core := explanationCore(*result)
	scorerScore, final, rank, found := explainedScorerHit(response, result.Mode, result.ID)
	if !found {
		return nil, false
	}
	result.Rank = rank
	result.Scorer = &contractv1.ExplainScorer{
		Name:        response.Scorer.Name,
		Weight:      response.Scorer.Weight,
		CandidateK:  response.Scorer.CandidateK,
		CoreScore:   core,
		ScorerScore: scorerScore,
		FinalScore:  final,
	}
	return nil, false
}

func explanationCore(result contractv1.ExplainResult) float64 {
	switch result.Mode {
	case contractv1.SearchKeyword:
		return result.Keyword.Score
	case contractv1.SearchFuzzy:
		return result.Fuzzy.Score
	case contractv1.SearchBM25:
		if result.Lexical != nil {
			return result.Lexical.Score
		}
		return 0
	default:
		return result.Hybrid.Total
	}
}

func explainedScorerHit(
	response contractv1.SearchResponse,
	mode contractv1.SearchMode,
	target string,
) (float64, float64, int, bool) {
	switch mode {
	case contractv1.SearchKeyword:
		for index, hit := range response.Keyword {
			if hit.ID == target && hit.ScorerScore != nil {
				return *hit.ScorerScore, hit.Score, index + 1, true
			}
		}
	case contractv1.SearchFuzzy:
		for index, hit := range response.Fuzzy {
			if hit.ID == target && hit.ScorerScore != nil {
				return *hit.ScorerScore, hit.Score, index + 1, true
			}
		}
	case contractv1.SearchBM25:
		for index, hit := range response.Lexical {
			if hit.ID == target && hit.ScorerScore != nil {
				return *hit.ScorerScore, hit.Score, index + 1, true
			}
		}
	default:
		for index, hit := range response.Hybrid {
			if hit.ID == target && hit.ScorerScore != nil {
				return *hit.ScorerScore, hit.TotalScore, index + 1, true
			}
		}
	}
	return 0, 0, 0, false
}

func truncateResponse(response *contractv1.SearchResponse, topK int) {
	if topK <= 0 {
		return
	}
	if len(response.Keyword) > topK {
		response.Keyword = response.Keyword[:topK]
	}
	if len(response.Fuzzy) > topK {
		response.Fuzzy = response.Fuzzy[:topK]
	}
	if len(response.Hybrid) > topK {
		response.Hybrid = response.Hybrid[:topK]
	}
	if len(response.Lexical) > topK {
		response.Lexical = response.Lexical[:topK]
	}
	if len(response.Semantic) > topK {
		response.Semantic = response.Semantic[:topK]
	}
	response.TopK = topK
}
