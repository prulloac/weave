package extensions

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

// SourceProtocolVersion is the source-provider protocol version.
const SourceProtocolVersion = "weave.source/v1"

// MaxSourceResponseBytes bounds one source-provider response.
const MaxSourceResponseBytes = 1 << 20

// SourceOptions controls one source materialization.
type SourceOptions struct {
	Worktree        bool           `json:"worktree,omitempty"`
	Submodules      bool           `json:"submodules,omitempty"`
	LFS             bool           `json:"lfs,omitempty"`
	Depth           int            `json:"depth,omitempty"`
	Permissive      bool           `json:"permissive,omitempty"`
	History         bool           `json:"history,omitempty"`
	HistoryLimit    int            `json:"history_limit,omitempty"`
	ProviderOptions map[string]any `json:"provider_options,omitempty"`
}

// SourceRequest is one source-provider request payload.
type SourceRequest struct {
	V        string        `json:"v"`
	Type     string        `json:"type"`
	SourceID string        `json:"source_id"`
	URI      string        `json:"uri"`
	Ref      string        `json:"ref,omitempty"`
	CacheDir string        `json:"cache_dir,omitempty"`
	Options  SourceOptions `json:"options,omitempty"`
}

// SourceSnapshot describes the materialized source tree.
type SourceSnapshot struct {
	Path     string `json:"path"`
	SourceID string `json:"source_id"`
	Revision string `json:"revision"`
	Ref      string `json:"ref,omitempty"`
	URI      string `json:"uri,omitempty"`
	ReadOnly bool   `json:"read_only,omitempty"`
}

// SyntheticDocument is one extension-owned synthetic graph document.
type SyntheticDocument struct {
	ID          string         `json:"id"`
	Path        string         `json:"path,omitempty"`
	Type        string         `json:"type"`
	ContentKind string         `json:"content_kind,omitempty"`
	MediaType   string         `json:"media_type,omitempty"`
	Title       string         `json:"title,omitempty"`
	Description string         `json:"description,omitempty"`
	Keywords    []string       `json:"keywords,omitempty"`
	Body        string         `json:"body,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	Scope       *string        `json:"scope,omitempty"`
	Consumes    []string       `json:"consumes,omitempty"`
}

// SyntheticRelation is one extension-owned relation.
type SyntheticRelation struct {
	From   string  `json:"from"`
	To     string  `json:"to,omitempty"`
	ToPath string  `json:"to_path,omitempty"`
	Kind   string  `json:"kind"`
	Raw    *string `json:"raw,omitempty"`
}

// SyntheticArtifact is one extension-owned export artifact.
type SyntheticArtifact struct {
	Path      string `json:"path"`
	MediaType string `json:"media_type,omitempty"`
	Content   any    `json:"content"`
}

// SourceResponse is one validated source-provider reply.
type SourceResponse struct {
	V              string              `json:"v"`
	Type           string              `json:"type,omitempty"`
	Snapshot       SourceSnapshot      `json:"snapshot"`
	Documents      []SyntheticDocument `json:"documents,omitempty"`
	Relations      []SyntheticRelation `json:"relations,omitempty"`
	Artifacts      []SyntheticArtifact `json:"artifacts,omitempty"`
	SourceMetadata map[string]any      `json:"source_metadata,omitempty"`
	Warnings       []string            `json:"warnings,omitempty"`
}

// MaterializeSource invokes a source provider and validates its snapshot identity.
func (r Runner) MaterializeSource(ctx context.Context, extension Extension, request SourceRequest) (SourceResponse, error) {
	request.V = SourceProtocolVersion
	request.Type = "source_request"
	payload, err := json.Marshal(request)
	if err != nil {
		return SourceResponse{}, fmt.Errorf("%w: %v", ErrProtocol, err)
	}
	if len(payload) > MaxSourceResponseBytes {
		return SourceResponse{}, fmt.Errorf("%w: %s request exceeds %d bytes", ErrProtocol, extension.Name, MaxSourceResponseBytes)
	}
	var response SourceResponse
	if err := r.invokeVersionWithLimit(ctx, extension, request, &response, MaxSourceResponseBytes, SourceProtocolVersion); err != nil {
		return SourceResponse{}, err
	}
	if response.Snapshot.SourceID != request.SourceID {
		return SourceResponse{}, fmt.Errorf("%w: source_id mismatch", ErrProtocol)
	}
	revision := strings.TrimSpace(response.Snapshot.Revision)
	if revision == "" {
		return SourceResponse{}, fmt.Errorf("%w: snapshot revision is required", ErrProtocol)
	}
	response.Snapshot.Revision = revision
	if len(response.Documents) == 0 {
		return SourceResponse{}, fmt.Errorf("%w: synthetic documents are required", ErrProtocol)
	}
	if len(response.Documents) > 1000 {
		return SourceResponse{}, fmt.Errorf("%w: too many synthetic documents", ErrProtocol)
	}
	seenDocuments := make(map[string]bool, len(response.Documents))
	for _, document := range response.Documents {
		if strings.TrimSpace(document.ID) == "" {
			return SourceResponse{}, fmt.Errorf("%w: synthetic document id is required", ErrProtocol)
		}
		if seenDocuments[document.ID] {
			return SourceResponse{}, fmt.Errorf("%w: duplicate synthetic document %q", ErrProtocol, document.ID)
		}
		seenDocuments[document.ID] = true
	}
	if len(response.Relations) > 5000 {
		return SourceResponse{}, fmt.Errorf("%w: too many synthetic relations", ErrProtocol)
	}
	for _, relation := range response.Relations {
		if strings.TrimSpace(relation.From) == "" || strings.TrimSpace(relation.Kind) == "" {
			return SourceResponse{}, fmt.Errorf("%w: relation from and kind are required", ErrProtocol)
		}
		if (strings.TrimSpace(relation.To) == "") == (strings.TrimSpace(relation.ToPath) == "") {
			return SourceResponse{}, fmt.Errorf("%w: relation requires exactly one of to or to_path", ErrProtocol)
		}
	}
	if len(response.Artifacts) > 100 {
		return SourceResponse{}, fmt.Errorf("%w: too many artifacts", ErrProtocol)
	}
	for _, artifact := range response.Artifacts {
		if err := validateArtifactPath(artifact.Path); err != nil {
			return SourceResponse{}, fmt.Errorf("%w: %v", ErrProtocol, err)
		}
	}
	if !filepath.IsAbs(response.Snapshot.Path) {
		return SourceResponse{}, fmt.Errorf("%w: snapshot path must be absolute", ErrProtocol)
	}
	if !request.Options.Worktree && request.CacheDir != "" {
		if err := requireWithin(request.CacheDir, response.Snapshot.Path); err != nil {
			return SourceResponse{}, err
		}
	}
	return response, nil
}

func requireWithin(root, path string) error {
	relative, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return fmt.Errorf("%w: snapshot path escapes cache", ErrProtocol)
	}
	return nil
}

func validateArtifactPath(path string) error {
	value := strings.TrimSpace(path)
	if value == "" {
		return fmt.Errorf("artifact path is required")
	}
	if strings.Contains(value, "\\") || filepath.IsAbs(value) || strings.HasPrefix(value, "/") {
		return fmt.Errorf("unsafe artifact path %q", path)
	}
	clean := filepath.ToSlash(filepath.Clean(value))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return fmt.Errorf("unsafe artifact path %q", path)
	}
	return nil
}

// WorkspaceSourceID derives the stable source identity for a workspace source.
// The ref and commit are deliberately excluded.
func WorkspaceSourceID(workspaceRoot, alias, canonicalURI string) string {
	return sourceID(workspaceRoot, alias, canonicalURI)
}

// SingleSourceID derives the stable source identity for a single-source ingest.
func SingleSourceID(canonicalURI string) string {
	return sourceID(canonicalURI)
}

func sourceID(parts ...string) string {
	hash := sha256.New()
	for _, part := range parts {
		hash.Write([]byte(part))
		hash.Write([]byte{0})
	}
	return "src-" + hex.EncodeToString(hash.Sum(nil))[:16]
}
