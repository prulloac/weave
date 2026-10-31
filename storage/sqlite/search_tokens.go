package sqlite

import (
	"context"
	"database/sql"
	"encoding/binary"
	"sort"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine/search"
)

func populateSearchTokens(
	ctx context.Context,
	tx *sql.Tx,
	root string,
	bundle *contractv1.Bundle,
) error {
	if _, err := tx.ExecContext(ctx, "DELETE FROM search_tokens WHERE bundle_root = ?", root); err != nil {
		return err
	}
	for _, concept := range bundle.Concepts {
		if concept.Privacy != nil {
			return nil
		}
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

	postings := map[string]map[string][]int32{
		string(contractv1.FieldTitle):       {},
		string(contractv1.FieldDescription): {},
		string(contractv1.FieldBody):        {},
	}
	for position, concept := range concepts {
		title := concept.ID
		if concept.Title != nil {
			title = *concept.Title
		}
		description := ""
		if concept.Description != nil {
			description = *concept.Description
		}
		texts := map[string]string{
			string(contractv1.FieldTitle):       title,
			string(contractv1.FieldDescription): description,
			string(contractv1.FieldBody):        search.StripMarkdown(concept.Body),
		}
		for _, field := range []string{
			string(contractv1.FieldTitle),
			string(contractv1.FieldDescription),
			string(contractv1.FieldBody),
		} {
			for _, token := range uniqueTokens(search.Tokenize(texts[field])) {
				postings[field][token] = append(postings[field][token], int32(position))
			}
		}
	}

	for _, field := range []string{
		string(contractv1.FieldTitle),
		string(contractv1.FieldDescription),
		string(contractv1.FieldBody),
	} {
		tokens := make([]string, 0, len(postings[field]))
		for token := range postings[field] {
			tokens = append(tokens, token)
		}
		sort.Strings(tokens)
		for _, token := range tokens {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO search_tokens (bundle_root, field, token, postings)
				VALUES (?, ?, ?, ?)`,
				root, field, token, encodePostings(postings[field][token])); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `
				INSERT OR IGNORE INTO search_vocabulary (bundle_root, field, token)
				VALUES (?, ?, ?)`, root, field, token); err != nil {
				return err
			}
		}
	}
	return nil
}

func updateSearchTokens(
	ctx context.Context,
	tx *sql.Tx,
	root string,
	bundle *contractv1.Bundle,
	changed []contractv1.OkfConcept,
) error {
	for _, concept := range changed {
		if concept.Privacy != nil {
			_, err := tx.ExecContext(ctx, "DELETE FROM search_tokens WHERE bundle_root = ?", root)
			return err
		}
	}
	var count int
	if err := tx.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM search_tokens WHERE bundle_root = ?", root).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		return populateSearchTokens(ctx, tx, root, bundle)
	}

	rows, err := tx.QueryContext(ctx, `
		SELECT field, token, postings FROM search_tokens WHERE bundle_root = ?`, root)
	if err != nil {
		return err
	}
	postings := make(map[string]map[string][]int32)
	for rows.Next() {
		var field, token string
		var blob []byte
		if err := rows.Scan(&field, &token, &blob); err != nil {
			rows.Close()
			return err
		}
		if postings[field] == nil {
			postings[field] = make(map[string][]int32)
		}
		postings[field][token] = decodePostings(blob)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	idRows, err := tx.QueryContext(ctx, `
		SELECT id FROM documents
		WHERE bundle_root = ? AND privacy_policy = 'none'
		ORDER BY id`, root)
	if err != nil {
		return err
	}
	indexes := make(map[string]int32)
	var next int32
	for idRows.Next() {
		var id string
		if err := idRows.Scan(&id); err != nil {
			idRows.Close()
			return err
		}
		indexes[id] = next
		next++
	}
	if err := idRows.Err(); err != nil {
		idRows.Close()
		return err
	}
	idRows.Close()

	dirty := make(map[string]map[string]bool)
	mark := func(field, token string) {
		if dirty[field] == nil {
			dirty[field] = make(map[string]bool)
		}
		dirty[field][token] = true
	}
	for _, concept := range changed {
		index, ok := indexes[concept.ID]
		if !ok {
			continue
		}
		for field, tokens := range postings {
			for token, posting := range tokens {
				if !containsPosting(posting, index) {
					continue
				}
				postings[field][token] = removePosting(posting, index)
				mark(field, token)
			}
		}
		title := concept.ID
		if concept.Title != nil {
			title = *concept.Title
		}
		description := ""
		if concept.Description != nil {
			description = *concept.Description
		}
		texts := map[string]string{
			string(contractv1.FieldTitle):       title,
			string(contractv1.FieldDescription): description,
			string(contractv1.FieldBody):        search.StripMarkdown(concept.Body),
		}
		for _, field := range []string{
			string(contractv1.FieldTitle),
			string(contractv1.FieldDescription),
			string(contractv1.FieldBody),
		} {
			for _, token := range uniqueTokens(search.Tokenize(texts[field])) {
				postings[field][token] = append(postingFor(postings, field, token), index)
				mark(field, token)
			}
		}
	}

	for field, tokens := range dirty {
		for token := range tokens {
			posting := postings[field][token]
			sort.Slice(posting, func(i, j int) bool { return posting[i] < posting[j] })
			if len(posting) == 0 {
				if _, err := tx.ExecContext(ctx,
					"DELETE FROM search_tokens WHERE bundle_root = ? AND field = ? AND token = ?",
					root, field, token); err != nil {
					return err
				}
				if _, err := tx.ExecContext(ctx,
					"DELETE FROM search_vocabulary WHERE bundle_root = ? AND field = ? AND token = ?",
					root, field, token); err != nil {
					return err
				}
				continue
			}
			if _, err := tx.ExecContext(ctx, `
				INSERT OR REPLACE INTO search_tokens (bundle_root, field, token, postings)
				VALUES (?, ?, ?, ?)`,
				root, field, token, encodePostings(posting)); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `
				INSERT OR IGNORE INTO search_vocabulary (bundle_root, field, token)
				VALUES (?, ?, ?)`, root, field, token); err != nil {
				return err
			}
		}
	}
	return nil
}

func postingFor(postings map[string]map[string][]int32, field, token string) []int32 {
	if postings[field] == nil {
		postings[field] = make(map[string][]int32)
	}
	return postings[field][token]
}

func removePosting(postings []int32, target int32) []int32 {
	index := sort.Search(len(postings), func(i int) bool {
		return postings[i] >= target
	})
	if index >= len(postings) || postings[index] != target {
		return postings
	}
	return append(postings[:index], postings[index+1:]...)
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

func encodePostings(values []int32) []byte {
	data := make([]byte, 4*len(values))
	for index, value := range values {
		binary.LittleEndian.PutUint32(data[index*4:], uint32(value))
	}
	return data
}

func decodePostings(data []byte) []int32 {
	values := make([]int32, len(data)/4)
	for index := range values {
		values[index] = int32(binary.LittleEndian.Uint32(data[index*4:]))
	}
	return values
}
