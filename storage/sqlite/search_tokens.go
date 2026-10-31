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
		}
	}
	return nil
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
