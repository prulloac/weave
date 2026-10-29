package search_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine/graph"
	"github.com/prulloac/weave/engine/okf"
	"github.com/prulloac/weave/engine/search"
	sqlitestore "github.com/prulloac/weave/storage/sqlite"
)

type bm25Calibration struct {
	Bundle         string     `json:"bundle"`
	BM25Baseline   baseline   `json:"bm25_baseline"`
	BM25Cases      []bm25Case `json:"bm25_cases"`
	HybridBaseline baseline   `json:"hybrid_baseline"`
	HybridCases    []bm25Case `json:"hybrid_cases"`
}

type bm25Case struct {
	Name     string        `json:"name"`
	Query    string        `json:"query"`
	TopK     int           `json:"top_k"`
	Expected []expectedHit `json:"expected"`
}

func TestBM25Calibration(t *testing.T) {
	data, err := os.ReadFile("testdata/calibration.json")
	if err != nil {
		t.Fatal(err)
	}
	var spec bm25Calibration
	if err := json.Unmarshal(data, &spec); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	bundle, err := okf.ParseBundle(ctx, "testdata/"+spec.Bundle, okf.Options{Permissive: true})
	if err != nil {
		t.Fatal(err)
	}
	store, err := sqlitestore.Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.Ingest(ctx, bundle, sqlitestore.IngestOptions{Mode: "permissive"}); err != nil {
		t.Fatal(err)
	}
	index := search.Build(bundle, graph.Build(bundle))

	bm25NDCG, bm25MRR := measureBM25(t, store, bundle.Root, spec.BM25Cases)
	t.Logf("bm25 calibration: ndcg=%.4f mrr=%.4f", bm25NDCG, bm25MRR)
	if bm25NDCG < spec.BM25Baseline.NDCG {
		t.Fatalf("bm25 ndcg %.4f below baseline %.4f", bm25NDCG, spec.BM25Baseline.NDCG)
	}
	if bm25MRR < spec.BM25Baseline.MRR {
		t.Fatalf("bm25 mrr %.4f below baseline %.4f", bm25MRR, spec.BM25Baseline.MRR)
	}

	hybridNDCG, hybridMRR := measureHybrid(t, store, index, bundle.Root, spec.HybridCases)
	t.Logf("hybrid calibration: ndcg=%.4f mrr=%.4f", hybridNDCG, hybridMRR)
	if hybridNDCG < spec.HybridBaseline.NDCG {
		t.Fatalf("hybrid ndcg %.4f below baseline %.4f", hybridNDCG, spec.HybridBaseline.NDCG)
	}
	if hybridMRR < spec.HybridBaseline.MRR {
		t.Fatalf("hybrid mrr %.4f below baseline %.4f", hybridMRR, spec.HybridBaseline.MRR)
	}
}

func measureBM25(
	t *testing.T,
	store *sqlitestore.Store,
	root string,
	cases []bm25Case,
) (float64, float64) {
	t.Helper()
	totalNDCG, totalMRR := 0.0, 0.0
	for _, testCase := range cases {
		testCase := testCase
		t.Run("bm25/"+testCase.Name, func(t *testing.T) {
			hits, err := store.SearchBM25(context.Background(), root, testCase.Query, testCase.TopK)
			if err != nil {
				t.Fatal(err)
			}
			ranked := lexicalIDs(hits)
			metrics := caseMetrics(t, ranked, testCase)
			totalNDCG += metrics.ndcg
			totalMRR += metrics.mrr
		})
	}
	if len(cases) == 0 {
		return 1, 1
	}
	return totalNDCG / float64(len(cases)), totalMRR / float64(len(cases))
}

func measureHybrid(
	t *testing.T,
	store *sqlitestore.Store,
	index *search.Index,
	root string,
	cases []bm25Case,
) (float64, float64) {
	t.Helper()
	totalNDCG, totalMRR := 0.0, 0.0
	for _, testCase := range cases {
		testCase := testCase
		t.Run("hybrid/"+testCase.Name, func(t *testing.T) {
			lexical, err := store.SearchBM25(context.Background(), root, testCase.Query, testCase.TopK)
			if err != nil {
				t.Fatal(err)
			}
			fuzzyResponse, err := search.Search(index, testCase.Query, contractv1.SearchFuzzy, testCase.TopK)
			if err != nil {
				t.Fatal(err)
			}
			hybrid := search.HybridBM25(lexical, fuzzyResponse.Fuzzy, testCase.TopK)
			ranked := hybridIDs(hybrid)
			metrics := caseMetrics(t, ranked, testCase)
			totalNDCG += metrics.ndcg
			totalMRR += metrics.mrr
		})
	}
	if len(cases) == 0 {
		return 1, 1
	}
	return totalNDCG / float64(len(cases)), totalMRR / float64(len(cases))
}

type caseMetric struct {
	ndcg float64
	mrr  float64
}

func caseMetrics(t *testing.T, ranked []string, testCase bm25Case) caseMetric {
	t.Helper()
	grades := make(map[string]int, len(testCase.Expected))
	for _, hit := range testCase.Expected {
		grades[hit.ID] = hit.Grade
	}
	if len(testCase.Expected) == 0 {
		if len(ranked) != 0 {
			t.Fatalf("expected no results, got %v", ranked)
		}
		return caseMetric{ndcg: 1}
	}
	value := caseMetric{
		ndcg: ndcg(ranked, grades, testCase.TopK),
		mrr:  mrr(ranked, grades),
	}
	t.Logf("ndcg@%d=%.4f mrr=%.4f ranked=%v", testCase.TopK, value.ndcg, value.mrr, ranked)
	return value
}

func lexicalIDs(hits []contractv1.LexicalHit) []string {
	ids := make([]string, len(hits))
	for index, hit := range hits {
		ids[index] = hit.ID
	}
	return ids
}

func hybridIDs(hits []contractv1.HybridHit) []string {
	ids := make([]string, len(hits))
	for index, hit := range hits {
		ids[index] = hit.ID
	}
	return ids
}
