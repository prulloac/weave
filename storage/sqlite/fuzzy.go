package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strings"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine/compression"
	"github.com/prulloac/weave/engine/search"
)

// ErrSearchIndexUnavailable asks the caller to fall back to the in-memory path.
var ErrSearchIndexUnavailable = errors.New("persistent search index unavailable")

type searchDocument struct {
	ID    string
	Title string
	Path  string
	Type  string
}

// FuzzyHits answers a fuzzy query from persistent token postings.
func (s *Store) FuzzyHits(
	ctx context.Context,
	root string,
	query string,
	topK int,
) ([]contractv1.FuzzyHit, error) {
	root, err := normalizeRoot(root)
	if err != nil {
		return nil, err
	}
	if topK <= 0 {
		topK = 10
	}
	if topK > 100 {
		topK = 100
	}
	queryTokens := search.Tokenize(query)
	if len(queryTokens) == 0 {
		return []contractv1.FuzzyHit{}, nil
	}

	documentIDs, err := s.searchDocumentIDs(ctx, root)
	if err != nil {
		return nil, err
	}
	vocabulary, err := s.searchVocabulary(ctx, root)
	if err != nil {
		return nil, err
	}
	if len(vocabulary) == 0 {
		return nil, ErrSearchIndexUnavailable
	}

	fields := []contractv1.SearchField{
		contractv1.FieldTitle,
		contractv1.FieldDescription,
		contractv1.FieldBody,
	}
	candidates := fuzzyCandidateScores(vocabulary, queryTokens, fields)
	postings, err := s.searchPostingsFor(ctx, root, candidates)
	if err != nil {
		return nil, err
	}

	fieldScores := make([]map[int32][]float64, len(fields))
	for index := range fieldScores {
		fieldScores[index] = make(map[int32][]float64)
	}
	bodyTokens := make(map[int32][]string)
	for fieldIndex, field := range fields {
		for token, values := range candidates[string(field)] {
			posting := postings[string(field)][token]
			for queryPosition, similarity := range values {
				if similarity == 0 {
					continue
				}
				for _, documentIndex := range posting {
					current := fieldScores[fieldIndex][documentIndex]
					if len(current) == 0 {
						current = make([]float64, len(queryTokens))
					}
					if similarity > current[queryPosition] {
						current[queryPosition] = similarity
						if field == contractv1.FieldBody {
							bodyValues := bodyTokens[documentIndex]
							if len(bodyValues) == 0 {
								bodyValues = make([]string, len(queryTokens))
							}
							bodyValues[queryPosition] = token
							bodyTokens[documentIndex] = bodyValues
						}
					}
					fieldScores[fieldIndex][documentIndex] = current
				}
			}
		}
	}

	seen := make(map[int32]bool)
	for fieldIndex := range fields {
		for documentIndex := range fieldScores[fieldIndex] {
			seen[documentIndex] = true
		}
	}
	results := make([]contractv1.FuzzyHit, 0, len(seen))
	bodyTerms := make(map[string][]string)
	for documentIndex := range seen {
		if int(documentIndex) >= len(documentIDs) {
			continue
		}
		weighted := 0.0
		fieldValues := make([]float64, len(fields))
		for fieldIndex, field := range fields {
			values := fieldScores[fieldIndex][documentIndex]
			fieldScore := 0.0
			if len(values) > 0 {
				for _, value := range values {
					fieldScore += value
				}
				fieldScore /= float64(len(queryTokens))
			}
			fieldValues[fieldIndex] = fieldScore
			weighted += fieldScore * search.FuzzyFieldWeight(field)
		}
		weighted /= search.FuzzyFieldWeightSum
		if weighted <= 0 {
			continue
		}

		matchedFields := make([]contractv1.SearchField, 0, len(fields))
		for fieldIndex, field := range fields {
			if fieldValues[fieldIndex]*search.FuzzyFieldWeight(field)/search.FuzzyFieldWeightSum > search.FuzzyFieldThreshold {
				matchedFields = append(matchedFields, field)
			}
		}
		sort.Slice(matchedFields, func(i, j int) bool {
			return matchedFields[i] < matchedFields[j]
		})

		id := documentIDs[documentIndex]
		hit := contractv1.FuzzyHit{
			ID:            id,
			Score:         search.RoundScore(weighted),
			MatchedFields: matchedFields,
		}
		if fieldValues[2] > 0 {
			bodyTerms[id] = bodyTokens[documentIndex]
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
	for index := range results {
		document, err := s.searchDocument(ctx, root, results[index].ID)
		if err != nil {
			return nil, err
		}
		results[index].Title = document.Title
		results[index].Path = document.Path
		results[index].Type = document.Type
		terms, ok := bodyTerms[results[index].ID]
		if !ok {
			continue
		}
		body, err := s.searchBodyText(ctx, root, results[index].ID)
		if err != nil {
			return nil, err
		}
		results[index].Snippet = search.Snippet(body, terms)
	}
	return results, nil
}

func (s *Store) searchDocumentIDs(ctx context.Context, root string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id
		FROM documents
		WHERE bundle_root = ? AND privacy_policy = 'none'
		ORDER BY id`, root)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *Store) searchDocument(ctx context.Context, root, id string) (searchDocument, error) {
	var document searchDocument
	var docType sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT id, title, file_path, doc_type
		FROM documents
		WHERE bundle_root = ? AND id = ?`, root, id).Scan(
		&document.ID, &document.Title, &document.Path, &docType)
	if err != nil {
		return searchDocument{}, err
	}
	document.Type = docType.String
	return document, nil
}

func fuzzyCandidateScores(
	vocabulary map[string][]string,
	queryTokens []string,
	fields []contractv1.SearchField,
) map[string]map[string][]float64 {
	candidates := make(map[string]map[string][]float64, len(fields))
	for _, field := range fields {
		candidates[string(field)] = make(map[string][]float64)
		for queryPosition, queryToken := range queryTokens {
			profile := search.NewTokenProfile(queryToken)
			for _, token := range vocabulary[string(field)] {
				similarity := profile.Similarity(token)
				if similarity < search.FuzzyThreshold {
					continue
				}
				values := candidates[string(field)][token]
				if values == nil {
					values = make([]float64, len(queryTokens))
				}
				if similarity > values[queryPosition] {
					values[queryPosition] = similarity
				}
				candidates[string(field)][token] = values
			}
		}
	}
	return candidates
}

func (s *Store) searchVocabulary(ctx context.Context, root string) (map[string][]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT field, token FROM search_vocabulary WHERE bundle_root = ?`, root)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	vocabulary := make(map[string][]string)
	for rows.Next() {
		var field, token string
		if err := rows.Scan(&field, &token); err != nil {
			return nil, err
		}
		vocabulary[field] = append(vocabulary[field], token)
	}
	return vocabulary, rows.Err()
}

func (s *Store) searchPostingsFor(
	ctx context.Context,
	root string,
	candidates map[string]map[string][]float64,
) (map[string]map[string][]int32, error) {
	postings := make(map[string]map[string][]int32)
	for field, tokens := range candidates {
		if len(tokens) == 0 {
			continue
		}
		names := make([]string, 0, len(tokens))
		args := make([]any, 0, len(tokens)+2)
		args = append(args, root, field)
		for token := range tokens {
			names = append(names, token)
			args = append(args, token)
		}
		query := "SELECT token, postings FROM search_tokens WHERE bundle_root = ? AND field = ? AND token IN (" +
			strings.TrimSuffix(strings.Repeat("?,", len(names)), ",") + ")"
		if len(names) > 400 {
			query = "SELECT token, postings FROM search_tokens WHERE bundle_root = ? AND field = ?"
			args = []any{root, field}
		}
		rows, err := s.db.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, err
		}
		if postings[field] == nil {
			postings[field] = make(map[string][]int32)
		}
		for rows.Next() {
			var token string
			var blob []byte
			if err := rows.Scan(&token, &blob); err != nil {
				rows.Close()
				return nil, err
			}
			postings[field][token] = decodePostings(blob)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
	}
	return postings, nil
}

func (s *Store) searchBodyText(ctx context.Context, root, id string) (string, error) {
	var blob []byte
	var codec string
	if err := s.db.QueryRowContext(ctx,
		"SELECT body_compressed, body_codec FROM documents WHERE bundle_root = ? AND id = ?",
		root, id).Scan(&blob, &codec); err != nil {
		return "", err
	}
	body, err := compression.Decode(compression.Codec(codec), blob)
	if err != nil {
		return "", nil
	}
	return search.StripMarkdown(string(body)), nil
}
