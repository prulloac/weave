package extensions_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/prulloac/weave/engine/extensions"
)

func BenchmarkScorerRequestMarshal100(b *testing.B) {
	snippet := strings.Repeat("snippet ", 64)
	candidates := make([]extensions.ScorerCandidate, 0, 100)
	for index := 0; index < 100; index++ {
		candidates = append(candidates, extensions.ScorerCandidate{
			ID:          "concept",
			Path:        "concept.md",
			Title:       "Concept",
			Description: "Short description",
			Type:        "Concept",
			Status:      "stable",
			Keywords:    []string{"alpha", "beta", "gamma", "delta"},
			Snippet:     &snippet,
			Scores: extensions.ScorerScores{
				Keyword: 0.8,
				Lexical: 0.7,
				Fuzzy:   0.6,
				Core:    0.75,
			},
		})
	}
	request := extensions.ScorerRequest{
		V:          "weave.cli/v1",
		Type:       "scorer_request",
		Query:      "alpha",
		Mode:       "hybrid",
		CandidateK: len(candidates),
		Candidates: candidates,
	}
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		if _, err := json.Marshal(request); err != nil {
			b.Fatal(err)
		}
	}
}
