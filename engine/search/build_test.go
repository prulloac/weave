package search_test

import (
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine/search"
)

func TestBuildKeywordsSkipsBodyIndex(t *testing.T) {
	title := "A"
	bundle := &contractv1.Bundle{
		Concepts: []contractv1.OkfConcept{{
			ID: "a", Path: "a.md", Type: "Concept", Title: &title,
			Keywords: []string{"alpha"}, Body: "# A\n\nalpha body term\n",
		}},
	}
	index := search.BuildKeywords(bundle)
	if len(index.KeywordPostings) == 0 {
		t.Fatal("keyword postings missing")
	}
	if index.TokenPostings != nil || index.TrigramIndex != nil {
		t.Fatal("keyword-only index built body structures")
	}
	response, err := search.Search(index, "alpha", contractv1.SearchKeyword, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Keyword) != 1 || response.Keyword[0].ID != "a" {
		t.Fatalf("unexpected keyword response: %+v", response.Keyword)
	}
}

func TestBuildWithoutTrigrams(t *testing.T) {
	title := "A"
	bundle := &contractv1.Bundle{
		Concepts: []contractv1.OkfConcept{{
			ID: "a", Path: "a.md", Type: "Concept", Title: &title,
			Keywords: []string{"alpha"}, Body: "# A\n\nalpha body term\n",
		}},
	}
	index := search.BuildWithOptions(bundle, nil, search.BuildOptions{Trigrams: false})
	if len(index.TokenPostings[contractv1.FieldBody]) == 0 {
		t.Fatal("token postings missing")
	}
	if len(index.TrigramIndex[contractv1.FieldBody]) != 0 {
		t.Fatal("trigram index should be empty")
	}
}
