package main

import (
	"encoding/json"
	"math"
	"reflect"
	"slices"
	"strings"
	"testing"
)

type bddBM25Explain struct {
	Query  string `json:"query"`
	Mode   string `json:"mode"`
	TopK   int    `json:"top_k"`
	ID     string `json:"id"`
	Rank   int    `json:"rank"`
	Found  bool   `json:"found"`
	Hybrid struct {
		KeywordWeight float64 `json:"keyword_weight"`
		LexicalWeight float64 `json:"lexical_weight"`
		FuzzyWeight   float64 `json:"fuzzy_weight"`
		Total         float64 `json:"total"`
	} `json:"hybrid"`
	Fuzzy struct {
		Score float64 `json:"score"`
	} `json:"fuzzy"`
	Lexical *struct {
		RawScore      float64  `json:"raw_score"`
		Score         float64  `json:"score"`
		MatchedFields []string `json:"matched_fields"`
		Tokens        []struct {
			QueryToken   string  `json:"query_token"`
			Matched      bool    `json:"matched"`
			Contribution float64 `json:"contribution"`
		} `json:"tokens"`
	} `json:"lexical"`
}

func runBM25Explain(t *testing.T, bundle, query, id string, extra ...string) (int, bddBM25Explain) {
	t.Helper()

	args := []string{"explain", bundle, query, "--mode", "bm25", "--id", id, "--json"}
	args = append(args, extra...)
	code, stdout, stderr := invokePersistent(t, args...)
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	envelope := decodeEnvelope(t, stdout)
	if !envelope.OK {
		t.Fatalf("expected explanation, got failure: %+v", envelope.Error)
	}
	var result bddBM25Explain
	if err := json.Unmarshal(envelope.Result, &result); err != nil {
		t.Fatalf("decode explanation: %v\n%s", err, envelope.Result)
	}
	return code, result
}

func TestBM25ExplainBehavior(t *testing.T) {
	t.Run("BM25 evidence matches the search score", func(t *testing.T) {
		bundle := bm25Fixture()
		ingestBM25(t, bundle)
		_, search := runBM25(t, bundle, "alpha beta")
		wantIndex := slices.Index(lexicalIDs(search), "both")
		if wantIndex < 0 {
			t.Fatalf("both missing from BM25 search: %v", lexicalIDs(search))
		}

		code, result := runBM25Explain(t, bundle, "alpha beta", "both")
		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
		if !result.Found || result.ID != "both" || result.Lexical == nil {
			t.Fatalf("unexpected explanation: %+v", result)
		}
		if result.Rank != wantIndex+1 {
			t.Fatalf("rank=%d, want %d", result.Rank, wantIndex+1)
		}
		hit := search.Lexical[wantIndex]
		if !almostEqual(result.Lexical.RawScore, hit.RawScore) ||
			!almostEqual(result.Lexical.Score, hit.Score) {
			t.Fatalf("lexical scores mismatch: explain=%+v search=%+v", result.Lexical, hit)
		}
		if len(result.Lexical.MatchedFields) == 0 {
			t.Fatal("expected matched fields")
		}
		for _, token := range []string{"alpha", "beta"} {
			if !hasLexicalToken(result, token) {
				t.Fatalf("expected token %s in %+v", token, result.Lexical.Tokens)
			}
		}
	})

	t.Run("known id outside top-K reports rank zero", func(t *testing.T) {
		bundle := bm25Fixture()
		ingestBM25(t, bundle)
		_, search := runBM25(t, bundle, "alpha")
		index := slices.Index(lexicalIDs(search), "body-alpha")
		if index <= 0 {
			t.Fatalf("body-alpha must rank below first: %v", lexicalIDs(search))
		}

		_, result := runBM25Explain(t, bundle, "alpha", "body-alpha", "--top-k", "1")
		if !result.Found || result.Rank != 0 || result.Lexical == nil {
			t.Fatalf("expected found=true rank=0, got %+v", result)
		}
		if !almostEqual(result.Lexical.RawScore, search.Lexical[index].RawScore) {
			t.Fatalf("raw score=%f, want %f", result.Lexical.RawScore, search.Lexical[index].RawScore)
		}
	})

	t.Run("unknown id returns CONCEPT_NOT_FOUND", func(t *testing.T) {
		bundle := bm25Fixture()
		ingestBM25(t, bundle)
		code, stdout, _ := invokePersistent(t, "explain", bundle, "alpha",
			"--mode", "bm25", "--id", "missing", "--json")
		if code != 1 {
			t.Fatalf("expected exit 1, got %d", code)
		}
		if failure := storageFailure(t, stdout); failure.Code != "CONCEPT_NOT_FOUND" {
			t.Fatalf("expected CONCEPT_NOT_FOUND, got %s", failure.Code)
		}
	})

	t.Run("empty query exits 1", func(t *testing.T) {
		bundle := bm25Fixture()
		ingestBM25(t, bundle)
		code, _, _ := invokePersistent(t, "explain", bundle, "", "--mode", "bm25", "--json")
		if code != 1 {
			t.Fatalf("expected exit 1, got %d", code)
		}
	})

	t.Run("BM25 explain requires persistence", func(t *testing.T) {
		storageHome(t)
		code, stdout, _ := invokePersistent(t, "explain", bm25Fixture(), "alpha",
			"--mode", "bm25", "--no-db", "--json")
		if code != 2 {
			t.Fatalf("expected exit 2, got %d", code)
		}
		if failure := storageFailure(t, stdout); failure.Code != "INVALID_REQUEST" {
			t.Fatalf("expected INVALID_REQUEST, got %s", failure.Code)
		}
	})

	t.Run("BM25 explain is deterministic", func(t *testing.T) {
		bundle := bm25Fixture()
		ingestBM25(t, bundle)
		_, first := runBM25Explain(t, bundle, "alpha beta", "both")
		_, second := runBM25Explain(t, bundle, "alpha beta", "both")
		if !reflect.DeepEqual(first, second) {
			t.Fatalf("explanations differ:\nfirst:  %+v\nsecond: %+v", first, second)
		}
	})

	t.Run("human output includes lexical evidence", func(t *testing.T) {
		bundle := bm25Fixture()
		ingestBM25(t, bundle)
		code, stdout, stderr := invokePersistent(t, "explain", bundle, "alpha beta",
			"--mode", "bm25", "--id", "both")
		if stderr != "" {
			t.Fatalf("expected empty stderr, got %q", stderr)
		}
		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
		for _, want := range []string{"lexical", "raw", "normalized", "alpha", "beta"} {
			if !strings.Contains(stdout, want) {
				t.Fatalf("expected human output to contain %q:\n%s", want, stdout)
			}
		}
	})

	t.Run("hybrid explanation includes BM25 and fuzzy components", func(t *testing.T) {
		bundle := bm25Fixture()
		ingestBM25(t, bundle)
		hybrid := runHybrid(t, bundle, "alpha beta")
		index := -1
		var hit bddHybridHit
		for i, candidate := range hybrid.Hybrid {
			if candidate.LexicalScore > 0 && candidate.FuzzyScore > 0 {
				index = i
				hit = candidate
				break
			}
		}
		if index < 0 {
			t.Fatalf("expected a hybrid hit with both components: %+v", hybrid.Hybrid)
		}

		code, stdout, stderr := invokePersistent(t, "explain", bundle, "alpha beta",
			"--mode", "hybrid", "--id", hit.ID, "--json")
		if code != 0 || stderr != "" {
			t.Fatalf("explain exit=%d stderr=%q", code, stderr)
		}
		envelope := decodeEnvelope(t, stdout)
		var result bddBM25Explain
		if err := json.Unmarshal(envelope.Result, &result); err != nil {
			t.Fatal(err)
		}
		if result.Lexical == nil {
			t.Fatalf("expected lexical component: %+v", result)
		}
		if result.Rank != index+1 {
			t.Fatalf("rank=%d, want %d", result.Rank, index+1)
		}
		if !almostEqual(result.Lexical.Score, hit.LexicalScore) ||
			!almostEqual(result.Fuzzy.Score, hit.FuzzyScore) {
			t.Fatalf("component mismatch: explain=%+v search=%+v", result, hit)
		}
		if result.Hybrid.LexicalWeight != 0.5 || result.Hybrid.FuzzyWeight != 0.5 {
			t.Fatalf("unexpected weights: %+v", result.Hybrid)
		}
		want := math.Round((0.5*result.Lexical.Score+0.5*result.Fuzzy.Score)*10000) / 10000
		if !almostEqual(result.Hybrid.Total, want) {
			t.Fatalf("hybrid total=%f, want %f", result.Hybrid.Total, want)
		}
	})
}

func hasLexicalToken(result bddBM25Explain, queryToken string) bool {
	if result.Lexical == nil {
		return false
	}
	for _, token := range result.Lexical.Tokens {
		if token.QueryToken == queryToken && token.Matched {
			return true
		}
	}
	return false
}
