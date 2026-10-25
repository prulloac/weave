// Package contractv1 contains the hand-written Go representation of the
// language-neutral Weave contract v1. The JSON Schemas under schema/ are
// authoritative; these structs are validated against them in CI.
package contractv1

type OkfVersion string

const (
	OkfVersion02 OkfVersion = "0.2"
)

type ConceptStatus string

const (
	StatusDraft      ConceptStatus = "draft"
	StatusStable     ConceptStatus = "stable"
	StatusDeprecated ConceptStatus = "deprecated"
)

type SearchField string

const (
	FieldTitle       SearchField = "title"
	FieldTags        SearchField = "tags"
	FieldDescription SearchField = "description"
	FieldBody        SearchField = "body"
	FieldPath        SearchField = "path"
	FieldType        SearchField = "type"
	FieldFrontmatter SearchField = "frontmatter"
)

type ErrorCode string

const (
	ErrInvalidRequest   ErrorCode = "INVALID_REQUEST"
	ErrInvalidRoot      ErrorCode = "INVALID_ROOT"
	ErrPermissionDenied ErrorCode = "PERMISSION_DENIED"
	ErrParseError       ErrorCode = "PARSE_ERROR"
	ErrConceptNotFound  ErrorCode = "CONCEPT_NOT_FOUND"
	ErrPathNotFound     ErrorCode = "PATH_NOT_FOUND"
	ErrInternal         ErrorCode = "INTERNAL"
)

type OperationName string

const (
	OpContract  OperationName = "contract"
	OpParse     OperationName = "parse"
	OpGraph     OperationName = "graph"
	OpFind      OperationName = "find"
	OpList      OperationName = "list"
	OpShow      OperationName = "show"
	OpBacklinks OperationName = "backlinks"
	OpPath      OperationName = "path"
	OpVersion   OperationName = "version"
)

// OkfLink is a single Markdown link extracted from a concept body.
type OkfLink struct {
	Raw              string  `json:"raw"`
	Target           string  `json:"target"`
	Resolved         *string `json:"resolved,omitempty"`
	ResolvesInBundle bool    `json:"resolves_in_bundle"`
}

// OkfConcept is one parsed OKF concept document.
type OkfConcept struct {
	ID          string         `json:"id"`
	Path        string         `json:"path"`
	Frontmatter map[string]any `json:"frontmatter"`
	Type        string         `json:"type"`
	Title       *string        `json:"title,omitempty"`
	Description *string        `json:"description,omitempty"`
	Tags        []string       `json:"tags,omitempty"`
	Status      *ConceptStatus `json:"status,omitempty"`
	Body        string         `json:"body"`
	Links       []OkfLink      `json:"links"`
}

// IndexEntry is one root index.md listing entry.
type IndexEntry struct {
	Title       string  `json:"title"`
	Target      string  `json:"target"`
	Description *string `json:"description,omitempty"`
}

// ConceptValidation records per-concept conformance.
type ConceptValidation struct {
	ID             string   `json:"id"`
	HasFrontmatter bool     `json:"has_frontmatter"`
	HasType        bool     `json:"has_type"`
	Warnings       []string `json:"warnings"`
}

// Validation records bundle-level conformance.
type Validation struct {
	Valid    bool                `json:"valid"`
	Concepts []ConceptValidation `json:"concepts"`
	Warnings []string            `json:"warnings"`
}

// Bundle is the parsed OKF bundle exposed by `weave parse`.
type Bundle struct {
	Root       string       `json:"root"`
	OkfVersion *OkfVersion  `json:"okf_version,omitempty"`
	Concepts   []OkfConcept `json:"concepts"`
	Index      []IndexEntry `json:"index,omitempty"`
	Validation Validation   `json:"validation"`
}

// GraphNode is a concept node with degree metadata.
type GraphNode struct {
	ID        string         `json:"id"`
	Path      string         `json:"path"`
	Type      string         `json:"type"`
	Title     string         `json:"title"`
	Status    *ConceptStatus `json:"status,omitempty"`
	Tags      []string       `json:"tags,omitempty"`
	InDegree  int            `json:"in_degree"`
	OutDegree int            `json:"out_degree"`
}

// GraphEdge is a directed edge between concepts.
type GraphEdge struct {
	From   string  `json:"from"`
	To     string  `json:"to"`
	Source string  `json:"source"`
	Raw    *string `json:"raw,omitempty"`
}

// Graph is the full graph view, including dangling targets.
type Graph struct {
	Nodes    []GraphNode          `json:"nodes"`
	Edges    []GraphEdge          `json:"edges"`
	Dangling map[string][]OkfLink `json:"dangling"`
}

// SerializedGraphNode is the lens-facing node shape.
type SerializedGraphNode struct {
	ID        string         `json:"id"`
	Title     string         `json:"title"`
	Type      string         `json:"type"`
	Status    *ConceptStatus `json:"status,omitempty"`
	Tags      []string       `json:"tags,omitempty"`
	InDegree  int            `json:"in_degree"`
	OutDegree int            `json:"out_degree"`
}

// SerializedGraphEdge is the lens-facing edge shape.
type SerializedGraphEdge struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Source string `json:"source"`
}

// SerializedGraph is the compact graph payload embedded in the lens shell.
type SerializedGraph struct {
	Nodes []SerializedGraphNode `json:"nodes"`
	Edges []SerializedGraphEdge `json:"edges"`
}

// SearchResult is one BM25 search hit.
type SearchResult struct {
	ID        string      `json:"id"`
	Title     string      `json:"title"`
	Path      string      `json:"path"`
	Type      string      `json:"type"`
	Score     float64     `json:"score"`
	BestField SearchField `json:"best_field"`
	Snippet   *string     `json:"snippet,omitempty"`
}

type SearchResults []SearchResult

// FindResult is the CLI-facing search hit.
type FindResult struct {
	ID      string  `json:"id"`
	Title   string  `json:"title"`
	Path    string  `json:"path"`
	Type    string  `json:"type"`
	Score   float64 `json:"score"`
	Snippet *string `json:"snippet,omitempty"`
}

type FindResults []FindResult

// ListResult is one metadata-filtered concept.
type ListResult struct {
	ID     string         `json:"id"`
	Title  string         `json:"title"`
	Path   string         `json:"path"`
	Type   string         `json:"type"`
	Status *ConceptStatus `json:"status,omitempty"`
	Tags   []string       `json:"tags,omitempty"`
}

type ListResults []ListResult

// ShowLink is an outbound link on a concept page.
type ShowLink struct {
	Raw              string `json:"raw"`
	Target           string `json:"target"`
	ResolvesInBundle bool   `json:"resolves_in_bundle"`
}

// ShowResult is the detailed concept view.
type ShowResult struct {
	ID          string         `json:"id"`
	Path        string         `json:"path"`
	Type        string         `json:"type"`
	Title       *string        `json:"title,omitempty"`
	Description *string        `json:"description,omitempty"`
	Tags        []string       `json:"tags,omitempty"`
	Status      *ConceptStatus `json:"status,omitempty"`
	Body        string         `json:"body"`
	Links       []ShowLink     `json:"links"`
	Backlinks   []string       `json:"backlinks"`
}

// Backlink is a concept that references another concept.
type Backlink struct {
	From  string `json:"from"`
	Title string `json:"title"`
}

// BacklinksResult lists incoming references.
type BacklinksResult struct {
	ID        string     `json:"id"`
	Backlinks []Backlink `json:"backlinks"`
}

// PathResult is a shortest directed path, or null when disconnected.
type PathResult struct {
	From string   `json:"from"`
	To   string   `json:"to"`
	Path []string `json:"path"`
}

// OperationDescriptor describes one machine operation for discovery.
type OperationDescriptor struct {
	Op     OperationName `json:"op"`
	Fields []string      `json:"fields"`
	Result string        `json:"result"`
}

// ContractInfo is returned by `weave contract --json`.
type ContractInfo struct {
	Contract      string                `json:"contract"`
	CLI           string                `json:"cli"`
	BinaryVersion string                `json:"binary_version"`
	Operations    []OperationDescriptor `json:"operations"`
	Errors        []ErrorCode           `json:"errors"`
}

// VersionInfo is returned by `weave version --json`.
type VersionInfo struct {
	BinaryVersion string `json:"binary_version"`
	Contract      string `json:"contract"`
	CLI           string `json:"cli"`
}

// OperationRequest is the transport-neutral request shape for stdio/MCP.
type OperationRequest struct {
	ID     string        `json:"id,omitempty"`
	Op     OperationName `json:"op"`
	Path   string        `json:"path,omitempty"`
	Query  string        `json:"query,omitempty"`
	Target string        `json:"target,omitempty"`
	From   string        `json:"from,omitempty"`
	To     string        `json:"to,omitempty"`
	Types  []string      `json:"types,omitempty"`
	Tags   []string      `json:"tags,omitempty"`
	Status string        `json:"status,omitempty"`
	Limit  *int          `json:"limit,omitempty"`
	Fields []SearchField `json:"fields,omitempty"`
}

// OperationError is the stable error shape shared by CLI and future transports.
type OperationError struct {
	Code    ErrorCode      `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

// SuccessEnvelope is a successful machine response.
type SuccessEnvelope struct {
	ID       string `json:"id,omitempty"`
	Contract string `json:"contract"`
	CLI      string `json:"cli"`
	OK       bool   `json:"ok"`
	Result   any    `json:"result"`
}

// FailureEnvelope is a failed machine response.
type FailureEnvelope struct {
	ID       string         `json:"id,omitempty"`
	Contract string         `json:"contract"`
	CLI      string         `json:"cli"`
	OK       bool           `json:"ok"`
	Error    OperationError `json:"error"`
}
