package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/prulloac/weave/engine/search"
)

// Tokenizer supplies index and query terms for one selected tokenizer extension.
type Tokenizer struct {
	Name     string
	Version  string
	CacheKey string
	ID       string
	Terms    func(ctx context.Context, text, mode string) ([]string, map[string][]string, error)
}

// TokenizerIdentity is the identity recorded in one bundle index.
type TokenizerIdentity struct {
	Name     string
	Version  string
	CacheKey string
	ID       string
}

// Builtin reports whether the bundle was indexed with the built-in tokenizer.
func (i TokenizerIdentity) Builtin() bool { return i.ID == "" || i.Name == "" }

func tokenizerColumns(tokenizer *Tokenizer) (name, version, cacheKey, id string) {
	if tokenizer == nil {
		return "", "", "", ""
	}
	return tokenizer.Name, tokenizer.Version, tokenizer.CacheKey, tokenizer.ID
}

// QueryGroups returns AND groups whose members are alternatives: the term itself plus any
// query expansions.
func (t *Tokenizer) QueryGroups(ctx context.Context, text string) ([][]string, error) {
	terms, expansions, err := t.Terms(ctx, text, "query")
	if err != nil {
		return nil, err
	}
	groups := make([][]string, 0, len(terms))
	for _, term := range terms {
		group := []string{term}
		group = append(group, expansions[term]...)
		groups = append(groups, group)
	}
	return groups, nil
}

// Match renders the query as an FTS5 expression.
func (t *Tokenizer) Match(ctx context.Context, text string) (string, error) {
	groups, err := t.QueryGroups(ctx, text)
	if err != nil {
		return "", err
	}
	parts := make([]string, 0, len(groups))
	for _, group := range groups {
		quoted := make([]string, 0, len(group))
		for _, term := range group {
			quoted = append(quoted, quoteFTSTerm(term))
		}
		parts = append(parts, "("+strings.Join(quoted, " OR ")+")")
	}
	return strings.Join(parts, " AND "), nil
}

func (s *Store) queryGroups(ctx context.Context, text string) ([][]string, error) {
	if s.Tokenizer != nil {
		return s.Tokenizer.QueryGroups(ctx, text)
	}
	tokens := search.Tokenize(text)
	groups := make([][]string, 0, len(tokens))
	for _, token := range tokens {
		groups = append(groups, []string{token})
	}
	return groups, nil
}

// TokenizerIdentity reads the identity recorded for one bundle.
func (s *Store) TokenizerIdentity(ctx context.Context, root string) (TokenizerIdentity, bool, error) {
	root, err := normalizeRoot(root)
	if err != nil {
		return TokenizerIdentity{}, false, err
	}
	var identity TokenizerIdentity
	err = s.db.QueryRowContext(ctx, `
		SELECT tokenizer_name, tokenizer_version, tokenizer_cache_key, tokenizer_id
		FROM bundles WHERE bundle_root = ?`, root).Scan(
		&identity.Name, &identity.Version, &identity.CacheKey, &identity.ID)
	if errors.Is(err, sql.ErrNoRows) {
		return TokenizerIdentity{}, false, nil
	}
	if err != nil {
		return TokenizerIdentity{}, false, err
	}
	return identity, true, nil
}
