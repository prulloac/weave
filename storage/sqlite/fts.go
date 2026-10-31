package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine/compression"
	"github.com/prulloac/weave/engine/privacy"
	"github.com/prulloac/weave/engine/search"
)

func populateFTS(ctx context.Context, tx *sql.Tx, root string) error {
	if _, err := tx.ExecContext(ctx, "DELETE FROM fts_documents WHERE bundle_root = ?", root); err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT id, title, keywords_json, description, body_compressed, body_codec, privacy_policy
		FROM documents WHERE bundle_root = ? ORDER BY id`, root)
	if err != nil {
		return err
	}
	type ftsRow struct {
		id, title, keywords, body, policy, codec string
		description                              sql.NullString
	}
	entries := make([]ftsRow, 0)
	for rows.Next() {
		var entry ftsRow
		if err := rows.Scan(&entry.id, &entry.title, &entry.keywords, &entry.description,
			&entry.body, &entry.codec, &entry.policy); err != nil {
			rows.Close()
			return err
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	for _, entry := range entries {
		if contractv1.PrivacyPolicy(entry.policy) == contractv1.PrivacyEncrypt {
			continue
		}
		decoded, err := compression.Decode(compression.Codec(entry.codec), []byte(entry.body))
		if err != nil {
			decoded = nil
		}
		var keywords []string
		if err := json.Unmarshal([]byte(entry.keywords), &keywords); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO fts_documents (bundle_root, id, title, keywords, description, body)
			VALUES (?, ?, ?, ?, ?, ?)`,
			root, entry.id, entry.title, strings.Join(keywords, " "),
			entry.description.String, search.StripMarkdown(string(decoded)))
		if err != nil {
			return err
		}
	}
	return nil
}

// SearchBM25 returns lexical hits for a bundle.
func (s *Store) SearchBM25(
	ctx context.Context,
	root string,
	query string,
	topK int,
) ([]contractv1.LexicalHit, error) {
	root, err := normalizeRoot(root)
	if err != nil {
		return nil, err
	}
	if topK == 0 {
		topK = 10
	}
	if topK < 1 || topK > 100 {
		return nil, search.ErrInvalidTopK
	}
	match := buildFTSQuery(query)
	aliases := privacy.ExpandQuery(query, s.privacyRefs(ctx, root), func(id string) ([]byte, error) {
		return privacy.ResolveKey(root, id)
	})
	if len(aliases) > 0 {
		if match == "" {
			match = strings.Join(aliases, " OR ")
		} else {
			match = "(" + match + ") OR (" + strings.Join(aliases, " OR ") + ")"
		}
	}
	if match == "" {
		return []contractv1.LexicalHit{}, nil
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT d.id, d.title, d.file_path, d.doc_type,
		       -bm25(fts_documents, 0.0, 0.0, 10.0, 8.0, 4.0, 1.0) AS raw
		FROM fts_documents
		JOIN documents d
		  ON d.bundle_root = fts_documents.bundle_root
		 AND d.id = fts_documents.id
		WHERE fts_documents.bundle_root = ?
		  AND fts_documents MATCH ?
		ORDER BY raw DESC, d.id ASC
		LIMIT ?`, root, match, topK)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	hits := make([]contractv1.LexicalHit, 0)
	maxScore := 0.0
	for rows.Next() {
		var hit contractv1.LexicalHit
		var docType sql.NullString
		if err := rows.Scan(&hit.ID, &hit.Title, &hit.Path, &docType, &hit.RawScore); err != nil {
			return nil, err
		}
		hit.Type = docType.String
		if hit.RawScore > maxScore {
			maxScore = hit.RawScore
		}
		hits = append(hits, hit)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if maxScore > 0 {
		for index := range hits {
			hits[index].Score = search.RoundScore(hits[index].RawScore / maxScore)
		}
	}
	return hits, nil
}

// BM25Explanation is one document's lexical evidence.
type BM25Explanation struct {
	Target  string
	Rank    int
	Lexical contractv1.ExplainLexical
}

// ExplainBM25 explains one document's BM25 score.
func (s *Store) ExplainBM25(
	ctx context.Context,
	root string,
	query string,
	topK int,
	target string,
) (BM25Explanation, error) {
	root, err := normalizeRoot(root)
	if err != nil {
		return BM25Explanation{}, err
	}
	positives, negatives := splitFTSQuery(query)
	aliases := privacy.ExpandQuery(query, s.privacyRefs(ctx, root), func(id string) ([]byte, error) {
		return privacy.ResolveKey(root, id)
	})
	for _, alias := range aliases {
		positives = append(positives, quoteFTSTerm(alias))
	}
	if len(positives) == 0 {
		return BM25Explanation{}, search.ErrEmptyQuery
	}
	match := strings.Join(positives, " OR ")
	if len(positives) > 1 {
		match = "(" + match + ")"
	}
	if len(negatives) > 0 {
		match += " NOT (" + strings.Join(negatives, " OR ") + ")"
	}
	hits, err := s.SearchBM25(ctx, root, query, topK)
	if err != nil {
		return BM25Explanation{}, err
	}
	rank := 0
	if target == "" {
		if len(hits) == 0 {
			return BM25Explanation{}, search.ErrNoResult
		}
		target = hits[0].ID
		rank = 1
	} else {
		exists, err := s.HasConcept(ctx, root, target)
		if err != nil {
			return BM25Explanation{}, err
		}
		if !exists {
			return BM25Explanation{}, search.ErrConceptNotFound
		}
		for index, hit := range hits {
			if hit.ID == target {
				rank = index + 1
				break
			}
		}
	}

	maxRaw := 0.0
	raw := 0.0
	for _, hit := range hits {
		if hit.RawScore > maxRaw {
			maxRaw = hit.RawScore
		}
		if hit.ID == target {
			raw = hit.RawScore
		}
	}
	if raw == 0 {
		raw, err = s.bm25Raw(ctx, root, match, target)
		if err != nil {
			return BM25Explanation{}, err
		}
	}
	score := 0.0
	if maxRaw > 0 {
		score = search.RoundScore(raw / maxRaw)
	}

	matchedFields := make([]contractv1.SearchField, 0)
	for _, field := range []contractv1.SearchField{
		contractv1.FieldTitle,
		contractv1.FieldKeywords,
		contractv1.FieldDescription,
		contractv1.FieldBody,
	} {
		expression := string(field) + ":(" + strings.Join(positives, " OR ") + ")"
		if len(negatives) > 0 {
			expression += " NOT (" + strings.Join(negatives, " OR ") + ")"
		}
		matched, err := s.ftsMatches(ctx, root, expression, target)
		if err != nil {
			return BM25Explanation{}, err
		}
		if matched {
			matchedFields = append(matchedFields, field)
		}
	}

	tokens := make([]contractv1.ExplainLexicalToken, 0, len(positives))
	for _, term := range positives {
		contribution, err := s.bm25Raw(ctx, root, term, target)
		if err != nil {
			return BM25Explanation{}, err
		}
		tokens = append(tokens, contractv1.ExplainLexicalToken{
			QueryToken:   strings.Trim(term, "\""),
			Matched:      contribution > 0,
			Contribution: contribution,
		})
	}

	return BM25Explanation{
		Target: target,
		Rank:   rank,
		Lexical: contractv1.ExplainLexical{
			RawScore:      raw,
			Score:         score,
			MatchedFields: matchedFields,
			Tokens:        tokens,
		},
	}, nil
}

func (s *Store) privacyRefs(ctx context.Context, root string) []privacy.KeyRef {
	rows, err := s.db.QueryContext(ctx, `
		SELECT privacy_policy, privacy_provider, privacy_key_id, privacy_spans_json
		FROM documents
		WHERE bundle_root = ? AND privacy_policy IN ('tokenize', 'hash', 'mixed')`, root)
	if err != nil {
		return nil
	}
	defer rows.Close()
	refs := make([]privacy.KeyRef, 0)
	seen := make(map[string]bool)
	add := func(policy contractv1.PrivacyPolicy, provider, keyID string) {
		if keyID == "" || (policy != contractv1.PrivacyTokenize && policy != contractv1.PrivacyHash) {
			return
		}
		key := string(policy) + "\x00" + provider + "\x00" + keyID
		if seen[key] {
			return
		}
		seen[key] = true
		refs = append(refs, privacy.KeyRef{Policy: policy, Provider: provider, KeyID: keyID})
	}
	for rows.Next() {
		var policy, provider, keyID, spansJSON string
		if err := rows.Scan(&policy, &provider, &keyID, &spansJSON); err != nil {
			return nil
		}
		stored := contractv1.PrivacyPolicy(policy)
		if stored == contractv1.PrivacyMixed {
			var spans []privacy.Span
			if err := json.Unmarshal([]byte(spansJSON), &spans); err == nil {
				for _, span := range spans {
					add(span.Policy, span.Provider, span.KeyID)
				}
			}
			continue
		}
		add(stored, provider, keyID)
	}
	return refs
}

// HasConcept reports whether a concept exists in a bundle.
func (s *Store) HasConcept(ctx context.Context, root, id string) (bool, error) {
	var found int
	err := s.db.QueryRowContext(ctx,
		"SELECT 1 FROM documents WHERE bundle_root = ? AND id = ?", root, id).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *Store) bm25Raw(ctx context.Context, root, match, id string) (float64, error) {
	var raw float64
	err := s.db.QueryRowContext(ctx, `
		SELECT -bm25(fts_documents, 0.0, 0.0, 10.0, 8.0, 4.0, 1.0)
		FROM fts_documents
		JOIN documents d
		  ON d.bundle_root = fts_documents.bundle_root
		 AND d.id = fts_documents.id
		WHERE fts_documents.bundle_root = ?
		  AND fts_documents MATCH ?
		  AND d.id = ?`, root, match, id).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return raw, nil
}

func (s *Store) ftsMatches(ctx context.Context, root, match, id string) (bool, error) {
	var found int
	err := s.db.QueryRowContext(ctx, `
		SELECT 1
		FROM fts_documents
		JOIN documents d
		  ON d.bundle_root = fts_documents.bundle_root
		 AND d.id = fts_documents.id
		WHERE fts_documents.bundle_root = ?
		  AND fts_documents MATCH ?
		  AND d.id = ?
		LIMIT 1`, root, match, id).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func buildFTSQuery(query string) string {
	positives, negatives := splitFTSQuery(query)
	if len(positives) == 0 {
		return ""
	}
	expression := strings.Join(positives, " OR ")
	if len(positives) > 1 {
		expression = "(" + expression + ")"
	}
	if len(negatives) > 0 {
		expression += " NOT (" + strings.Join(negatives, " OR ") + ")"
	}
	return expression
}

func splitFTSQuery(query string) ([]string, []string) {
	query = strings.TrimSpace(query)
	positives := make([]string, 0)
	negatives := make([]string, 0)
	index := 0
	for index < len(query) {
		for index < len(query) && (query[index] == ' ' || query[index] == '\t') {
			index++
		}
		if index >= len(query) {
			break
		}
		negative := false
		if query[index] == '-' {
			negative = true
			index++
			if index >= len(query) {
				break
			}
		}
		start := index
		if query[index] == '"' {
			index++
			for index < len(query) && query[index] != '"' {
				index++
			}
			if index < len(query) {
				index++
			}
		} else {
			for index < len(query) && query[index] != ' ' && query[index] != '\t' {
				index++
			}
		}
		term := strings.TrimSpace(query[start:index])
		if term == "" || term == "-\"\"" {
			continue
		}
		term = quoteFTSTerm(term)
		if negative {
			negatives = append(negatives, term)
		} else {
			positives = append(positives, term)
		}
	}
	return positives, negatives
}

func quoteFTSTerm(term string) string {
	if strings.HasPrefix(term, "\"") {
		return term
	}
	// FTS5 treats punctuation such as "-" as an operator; quote literal terms
	// so hyphenated identifiers match as phrases.
	return `"` + strings.ReplaceAll(term, `"`, `""`) + `"`
}
