package search

import (
	"sort"
	"strings"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine/graph"
)

var Fields = []contractv1.SearchField{
	contractv1.FieldTitle,
	contractv1.FieldDescription,
	contractv1.FieldBody,
}

// Document is the indexed projection of a concept.
type Document struct {
	ID            string
	Title         string
	Description   string
	Body          string
	BodyText      string
	Type          string
	Path          string
	Keywords      []string
	KeywordOrigin map[string]string
	FieldTokens   map[contractv1.SearchField][]string
}

// Index is the in-memory keyword and fuzzy index.
type Index struct {
	Documents       []Document
	KeywordPostings map[string]map[int]bool
	TokenPostings   map[contractv1.SearchField]map[string]map[int]bool
	TrigramIndex    map[contractv1.SearchField]map[string]map[string]bool
}

// Build indexes a parsed bundle. The graph is accepted for future
// relationship-aware ranking but is not used by the v1 scoring formula.
func Build(bundle *contractv1.Bundle, _ *graph.Graph) *Index {
	index := &Index{
		Documents:       make([]Document, 0, len(bundle.Concepts)),
		KeywordPostings: make(map[string]map[int]bool),
		TokenPostings:   make(map[contractv1.SearchField]map[string]map[int]bool),
		TrigramIndex:    make(map[contractv1.SearchField]map[string]map[string]bool),
	}
	for _, field := range Fields {
		index.TokenPostings[field] = make(map[string]map[int]bool)
		index.TrigramIndex[field] = make(map[string]map[string]bool)
	}

	concepts := make([]contractv1.OkfConcept, 0, len(bundle.Concepts))
	for _, concept := range bundle.Concepts {
		if concept.Privacy != nil && concept.Privacy.Policy == contractv1.PrivacyEncrypt {
			continue
		}
		concepts = append(concepts, concept)
	}
	sort.Slice(concepts, func(i, j int) bool {
		return concepts[i].ID < concepts[j].ID
	})

	for position, concept := range concepts {
		title := concept.ID
		if concept.Title != nil {
			title = *concept.Title
		}
		description := ""
		if concept.Description != nil {
			description = *concept.Description
		}

		document := Document{
			ID:            concept.ID,
			Title:         title,
			Description:   description,
			Body:          concept.Body,
			BodyText:      StripMarkdown(concept.Body),
			Type:          concept.Type,
			Path:          concept.Path,
			Keywords:      concept.Keywords,
			KeywordOrigin: make(map[string]string),
			FieldTokens:   make(map[contractv1.SearchField][]string),
		}

		fieldText := map[contractv1.SearchField]string{
			contractv1.FieldTitle:       title,
			contractv1.FieldDescription: description,
			contractv1.FieldBody:        document.BodyText,
		}
		for _, field := range Fields {
			tokens := uniqueTokens(Tokenize(fieldText[field]))
			document.FieldTokens[field] = tokens
			for _, token := range tokens {
				posting := index.TokenPostings[field][token]
				if posting == nil {
					posting = make(map[int]bool)
					index.TokenPostings[field][token] = posting
				}
				posting[position] = true
				for _, trigram := range trigrams(token) {
					tokenSet := index.TrigramIndex[field][trigram]
					if tokenSet == nil {
						tokenSet = make(map[string]bool)
						index.TrigramIndex[field][trigram] = tokenSet
					}
					tokenSet[token] = true
				}
			}
		}

		for _, keyword := range concept.Keywords {
			for _, token := range Tokenize(keyword) {
				posting := index.KeywordPostings[token]
				if posting == nil {
					posting = make(map[int]bool)
					index.KeywordPostings[token] = posting
				}
				posting[position] = true
				if _, exists := document.KeywordOrigin[token]; !exists {
					document.KeywordOrigin[token] = keyword
				}
			}
		}

		index.Documents = append(index.Documents, document)
	}

	return index
}

func uniqueTokens(tokens []string) []string {
	seen := make(map[string]bool, len(tokens))
	result := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if token == "" || seen[token] {
			continue
		}
		seen[token] = true
		result = append(result, token)
	}
	return result
}

func trigrams(token string) []string {
	padded := "#" + strings.ToLower(token) + "#"
	runes := []rune(padded)
	if len(runes) < 3 {
		return []string{padded}
	}
	result := make([]string, 0, len(runes)-2)
	for i := 0; i+3 <= len(runes); i++ {
		result = append(result, string(runes[i:i+3]))
	}
	return result
}
