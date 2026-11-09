package extensions

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
)

// MaxScorerRequest bounds one scorer request frame.
const MaxScorerRequest = 8 << 20

// ScorerScores carries the normalized core dimensions for one candidate.
type ScorerScores struct {
	Keyword float64 `json:"keyword"`
	Lexical float64 `json:"lexical"`
	Fuzzy   float64 `json:"fuzzy"`
	Core    float64 `json:"core"`
}

// ScorerCandidate is one scored candidate with safe document context.
type ScorerCandidate struct {
	ID          string       `json:"id"`
	Path        string       `json:"path"`
	Title       string       `json:"title,omitempty"`
	Description string       `json:"description,omitempty"`
	Type        string       `json:"type,omitempty"`
	Status      string       `json:"status,omitempty"`
	Keywords    []string     `json:"keywords,omitempty"`
	Snippet     *string      `json:"snippet,omitempty"`
	Scores      ScorerScores `json:"scores"`
}

// ScorerRequest is one scorer call payload.
type ScorerRequest struct {
	V          string            `json:"v"`
	Type       string            `json:"type"`
	Query      string            `json:"query"`
	Mode       string            `json:"mode"`
	CandidateK int               `json:"candidate_k"`
	Candidates []ScorerCandidate `json:"candidates"`
}

// ScorerCandidateScore is one scorer reply entry.
type ScorerCandidateScore struct {
	ID    string  `json:"id"`
	Score float64 `json:"score"`
}

type scorerResponse struct {
	V      string                 `json:"v"`
	Ranked []ScorerCandidateScore `json:"ranked_candidates"`
}

// Score invokes a scorer and validates that it covers the candidate set.
func (r Runner) Score(ctx context.Context, extension Extension, request ScorerRequest) (map[string]float64, error) {
	request.V = ProtocolVersion
	request.Type = "scorer_request"
	request.CandidateK = len(request.Candidates)
	payload, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrProtocol, err)
	}
	if len(payload) > MaxScorerRequest {
		return nil, fmt.Errorf("%w: %s request exceeds %d bytes", ErrProtocol, extension.Name, MaxScorerRequest)
	}
	var response scorerResponse
	if err := r.invoke(ctx, extension, request, &response); err != nil {
		return nil, err
	}
	if len(response.Ranked) != len(request.Candidates) {
		return nil, fmt.Errorf("%w: expected %d candidates, got %d", ErrProtocol, len(request.Candidates), len(response.Ranked))
	}
	scores := make(map[string]float64, len(response.Ranked))
	for _, candidate := range request.Candidates {
		scores[candidate.ID] = -1
	}
	for _, ranked := range response.Ranked {
		if _, expected := scores[ranked.ID]; !expected {
			return nil, fmt.Errorf("%w: unknown candidate %q", ErrProtocol, ranked.ID)
		}
		if scores[ranked.ID] != -1 {
			return nil, fmt.Errorf("%w: duplicate candidate %q", ErrProtocol, ranked.ID)
		}
		if math.IsNaN(ranked.Score) || math.IsInf(ranked.Score, 0) {
			return nil, fmt.Errorf("%w: invalid score for %q", ErrProtocol, ranked.ID)
		}
		if ranked.Score < 0 {
			ranked.Score = 0
		}
		if ranked.Score > 1 {
			ranked.Score = 1
		}
		scores[ranked.ID] = ranked.Score
	}
	for id, score := range scores {
		if score < 0 {
			return nil, fmt.Errorf("%w: missing candidate %q", ErrProtocol, id)
		}
	}
	return scores, nil
}
