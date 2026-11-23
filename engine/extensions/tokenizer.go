package extensions

import (
	"context"
	"fmt"
	"strings"
)

const (
	maxTokenizerText  = 1 << 20
	maxTokenizerTerms = 4096
	maxTermLength     = 128
	maxExpansions     = 32
)

// TokenizerRequest is one tokenizer call payload.
type TokenizerRequest struct {
	V        string         `json:"v"`
	Type     string         `json:"type"`
	Text     string         `json:"text"`
	Language string         `json:"language,omitempty"`
	Mode     string         `json:"mode"`
	Options  map[string]any `json:"options,omitempty"`
}

// TokenizerResponse is one validated tokenizer reply.
type TokenizerResponse struct {
	V          string              `json:"v"`
	Type       string              `json:"type"`
	Terms      []string            `json:"terms"`
	Expansions map[string][]string `json:"expansions,omitempty"`
}

// Tokenize invokes a tokenizer and validates its term stream and query expansions.
func (r Runner) Tokenize(ctx context.Context, extension Extension, request TokenizerRequest) (TokenizerResponse, error) {
	if request.Mode != "index" && request.Mode != "query" {
		return TokenizerResponse{}, fmt.Errorf("%w: invalid tokenizer mode %q", ErrProtocol, request.Mode)
	}
	request.V = ProtocolVersion
	request.Type = "tokenizer_request"
	request.Text = truncate(request.Text, maxTokenizerText)
	var response TokenizerResponse
	if err := r.invokeRole(ctx, extension, "tokenizer", request, &response); err != nil {
		return TokenizerResponse{}, err
	}
	if response.Terms == nil {
		return TokenizerResponse{}, fmt.Errorf("%w: tokenizer response has no terms", ErrProtocol)
	}
	if len(response.Terms) > maxTokenizerTerms {
		return TokenizerResponse{}, fmt.Errorf("%w: tokenizer returned %d terms", ErrProtocol, len(response.Terms))
	}
	for _, term := range response.Terms {
		if term == "" || len(term) > maxTermLength {
			return TokenizerResponse{}, fmt.Errorf("%w: invalid tokenizer term", ErrProtocol)
		}
	}
	if len(response.Expansions) > 0 && request.Mode != "query" {
		return TokenizerResponse{}, fmt.Errorf("%w: tokenizer expansions are query-only", ErrProtocol)
	}
	total := 0
	for term, expansions := range response.Expansions {
		if strings.TrimSpace(term) == "" || len(expansions) > maxExpansions {
			return TokenizerResponse{}, fmt.Errorf("%w: invalid tokenizer expansion for %q", ErrProtocol, term)
		}
		for _, expansion := range expansions {
			if expansion == "" || len(expansion) > maxTermLength {
				return TokenizerResponse{}, fmt.Errorf("%w: invalid tokenizer expansion for %q", ErrProtocol, term)
			}
			total++
		}
	}
	if total > maxTokenizerTerms {
		return TokenizerResponse{}, fmt.Errorf("%w: tokenizer returned %d expansions", ErrProtocol, total)
	}
	return response, nil
}
