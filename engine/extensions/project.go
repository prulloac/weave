package extensions

import (
	"context"
	"encoding/json"
	"fmt"
)

// MaxProjectAnalysisBytes bounds one project analysis request and response.
const MaxProjectAnalysisBytes = 64 << 20

// ProjectFile is one file offered to a project analyzer.
type ProjectFile struct {
	ID          string `json:"id"`
	Path        string `json:"path"`
	MediaType   string `json:"media_type,omitempty"`
	ContentHash string `json:"content_hash,omitempty"`
}

// ProjectOptions controls one project analysis.
type ProjectOptions struct {
	SymbolDocuments bool     `json:"symbol_documents"`
	Relations       []string `json:"relations,omitempty"`
}

// ProjectAnalysisRequest is one project-wide analysis payload.
type ProjectAnalysisRequest struct {
	V          string         `json:"v"`
	Type       string         `json:"type"`
	BundleRoot string         `json:"bundle_root"`
	Files      []ProjectFile  `json:"files"`
	Options    ProjectOptions `json:"options"`
}

// ProjectDocument is one file, symbol or external document in the response.
type ProjectDocument struct {
	ID              string            `json:"id"`
	Title           string            `json:"title,omitempty"`
	Description     string            `json:"description,omitempty"`
	Keywords        []string          `json:"keywords,omitempty"`
	Body            string            `json:"body,omitempty"`
	ContentMetadata map[string]any    `json:"content_metadata,omitempty"`
	SymbolDocuments []ProjectDocument `json:"symbol_documents,omitempty"`
}

// ProjectRelation is one relation between two documents.
type ProjectRelation struct {
	From string `json:"from"`
	To   string `json:"to"`
	Kind string `json:"kind"`
}

// ProjectAnalysisResponse is the validated project analysis result.
type ProjectAnalysisResponse struct {
	V                 string            `json:"v"`
	Documents         []ProjectDocument `json:"documents"`
	ExternalDocuments []ProjectDocument `json:"external_documents,omitempty"`
	Relations         []ProjectRelation `json:"relations,omitempty"`
	Warnings          []string          `json:"warnings,omitempty"`
}

// ProjectAnalysis invokes a project analyzer and validates file coverage.
func (r Runner) ProjectAnalysis(ctx context.Context, extension Extension, request ProjectAnalysisRequest) (ProjectAnalysisResponse, error) {
	request.V = ProtocolVersion
	request.Type = "project_analysis_request"
	payload, err := json.Marshal(request)
	if err != nil {
		return ProjectAnalysisResponse{}, fmt.Errorf("%w: %v", ErrProtocol, err)
	}
	if len(payload) > MaxProjectAnalysisBytes {
		return ProjectAnalysisResponse{}, fmt.Errorf("%w: %s request exceeds %d bytes", ErrProtocol, extension.Name, MaxProjectAnalysisBytes)
	}
	var response ProjectAnalysisResponse
	if err := r.invokeWithLimit(ctx, extension, request, &response, MaxProjectAnalysisBytes); err != nil {
		return ProjectAnalysisResponse{}, err
	}
	if len(response.Documents) != len(request.Files) {
		return ProjectAnalysisResponse{}, fmt.Errorf("%w: expected %d documents, got %d", ErrProtocol, len(request.Files), len(response.Documents))
	}
	expected := make(map[string]bool, len(request.Files))
	for _, file := range request.Files {
		expected[file.ID] = true
	}
	seen := make(map[string]bool, len(response.Documents))
	for _, document := range response.Documents {
		if document.ID == "" || !expected[document.ID] {
			return ProjectAnalysisResponse{}, fmt.Errorf("%w: unknown document %q", ErrProtocol, document.ID)
		}
		if seen[document.ID] {
			return ProjectAnalysisResponse{}, fmt.Errorf("%w: duplicate document %q", ErrProtocol, document.ID)
		}
		seen[document.ID] = true
	}
	for id := range expected {
		if !seen[id] {
			return ProjectAnalysisResponse{}, fmt.Errorf("%w: missing document %q", ErrProtocol, id)
		}
	}
	return response, nil
}
