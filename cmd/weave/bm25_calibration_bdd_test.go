package main

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"
)

type bddHybridResponse struct {
	Query  string         `json:"query"`
	Mode   string         `json:"mode"`
	TopK   int            `json:"top_k"`
	Hybrid []bddHybridHit `json:"hybrid"`
}

type bddHybridHit struct {
	ID           string  `json:"id"`
	KeywordScore float64 `json:"keyword_score"`
	LexicalScore float64 `json:"lexical_score"`
	FuzzyScore   float64 `json:"fuzzy_score"`
	TotalScore   float64 `json:"total_score"`
}

func runHybrid(t *testing.T, bundle, query string, extra ...string) bddHybridResponse {
	t.Helper()

	args := []string{"search", bundle, query, "--mode", "hybrid", "--json"}
	args = append(args, extra...)
	code, stdout, stderr := invokePersistent(t, args...)
	if code != 0 || stderr != "" {
		t.Fatalf("hybrid search exit=%d stderr=%q", code, stderr)
	}
	envelope := decodeEnvelope(t, stdout)
	if !envelope.OK {
		t.Fatalf("hybrid search failed: %+v", envelope.Error)
	}
	var response bddHybridResponse
	if err := json.Unmarshal(envelope.Result, &response); err != nil {
		t.Fatalf("decode hybrid response: %v", err)
	}
	return response
}

func TestBM25CalibrationBehavior(t *testing.T) {
	t.Run("persistent hybrid uses normalized BM25 plus fuzzy", func(t *testing.T) {
		bundle := bm25Fixture()
		ingestBM25(t, bundle)

		response := runHybrid(t, bundle, "alpha beta")
		if response.Mode != "hybrid" {
			t.Fatalf("mode=%q, want hybrid", response.Mode)
		}
		if len(response.Hybrid) == 0 {
			t.Fatal("expected hybrid results")
		}
		hasLexical := false
		for _, hit := range response.Hybrid {
			if hit.LexicalScore > 0 {
				hasLexical = true
			}
			want := math.Round((0.5*hit.LexicalScore+0.5*hit.FuzzyScore)*10000) / 10000
			if !almostEqual(hit.TotalScore, want) {
				t.Fatalf("total=%f, want %f for %+v", hit.TotalScore, want, hit)
			}
			if hit.KeywordScore != 0 {
				t.Fatalf("persistent hybrid should not use keyword scores: %+v", hit)
			}
		}
		if !hasLexical {
			t.Fatalf("expected at least one lexical contribution: %+v", response.Hybrid)
		}
	})

	t.Run("hybrid is deterministic", func(t *testing.T) {
		bundle := bm25Fixture()
		ingestBM25(t, bundle)

		first := runHybrid(t, bundle, "alpha beta")
		second := runHybrid(t, bundle, "alpha beta")
		if !reflect.DeepEqual(first.Hybrid, second.Hybrid) {
			t.Fatalf("hybrid is not deterministic:\nfirst:  %+v\nsecond: %+v", first.Hybrid, second.Hybrid)
		}
	})

	t.Run("--no-db keeps the legacy hybrid fallback", func(t *testing.T) {
		storageHome(t)

		response := runHybrid(t, bm25Fixture(), "alpha", "--no-db")
		if len(response.Hybrid) == 0 {
			t.Fatal("expected legacy hybrid results")
		}
		for _, hit := range response.Hybrid {
			if hit.LexicalScore != 0 {
				t.Fatalf("legacy hybrid should not set lexical_score: %+v", hit)
			}
		}
	})
}
