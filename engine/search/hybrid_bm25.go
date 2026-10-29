package search

import (
	"sort"

	contractv1 "github.com/prulloac/weave/contract/v1"
)

// HybridBM25 combines normalized BM25 and fuzzy scores.
func HybridBM25(
	lexical []contractv1.LexicalHit,
	fuzzy []contractv1.FuzzyHit,
	topK int,
) []contractv1.HybridHit {
	if topK <= 0 {
		topK = defaultTopK
	}
	if topK > maxTopK {
		topK = maxTopK
	}

	lexicalByID := make(map[string]contractv1.LexicalHit, len(lexical))
	fuzzyByID := make(map[string]contractv1.FuzzyHit, len(fuzzy))
	ids := make(map[string]bool)
	for _, hit := range lexical {
		lexicalByID[hit.ID] = hit
		ids[hit.ID] = true
	}
	for _, hit := range fuzzy {
		fuzzyByID[hit.ID] = hit
		ids[hit.ID] = true
	}

	results := make([]contractv1.HybridHit, 0, len(ids))
	for id := range ids {
		lexicalHit, hasLexical := lexicalByID[id]
		fuzzyHit, hasFuzzy := fuzzyByID[id]
		lexicalScore := 0.0
		fuzzyScore := 0.0
		title := ""
		path := ""
		docType := ""
		if hasLexical {
			lexicalScore = lexicalHit.Score
			title = lexicalHit.Title
			path = lexicalHit.Path
			docType = lexicalHit.Type
		}
		if hasFuzzy {
			fuzzyScore = fuzzyHit.Score
			if title == "" {
				title = fuzzyHit.Title
				path = fuzzyHit.Path
				docType = fuzzyHit.Type
			}
		}

		hit := contractv1.HybridHit{
			ID:              id,
			Title:           title,
			Path:            path,
			Type:            docType,
			KeywordScore:    0,
			LexicalScore:    lexicalScore,
			FuzzyScore:      fuzzyScore,
			TotalScore:      RoundScore(keywordWeight*lexicalScore + fuzzyWeight*fuzzyScore),
			MatchedKeywords: []string{},
			MatchedFields:   []contractv1.SearchField{},
		}
		if hasFuzzy {
			hit.MatchedFields = fuzzyHit.MatchedFields
			hit.Snippet = fuzzyHit.Snippet
		}
		results = append(results, hit)
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].TotalScore != results[j].TotalScore {
			return results[i].TotalScore > results[j].TotalScore
		}
		if results[i].LexicalScore != results[j].LexicalScore {
			return results[i].LexicalScore > results[j].LexicalScore
		}
		if results[i].FuzzyScore != results[j].FuzzyScore {
			return results[i].FuzzyScore > results[j].FuzzyScore
		}
		return results[i].ID < results[j].ID
	})
	if len(results) > topK {
		results = results[:topK]
	}
	return results
}
