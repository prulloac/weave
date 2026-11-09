package operations

import (
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine"
)

func TestBoundKeywords(t *testing.T) {
	keywords := make([]string, 0, 80)
	for index := 0; index < 80; index++ {
		keywords = append(keywords, "keyword")
	}
	bounded := boundKeywords(keywords)
	if len(bounded) != maxScorerKeywords {
		t.Fatalf("expected %d keywords, got %d", maxScorerKeywords, len(bounded))
	}
	bounded[0] = "changed"
	if keywords[0] == "changed" {
		t.Fatal("boundKeywords must copy the slice")
	}
}

func TestScorerCandidatesCarrySafeContext(t *testing.T) {
	description := "short description"
	title := "Alpha"
	eng := engine.FromBundle(&contractv1.Bundle{
		Root: "/bundle",
		Concepts: []contractv1.OkfConcept{{
			ID:          "alpha",
			Path:        "alpha.md",
			Type:        "Concept",
			Keywords:    []string{"alpha"},
			Title:       &title,
			Description: &description,
			Body:        "alpha body text",
		}},
	})
	response := contractv1.SearchResponse{
		Keyword: []contractv1.KeywordHit{{ID: "alpha", Path: "alpha.md", Title: "Alpha", Type: "Concept", Score: 1}},
	}
	candidates := scorerCandidates(response, contractv1.SearchKeyword, eng, "alpha")
	if len(candidates) != 1 {
		t.Fatalf("expected one candidate, got %d", len(candidates))
	}
	candidate := candidates[0]
	if candidate.ID != "alpha" || candidate.Description != description || candidate.Snippet == nil {
		t.Fatalf("unexpected candidate: %+v", candidate)
	}
	if candidate.Scores.Core != 1 || candidate.Scores.Keyword != 1 {
		t.Fatalf("unexpected scores: %+v", candidate.Scores)
	}
}
