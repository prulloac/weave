package search

import (
	"errors"
	"sort"
	"strings"
	"unicode/utf8"

	contractv1 "github.com/prulloac/weave/contract/v1"
)

var (
	// ErrConceptNotFound is returned when an explain target is unknown.
	ErrConceptNotFound = errors.New("concept not found")
	// ErrEmptyQuery is returned when there is no query to explain.
	ErrEmptyQuery = errors.New("query is empty")
	// ErrNoResult is returned when no result exists to explain.
	ErrNoResult = errors.New("no result to explain")
)

// Explain returns the score breakdown for one search result. When target is
// empty, the rank-1 result in the selected mode is explained.
func Explain(
	index *Index,
	query string,
	mode contractv1.SearchMode,
	topK int,
	target string,
) (contractv1.ExplainResult, error) {
	if mode == "" {
		mode = contractv1.SearchHybrid
	}
	if topK == 0 {
		topK = defaultTopK
	}
	tokens := Tokenize(query)
	if len(tokens) == 0 {
		return contractv1.ExplainResult{}, ErrEmptyQuery
	}

	response, err := Search(index, query, mode, topK)
	if err != nil {
		return contractv1.ExplainResult{}, err
	}
	if target == "" {
		target = rankOneID(response, mode)
		if target == "" {
			return contractv1.ExplainResult{}, ErrNoResult
		}
	}

	documentIndex, ok := documentIndexByID(index, target)
	if !ok {
		return contractv1.ExplainResult{}, ErrConceptNotFound
	}

	keywordScore, matchedKeywords, keywordTokens := explainKeyword(index, tokens, documentIndex)
	fuzzyScore, matchedFields, fuzzyFields, fuzzyTokens, snippet := explainFuzzy(index, tokens, documentIndex)
	hybridTotal := RoundScore(keywordWeight*keywordScore + fuzzyWeight*fuzzyScore)

	return contractv1.ExplainResult{
		Query: query,
		Mode:  mode,
		TopK:  topK,
		ID:    target,
		Rank:  rankInResponse(response, mode, target),
		Found: true,
		Keyword: contractv1.ExplainKeyword{
			Score:           keywordScore,
			MatchedKeywords: matchedKeywords,
			Tokens:          keywordTokens,
		},
		Fuzzy: contractv1.ExplainFuzzy{
			Score:         fuzzyScore,
			MatchedFields: matchedFields,
			Fields:        fuzzyFields,
			Tokens:        fuzzyTokens,
			Snippet:       snippet,
		},
		Hybrid: contractv1.ExplainHybrid{
			KeywordWeight: keywordWeight,
			FuzzyWeight:   fuzzyWeight,
			Total:         hybridTotal,
		},
	}, nil
}

func documentIndexByID(index *Index, id string) (int, bool) {
	for position, document := range index.Documents {
		if document.ID == id {
			return position, true
		}
	}
	return 0, false
}

func rankOneID(response contractv1.SearchResponse, mode contractv1.SearchMode) string {
	switch mode {
	case contractv1.SearchKeyword:
		if len(response.Keyword) > 0 {
			return response.Keyword[0].ID
		}
	case contractv1.SearchFuzzy:
		if len(response.Fuzzy) > 0 {
			return response.Fuzzy[0].ID
		}
	default:
		if len(response.Hybrid) > 0 {
			return response.Hybrid[0].ID
		}
	}
	return ""
}

func rankInResponse(response contractv1.SearchResponse, mode contractv1.SearchMode, id string) int {
	switch mode {
	case contractv1.SearchKeyword:
		for index, hit := range response.Keyword {
			if hit.ID == id {
				return index + 1
			}
		}
	case contractv1.SearchFuzzy:
		for index, hit := range response.Fuzzy {
			if hit.ID == id {
				return index + 1
			}
		}
	default:
		for index, hit := range response.Hybrid {
			if hit.ID == id {
				return index + 1
			}
		}
	}
	return 0
}

func explainKeyword(
	index *Index,
	queryTokens []string,
	documentIndex int,
) (float64, []string, []contractv1.ExplainKeywordToken) {
	total := 0.0
	matched := make(map[string]bool)
	tokens := make([]contractv1.ExplainKeywordToken, 0, len(queryTokens))

	for _, queryToken := range queryTokens {
		bestScore := 0.0
		bestOrigin := ""
		bestMatch := contractv1.ExplainMatchNone

		for token, postings := range index.KeywordPostings {
			if !postings[documentIndex] {
				continue
			}
			score, match := keywordTokenScore(queryToken, token)
			if score <= bestScore {
				continue
			}
			bestScore = score
			bestOrigin = index.Documents[documentIndex].KeywordOrigin[token]
			bestMatch = match
		}
		if bestOrigin != "" {
			matched[bestOrigin] = true
		}
		total += bestScore
		tokens = append(tokens, contractv1.ExplainKeywordToken{
			QueryToken:     queryToken,
			MatchedKeyword: bestOrigin,
			Match:          bestMatch,
			Score:          RoundScore(bestScore),
		})
	}

	matchedKeywords := make([]string, 0, len(matched))
	for keyword := range matched {
		matchedKeywords = append(matchedKeywords, keyword)
	}
	sort.Strings(matchedKeywords)

	return RoundScore(total / float64(len(queryTokens))), matchedKeywords, tokens
}

func keywordTokenScore(queryToken, token string) (float64, contractv1.ExplainKeywordMatch) {
	switch {
	case token == queryToken:
		return keywordExact, contractv1.ExplainMatchExact
	case strings.HasPrefix(token, queryToken) &&
		utf8.RuneCountInString(token) > utf8.RuneCountInString(queryToken):
		return keywordPrefix, contractv1.ExplainMatchPrefix
	default:
		return 0, contractv1.ExplainMatchNone
	}
}

func explainFuzzy(
	index *Index,
	queryTokens []string,
	documentIndex int,
) (float64, []contractv1.SearchField, []contractv1.ExplainFuzzyField, []contractv1.ExplainFuzzyToken, *string) {
	fieldScores := make(map[contractv1.SearchField]float64, len(Fields))
	fieldTokens := make(map[contractv1.SearchField][]string, len(Fields))
	evidence := make([]contractv1.ExplainFuzzyToken, 0)

	for _, field := range Fields {
		values := make([]float64, len(queryTokens))
		tokens := make([]string, len(queryTokens))
		for queryPosition, queryToken := range queryTokens {
			candidates := fuzzyCandidates(index, field, queryToken)
			bestSimilarity := 0.0
			bestToken := ""
			for token := range candidates {
				if !index.TokenPostings[field][token][documentIndex] {
					continue
				}
				similarity := tokenSimilarity(queryToken, token)
				if similarity < fuzzyThreshold || similarity <= bestSimilarity {
					continue
				}
				bestSimilarity = similarity
				bestToken = token
			}
			values[queryPosition] = bestSimilarity
			tokens[queryPosition] = bestToken
			if bestSimilarity > 0 {
				evidence = append(evidence, contractv1.ExplainFuzzyToken{
					QueryToken:   queryToken,
					MatchedToken: bestToken,
					Field:        field,
					Similarity:   RoundScore(bestSimilarity),
				})
			}
		}
		fieldTokens[field] = tokens
		score := 0.0
		for _, value := range values {
			score += value
		}
		fieldScores[field] = score / float64(len(queryTokens))
	}

	fields := make([]contractv1.ExplainFuzzyField, 0, len(Fields))
	matchedFields := make([]contractv1.SearchField, 0, len(Fields))
	weighted := 0.0
	for _, field := range Fields {
		contribution := fieldScores[field] * fuzzyWeights[field] / fieldWeightSum
		weighted += contribution
		fields = append(fields, contractv1.ExplainFuzzyField{
			Field:        field,
			Score:        RoundScore(fieldScores[field]),
			Weight:       fuzzyWeights[field],
			Contribution: RoundScore(contribution),
		})
		if contribution > fieldThreshold {
			matchedFields = append(matchedFields, field)
		}
	}
	sort.Slice(matchedFields, func(i, j int) bool {
		return matchedFields[i] < matchedFields[j]
	})
	sort.Slice(evidence, func(i, j int) bool {
		if evidence[i].QueryToken != evidence[j].QueryToken {
			return evidence[i].QueryToken < evidence[j].QueryToken
		}
		if evidence[i].Field != evidence[j].Field {
			return evidence[i].Field < evidence[j].Field
		}
		return evidence[i].MatchedToken < evidence[j].MatchedToken
	})

	var snippetText *string
	if fieldScores[contractv1.FieldBody] > 0 {
		snippetText = snippet(index.Documents[documentIndex].BodyText, fieldTokens[contractv1.FieldBody])
	}

	return RoundScore(weighted), matchedFields, fields, evidence, snippetText
}
