package extensions

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// SourceProtocolVersion is the source-provider protocol version.
const SourceProtocolVersion = "weave.source/v1"

// MaxSourceResponseBytes bounds one source-provider response.
const MaxSourceResponseBytes = 1 << 20

var sourceCommitPattern = regexp.MustCompile(`^[0-9a-f]{40}$|^[0-9a-f]{64}$`)

// SourceOptions controls one source materialization.
type SourceOptions struct {
	Worktree   bool `json:"worktree,omitempty"`
	Submodules bool `json:"submodules,omitempty"`
	LFS        bool `json:"lfs,omitempty"`
	Depth      int  `json:"depth,omitempty"`
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
	Commit   string `json:"commit"`
	Ref      string `json:"ref,omitempty"`
	URI      string `json:"uri,omitempty"`
	ReadOnly bool   `json:"read_only,omitempty"`
}

// SourceRepository describes the repository itself.
type SourceRepository struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	Readme      string `json:"readme,omitempty"`
	Host        string `json:"host,omitempty"`
	Owner       string `json:"owner,omitempty"`
	License     string `json:"license,omitempty"`
}

// SourceResponse is one validated source-provider reply.
type SourceResponse struct {
	V          string           `json:"v"`
	Type       string           `json:"type,omitempty"`
	Snapshot   SourceSnapshot   `json:"snapshot"`
	Repository SourceRepository `json:"repository"`
	Warnings   []string         `json:"warnings,omitempty"`
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
	if !sourceCommitPattern.MatchString(response.Snapshot.Commit) {
		return SourceResponse{}, fmt.Errorf("%w: invalid commit %q", ErrProtocol, response.Snapshot.Commit)
	}
	if !filepath.IsAbs(response.Snapshot.Path) {
		return SourceResponse{}, fmt.Errorf("%w: snapshot path must be absolute", ErrProtocol)
	}
	if response.Repository.ID != "_repo" {
		return SourceResponse{}, fmt.Errorf("%w: repository id must be _repo", ErrProtocol)
	}
	if strings.TrimSpace(response.Repository.Name) == "" {
		return SourceResponse{}, fmt.Errorf("%w: repository name is required", ErrProtocol)
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
