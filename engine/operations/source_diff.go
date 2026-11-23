package operations

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine/extensions"
)

// ErrSourceUnsupported marks a provider that does not advertise the diff capability.
var ErrSourceUnsupported = errors.New("source provider does not support diff")

// SourceDiffOptions carries the resolved source identity for one comparison.
type SourceDiffOptions struct {
	WorkspaceRoot string
	Alias         string
	Provider      string
	URI           string
	SourceID      string
	CacheDir      string
	From          string
	To            string
	Format        string
	Path          string
	Context       int
	Timeout       time.Duration
	Permissive    bool
	Warn          func(string)
}

// DiffSource resolves a source provider, checks its requirements and capability, and returns the
// comparison between two refs.
func DiffSource(ctx context.Context, options SourceDiffOptions) (contractv1.SourceDiffResult, error) {
	if strings.TrimSpace(options.Provider) == "" || options.Provider == "path" {
		return contractv1.SourceDiffResult{}, ErrSourceUnsupported
	}
	if strings.TrimSpace(options.From) == "" || strings.TrimSpace(options.To) == "" {
		return contractv1.SourceDiffResult{}, fmt.Errorf("%w: --from and --to are required", extensions.ErrProtocol)
	}
	if options.Format == "" {
		options.Format = "changes"
	}
	if options.Format != "changes" && options.Format != "patch" {
		return contractv1.SourceDiffResult{}, fmt.Errorf("%w: --format must be json or patch", extensions.ErrProtocol)
	}
	timeout := options.Timeout
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	extension, err := extensions.Resolve(options.WorkspaceRoot, options.Provider)
	if err != nil {
		return contractv1.SourceDiffResult{}, err
	}
	manifest, err := extensions.DescribeWithTimeout(ctx, extension, timeout)
	if err != nil {
		return contractv1.SourceDiffResult{}, err
	}
	if !manifest.SupportsCapability("diff") {
		return contractv1.SourceDiffResult{}, fmt.Errorf("%w: %s", ErrSourceUnsupported, options.Provider)
	}
	warnings, err := extensions.CheckRequirements(ctx, options.WorkspaceRoot, options.Provider, manifest.Requires, timeout)
	for _, warning := range warnings {
		if options.Warn != nil {
			options.Warn(warning)
		}
	}
	if err != nil {
		return contractv1.SourceDiffResult{}, err
	}
	if warning, failure := extensions.CheckDescriptorVersion(extension.Name, manifest.Version); failure != nil {
		return contractv1.SourceDiffResult{}, failure
	} else if warning != "" && options.Warn != nil {
		options.Warn(warning)
	}

	runner := extensions.Runner{Timeout: timeout}
	response, err := runner.DiffSource(ctx, extension, extensions.SourceDiffRequest{
		SourceID: options.SourceID,
		URI:      options.URI,
		CacheDir: options.CacheDir,
		From:     options.From,
		To:       options.To,
		Options: extensions.SourceDiffOptions{
			Format:     options.Format,
			Context:    options.Context,
			Permissive: options.Permissive,
		},
	})
	if err != nil {
		return contractv1.SourceDiffResult{}, err
	}

	matcher, err := sourcePathMatcher(options.Path)
	if err != nil {
		return contractv1.SourceDiffResult{}, err
	}
	result := contractv1.SourceDiffResult{
		From:      response.From,
		To:        response.To,
		Truncated: response.Truncated,
		Changes:   make([]contractv1.SourceDiffChange, 0, len(response.Changes)),
	}
	for _, change := range response.Changes {
		if matcher != nil && !matcher.MatchString(change.Path) {
			continue
		}
		result.Changes = append(result.Changes, contractv1.SourceDiffChange{
			Kind:         change.Kind,
			Path:         change.Path,
			PreviousPath: change.PreviousPath,
			Binary:       change.Binary,
			Patch:        change.Patch,
		})
	}
	return result, nil
}

// sourcePathMatcher compiles a bundle-relative glob with `**` support.
func sourcePathMatcher(pattern string) (*regexp.Regexp, error) {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return nil, nil
	}
	var expression strings.Builder
	expression.WriteString("^")
	for index := 0; index < len(pattern); index++ {
		character := pattern[index]
		switch character {
		case '*':
			if index+1 < len(pattern) && pattern[index+1] == '*' {
				index++
				expression.WriteString(".*")
				continue
			}
			expression.WriteString("[^/]*")
		case '?':
			expression.WriteString("[^/]")
		default:
			expression.WriteString(regexp.QuoteMeta(string(character)))
		}
	}
	expression.WriteString("$")
	return regexp.Compile(expression.String())
}
