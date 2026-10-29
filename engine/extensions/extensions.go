// Package extensions discovers and invokes weave-<name> subprocesses.
package extensions

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// ProtocolVersion is the extension protocol version.
const ProtocolVersion = "weave.cli/v1"

// DefaultTimeout bounds one extension invocation.
const DefaultTimeout = 5 * time.Second

const (
	maxOutput = 1 << 20
	maxBody   = 64 << 10
	maxStderr = 4 << 10
)

// Errors reported by discovery and invocation.
var (
	ErrNotFound = errors.New("extension not found")
	ErrTimeout  = errors.New("extension timed out")
	ErrFailed   = errors.New("extension failed")
	ErrProtocol = errors.New("extension protocol error")
)

// Extension is one discovered executable.
type Extension struct {
	Name   string
	Path   string
	Source string
}

var namePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// ValidName reports whether name is a valid extension identifier.
func ValidName(name string) bool {
	return namePattern.MatchString(name)
}

// Discover lists executable weave-<name> binaries in root/.weave/bin and PATH.
func Discover(root string) []Extension {
	found := make(map[string]Extension)
	if strings.TrimSpace(root) != "" {
		discoverDir(filepath.Join(root, ".weave", "bin"), "bundle", found)
	}
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if dir == "" {
			continue
		}
		discoverDir(dir, "path", found)
	}
	extensions := make([]Extension, 0, len(found))
	for _, extension := range found {
		extensions = append(extensions, extension)
	}
	sort.Slice(extensions, func(i, j int) bool {
		if extensions[i].Name != extensions[j].Name {
			return extensions[i].Name < extensions[j].Name
		}
		return extensions[i].Source < extensions[j].Source
	})
	return extensions
}

// Resolve returns the executable for one extension name.
func Resolve(root, name string) (Extension, error) {
	if !ValidName(name) {
		return Extension{}, fmt.Errorf("%w: invalid name %q", ErrNotFound, name)
	}
	if strings.TrimSpace(root) != "" {
		path := filepath.Join(root, ".weave", "bin", "weave-"+name)
		if executable(path) {
			return Extension{Name: name, Path: path, Source: "bundle"}, nil
		}
	}
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if dir == "" {
			continue
		}
		path := filepath.Join(dir, "weave-"+name)
		if executable(path) {
			return Extension{Name: name, Path: path, Source: "path"}, nil
		}
	}
	return Extension{}, fmt.Errorf("%w: %s", ErrNotFound, name)
}

func discoverDir(dir, source string, found map[string]Extension) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "weave-") {
			continue
		}
		short := strings.TrimPrefix(name, "weave-")
		if !ValidName(short) {
			continue
		}
		if _, exists := found[short]; exists {
			continue
		}
		path := filepath.Join(dir, name)
		if !executable(path) {
			continue
		}
		found[short] = Extension{Name: short, Path: path, Source: source}
	}
}

func executable(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	return info.Mode().Perm()&0o111 != 0
}

// Runner invokes extensions with bounded time and output.
type Runner struct {
	Timeout time.Duration
}

// EnrichmentRequest is one enrichment call payload.
type EnrichmentRequest struct {
	V           string         `json:"v"`
	Type        string         `json:"type"`
	DocID       string         `json:"doc_id"`
	Title       string         `json:"title"`
	Frontmatter map[string]any `json:"frontmatter"`
	Body        string         `json:"body"`
}

// EnrichmentResponse is one validated enrichment reply.
type EnrichmentResponse struct {
	V                string         `json:"v"`
	InferredKeywords []string       `json:"inferred_keywords"`
	CustomMetadata   map[string]any `json:"custom_metadata"`
}

// Enrich invokes an enricher for one concept.
func (r Runner) Enrich(ctx context.Context, extension Extension, request EnrichmentRequest) (EnrichmentResponse, error) {
	request.V = ProtocolVersion
	request.Type = "enrichment_request"
	request.Body = truncate(request.Body, maxBody)
	var response EnrichmentResponse
	if err := r.invoke(ctx, extension, request, &response); err != nil {
		return EnrichmentResponse{}, err
	}
	response.InferredKeywords = normalizeKeywords(response.InferredKeywords)
	return response, nil
}

// Candidate is one rerank input.
type Candidate struct {
	ID           string  `json:"id"`
	KeywordScore float64 `json:"keyword_score"`
	LexicalScore float64 `json:"lexical_score"`
	FuzzyScore   float64 `json:"fuzzy_score"`
}

// RerankRequest is one rerank call payload.
type RerankRequest struct {
	V          string      `json:"v"`
	Type       string      `json:"type"`
	Query      string      `json:"query"`
	Candidates []Candidate `json:"candidates"`
}

// RankedCandidate is one rerank reply entry.
type RankedCandidate struct {
	ID          string  `json:"id"`
	RerankScore float64 `json:"rerank_score"`
}

type rerankResponse struct {
	V      string            `json:"v"`
	Ranked []RankedCandidate `json:"ranked_candidates"`
}

// Rerank invokes a reranker and validates that it covers the candidate set.
func (r Runner) Rerank(ctx context.Context, extension Extension, request RerankRequest) (map[string]float64, error) {
	request.V = ProtocolVersion
	request.Type = "rerank_request"
	var response rerankResponse
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
		if math.IsNaN(ranked.RerankScore) || math.IsInf(ranked.RerankScore, 0) {
			return nil, fmt.Errorf("%w: invalid score for %q", ErrProtocol, ranked.ID)
		}
		if ranked.RerankScore < 0 {
			ranked.RerankScore = 0
		}
		if ranked.RerankScore > 1 {
			ranked.RerankScore = 1
		}
		scores[ranked.ID] = ranked.RerankScore
	}
	for id, score := range scores {
		if score < 0 {
			return nil, fmt.Errorf("%w: missing candidate %q", ErrProtocol, id)
		}
	}
	return scores, nil
}

func (r Runner) invoke(ctx context.Context, extension Extension, payload, target any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrProtocol, err)
	}
	timeout := r.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	runContext, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	command := exec.CommandContext(runContext, extension.Path)
	command.WaitDelay = 100 * time.Millisecond
	command.Stdin = bytes.NewReader(append(data, '\n'))
	stdout := &cappedBuffer{limit: maxOutput}
	stderr := &cappedBuffer{limit: maxStderr}
	command.Stdout = stdout
	command.Stderr = stderr
	err = command.Run()
	if runContext.Err() == context.DeadlineExceeded {
		return fmt.Errorf("%w: %s", ErrTimeout, extension.Name)
	}
	if err != nil {
		return fmt.Errorf("%w: %s: %s", ErrFailed, extension.Name, stderrLine(stderr.String()))
	}
	if stdout.exceeded {
		return fmt.Errorf("%w: %s produced oversized output", ErrProtocol, extension.Name)
	}
	var envelope struct {
		V string `json:"v"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		return fmt.Errorf("%w: %s returned invalid JSON", ErrProtocol, extension.Name)
	}
	if envelope.V != ProtocolVersion {
		return fmt.Errorf("%w: %s reported version %q", ErrProtocol, extension.Name, envelope.V)
	}
	if err := json.Unmarshal(stdout.Bytes(), target); err != nil {
		return fmt.Errorf("%w: %s returned invalid payload", ErrProtocol, extension.Name)
	}
	return nil
}

type cappedBuffer struct {
	buffer   bytes.Buffer
	limit    int
	exceeded bool
}

func (b *cappedBuffer) Write(data []byte) (int, error) {
	remaining := b.limit - b.buffer.Len()
	if remaining <= 0 {
		b.exceeded = true
		return len(data), nil
	}
	if len(data) > remaining {
		b.buffer.Write(data[:remaining])
		b.exceeded = true
		return len(data), nil
	}
	b.buffer.Write(data)
	return len(data), nil
}

func (b *cappedBuffer) Bytes() []byte { return b.buffer.Bytes() }

func (b *cappedBuffer) String() string { return b.buffer.String() }

func normalizeKeywords(values []string) []string {
	keywords := make([]string, 0, len(values))
	seen := make(map[string]bool)
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" || len(trimmed) > 128 {
			continue
		}
		key := strings.ToLower(trimmed)
		if seen[key] {
			continue
		}
		seen[key] = true
		keywords = append(keywords, trimmed)
		if len(keywords) == 32 {
			break
		}
	}
	return keywords
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}

func stderrLine(value string) string {
	line := strings.TrimSpace(value)
	if index := strings.IndexByte(line, '\n'); index >= 0 {
		line = line[:index]
	}
	return truncate(line, 200)
}
