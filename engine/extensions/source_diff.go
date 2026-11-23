package extensions

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// MaxSourceDiffResponseBytes bounds one source diff response.
const MaxSourceDiffResponseBytes = 16 << 20

// SourceDiffOptions controls one snapshot comparison.
type SourceDiffOptions struct {
	Format     string `json:"format,omitempty"`
	Context    int    `json:"context,omitempty"`
	Permissive bool   `json:"permissive,omitempty"`
}

// SourceDiffRequest is one source-provider diff request.
type SourceDiffRequest struct {
	V        string            `json:"v"`
	Type     string            `json:"type"`
	SourceID string            `json:"source_id"`
	URI      string            `json:"uri"`
	CacheDir string            `json:"cache_dir,omitempty"`
	From     string            `json:"from"`
	To       string            `json:"to"`
	Options  SourceDiffOptions `json:"options,omitempty"`
}

// SourceDiffChange is one changed path between two snapshots.
type SourceDiffChange struct {
	Kind         string `json:"kind"`
	Path         string `json:"path"`
	PreviousPath string `json:"previous_path,omitempty"`
	Binary       bool   `json:"binary,omitempty"`
	Patch        string `json:"patch,omitempty"`
}

// SourceDiffResponse is one validated source-provider diff reply.
type SourceDiffResponse struct {
	V         string             `json:"v"`
	Type      string             `json:"type,omitempty"`
	From      string             `json:"from"`
	To        string             `json:"to"`
	Truncated bool               `json:"truncated,omitempty"`
	Changes   []SourceDiffChange `json:"changes"`
}

// DiffSource invokes a source provider and validates its snapshot comparison.
func (r Runner) DiffSource(ctx context.Context, extension Extension, request SourceDiffRequest) (SourceDiffResponse, error) {
	if strings.TrimSpace(request.From) == "" || strings.TrimSpace(request.To) == "" {
		return SourceDiffResponse{}, fmt.Errorf("%w: diff requires from and to refs", ErrProtocol)
	}
	if request.Options.Format == "" {
		request.Options.Format = "changes"
	}
	if request.Options.Format != "changes" && request.Options.Format != "patch" {
		return SourceDiffResponse{}, fmt.Errorf("%w: invalid diff format %q", ErrProtocol, request.Options.Format)
	}
	if request.Options.Context < 0 || request.Options.Context > 100 {
		return SourceDiffResponse{}, fmt.Errorf("%w: diff context must be between 0 and 100", ErrProtocol)
	}
	request.V = SourceProtocolVersion
	request.Type = "source_diff_request"
	payload, err := json.Marshal(request)
	if err != nil {
		return SourceDiffResponse{}, fmt.Errorf("%w: %v", ErrProtocol, err)
	}
	if len(payload) > MaxSourceDiffResponseBytes {
		return SourceDiffResponse{}, fmt.Errorf("%w: %s diff request exceeds %d bytes", ErrProtocol, extension.Name, MaxSourceDiffResponseBytes)
	}
	var response SourceDiffResponse
	if err := r.invokeVersionWithLimit(ctx, extension, request, &response, MaxSourceDiffResponseBytes, SourceProtocolVersion); err != nil {
		return SourceDiffResponse{}, err
	}
	if strings.TrimSpace(response.From) == "" || strings.TrimSpace(response.To) == "" {
		return SourceDiffResponse{}, fmt.Errorf("%w: diff response must echo from and to", ErrProtocol)
	}
	if len(response.Changes) > 10000 {
		return SourceDiffResponse{}, fmt.Errorf("%w: diff returned %d changes", ErrProtocol, len(response.Changes))
	}
	seen := make(map[string]bool, len(response.Changes))
	for index := range response.Changes {
		change := &response.Changes[index]
		switch change.Kind {
		case "added", "removed", "modified", "renamed":
		default:
			return SourceDiffResponse{}, fmt.Errorf("%w: unknown change kind %q", ErrProtocol, change.Kind)
		}
		if err := validateSourcePath(change.Path); err != nil {
			return SourceDiffResponse{}, fmt.Errorf("%w: %v", ErrProtocol, err)
		}
		if change.Kind == "renamed" {
			if err := validateSourcePath(change.PreviousPath); err != nil {
				return SourceDiffResponse{}, fmt.Errorf("%w: %v", ErrProtocol, err)
			}
		} else if change.PreviousPath != "" {
			return SourceDiffResponse{}, fmt.Errorf("%w: previous_path is only valid for renames", ErrProtocol)
		}
		if seen[change.Path] {
			return SourceDiffResponse{}, fmt.Errorf("%w: duplicate change path %q", ErrProtocol, change.Path)
		}
		seen[change.Path] = true
		if change.Binary {
			change.Patch = ""
		}
	}
	sort.Slice(response.Changes, func(i, j int) bool {
		if response.Changes[i].Path != response.Changes[j].Path {
			return response.Changes[i].Path < response.Changes[j].Path
		}
		return response.Changes[i].Kind < response.Changes[j].Kind
	})
	return response, nil
}

func validateSourcePath(path string) error {
	value := strings.TrimSpace(path)
	if value == "" {
		return fmt.Errorf("change path is required")
	}
	if strings.Contains(value, "\\") || filepath.IsAbs(value) || strings.HasPrefix(value, "/") {
		return fmt.Errorf("unsafe change path %q", path)
	}
	clean := filepath.ToSlash(filepath.Clean(value))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return fmt.Errorf("unsafe change path %q", path)
	}
	return nil
}
