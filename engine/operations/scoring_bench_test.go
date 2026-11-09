package operations

import (
	"fmt"
	"strings"
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine"
)

func scorerBenchBundle(size int) *engine.Engine {
	concepts := make([]contractv1.OkfConcept, 0, size)
	for index := 0; index < size; index++ {
		title := fmt.Sprintf("Concept %03d", index)
		description := strings.Repeat("context ", 8)
		concepts = append(concepts, contractv1.OkfConcept{
			ID:          fmt.Sprintf("concepts/%03d", index),
			Path:        fmt.Sprintf("concepts/%03d.md", index),
			Type:        "Concept",
			Keywords:    []string{"alpha", "beta", "gamma", "delta"},
			Title:       &title,
			Description: &description,
			Body:        strings.Repeat("alpha body text ", 32),
		})
	}
	return engine.FromBundle(&contractv1.Bundle{Root: "/bundle", Concepts: concepts})
}

func scorerBenchResponse(size int) contractv1.SearchResponse {
	hybrid := make([]contractv1.HybridHit, 0, size)
	for index := 0; index < size; index++ {
		hybrid = append(hybrid, contractv1.HybridHit{
			ID:            fmt.Sprintf("concepts/%03d", index),
			Title:         fmt.Sprintf("Concept %03d", index),
			Path:          fmt.Sprintf("concepts/%03d.md", index),
			Type:          "Concept",
			KeywordScore:  0.8,
			LexicalScore:  0.7,
			FuzzyScore:    0.6,
			TotalScore:    0.75,
			MatchedFields: []contractv1.SearchField{contractv1.FieldTitle},
		})
	}
	return contractv1.SearchResponse{Query: "alpha", Mode: contractv1.SearchHybrid, TopK: size, Hybrid: hybrid}
}

func BenchmarkScorerCandidates100(b *testing.B) {
	eng := scorerBenchBundle(100)
	response := scorerBenchResponse(100)
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		_ = scorerCandidates(response, contractv1.SearchHybrid, eng, "alpha")
	}
}

func BenchmarkScorerBlend100(b *testing.B) {
	response := scorerBenchResponse(100)
	scores := make(map[string]float64, 100)
	for index := 0; index < 100; index++ {
		scores[fmt.Sprintf("concepts/%03d", index)] = 0.5
	}
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		copy := response
		copy.Hybrid = append([]contractv1.HybridHit(nil), response.Hybrid...)
		blendScorer(&copy, contractv1.SearchHybrid, scores, 0.5)
	}
}
