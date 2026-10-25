package search

import (
	"math"
	"sort"
	"strings"

	contractv1 "github.com/prulloac/weave/contract/v1"
)

const (
	bm25K1 = 1.2
	bm25B  = 0.75
)

var fieldWeights = map[contractv1.SearchField]float64{
	contractv1.FieldTitle:       10,
	contractv1.FieldTags:        6,
	contractv1.FieldDescription: 4,
	contractv1.FieldBody:        3,
	contractv1.FieldPath:        2,
	contractv1.FieldType:        2,
	contractv1.FieldFrontmatter: 1,
}

// Options controls a search query.
type Options struct {
	Limit    int
	Fields   []contractv1.SearchField
	MinScore float64
}

// Query ranks documents with the contract BM25 semantics.
func Query(index *Index, query string, options Options) []contractv1.SearchResult {
	queryTokens := Tokenize(query)
	if len(queryTokens) == 0 {
		return []contractv1.SearchResult{}
	}

	fields := Fields
	if len(options.Fields) > 0 {
		allowed := make(map[contractv1.SearchField]bool, len(options.Fields))
		for _, field := range options.Fields {
			allowed[field] = true
		}
		fields = make([]contractv1.SearchField, 0, len(Fields))
		for _, field := range Fields {
			if allowed[field] {
				fields = append(fields, field)
			}
		}
	}

	limit := options.Limit
	if limit <= 0 {
		limit = 20
	}
	totalDocs := len(index.Documents)

	candidates := make(map[int]bool, totalDocs)
	for position := 0; position < totalDocs; position++ {
		candidates[position] = true
	}

	fieldMatches := make(map[contractv1.SearchField]map[string][]string, len(fields))
	for _, field := range fields {
		fieldMatches[field] = make(map[string][]string, len(queryTokens))
	}

	for _, queryToken := range queryTokens {
		matching := make(map[int]bool)
		for _, field := range fields {
			tokens := make([]string, 0)
			for indexToken, docSet := range index.Postings[field] {
				if !strings.HasPrefix(indexToken, queryToken) {
					continue
				}
				tokens = append(tokens, indexToken)
				for docIndex := range docSet {
					matching[docIndex] = true
				}
			}
			fieldMatches[field][queryToken] = tokens
		}
		for candidate := range candidates {
			if !matching[candidate] {
				delete(candidates, candidate)
			}
		}
		if len(candidates) == 0 {
			return []contractv1.SearchResult{}
		}
	}

	results := make([]contractv1.SearchResult, 0, len(candidates))
	for docIndex := range candidates {
		score := 0.0
		bestField := contractv1.FieldBody
		bestWeighted := 0.0
		bodyTerms := make([]string, 0)
		bodyContributed := false

		for _, field := range fields {
			docLength := index.Lengths[docIndex][field]
			averageLength := index.AverageLength[field]
			fieldScore := 0.0

			for _, queryToken := range queryTokens {
				bestTermScore := 0.0
				for _, indexToken := range fieldMatches[field][queryToken] {
					docSet := index.Postings[field][indexToken]
					if !docSet[docIndex] {
						continue
					}
					frequency := index.TF[docIndex][field][indexToken]
					if frequency <= 0 || docLength <= 0 || averageLength <= 0 {
						continue
					}
					denominator := float64(frequency) + bm25K1*(1-bm25B+bm25B*(float64(docLength)/averageLength))
					termScore := idf(totalDocs, len(docSet)) * (float64(frequency) * (bm25K1 + 1) / denominator)
					if termScore > bestTermScore {
						bestTermScore = termScore
					}
					if field == contractv1.FieldBody {
						bodyTerms = append(bodyTerms, indexToken)
					}
				}
				fieldScore += bestTermScore
			}

			weighted := fieldScore * fieldWeights[field]
			score += weighted
			if weighted > bestWeighted {
				bestWeighted = weighted
				bestField = field
				bodyContributed = field == contractv1.FieldBody
			} else if weighted > 0 && field == contractv1.FieldBody {
				bodyContributed = true
			}
		}

		score += float64(index.DegreeBoost[index.Documents[docIndex].ID])
		if score < options.MinScore {
			continue
		}

		document := index.Documents[docIndex]
		result := contractv1.SearchResult{
			ID:        document.ID,
			Title:     document.Title,
			Path:      document.Path,
			Type:      document.Type,
			Score:     score,
			BestField: bestField,
		}
		if bodyContributed && len(bodyTerms) > 0 {
			result.Snippet = extractSnippet(document.BodyRaw, bodyTerms)
		}
		results = append(results, result)
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		return results[i].ID < results[j].ID
	})

	if len(results) > limit {
		results = results[:limit]
	}
	return results
}

func idf(totalDocs, documentFrequency int) float64 {
	return math.Log((float64(totalDocs-documentFrequency)+0.5)/(float64(documentFrequency)+0.5) + 1)
}

func extractSnippet(body string, terms []string) *string {
	lowerBody := strings.ToLower(body)
	position := -1
	for _, term := range terms {
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
	var snippet string
	if start > 0 {
		snippet = "…" + body[start+1:end]
	} else {
		snippet = body[start:end]
	}
	if end < len(body) {
		snippet += "…"
	}
	return &snippet
}
