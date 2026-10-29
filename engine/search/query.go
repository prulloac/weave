package search

import (
	"errors"
	"sort"
	"strings"
	"unicode/utf8"

	contractv1 "github.com/prulloac/weave/contract/v1"
)

const (
	defaultTopK    = 10
	maxTopK        = 100
	fuzzyThreshold = 0.55
	fieldThreshold = 0.05
	keywordExact   = 1.0
	keywordPrefix  = 0.75
	keywordWeight  = 0.50
	fuzzyWeight    = 0.50
	fieldWeightSum = 1.90
)

var (
	ErrInvalidTopK = errors.New("top_k must be between 1 and 100")
	ErrInvalidMode = errors.New("unknown search mode")
)

var fuzzyWeights = map[contractv1.SearchField]float64{
	contractv1.FieldTitle:       1.00,
	contractv1.FieldDescription: 0.60,
	contractv1.FieldBody:        0.30,
}

// Search runs the keyword, fuzzy and hybrid dimensions.
func Search(
	index *Index,
	query string,
	mode contractv1.SearchMode,
	topK int,
) (contractv1.SearchResponse, error) {
	if mode == "" {
		mode = contractv1.SearchHybrid
	}
	switch mode {
	case contractv1.SearchKeyword, contractv1.SearchFuzzy, contractv1.SearchHybrid:
	default:
		return contractv1.SearchResponse{}, ErrInvalidMode
	}

	if topK == 0 {
		topK = defaultTopK
	}
	if topK < 1 || topK > maxTopK {
		return contractv1.SearchResponse{}, ErrInvalidTopK
	}

	response := contractv1.SearchResponse{
		Query:   query,
		Mode:    mode,
		TopK:    topK,
		Keyword: []contractv1.KeywordHit{},
		Fuzzy:   []contractv1.FuzzyHit{},
		Hybrid:  []contractv1.HybridHit{},
	}
	tokens := Tokenize(query)
	if len(tokens) == 0 {
		return response, nil
	}

	keywordHits := keywordSearch(index, tokens, topK)
	fuzzyHits := fuzzySearch(index, tokens, topK)
	response.Keyword = keywordHits
	response.Fuzzy = fuzzyHits
	response.Hybrid = hybridSearch(index, keywordHits, fuzzyHits, topK)
	return response, nil
}

func keywordSearch(index *Index, queryTokens []string, topK int) []contractv1.KeywordHit {
	scores := make([]map[int]float64, len(queryTokens))
	origins := make([]map[int]string, len(queryTokens))
	candidates := make(map[int]bool)

	for queryPosition, queryToken := range queryTokens {
		scores[queryPosition] = make(map[int]float64)
		origins[queryPosition] = make(map[int]string)

		for token, postings := range index.KeywordPostings {
			score, _ := keywordTokenScore(queryToken, token)
			if score == 0 {
				continue
			}

			for documentIndex := range postings {
				if score > scores[queryPosition][documentIndex] {
					scores[queryPosition][documentIndex] = score
					origins[queryPosition][documentIndex] = index.Documents[documentIndex].KeywordOrigin[token]
				}
			}
		}

		if queryPosition == 0 {
			for documentIndex := range scores[queryPosition] {
				candidates[documentIndex] = true
			}
		} else {
			for documentIndex := range candidates {
				if _, exists := scores[queryPosition][documentIndex]; !exists {
					delete(candidates, documentIndex)
				}
			}
		}
		if len(candidates) == 0 {
			return []contractv1.KeywordHit{}
		}
	}

	results := make([]contractv1.KeywordHit, 0, len(candidates))
	for documentIndex := range candidates {
		total := 0.0
		matched := make(map[string]bool)
		for queryPosition := range queryTokens {
			total += scores[queryPosition][documentIndex]
			if origin := origins[queryPosition][documentIndex]; origin != "" {
				matched[origin] = true
			}
		}
		document := index.Documents[documentIndex]
		results = append(results, contractv1.KeywordHit{
			ID:              document.ID,
			Title:           document.Title,
			Path:            document.Path,
			Type:            document.Type,
			Keywords:        document.Keywords,
			MatchedKeywords: sortedKeys(matched),
			Score:           RoundScore(total / float64(len(queryTokens))),
		})
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		return results[i].ID < results[j].ID
	})
	if len(results) > topK {
		results = results[:topK]
	}
	return results
}

func fuzzySearch(index *Index, queryTokens []string, topK int) []contractv1.FuzzyHit {
	fieldScores := make([]map[int][]float64, len(Fields))
	bodyTokens := make(map[int][]string)
	for position := range Fields {
		fieldScores[position] = make(map[int][]float64)
	}

	for fieldPosition, field := range Fields {
		for queryPosition, queryToken := range queryTokens {
			candidates := fuzzyCandidates(index, field, queryToken)
			for token := range candidates {
				similarity := tokenSimilarity(queryToken, token)
				if similarity < fuzzyThreshold {
					continue
				}
				for documentIndex := range index.TokenPostings[field][token] {
					values := fieldScores[fieldPosition][documentIndex]
					if len(values) == 0 {
						values = make([]float64, len(queryTokens))
					}
					if similarity > values[queryPosition] {
						values[queryPosition] = similarity
						if field == contractv1.FieldBody {
							bodyValues := bodyTokens[documentIndex]
							if len(bodyValues) == 0 {
								bodyValues = make([]string, len(queryTokens))
							}
							bodyValues[queryPosition] = token
							bodyTokens[documentIndex] = bodyValues
						}
					}
					fieldScores[fieldPosition][documentIndex] = values
				}
			}
		}
	}

	results := make([]contractv1.FuzzyHit, 0)
	seen := make(map[int]bool)
	for fieldPosition := range Fields {
		for documentIndex := range fieldScores[fieldPosition] {
			seen[documentIndex] = true
		}
	}

	for documentIndex := range seen {
		weighted := 0.0
		fieldValues := make(map[contractv1.SearchField]float64, len(Fields))
		for fieldPosition, field := range Fields {
			values := fieldScores[fieldPosition][documentIndex]
			fieldScore := 0.0
			if len(values) > 0 {
				for _, value := range values {
					fieldScore += value
				}
				fieldScore /= float64(len(queryTokens))
			}
			fieldValues[field] = fieldScore
			weighted += fieldScore * fuzzyWeights[field]
		}
		weighted /= fieldWeightSum
		if weighted <= 0 {
			continue
		}

		matchedFields := make([]contractv1.SearchField, 0, len(Fields))
		for _, field := range Fields {
			if fieldValues[field]*fuzzyWeights[field]/fieldWeightSum > fieldThreshold {
				matchedFields = append(matchedFields, field)
			}
		}
		sort.Slice(matchedFields, func(i, j int) bool {
			return matchedFields[i] < matchedFields[j]
		})

		document := index.Documents[documentIndex]
		hit := contractv1.FuzzyHit{
			ID:            document.ID,
			Title:         document.Title,
			Path:          document.Path,
			Type:          document.Type,
			Score:         RoundScore(weighted),
			MatchedFields: matchedFields,
		}
		if fieldValues[contractv1.FieldBody] > 0 {
			hit.Snippet = snippet(document.BodyText, bodyTokens[documentIndex])
		}
		results = append(results, hit)
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		return results[i].ID < results[j].ID
	})
	if len(results) > topK {
		results = results[:topK]
	}
	return results
}

func fuzzyCandidates(index *Index, field contractv1.SearchField, queryToken string) map[string]bool {
	candidates := make(map[string]bool)
	if utf8.RuneCountInString(queryToken) < 3 {
		for token := range index.TokenPostings[field] {
			if token == queryToken || strings.HasPrefix(token, queryToken) {
				candidates[token] = true
			}
		}
		return candidates
	}

	if _, ok := index.TokenPostings[field][queryToken]; ok {
		candidates[queryToken] = true
	}
	for _, trigram := range trigrams(queryToken) {
		for token := range index.TrigramIndex[field][trigram] {
			candidates[token] = true
		}
	}
	return candidates
}

func hybridSearch(
	index *Index,
	keywordHits []contractv1.KeywordHit,
	fuzzyHits []contractv1.FuzzyHit,
	topK int,
) []contractv1.HybridHit {
	keywordByID := make(map[string]contractv1.KeywordHit, len(keywordHits))
	fuzzyByID := make(map[string]contractv1.FuzzyHit, len(fuzzyHits))
	ids := make(map[string]bool)

	for _, hit := range keywordHits {
		keywordByID[hit.ID] = hit
		ids[hit.ID] = true
	}
	for _, hit := range fuzzyHits {
		fuzzyByID[hit.ID] = hit
		ids[hit.ID] = true
	}

	documentByID := make(map[string]Document, len(index.Documents))
	for _, document := range index.Documents {
		documentByID[document.ID] = document
	}

	results := make([]contractv1.HybridHit, 0, len(ids))
	for id := range ids {
		keywordHit, hasKeyword := keywordByID[id]
		fuzzyHit, hasFuzzy := fuzzyByID[id]
		keywordScore := 0.0
		fuzzyScore := 0.0
		if hasKeyword {
			keywordScore = keywordHit.Score
		}
		if hasFuzzy {
			fuzzyScore = fuzzyHit.Score
		}
		document := documentByID[id]

		hit := contractv1.HybridHit{
			ID:              id,
			Title:           document.Title,
			Path:            document.Path,
			Type:            document.Type,
			KeywordScore:    keywordScore,
			FuzzyScore:      fuzzyScore,
			TotalScore:      RoundScore(keywordWeight*keywordScore + fuzzyWeight*fuzzyScore),
			MatchedKeywords: []string{},
			MatchedFields:   []contractv1.SearchField{},
		}
		if hasKeyword {
			hit.MatchedKeywords = keywordHit.MatchedKeywords
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
		if results[i].FuzzyScore != results[j].FuzzyScore {
			return results[i].FuzzyScore > results[j].FuzzyScore
		}
		if results[i].KeywordScore != results[j].KeywordScore {
			return results[i].KeywordScore > results[j].KeywordScore
		}
		return results[i].ID < results[j].ID
	})
	if len(results) > topK {
		results = results[:topK]
	}
	return results
}

func snippet(body string, terms []string) *string {
	lowerBody := strings.ToLower(body)
	position := -1
	for _, term := range terms {
		if term == "" {
			continue
		}
		found := strings.Index(lowerBody, strings.ToLower(term))
		if found != -1 && (position == -1 || found < position) {
			position = found
		}
	}
	if position == -1 {
		return nil
	}

	start := max(0, position-40)
	end := min(len(body), start+120)
	var result string
	if start > 0 {
		result = "…" + body[start+1:end]
	} else {
		result = body[start:end]
	}
	if end < len(body) {
		result += "…"
	}
	return &result
}

func sortedKeys(values map[string]bool) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
