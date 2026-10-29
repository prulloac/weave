package search_test

import (
	"context"
	"errors"
	"math"
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine/graph"
	"github.com/prulloac/weave/engine/okf"
	"github.com/prulloac/weave/engine/search"
)

func TestExplainHybridMatchesSearch(t *testing.T) {
	index := explainIndex(t)
	response, err := search.Search(index, "oauth", contractv1.SearchHybrid, 10)
	if err != nil {
		t.Fatal(err)
	}
	result, err := search.Explain(index, "oauth", contractv1.SearchHybrid, 10, "oauth-token-rotation")
	if err != nil {
		t.Fatal(err)
	}

	if !result.Found || result.ID != "oauth-token-rotation" {
		t.Fatalf("unexpected target: %+v", result)
	}
	if result.Rank != explainRank(response, contractv1.SearchHybrid, result.ID) {
		t.Fatalf("rank=%d, want %d", result.Rank, explainRank(response, contractv1.SearchHybrid, result.ID))
	}
	keyword, ok := keywordHit(response, result.ID)
	if !ok || !almost(result.Keyword.Score, keyword.Score) {
		t.Fatalf("keyword score=%f, want %+v", result.Keyword.Score, keyword)
	}
	fuzzy, ok := fuzzyHit(response, result.ID)
	if !ok || !almost(result.Fuzzy.Score, fuzzy.Score) {
		t.Fatalf("fuzzy score=%f, want %+v", result.Fuzzy.Score, fuzzy)
	}
	hybrid, ok := hybridHit(response, result.ID)
	if !ok || !almost(result.Hybrid.Total, hybrid.TotalScore) {
		t.Fatalf("hybrid total=%f, want %+v", result.Hybrid.Total, hybrid)
	}
	if len(result.Keyword.Tokens) != 1 {
		t.Fatalf("expected one keyword token, got %+v", result.Keyword.Tokens)
	}
	if result.Fuzzy.Snippet == nil {
		t.Fatal("expected a body snippet")
	}
}

func TestExplainKeywordEvidence(t *testing.T) {
	index := explainIndex(t)
	result, err := search.Explain(index, "oauth sec", contractv1.SearchKeyword, 10, "oauth-token-rotation")
	if err != nil {
		t.Fatal(err)
	}

	if !almost(result.Keyword.Score, 0.875) {
		t.Fatalf("keyword score=%f, want 0.875", result.Keyword.Score)
	}
	exact := keywordToken(result, "oauth")
	if exact.Match != contractv1.ExplainMatchExact || exact.MatchedKeyword != "oauth" {
		t.Fatalf("unexpected oauth evidence: %+v", exact)
	}
	prefix := keywordToken(result, "sec")
	if prefix.Match != contractv1.ExplainMatchPrefix || prefix.MatchedKeyword != "security" {
		t.Fatalf("unexpected sec evidence: %+v", prefix)
	}
}

func TestExplainKnownIDOutsideTopK(t *testing.T) {
	index := explainIndex(t)
	full, err := search.Search(index, "tokens", contractv1.SearchHybrid, 10)
	if err != nil {
		t.Fatal(err)
	}
	result, err := search.Explain(index, "tokens", contractv1.SearchHybrid, 1, "token-storage")
	if err != nil {
		t.Fatal(err)
	}
	if !result.Found || result.Rank != 0 {
		t.Fatalf("expected found=true rank=0, got found=%t rank=%d", result.Found, result.Rank)
	}

	keyword, _ := keywordHit(full, "token-storage")
	fuzzy, _ := fuzzyHit(full, "token-storage")
	hybrid, _ := hybridHit(full, "token-storage")
	if !almost(result.Keyword.Score, keyword.Score) ||
		!almost(result.Fuzzy.Score, fuzzy.Score) ||
		!almost(result.Hybrid.Total, hybrid.TotalScore) {
		t.Fatalf("outside-top-K scores do not match full search: %+v", result)
	}
}

func TestExplainErrors(t *testing.T) {
	index := explainIndex(t)

	if _, err := search.Explain(index, "oauth", contractv1.SearchHybrid, 10, "missing"); !errors.Is(err, search.ErrConceptNotFound) {
		t.Fatalf("expected ErrConceptNotFound, got %v", err)
	}
	if _, err := search.Explain(index, "", contractv1.SearchHybrid, 10, ""); !errors.Is(err, search.ErrEmptyQuery) {
		t.Fatalf("expected ErrEmptyQuery, got %v", err)
	}
	if _, err := search.Explain(index, "zzzz", contractv1.SearchHybrid, 10, ""); !errors.Is(err, search.ErrNoResult) {
		t.Fatalf("expected ErrNoResult, got %v", err)
	}
}

func explainIndex(t *testing.T) *search.Index {
	t.Helper()
	bundle, err := okf.ParseBundle(context.Background(), "testdata/bundle", okf.Options{})
	if err != nil {
		t.Fatal(err)
	}
	return search.Build(bundle, graph.Build(bundle))
}

func explainRank(response contractv1.SearchResponse, mode contractv1.SearchMode, id string) int {
	var hits []string
	switch mode {
	case contractv1.SearchKeyword:
		for _, hit := range response.Keyword {
			hits = append(hits, hit.ID)
		}
	case contractv1.SearchFuzzy:
		for _, hit := range response.Fuzzy {
			hits = append(hits, hit.ID)
		}
	default:
		for _, hit := range response.Hybrid {
			hits = append(hits, hit.ID)
		}
	}
	for index, candidate := range hits {
		if candidate == id {
			return index + 1
		}
	}
	return 0
}

func keywordHit(response contractv1.SearchResponse, id string) (contractv1.KeywordHit, bool) {
	for _, hit := range response.Keyword {
		if hit.ID == id {
			return hit, true
		}
	}
	return contractv1.KeywordHit{}, false
}

func fuzzyHit(response contractv1.SearchResponse, id string) (contractv1.FuzzyHit, bool) {
	for _, hit := range response.Fuzzy {
		if hit.ID == id {
			return hit, true
		}
	}
	return contractv1.FuzzyHit{}, false
}

func hybridHit(response contractv1.SearchResponse, id string) (contractv1.HybridHit, bool) {
	for _, hit := range response.Hybrid {
		if hit.ID == id {
			return hit, true
		}
	}
	return contractv1.HybridHit{}, false
}

func keywordToken(result contractv1.ExplainResult, queryToken string) contractv1.ExplainKeywordToken {
	for _, token := range result.Keyword.Tokens {
		if token.QueryToken == queryToken {
			return token
		}
	}
	return contractv1.ExplainKeywordToken{}
}

func almost(left, right float64) bool {
	return math.Abs(left-right) < 1e-9
}
