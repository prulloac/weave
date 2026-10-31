package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"sort"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine/search"
)

// KeywordHits answers a keyword query from metadata only.
func (s *Store) KeywordHits(
	ctx context.Context,
	root string,
	query string,
	topK int,
) ([]contractv1.KeywordHit, error) {
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
		return []contractv1.KeywordHit{}, nil
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, title, file_path, doc_type, keywords_json
		FROM documents
		WHERE bundle_root = ? AND privacy_policy != 'encrypt'`, root)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	hits := make([]contractv1.KeywordHit, 0)
	for rows.Next() {
		var id, title, path, keywordsJSON string
		var docType sql.NullString
		if err := rows.Scan(&id, &title, &path, &docType, &keywordsJSON); err != nil {
			return nil, err
		}
		var keywords []string
		if err := json.Unmarshal([]byte(keywordsJSON), &keywords); err != nil {
			return nil, err
		}
		hit, ok := keywordHit(queryTokens, id, title, path, docType.String, keywords)
		if !ok {
			continue
		}
		hits = append(hits, hit)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Score != hits[j].Score {
			return hits[i].Score > hits[j].Score
		}
		return hits[i].ID < hits[j].ID
	})
	if len(hits) > topK {
		hits = hits[:topK]
	}
	return hits, nil
}

func keywordHit(
	queryTokens []string,
	id, title, path, docType string,
	keywords []string,
) (contractv1.KeywordHit, bool) {
	matched := make(map[string]bool)
	total := 0.0
	for _, queryToken := range queryTokens {
		best := 0.0
		bestKeyword := ""
		for _, keyword := range keywords {
			for _, token := range search.Tokenize(keyword) {
				score := search.KeywordTokenScore(queryToken, token)
				if score > best {
					best = score
					bestKeyword = keyword
				}
			}
		}
		if best == 0 {
			return contractv1.KeywordHit{}, false
		}
		total += best
		if bestKeyword != "" {
			matched[bestKeyword] = true
		}
	}
	matchedKeywords := make([]string, 0, len(matched))
	for keyword := range matched {
		matchedKeywords = append(matchedKeywords, keyword)
	}
	sort.Strings(matchedKeywords)
	return contractv1.KeywordHit{
		ID:              id,
		Title:           title,
		Path:            path,
		Type:            docType,
		Keywords:        keywords,
		MatchedKeywords: matchedKeywords,
		Score:           search.RoundScore(total / float64(len(queryTokens))),
	}, true
}
