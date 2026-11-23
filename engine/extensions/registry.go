package extensions

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// FailurePolicy describes what core does when a role invocation fails.
type FailurePolicy string

const (
	// FailureFallback keeps core results and records a warning.
	FailureFallback FailurePolicy = "fallback"
	// FailureOperation fails the operation that requested the role.
	FailureOperation FailurePolicy = "fail-operation"
	// FailureRefuse refuses selection before invoking the extension.
	FailureRefuse FailurePolicy = "refuse"
)

// RoleDescriptor describes one extension role once, so discovery, conformance checks and limits
// share a single definition. The runtime methods stay typed; the descriptor names their transport
// and validation.
type RoleDescriptor struct {
	Name             string
	Protocol         string
	Timeout          time.Duration
	Failure          FailurePolicy
	CLI              string
	RequestType      string
	ResponseType     string
	MaxRequestBytes  int
	MaxResponseBytes int
	conformance      func(context.Context, Runner, Extension) error
}

// roleRegistry is the static, compiled-in role table. A new role adds one entry and its typed
// request and response validation; it does not add another discovery or invocation path.
var roleRegistry = []RoleDescriptor{
	{
		Name:             "enrichment",
		Protocol:         ProtocolVersion,
		Timeout:          DefaultTimeout,
		Failure:          FailureFallback,
		CLI:              "--with-enricher",
		RequestType:      "enrichment_request",
		ResponseType:     "enrichment_response",
		MaxRequestBytes:  1 << 20,
		MaxResponseBytes: 1 << 20,
	},
	{
		Name:             "project",
		Protocol:         ProtocolVersion,
		Timeout:          DefaultTimeout,
		Failure:          FailureOperation,
		CLI:              "--with-enricher",
		RequestType:      "project_analysis_request",
		ResponseType:     "project_analysis_response",
		MaxRequestBytes:  MaxProjectAnalysisBytes,
		MaxResponseBytes: MaxProjectAnalysisBytes,
	},
	{
		Name:             "source",
		Protocol:         SourceProtocolVersion,
		Timeout:          DefaultTimeout,
		Failure:          FailureOperation,
		CLI:              "--git",
		RequestType:      "source_request",
		ResponseType:     "source_response",
		MaxRequestBytes:  MaxSourceResponseBytes,
		MaxResponseBytes: MaxSourceResponseBytes,
	},
	{
		Name:             "rerank",
		Protocol:         ProtocolVersion,
		Timeout:          DefaultTimeout,
		Failure:          FailureFallback,
		CLI:              "--rerank",
		RequestType:      "rerank_request",
		ResponseType:     "rerank_response",
		MaxRequestBytes:  1 << 20,
		MaxResponseBytes: 1 << 20,
	},
	{
		Name:             "scorer",
		Protocol:         ProtocolVersion,
		Timeout:          10 * time.Second,
		Failure:          FailureFallback,
		CLI:              "--scorer",
		RequestType:      "scorer_request",
		ResponseType:     "scorer_response",
		MaxRequestBytes:  MaxScorerRequest,
		MaxResponseBytes: 1 << 20,
	},
	{
		Name:             "embedder",
		Protocol:         ProtocolVersion,
		Timeout:          DefaultTimeout,
		Failure:          FailureOperation,
		CLI:              "--with-embedder, --embedder",
		RequestType:      "embedding_request",
		ResponseType:     "embedding_response",
		MaxRequestBytes:  1 << 20,
		MaxResponseBytes: 1 << 20,
	},
	{
		Name:             "tokenizer",
		Protocol:         ProtocolVersion,
		Timeout:          DefaultTimeout,
		Failure:          FailureOperation,
		CLI:              "--tokenizer",
		RequestType:      "tokenizer_request",
		ResponseType:     "tokenizer_response",
		MaxRequestBytes:  1 << 20,
		MaxResponseBytes: 1 << 20,
	},
}

func init() {
	for index := range roleRegistry {
		switch roleRegistry[index].Name {
		case "enrichment":
			roleRegistry[index].conformance = func(ctx context.Context, runner Runner, extension Extension) error {
				_, err := runner.Enrich(ctx, extension, cannedEnrichmentRequest())
				return err
			}
		case "project":
			roleRegistry[index].conformance = func(ctx context.Context, runner Runner, extension Extension) error {
				_, err := runner.ProjectAnalysis(ctx, extension, cannedProjectRequest())
				return err
			}
		case "source":
			roleRegistry[index].conformance = func(ctx context.Context, runner Runner, extension Extension) error {
				_, err := runner.MaterializeSource(ctx, extension, cannedSourceRequest())
				return err
			}
		case "rerank":
			roleRegistry[index].conformance = func(ctx context.Context, runner Runner, extension Extension) error {
				_, err := runner.Rerank(ctx, extension, cannedRerankRequest())
				return err
			}
		case "scorer":
			roleRegistry[index].conformance = func(ctx context.Context, runner Runner, extension Extension) error {
				_, err := runner.Score(ctx, extension, cannedScorerRequest())
				return err
			}
		case "embedder":
			roleRegistry[index].conformance = func(ctx context.Context, runner Runner, extension Extension) error {
				_, err := runner.Embed(ctx, extension, cannedEmbeddingRequest())
				return err
			}
		case "tokenizer":
			roleRegistry[index].conformance = func(ctx context.Context, runner Runner, extension Extension) error {
				_, err := runner.Tokenize(ctx, extension, cannedTokenizerRequest())
				return err
			}
		}
	}
}

// Roles returns the registered roles in declaration order.
func Roles() []RoleDescriptor {
	return append([]RoleDescriptor(nil), roleRegistry...)
}

// Role returns the descriptor for one role name.
func Role(name string) (RoleDescriptor, bool) {
	for _, role := range roleRegistry {
		if role.Name == name {
			return role, true
		}
	}
	return RoleDescriptor{}, false
}

// RoleNames returns the registered role names in declaration order.
func RoleNames() []string {
	names := make([]string, 0, len(roleRegistry))
	for _, role := range roleRegistry {
		names = append(names, role.Name)
	}
	return names
}

// RunRoleCheck invokes the canonical conformance request for one role.
func (r Runner) RunRoleCheck(ctx context.Context, extension Extension, role string) error {
	descriptor, ok := Role(role)
	if !ok {
		return fmt.Errorf("%w: unknown role %q", ErrProtocol, role)
	}
	return descriptor.conformance(ctx, r, extension)
}

// invokeRole sends one role payload through the shared bounded invocation path.
func (r Runner) invokeRole(ctx context.Context, extension Extension, role string, payload, target any) error {
	descriptor, ok := Role(role)
	if !ok {
		return fmt.Errorf("%w: unknown role %q", ErrProtocol, role)
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrProtocol, err)
	}
	if descriptor.MaxRequestBytes > 0 && len(data) > descriptor.MaxRequestBytes {
		return fmt.Errorf("%w: %s %s request exceeds %d bytes", ErrProtocol, extension.Name, role, descriptor.MaxRequestBytes)
	}
	limit := descriptor.MaxResponseBytes
	if limit <= 0 {
		limit = maxOutput
	}
	return r.invokeVersionWithLimit(ctx, extension, payload, target, limit, descriptor.Protocol)
}

// SelectRole resolves, trusts and validates one extension for a role, then enforces its requirements
// and descriptor minimum. It does not invoke the extension.
func SelectRole(
	ctx context.Context,
	root, name, role string,
	timeout time.Duration,
) (Extension, Manifest, []string, error) {
	if _, ok := Role(role); !ok {
		return Extension{}, Manifest{}, nil, fmt.Errorf("%w: unknown role %q", ErrProtocol, role)
	}
	extension, err := Resolve(root, name)
	if err != nil {
		return Extension{}, Manifest{}, nil, err
	}
	if err := CheckTrusted(root, extension); err != nil {
		return Extension{}, Manifest{}, nil, err
	}
	manifest, err := DescribeWithTimeout(ctx, extension, timeout)
	if err != nil {
		return extension, Manifest{}, nil, err
	}
	if len(manifest.Roles) > 0 {
		declared := false
		for _, value := range manifest.Roles {
			if value == role {
				declared = true
				break
			}
		}
		if !declared {
			return Extension{}, Manifest{}, nil, fmt.Errorf("%w: %s does not declare role %q", ErrProtocol, name, role)
		}
	}
	warnings, err := CheckRequirements(ctx, root, name, manifest.Requires, timeout)
	if err != nil {
		return Extension{}, manifest, warnings, err
	}
	if role == "source" || role == "project" {
		warning, failure := CheckDescriptorVersion(extension.Name, manifest.Version)
		if warning != "" {
			warnings = append(warnings, warning)
		}
		if failure != nil {
			return Extension{}, manifest, warnings, failure
		}
	}
	return extension, manifest, warnings, nil
}
