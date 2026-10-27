package search_test

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"sort"
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine/graph"
	"github.com/prulloac/weave/engine/okf"
	"github.com/prulloac/weave/engine/search"
)

type calibration struct {
	Version  int               `json:"version"`
	Bundle   string            `json:"bundle"`
	Baseline baseline          `json:"baseline"`
	Cases    []calibrationCase `json:"cases"`
}

type baseline struct {
	NDCG float64 `json:"ndcg@10"`
	MRR  float64 `json:"mrr@10"`
}

type calibrationCase struct {
	Name     string                `json:"name"`
	Query    string                `json:"query"`
	Mode     contractv1.SearchMode `json:"mode"`
	TopK     int                   `json:"top_k"`
	Expected []expectedHit         `json:"expected"`
}

type expectedHit struct {
	ID    string `json:"id"`
	Grade int    `json:"grade"`
}

func TestCalibration(t *testing.T) {
	data, err := os.ReadFile("testdata/calibration.json")
	if err != nil {
		t.Fatal(err)
	}
	var spec calibration
	if err := json.Unmarshal(data, &spec); err != nil {
		t.Fatal(err)
	}

	bundle, err := okf.ParseBundle(context.Background(), "testdata/"+spec.Bundle, okf.Options{})
	if err != nil {
		t.Fatal(err)
	}
	graphView := graph.Build(bundle)
	index := search.Build(bundle, graphView)

	totalNDCG := 0.0
	totalMRR := 0.0
	measured := 0

	for _, testCase := range spec.Cases {
		testCase := testCase
		t.Run(testCase.Name, func(t *testing.T) {
			response, err := search.Search(index, testCase.Query, testCase.Mode, testCase.TopK)
			if err != nil {
				t.Fatal(err)
			}
			ranked := rankedIDs(response, testCase.Mode)
			if len(testCase.Expected) == 0 {
				if len(ranked) != 0 {
					t.Fatalf("expected no results, got %v", ranked)
				}
				return
			}

			grades := make(map[string]int, len(testCase.Expected))
			for _, hit := range testCase.Expected {
				grades[hit.ID] = hit.Grade
			}
			ndcgValue := ndcg(ranked, grades, testCase.TopK)
			mrrValue := mrr(ranked, grades)
			totalNDCG += ndcgValue
			totalMRR += mrrValue
			measured++
			t.Logf("ndcg@%d=%.4f mrr=%.4f ranked=%v", testCase.TopK, ndcgValue, mrrValue, ranked)
		})
	}

	if measured == 0 {
		t.Fatal("no calibration cases measured")
	}
	ndcgAverage := totalNDCG / float64(measured)
	mrrAverage := totalMRR / float64(measured)
	t.Logf("calibration: ndcg=%.4f mrr=%.4f", ndcgAverage, mrrAverage)

	if ndcgAverage < spec.Baseline.NDCG {
		t.Fatalf("ndcg %.4f below baseline %.4f", ndcgAverage, spec.Baseline.NDCG)
	}
	if mrrAverage < spec.Baseline.MRR {
		t.Fatalf("mrr %.4f below baseline %.4f", mrrAverage, spec.Baseline.MRR)
	}
}

func rankedIDs(response contractv1.SearchResponse, mode contractv1.SearchMode) []string {
	result := make([]string, 0)
	switch mode {
	case contractv1.SearchKeyword:
		for _, hit := range response.Keyword {
			result = append(result, hit.ID)
		}
	case contractv1.SearchFuzzy:
		for _, hit := range response.Fuzzy {
			result = append(result, hit.ID)
		}
	default:
		for _, hit := range response.Hybrid {
			result = append(result, hit.ID)
		}
	}
	return result
}

func ndcg(ranked []string, grades map[string]int, k int) float64 {
	dcg := 0.0
	for position, id := range ranked {
		if position >= k {
			break
		}
		grade := grades[id]
		if grade > 0 {
			dcg += (math.Pow(2, float64(grade)) - 1) / math.Log2(float64(position+2))
		}
	}

	idealGrades := make([]int, 0, len(grades))
	for _, grade := range grades {
		idealGrades = append(idealGrades, grade)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(idealGrades)))
	idcg := 0.0
	for position, grade := range idealGrades {
		if position >= k {
			break
		}
		idcg += (math.Pow(2, float64(grade)) - 1) / math.Log2(float64(position+2))
	}
	if idcg == 0 {
		return 1
	}
	return dcg / idcg
}

func mrr(ranked []string, grades map[string]int) float64 {
	for position, id := range ranked {
		if grades[id] > 0 {
			return 1 / float64(position+1)
		}
	}
	return 0
}
