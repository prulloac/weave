// Package extensions discovers and invokes weave-<name> subprocesses.
package extensions

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
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
	DocPath     string         `json:"doc_path,omitempty"`
	BundleRoot  string         `json:"bundle_root,omitempty"`
	Workspace   string         `json:"workspace,omitempty"`
	Title       string         `json:"title"`
	Frontmatter map[string]any `json:"frontmatter"`
	Body        string         `json:"body"`
}

// EnrichmentResponse is one validated enrichment reply.
type EnrichmentResponse struct {
	V                       string         `json:"v"`
	InferredTitle           string         `json:"inferred_title"`
	InferredDescription     string         `json:"inferred_description"`
	InferredText            string         `json:"inferred_text"`
	InferredTextSource      string         `json:"inferred_text_source"`
	InferredContentMetadata map[string]any `json:"inferred_content_metadata"`
	InferredKeywords        []string       `json:"inferred_keywords"`
	Warnings                []string       `json:"warnings"`
	CustomMetadata          map[string]any `json:"custom_metadata"`
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

// EmbeddingRequest is one embedding call payload.
type EmbeddingRequest struct {
	V         string `json:"v"`
	Type      string `json:"type"`
	Text      string `json:"text"`
	MediaType string `json:"media_type"`
	Model     string `json:"model"`
}

type embeddingResponse struct {
	V      string `json:"v"`
	Model  string `json:"model"`
	Dim    int    `json:"dim"`
	Vector string `json:"vector"`
}

// Embedding is one validated embedding reply.
type Embedding struct {
	Model  string
	Dim    int
	Vector []float32
}

// Embed invokes an embedder and decodes its base64 float32 vector.
func (r Runner) Embed(ctx context.Context, extension Extension, request EmbeddingRequest) (Embedding, error) {
	request.V = ProtocolVersion
	request.Type = "embedding_request"
	request.Text = truncate(request.Text, 8<<10)
	var response embeddingResponse
	if err := r.invoke(ctx, extension, request, &response); err != nil {
		return Embedding{}, err
	}
	if request.Model != "" && response.Model != request.Model {
		return Embedding{}, fmt.Errorf("%w: model mismatch", ErrProtocol)
	}
	if response.Dim <= 0 || response.Model == "" {
		return Embedding{}, fmt.Errorf("%w: invalid embedding metadata", ErrProtocol)
	}
	raw, err := base64.StdEncoding.DecodeString(response.Vector)
	if err != nil || len(raw) != response.Dim*4 {
		return Embedding{}, fmt.Errorf("%w: invalid embedding vector", ErrProtocol)
	}
	vector := make([]float32, response.Dim)
	for index := range vector {
		vector[index] = math.Float32frombits(binary.LittleEndian.Uint32(raw[index*4:]))
		if math.IsNaN(float64(vector[index])) || math.IsInf(float64(vector[index]), 0) {
			return Embedding{}, fmt.Errorf("%w: invalid embedding value", ErrProtocol)
		}
	}
	return Embedding{Model: response.Model, Dim: response.Dim, Vector: vector}, nil
}

func (r Runner) invoke(ctx context.Context, extension Extension, payload, target any) error {
	return r.invokeWithLimit(ctx, extension, payload, target, maxOutput)
}

func (r Runner) invokeWithLimit(ctx context.Context, extension Extension, payload, target any, outputLimit int) error {
	return r.invokeVersionWithLimit(ctx, extension, payload, target, outputLimit, ProtocolVersion)
}

func (r Runner) invokeVersionWithLimit(ctx context.Context, extension Extension, payload, target any, outputLimit int, protocolVersion string) error {
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
	stdout := &cappedBuffer{limit: outputLimit}
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
	if envelope.V != protocolVersion {
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
