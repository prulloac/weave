package search_test

import (
	"errors"
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine/search"
)

func filterBundle() *contractv1.Bundle {
	stable := contractv1.StatusStable
	draft := contractv1.StatusDraft
	return &contractv1.Bundle{Concepts: []contractv1.OkfConcept{
		{ID: "a", Path: "a.md", Type: "Concept", Status: &stable, Tags: []string{"storage"}, Keywords: []string{"alpha", "storage"}, Body: "# A\n\nalpha storage body\n"},
		{ID: "b", Path: "b.md", Type: "Concept", Status: &draft, Tags: []string{"storage"}, Keywords: []string{"alpha"}, Body: "# B\n\nalpha body\n"},
		{ID: "c", Path: "c.md", Type: "Table", Status: &stable, Tags: []string{"data"}, Keywords: []string{"alpha", "storage"}, Body: "# C\n\nalpha storage body\n"},
		{ID: "d", Path: "d.md", Type: "Concept", Status: &stable, Tags: []string{"data"}, Body: "# D\n\nalpha body\n"},
	}}
}

func filterIDs(hits []contractv1.KeywordHit) []string {
	ids := make([]string, 0, len(hits))
	for _, hit := range hits {
		ids = append(ids, hit.ID)
	}
	return ids
}

func TestSearchFiltersByType(t *testing.T) {
	index := search.Build(filterBundle(), nil)
	response, err := search.SearchWithFilters(index, "alpha", contractv1.SearchKeyword, 10,
		[]contractv1.SearchFilter{{Field: "type", Value: "Concept"}})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a", "b"}
	got := filterIDs(response.Keyword)
	if len(got) != len(want) {
		t.Fatalf("hits = %v, want %v", got, want)
	}
	for position := range want {
		if got[position] != want[position] {
			t.Fatalf("hits = %v, want %v", got, want)
		}
	}
}

func TestSearchFiltersCombineWithAnd(t *testing.T) {
	index := search.Build(filterBundle(), nil)
	response, err := search.SearchWithFilters(index, "alpha", contractv1.SearchKeyword, 10,
		[]contractv1.SearchFilter{
			{Field: "type", Value: "Concept"},
			{Field: "status", Value: "stable"},
		})
	if err != nil {
		t.Fatal(err)
	}
	if got := filterIDs(response.Keyword); len(got) != 1 || got[0] != "a" {
		t.Fatalf("hits = %v, want [a]", got)
	}
}

func TestSearchFilterKeywordIsCaseInsensitive(t *testing.T) {
	index := search.Build(filterBundle(), nil)
	response, err := search.SearchWithFilters(index, "alpha", contractv1.SearchKeyword, 10,
		[]contractv1.SearchFilter{{Field: "keyword", Value: "STORAGE"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := filterIDs(response.Keyword); len(got) != 2 || got[0] != "a" || got[1] != "c" {
		t.Fatalf("hits = %v, want [a c]", got)
	}
}

func TestSearchFilterTag(t *testing.T) {
	index := search.Build(filterBundle(), nil)
	response, err := search.SearchWithFilters(index, "alpha", contractv1.SearchKeyword, 10,
		[]contractv1.SearchFilter{{Field: "tag", Value: "storage"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := filterIDs(response.Keyword); len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("hits = %v, want [a b]", got)
	}
}

func TestSearchFiltersBeforeTopK(t *testing.T) {
	index := search.Build(filterBundle(), nil)
	response, err := search.SearchWithFilters(index, "alpha", contractv1.SearchKeyword, 1,
		[]contractv1.SearchFilter{{Field: "type", Value: "Concept"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Keyword) != 1 || response.Keyword[0].Type != "Concept" {
		t.Fatalf("top-k applied before filtering: %+v", response.Keyword)
	}
}

func TestSearchFiltersKeepScoresAndOrder(t *testing.T) {
	index := search.Build(filterBundle(), nil)
	unfiltered, err := search.Search(index, "alpha", contractv1.SearchKeyword, 10)
	if err != nil {
		t.Fatal(err)
	}
	filtered, err := search.SearchWithFilters(index, "alpha", contractv1.SearchKeyword, 10,
		[]contractv1.SearchFilter{{Field: "type", Value: "Concept"}})
	if err != nil {
		t.Fatal(err)
	}
	scores := make(map[string]float64, len(unfiltered.Keyword))
	for _, hit := range unfiltered.Keyword {
		scores[hit.ID] = hit.Score
	}
	for _, hit := range filtered.Keyword {
		if hit.Score != scores[hit.ID] {
			t.Fatalf("score changed for %s: %v != %v", hit.ID, hit.Score, scores[hit.ID])
		}
	}
}

func TestSearchFilterUnknownField(t *testing.T) {
	index := search.Build(filterBundle(), nil)
	_, err := search.SearchWithFilters(index, "alpha", contractv1.SearchKeyword, 10,
		[]contractv1.SearchFilter{{Field: "colour", Value: "red"}})
	if !errors.Is(err, search.ErrInvalidFilter) {
		t.Fatalf("expected ErrInvalidFilter, got %v", err)
	}
}

func TestSearchFilterEmptyValue(t *testing.T) {
	index := search.Build(filterBundle(), nil)
	_, err := search.SearchWithFilters(index, "alpha", contractv1.SearchKeyword, 10,
		[]contractv1.SearchFilter{{Field: "type", Value: "  "}})
	if !errors.Is(err, search.ErrInvalidFilter) {
		t.Fatalf("expected ErrInvalidFilter, got %v", err)
	}
}
