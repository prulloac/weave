// Package mcp exposes Weave operations as an in-process tool harness.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine/operations"
)

// Options configures an MCP server.
type Options struct {
	Bundle         string
	Sources        []contractv1.SourceSpec
	DB             string
	TopKDefault    int
	AllowSensitive bool
}

// ToolDescriptor describes one MCP tool.
type ToolDescriptor struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

// ResourceDescriptor describes one static MCP resource.
type ResourceDescriptor struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description"`
	MimeType    string `json:"mimeType"`
}

// ResourceTemplate describes one parameterized MCP resource.
type ResourceTemplate struct {
	URITemplate string `json:"uriTemplate"`
	Name        string `json:"name"`
	Description string `json:"description"`
	MimeType    string `json:"mimeType"`
}

// ResourceContent is one resolved resource payload.
type ResourceContent struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType"`
	Text     string `json:"text"`
}

// Error is a structured MCP tool error.
type Error struct {
	Code    contractv1.ErrorCode `json:"code"`
	Message string               `json:"message"`
}

func (e *Error) Error() string { return e.Message }

// Server is a bundle-bound MCP tool dispatcher.
type Server struct {
	executor       *operations.Executor
	bundle         string
	sources        []contractv1.SourceSpec
	sourcePaths    map[string]string
	db             string
	topKDefault    int
	allowSensitive bool
	tools          []ToolDescriptor
}

// New creates an in-process MCP tool server.
func New(executor *operations.Executor, options Options) (*Server, error) {
	if executor == nil {
		return nil, fmt.Errorf("executor is required")
	}
	if options.Bundle == "" && len(options.Sources) == 0 {
		return nil, fmt.Errorf("bundle is required")
	}
	topK := options.TopKDefault
	if topK == 0 {
		topK = 10
	}
	if topK < 1 || topK > 100 {
		return nil, fmt.Errorf("top_k default must be between 1 and 100")
	}
	sources := append([]contractv1.SourceSpec(nil), options.Sources...)
	paths := make(map[string]string, len(sources))
	aliases := make(map[string]bool, len(sources))
	for _, source := range sources {
		if source.Alias == "" || source.Path == "" {
			return nil, fmt.Errorf("source alias and path are required")
		}
		if aliases[source.Alias] {
			return nil, fmt.Errorf("duplicate source alias %q", source.Alias)
		}
		aliases[source.Alias] = true
		paths[source.Alias] = source.Path
	}
	primary := options.Bundle
	if len(sources) > 0 {
		primary = sources[0].Path
	}
	if primary == "" {
		return nil, fmt.Errorf("bundle is required")
	}
	return &Server{
		executor:       executor,
		bundle:         primary,
		sources:        sources,
		sourcePaths:    paths,
		db:             options.DB,
		topKDefault:    topK,
		allowSensitive: options.AllowSensitive,
		tools:          toolDescriptors(),
	}, nil
}

// Tools returns the advertised tool descriptors.
func (s *Server) Tools() []ToolDescriptor {
	return append([]ToolDescriptor(nil), s.tools...)
}

// Call dispatches one tool invocation.
func (s *Server) Call(ctx context.Context, name string, arguments map[string]any) (any, error) {
	if arguments == nil {
		arguments = map[string]any{}
	}

	var request contractv1.OperationRequest
	switch name {
	case "weave_contract":
		request = contractv1.OperationRequest{Op: contractv1.OpContract}
	case "weave_search":
		query, err := requiredString(arguments, "query")
		if err != nil {
			return nil, err
		}
		mode, err := optionalString(arguments, "mode")
		if err != nil {
			return nil, err
		}
		topK, err := s.topKArgument(arguments)
		if err != nil {
			return nil, err
		}
		request = contractv1.OperationRequest{
			Op:    contractv1.OpSearch,
			Query: query,
			Mode:  contractv1.SearchMode(mode),
			TopK:  topK,
		}
	case "weave_explain":
		query, err := requiredString(arguments, "query")
		if err != nil {
			return nil, err
		}
		id, err := optionalString(arguments, "id")
		if err != nil {
			return nil, err
		}
		mode, err := optionalString(arguments, "mode")
		if err != nil {
			return nil, err
		}
		topK, err := s.topKArgument(arguments)
		if err != nil {
			return nil, err
		}
		request = contractv1.OperationRequest{
			Op:     contractv1.OpExplain,
			Query:  query,
			Target: id,
			Mode:   contractv1.SearchMode(mode),
			TopK:   topK,
		}
	case "weave_relate":
		id, err := requiredString(arguments, "id")
		if err != nil {
			return nil, err
		}
		depth, err := optionalInt(arguments, "depth")
		if err != nil {
			return nil, err
		}
		direction, err := optionalString(arguments, "direction")
		if err != nil {
			return nil, err
		}
		request = contractv1.OperationRequest{
			Op:        contractv1.OpRelate,
			Target:    id,
			Depth:     depth,
			Direction: contractv1.RelateDirection(direction),
		}
	case "weave_audit":
		failOn, err := optionalString(arguments, "fail_on")
		if err != nil {
			return nil, err
		}
		request = contractv1.OperationRequest{
			Op:     contractv1.OpAudit,
			FailOn: contractv1.AuditFailOn(failOn),
		}
	case "weave_list":
		types, err := optionalStringSlice(arguments, "types")
		if err != nil {
			return nil, err
		}
		tags, err := optionalStringSlice(arguments, "tags")
		if err != nil {
			return nil, err
		}
		status, err := optionalString(arguments, "status")
		if err != nil {
			return nil, err
		}
		limit, err := optionalInt(arguments, "limit")
		if err != nil {
			return nil, err
		}
		request = contractv1.OperationRequest{
			Op:     contractv1.OpList,
			Types:  types,
			Tags:   tags,
			Status: status,
			Limit:  limit,
		}
	case "weave_show", "weave_backlinks":
		id, err := requiredString(arguments, "id")
		if err != nil {
			return nil, err
		}
		op := contractv1.OpShow
		decrypt := false
		if name == "weave_backlinks" {
			op = contractv1.OpBacklinks
		} else {
			decrypt = s.allowSensitive
		}
		request = contractv1.OperationRequest{Op: op, Target: id, Decrypt: decrypt}
	case "weave_path":
		from, err := requiredString(arguments, "from")
		if err != nil {
			return nil, err
		}
		to, err := requiredString(arguments, "to")
		if err != nil {
			return nil, err
		}
		request = contractv1.OperationRequest{Op: contractv1.OpPath, From: from, To: to}
	default:
		return nil, &Error{
			Code:    contractv1.ErrInvalidRequest,
			Message: "unknown tool: " + name,
		}
	}

	request.Path = s.bundle
	request.DB = s.db
	request.NoDB = s.db == ""
	request.VerifyKeys = false

	if name != "weave_contract" {
		sourceName, err := optionalString(arguments, "source")
		if err != nil {
			return nil, err
		}
		if sourceName != "" {
			path, ok := s.sourcePaths[sourceName]
			if !ok {
				return nil, &Error{
					Code:    contractv1.ErrInvalidRequest,
					Message: "unknown source: " + sourceName,
				}
			}
			request.Path = path
			request.Sources = nil
			request.Target = stripSourcePrefix(request.Target, sourceName)
			request.From = stripSourcePrefix(request.From, sourceName)
			request.To = stripSourcePrefix(request.To, sourceName)
			value, err := s.execute(ctx, request)
			if err != nil {
				return nil, err
			}
			return annotateResult(value, sourceName), nil
		}
		if len(s.sources) > 1 {
			request.Path = s.bundle
			request.Sources = s.sources
		}
	}

	return s.execute(ctx, request)
}

func stripSourcePrefix(target, alias string) string {
	prefix := alias + ":"
	if strings.HasPrefix(target, prefix) {
		return strings.TrimPrefix(target, prefix)
	}
	return target
}

func annotateResult(value any, alias string) any {
	switch typed := value.(type) {
	case *contractv1.ShowResult:
		typed.Source = alias
	case *contractv1.PathResult:
		typed.Source = alias
	case contractv1.RelateResult:
		typed.Source = alias
		for index := range typed.Nodes {
			typed.Nodes[index].Source = alias
		}
		return typed
	case contractv1.ExplainResult:
		typed.Source = alias
		return typed
	case contractv1.SearchResponse:
		for index := range typed.Keyword {
			typed.Keyword[index].Source = alias
		}
		for index := range typed.Fuzzy {
			typed.Fuzzy[index].Source = alias
		}
		for index := range typed.Hybrid {
			typed.Hybrid[index].Source = alias
		}
		for index := range typed.Lexical {
			typed.Lexical[index].Source = alias
		}
		return typed
	case []contractv1.ListResult:
		for index := range typed {
			typed[index].Source = alias
		}
		return typed
	case contractv1.AuditReport:
		for index := range typed.Findings {
			typed.Findings[index].Source = alias
		}
		return typed
	}
	return value
}

func (s *Server) execute(ctx context.Context, request contractv1.OperationRequest) (any, error) {
	if request.Path == "" {
		request.Path = s.bundle
	}
	request.DB = s.db
	request.NoDB = s.db == ""
	value := s.executor.Execute(ctx, request)
	switch envelope := value.(type) {
	case contractv1.SuccessEnvelope:
		return envelope.Result, nil
	case contractv1.FailureEnvelope:
		return nil, &Error{Code: envelope.Error.Code, Message: envelope.Error.Message}
	default:
		return nil, &Error{Code: contractv1.ErrInternal, Message: "internal tool error"}
	}
}

// Resources returns the static resource descriptors.
func (s *Server) Resources() []ResourceDescriptor {
	return []ResourceDescriptor{
		{
			URI:         "weave://bundle/index",
			Name:        "Bundle index",
			Description: "Root index entries for the bundle.",
			MimeType:    "application/json",
		},
		{
			URI:         "weave://bundle/stats",
			Name:        "Bundle stats",
			Description: "Bundle counts and validation summary.",
			MimeType:    "application/json",
		},
	}
}

// ResourceTemplates returns the parameterized resource descriptors.
func (s *Server) ResourceTemplates() []ResourceTemplate {
	return []ResourceTemplate{
		{
			URITemplate: "weave://concept/{id}",
			Name:        "Concept",
			Description: "Concept metadata and body.",
			MimeType:    "application/json",
		},
		{
			URITemplate: "weave://graph/neighborhood/{id}",
			Name:        "Graph neighborhood",
			Description: "Bounded graph neighborhood for a concept.",
			MimeType:    "application/json",
		},
		{
			URITemplate: "weave://source/{source}/bundle/index",
			Name:        "Source bundle index",
			Description: "Root index entries for one source.",
			MimeType:    "application/json",
		},
		{
			URITemplate: "weave://source/{source}/bundle/stats",
			Name:        "Source bundle stats",
			Description: "Bundle counts and validation summary for one source.",
			MimeType:    "application/json",
		},
		{
			URITemplate: "weave://source/{source}/concept/{id}",
			Name:        "Source concept",
			Description: "Concept metadata and body from one source.",
			MimeType:    "application/json",
		},
		{
			URITemplate: "weave://source/{source}/graph/neighborhood/{id}",
			Name:        "Source graph neighborhood",
			Description: "Bounded graph neighborhood for a concept in one source.",
			MimeType:    "application/json",
		},
	}
}

// ReadResource resolves one resource URI.
func (s *Server) ReadResource(ctx context.Context, uri string) (ResourceContent, error) {
	payload, err := s.resourcePayload(ctx, uri)
	if err != nil {
		return ResourceContent{}, err
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return ResourceContent{}, &Error{Code: contractv1.ErrInternal, Message: err.Error()}
	}
	return ResourceContent{
		URI:      uri,
		MimeType: "application/json",
		Text:     string(data),
	}, nil
}

func (s *Server) resourcePayload(ctx context.Context, uri string) (any, error) {
	path := s.bundle
	rest := uri
	sourceAlias := ""
	if strings.HasPrefix(uri, "weave://source/") {
		tail := strings.TrimPrefix(uri, "weave://source/")
		slash := strings.Index(tail, "/")
		if slash <= 0 {
			return nil, &Error{Code: contractv1.ErrInvalidRequest, Message: "resource not found: " + uri}
		}
		alias := tail[:slash]
		resolved, exists := s.sourcePaths[alias]
		if !exists {
			return nil, &Error{Code: contractv1.ErrInvalidRequest, Message: "resource not found: " + uri}
		}
		path = resolved
		rest = "weave://" + tail[slash+1:]
		sourceAlias = alias
	}

	switch rest {
	case "weave://bundle/index":
		value, err := s.execute(ctx, contractv1.OperationRequest{Op: contractv1.OpParse, Path: path})
		if err != nil {
			return nil, err
		}
		bundle, ok := value.(*contractv1.Bundle)
		if !ok {
			return nil, &Error{Code: contractv1.ErrInternal, Message: "unexpected bundle payload"}
		}
		return bundle.Index, nil
	case "weave://bundle/stats":
		value, err := s.execute(ctx, contractv1.OperationRequest{Op: contractv1.OpParse, Path: path})
		if err != nil {
			return nil, err
		}
		bundle, ok := value.(*contractv1.Bundle)
		if !ok {
			return nil, &Error{Code: contractv1.ErrInternal, Message: "unexpected bundle payload"}
		}
		graphValue, err := s.execute(ctx, contractv1.OperationRequest{Op: contractv1.OpGraph, Path: path})
		if err != nil {
			return nil, err
		}
		view, ok := graphValue.(contractv1.Graph)
		if !ok {
			return nil, &Error{Code: contractv1.ErrInternal, Message: "unexpected graph payload"}
		}
		orphans := 0
		for _, node := range view.Nodes {
			if node.InDegree == 0 && node.OutDegree == 0 {
				orphans++
			}
		}
		version := ""
		if bundle.OkfVersion != nil {
			version = string(*bundle.OkfVersion)
		}
		return map[string]any{
			"concepts":      len(bundle.Concepts),
			"edges":         len(view.Edges),
			"orphans":       orphans,
			"index_entries": len(bundle.Index),
			"valid":         bundle.Validation.Valid,
			"okf_version":   version,
		}, nil
	}

	for _, prefix := range []string{"weave://concept/", "weave://graph/neighborhood/"} {
		if !strings.HasPrefix(rest, prefix) {
			continue
		}
		id, err := url.PathUnescape(strings.TrimPrefix(rest, prefix))
		if err != nil || id == "" {
			return nil, &Error{Code: contractv1.ErrInvalidRequest, Message: "resource not found: " + uri}
		}
		request := contractv1.OperationRequest{Path: path, Target: id}
		if prefix == "weave://concept/" {
			request.Op = contractv1.OpShow
		} else {
			request.Op = contractv1.OpRelate
		}
		value, err := s.execute(ctx, request)
		if err != nil {
			return nil, err
		}
		if sourceAlias != "" {
			return annotateResult(value, sourceAlias), nil
		}
		return value, nil
	}

	return nil, &Error{Code: contractv1.ErrInvalidRequest, Message: "resource not found: " + uri}
}

func (s *Server) topKArgument(arguments map[string]any) (*int, error) {
	topK, err := optionalInt(arguments, "top_k")
	if err != nil {
		return nil, err
	}
	if topK == nil {
		value := s.topKDefault
		return &value, nil
	}
	if *topK < 1 || *topK > 100 {
		return nil, invalid("top_k must be between 1 and 100")
	}
	return topK, nil
}

func requiredString(arguments map[string]any, name string) (string, error) {
	value, ok := arguments[name]
	if !ok || value == nil {
		return "", invalid(name + " is required")
	}
	text, ok := value.(string)
	if !ok {
		return "", invalid(name + " must be a string")
	}
	return text, nil
}

func optionalString(arguments map[string]any, name string) (string, error) {
	value, ok := arguments[name]
	if !ok || value == nil {
		return "", nil
	}
	text, ok := value.(string)
	if !ok {
		return "", invalid(name + " must be a string")
	}
	return text, nil
}

func optionalStringSlice(arguments map[string]any, name string) ([]string, error) {
	value, ok := arguments[name]
	if !ok || value == nil {
		return nil, nil
	}
	raw, ok := value.([]any)
	if !ok {
		return nil, invalid(name + " must be an array of strings")
	}
	values := make([]string, 0, len(raw))
	for _, entry := range raw {
		text, ok := entry.(string)
		if !ok {
			return nil, invalid(name + " must be an array of strings")
		}
		values = append(values, text)
	}
	return values, nil
}

func optionalInt(arguments map[string]any, name string) (*int, error) {
	value, ok := arguments[name]
	if !ok || value == nil {
		return nil, nil
	}
	switch typed := value.(type) {
	case int:
		result := typed
		return &result, nil
	case int64:
		result := int(typed)
		return &result, nil
	case float64:
		if typed != float64(int(typed)) {
			return nil, invalid(name + " must be an integer")
		}
		result := int(typed)
		return &result, nil
	default:
		return nil, invalid(name + " must be an integer")
	}
}

func invalid(message string) *Error {
	return &Error{Code: contractv1.ErrInvalidRequest, Message: message}
}

func toolDescriptors() []ToolDescriptor {
	tools := []ToolDescriptor{
		{
			Name:        "weave_search",
			Description: "Search a bundle by keyword, fuzzy, hybrid or BM25 ranking.",
			InputSchema: objectSchema(
				map[string]any{
					"query": map[string]any{"type": "string"},
					"mode":  map[string]any{"type": "string", "enum": []any{"keyword", "fuzzy", "hybrid", "bm25"}},
					"top_k": map[string]any{"type": "integer", "minimum": 1, "maximum": 100},
				},
				[]any{"query"},
			),
		},
		{
			Name:        "weave_explain",
			Description: "Explain a search result.",
			InputSchema: objectSchema(
				map[string]any{
					"query": map[string]any{"type": "string"},
					"id":    map[string]any{"type": "string"},
					"mode":  map[string]any{"type": "string", "enum": []any{"keyword", "fuzzy", "hybrid", "bm25"}},
					"top_k": map[string]any{"type": "integer", "minimum": 1, "maximum": 100},
				},
				[]any{"query"},
			),
		},
		{
			Name:        "weave_relate",
			Description: "Return a bounded graph neighborhood.",
			InputSchema: objectSchema(
				map[string]any{
					"id":        map[string]any{"type": "string"},
					"depth":     map[string]any{"type": "integer", "minimum": 0, "maximum": 5},
					"direction": map[string]any{"type": "string", "enum": []any{"out", "in", "both"}},
				},
				[]any{"id"},
			),
		},
		{
			Name:        "weave_audit",
			Description: "Audit bundle conformance and graph quality.",
			InputSchema: objectSchema(
				map[string]any{
					"fail_on": map[string]any{"type": "string", "enum": []any{"error", "warning", "none"}},
				},
				[]any{},
			),
		},
		{
			Name:        "weave_list",
			Description: "List concepts with optional metadata filters.",
			InputSchema: objectSchema(
				map[string]any{
					"types":  map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					"tags":   map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					"status": map[string]any{"type": "string"},
					"limit":  map[string]any{"type": "integer", "minimum": 1},
				},
				[]any{},
			),
		},
		{
			Name:        "weave_show",
			Description: "Show one concept.",
			InputSchema: objectSchema(
				map[string]any{"id": map[string]any{"type": "string"}},
				[]any{"id"},
			),
		},
		{
			Name:        "weave_backlinks",
			Description: "List incoming references for a concept.",
			InputSchema: objectSchema(
				map[string]any{"id": map[string]any{"type": "string"}},
				[]any{"id"},
			),
		},
		{
			Name:        "weave_path",
			Description: "Find a shortest path between concepts.",
			InputSchema: objectSchema(
				map[string]any{
					"from": map[string]any{"type": "string"},
					"to":   map[string]any{"type": "string"},
				},
				[]any{"from", "to"},
			),
		},
		{
			Name:        "weave_contract",
			Description: "Return the operations contract.",
			InputSchema: objectSchema(map[string]any{}, []any{}),
		},
	}
	for index := range tools {
		if tools[index].Name == "weave_contract" {
			continue
		}
		properties, ok := tools[index].InputSchema["properties"].(map[string]any)
		if !ok {
			continue
		}
		properties["source"] = map[string]any{"type": "string"}
	}
	return tools
}

func objectSchema(properties map[string]any, required []any) map[string]any {
	return map[string]any{
		"type":                 "object",
		"properties":           properties,
		"required":             required,
		"additionalProperties": false,
	}
}
