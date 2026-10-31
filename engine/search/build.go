// Package search builds keyword, fuzzy and hybrid indexes and rankings.
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
	BodyText      string
	Type          string
	Path          string
	Keywords      []string
	KeywordOrigin map[string]string
}

// Index is the in-memory keyword and fuzzy index.
type Index struct {
	Documents       []Document
	KeywordPostings map[string]map[int]bool
	TokenPostings   map[contractv1.SearchField]map[string]map[int]bool
	TrigramIndex    map[contractv1.SearchField]map[string]map[string]bool
}

// BuildOptions controls which search structures are materialized.
type BuildOptions struct {
	KeywordOnly bool
	Trigrams    bool
}

// Build indexes a parsed bundle. The graph is accepted for future
// relationship-aware ranking but is not used by the v1 scoring formula.
func Build(bundle *contractv1.Bundle, graph *graph.Graph) *Index {
	return BuildWithOptions(bundle, graph, BuildOptions{Trigrams: true})
}

// BuildKeywords indexes only the frontmatter keyword dimension.
func BuildKeywords(bundle *contractv1.Bundle) *Index {
	return BuildWithOptions(bundle, nil, BuildOptions{KeywordOnly: true})
}

// BuildWithOptions indexes a parsed bundle with explicit structure choices.
func BuildWithOptions(bundle *contractv1.Bundle, _ *graph.Graph, options BuildOptions) *Index {
	index := &Index{
		Documents:       make([]Document, 0, len(bundle.Concepts)),
		KeywordPostings: make(map[string]map[int]bool),
	}
	if !options.KeywordOnly {
		index.TokenPostings = make(map[contractv1.SearchField]map[string]map[int]bool)
		index.TrigramIndex = make(map[contractv1.SearchField]map[string]map[string]bool)
		for _, field := range Fields {
			index.TokenPostings[field] = make(map[string]map[int]bool)
			index.TrigramIndex[field] = make(map[string]map[string]bool)
		}
	}
	trigramCache := make(map[string][]string)

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
			Type:          concept.Type,
			Path:          concept.Path,
			Keywords:      concept.Keywords,
			KeywordOrigin: make(map[string]string),
		}

		if !options.KeywordOnly {
			document.BodyText = StripMarkdown(concept.Body)
			fieldText := map[contractv1.SearchField]string{
				contractv1.FieldTitle:       title,
				contractv1.FieldDescription: description,
				contractv1.FieldBody:        document.BodyText,
			}
			for _, field := range Fields {
				tokens := uniqueTokens(Tokenize(fieldText[field]))
				for _, token := range tokens {
					posting := index.TokenPostings[field][token]
					if posting == nil {
						posting = make(map[int]bool)
						index.TokenPostings[field][token] = posting
					}
					posting[position] = true
					if !options.Trigrams {
						continue
					}
					cached, ok := trigramCache[token]
					if !ok {
						cached = trigrams(token)
						trigramCache[token] = cached
					}
					for _, trigram := range cached {
						tokenSet := index.TrigramIndex[field][trigram]
						if tokenSet == nil {
							tokenSet = make(map[string]bool)
							index.TrigramIndex[field][trigram] = tokenSet
						}
						tokenSet[token] = true
					}
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
