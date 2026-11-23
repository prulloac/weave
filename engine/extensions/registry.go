package extensions

import (
	"context"
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
	Name        string
	Protocol    string
	Timeout     time.Duration
	Failure     FailurePolicy
	CLI         string
	conformance func(context.Context, Runner, Extension) error
}

// roleRegistry is the static, compiled-in role table. A new role adds one entry and its typed
// request and response validation; it does not add another discovery or invocation path.
var roleRegistry = []RoleDescriptor{
	{
		Name:     "enrichment",
		Protocol: ProtocolVersion,
		Timeout:  DefaultTimeout,
		Failure:  FailureFallback,
		CLI:      "--with-enricher",
		conformance: func(ctx context.Context, runner Runner, extension Extension) error {
			_, err := runner.Enrich(ctx, extension, cannedEnrichmentRequest())
			return err
		},
	},
	{
		Name:     "project",
		Protocol: ProtocolVersion,
		Timeout:  DefaultTimeout,
		Failure:  FailureOperation,
		CLI:      "--with-enricher",
		conformance: func(ctx context.Context, runner Runner, extension Extension) error {
			_, err := runner.ProjectAnalysis(ctx, extension, cannedProjectRequest())
			return err
		},
	},
	{
		Name:     "source",
		Protocol: SourceProtocolVersion,
		Timeout:  DefaultTimeout,
		Failure:  FailureOperation,
		CLI:      "--git",
		conformance: func(ctx context.Context, runner Runner, extension Extension) error {
			_, err := runner.MaterializeSource(ctx, extension, cannedSourceRequest())
			return err
		},
	},
	{
		Name:     "rerank",
		Protocol: ProtocolVersion,
		Timeout:  DefaultTimeout,
		Failure:  FailureFallback,
		CLI:      "--rerank",
		conformance: func(ctx context.Context, runner Runner, extension Extension) error {
			_, err := runner.Rerank(ctx, extension, cannedRerankRequest())
			return err
		},
	},
	{
		Name:     "scorer",
		Protocol: ProtocolVersion,
		Timeout:  10 * time.Second,
		Failure:  FailureFallback,
		CLI:      "--scorer",
		conformance: func(ctx context.Context, runner Runner, extension Extension) error {
			_, err := runner.Score(ctx, extension, cannedScorerRequest())
			return err
		},
	},
	{
		Name:     "embedder",
		Protocol: ProtocolVersion,
		Timeout:  DefaultTimeout,
		Failure:  FailureOperation,
		CLI:      "--with-embedder, --embedder",
		conformance: func(ctx context.Context, runner Runner, extension Extension) error {
			_, err := runner.Embed(ctx, extension, cannedEmbeddingRequest())
			return err
		},
	},
	{
		Name:     "tokenizer",
		Protocol: ProtocolVersion,
		Timeout:  DefaultTimeout,
		Failure:  FailureOperation,
		CLI:      "--tokenizer",
		conformance: func(ctx context.Context, runner Runner, extension Extension) error {
			_, err := runner.Tokenize(ctx, extension, cannedTokenizerRequest())
			return err
		},
	},
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
