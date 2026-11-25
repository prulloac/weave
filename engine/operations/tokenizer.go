package operations

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine/extensions"
	sqlitestore "github.com/prulloac/weave/storage/sqlite"
)

var (
	// ErrTokenizerNotFound marks a missing tokenizer extension.
	ErrTokenizerNotFound = errors.New("tokenizer not found")
	// ErrTokenizerChanged marks an index built with a different tokenizer.
	ErrTokenizerChanged = errors.New("tokenizer changed")
	// ErrTokenizerProtocol marks invalid tokenizer output.
	ErrTokenizerProtocol = errors.New("tokenizer protocol error")
	// ErrTokenizerTimeout marks a tokenizer timeout.
	ErrTokenizerTimeout = errors.New("tokenizer timed out")
)

func (e *Executor) newTokenizer(ctx context.Context, root, name string, timeout time.Duration) (*sqlitestore.Tokenizer, sqlitestore.TokenizerIdentity, error) {
	if timeout <= 0 {
		timeout = extensions.DefaultTimeout
	}
	extension, manifest, warnings, err := extensions.SelectRole(ctx, root, name, "tokenizer", timeout)
	for _, warning := range warnings {
		e.warn("%s", warning)
	}
	if err != nil {
		switch {
		case errors.Is(err, extensions.ErrNotFound):
			return nil, sqlitestore.TokenizerIdentity{}, fmt.Errorf("%w: %s", ErrTokenizerNotFound, name)
		case errors.Is(err, extensions.ErrTimeout):
			return nil, sqlitestore.TokenizerIdentity{}, fmt.Errorf("%w: %s", ErrTokenizerTimeout, name)
		case errors.Is(err, extensions.ErrProtocol):
			return nil, sqlitestore.TokenizerIdentity{}, fmt.Errorf("%w: %s", ErrTokenizerProtocol, name)
		default:
			return nil, sqlitestore.TokenizerIdentity{}, err
		}
	}
	identity := sqlitestore.TokenizerIdentity{
		Name:     extension.Name,
		Version:  manifest.Version,
		CacheKey: manifest.CacheKey,
	}
	if identity.Version == "" {
		identity.Version = "unversioned"
	}
	identity.ID = tokenizerID(identity)
	runner := extensions.Runner{Timeout: timeout}
	tokenizer := &sqlitestore.Tokenizer{
		Name:     identity.Name,
		Version:  identity.Version,
		CacheKey: identity.CacheKey,
		ID:       identity.ID,
		Terms: func(ctx context.Context, text, mode string) ([]string, map[string][]string, error) {
			response, err := runner.Tokenize(ctx, extension, extensions.TokenizerRequest{Text: text, Mode: mode})
			if err != nil {
				switch {
				case errors.Is(err, extensions.ErrTimeout):
					return nil, nil, fmt.Errorf("%w: %s", ErrTokenizerTimeout, name)
				case errors.Is(err, extensions.ErrProtocol):
					return nil, nil, fmt.Errorf("%w: %s", ErrTokenizerProtocol, name)
				default:
					return nil, nil, err
				}
			}
			return response.Terms, response.Expansions, nil
		},
	}
	return tokenizer, identity, nil
}

func tokenizerErrorCode(err error) (contractv1.ErrorCode, bool) {
	switch {
	case errors.Is(err, ErrTokenizerNotFound):
		return contractv1.ErrTokenizerNotFound, true
	case errors.Is(err, ErrTokenizerChanged):
		return contractv1.ErrTokenizerChanged, true
	case errors.Is(err, ErrTokenizerProtocol), errors.Is(err, extensions.ErrProtocol):
		return contractv1.ErrTokenizerProtocol, true
	case errors.Is(err, ErrTokenizerTimeout), errors.Is(err, extensions.ErrTimeout):
		return contractv1.ErrTokenizerTimeout, true
	}
	return "", false
}

func tokenizerID(identity sqlitestore.TokenizerIdentity) string {
	sum := sha256.Sum256([]byte(identity.Name + "\x00" + identity.Version + "\x00" + identity.CacheKey))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// writeTokenizer resolves the tokenizer for one write. Ingest resets to builtin unless the flag is
// set; rebuild and sync keep the recorded tokenizer when the flag is absent.
func (e *Executor) writeTokenizer(ctx context.Context, store *sqlitestore.Store, request contractv1.OperationRequest, timeout time.Duration) (*sqlitestore.Tokenizer, error) {
	name := request.Tokenizer
	if name == "" && request.Op != contractv1.OpIngest {
		identity, ok, err := store.TokenizerIdentity(ctx, request.Path)
		if err != nil {
			return nil, err
		}
		if ok && !identity.Builtin() {
			name = identity.Name
		}
	}
	if name == "" {
		return nil, nil
	}
	tokenizer, _, err := e.newTokenizer(ctx, request.Path, name, timeout)
	return tokenizer, err
}

// transientSearchTokenizer resolves --tokenizer for a --no-db search. A failure falls back to the
// built-in tokenizer with one warning because there is no stored index identity to enforce.
func (e *Executor) transientSearchTokenizer(
	ctx context.Context,
	request contractv1.OperationRequest,
	timeout time.Duration,
) (*sqlitestore.Tokenizer, [][]string, bool) {
	if request.Tokenizer == "" {
		return nil, nil, false
	}
	tokenizer, _, err := e.newTokenizer(ctx, request.Path, request.Tokenizer, timeout)
	if err != nil {
		e.warn("tokenizer %s unavailable: %v; using the built-in tokenizer", request.Tokenizer, err)
		return nil, nil, false
	}
	groups, err := tokenizer.QueryGroups(ctx, request.Query)
	if err != nil {
		e.warn("tokenizer %s failed: %v; using the built-in tokenizer", request.Tokenizer, err)
		return nil, nil, false
	}
	return tokenizer, groups, true
}

// searchTokenizer resolves the tokenizer recorded in one index and enforces identity.
func (e *Executor) searchTokenizer(ctx context.Context, store *sqlitestore.Store, request contractv1.OperationRequest, timeout time.Duration) error {
	identity, _, err := store.TokenizerIdentity(ctx, request.Path)
	if err != nil {
		return err
	}
	if request.Tokenizer != "" {
		tokenizer, requested, err := e.newTokenizer(ctx, request.Path, request.Tokenizer, timeout)
		if err != nil {
			return err
		}
		if identity.ID != requested.ID {
			recorded := identity.Name
			if identity.Builtin() {
				recorded = "builtin"
			}
			return fmt.Errorf("%w: index uses %s, requested %s", ErrTokenizerChanged, recorded, requested.Name)
		}
		store.Tokenizer = tokenizer
		return nil
	}
	if identity.Builtin() {
		return nil
	}
	tokenizer, current, err := e.newTokenizer(ctx, request.Path, identity.Name, timeout)
	if err != nil {
		return err
	}
	if current.ID != identity.ID {
		return fmt.Errorf("%w: index uses %s %s, installed %s %s", ErrTokenizerChanged, identity.Name, identity.Version, current.Name, current.Version)
	}
	store.Tokenizer = tokenizer
	return nil
}
