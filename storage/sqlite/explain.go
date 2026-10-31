package sqlite

import (
	"context"
	"sort"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine/search"
)

// ExplainFuzzy returns fuzzy evidence for one target from persistent postings.
func (s *Store) ExplainFuzzy(
	ctx context.Context,
	root string,
	query string,
	target string,
) (contractv1.ExplainFuzzy, error) {
	root, err := normalizeRoot(root)
	if err != nil {
		return contractv1.ExplainFuzzy{}, err
	}
	queryTokens := search.Tokenize(query)
	if len(queryTokens) == 0 {
		return contractv1.ExplainFuzzy{}, search.ErrEmptyQuery
	}
	documentIDs, err := s.searchDocumentIDs(ctx, root)
	if err != nil {
		return contractv1.ExplainFuzzy{}, err
	}
	documentIndex := sort.SearchStrings(documentIDs, target)
	if documentIndex >= len(documentIDs) || documentIDs[documentIndex] != target {
		return contractv1.ExplainFuzzy{}, search.ErrConceptNotFound
	}
	postings, err := s.searchTokenPostings(ctx, root)
	if err != nil {
		return contractv1.ExplainFuzzy{}, err
	}
	if len(postings) == 0 {
		return contractv1.ExplainFuzzy{}, ErrSearchIndexUnavailable
	}

	fields := []contractv1.SearchField{
		contractv1.FieldTitle,
		contractv1.FieldDescription,
		contractv1.FieldBody,
	}
	fieldScores := make([]float64, len(fields))
	fieldTokens := make([][]string, len(fields))
	evidence := make([]contractv1.ExplainFuzzyToken, 0)
	targetIndex := int32(documentIndex)

	for fieldIndex, field := range fields {
		values := make([]float64, len(queryTokens))
		tokens := make([]string, len(queryTokens))
		for queryPosition, queryToken := range queryTokens {
			profile := search.NewTokenProfile(queryToken)
			bestSimilarity := 0.0
			bestToken := ""
			for token, posting := range postings[string(field)] {
				if !containsPosting(posting, targetIndex) {
					continue
				}
				similarity := profile.Similarity(token)
				if similarity < search.FuzzyThreshold || similarity <= bestSimilarity {
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
					Similarity:   search.RoundScore(bestSimilarity),
				})
			}
		}
		fieldTokens[fieldIndex] = tokens
		score := 0.0
		for _, value := range values {
			score += value
		}
		fieldScores[fieldIndex] = score / float64(len(queryTokens))
	}

	explainFields := make([]contractv1.ExplainFuzzyField, 0, len(fields))
	matchedFields := make([]contractv1.SearchField, 0, len(fields))
	weighted := 0.0
	for index, field := range fields {
		contribution := fieldScores[index] * search.FuzzyFieldWeight(field) / search.FuzzyFieldWeightSum
		weighted += contribution
		explainFields = append(explainFields, contractv1.ExplainFuzzyField{
			Field:        field,
			Score:        search.RoundScore(fieldScores[index]),
			Weight:       search.FuzzyFieldWeight(field),
			Contribution: search.RoundScore(contribution),
		})
		if contribution > search.FuzzyFieldThreshold {
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

	var snippet *string
	if fieldScores[2] > 0 {
		body, err := s.searchBodyText(ctx, root, target)
		if err != nil {
			return contractv1.ExplainFuzzy{}, err
		}
		snippet = search.Snippet(body, fieldTokens[2])
	}
	return contractv1.ExplainFuzzy{
		Score:         search.RoundScore(weighted),
		MatchedFields: matchedFields,
		Fields:        explainFields,
		Tokens:        evidence,
		Snippet:       snippet,
	}, nil
}

func containsPosting(postings []int32, target int32) bool {
	index := sort.Search(len(postings), func(i int) bool {
		return postings[i] >= target
	})
	return index < len(postings) && postings[index] == target
}
