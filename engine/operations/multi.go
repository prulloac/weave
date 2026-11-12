package operations

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine/extensions"
	"github.com/prulloac/weave/engine/okf"
)

type resolvedSource struct {
	Alias      string
	Path       string
	InputPath  string
	Provider   string
	Ref        string
	Commit     string
	Worktree   bool
	Repository extensions.SourceRepository
}

func (e *Executor) executeMulti(
	ctx context.Context,
	request contractv1.OperationRequest,
) any {
	sources, err := e.resolveSources(ctx, request)
	if err != nil {
		return failure(extensionErrorCode(err), err.Error(), nil)
	}

	switch request.Op {
	case contractv1.OpSearch:
		return e.multiSearch(ctx, request, sources)
	case contractv1.OpList:
		return e.multiList(ctx, request, sources)
	case contractv1.OpAudit:
		return e.multiAudit(ctx, request, sources)
	case contractv1.OpParse:
		return e.multiParse(ctx, request, sources)
	case contractv1.OpGraph:
		return e.multiGraph(ctx, request, sources)
	case contractv1.OpShow, contractv1.OpBacklinks, contractv1.OpRelate, contractv1.OpPath:
		return e.multiTarget(ctx, request, sources)
	case contractv1.OpExplain:
		return e.multiExplain(ctx, request, sources)
	case contractv1.OpIngest, contractv1.OpRebuild, contractv1.OpSync:
		return e.multiWrite(ctx, request, sources)
	default:
		return failure(contractv1.ErrInvalidRequest, "--source is not supported for this operation", map[string]any{
			"op": request.Op,
		})
	}
}

func (e *Executor) resolveSources(ctx context.Context, request contractv1.OperationRequest) ([]resolvedSource, error) {
	aliases := make(map[string]bool)
	paths := make(map[string]bool)
	sources := make([]resolvedSource, 0, len(request.Sources))
	for _, spec := range request.Sources {
		alias := strings.TrimSpace(spec.Alias)
		inputPath := strings.TrimSpace(spec.Path)
		if alias == "" || inputPath == "" {
			return nil, fmt.Errorf("source alias and path are required")
		}
		if !validSourceAlias(alias) {
			return nil, fmt.Errorf("invalid source alias %q", alias)
		}
		if aliases[alias] {
			return nil, fmt.Errorf("duplicate source alias %q", alias)
		}
		canonical := canonicalSourceIdentity(inputPath)
		if paths[canonical] {
			return nil, fmt.Errorf("duplicate source path %q", inputPath)
		}
		aliases[alias] = true
		paths[canonical] = true

		source := resolvedSource{
			Alias:     alias,
			Path:      inputPath,
			InputPath: inputPath,
			Provider:  strings.TrimSpace(spec.Provider),
			Ref:       strings.TrimSpace(spec.Ref),
		}
		switch source.Provider {
		case "", "path":
			source.Provider = "path"
		case "git":
			provider, err := resolveSourceProvider(inputPath, request.Path)
			if err != nil {
				return nil, err
			}
			sourceID := extensions.SingleSourceID(inputPath)
			if len(request.Sources) > 1 {
				sourceID = extensions.WorkspaceSourceID("", alias, inputPath)
			}
			runner := extensions.Runner{Timeout: sourceTimeout(request)}
			response, err := runner.MaterializeSource(ctx, provider, extensions.SourceRequest{
				SourceID: sourceID,
				URI:      inputPath,
				Ref:      source.Ref,
				CacheDir: sourceCacheDir(),
				Options:  extensions.SourceOptions{Worktree: spec.Worktree},
			})
			if err != nil {
				return nil, err
			}
			source.Path = response.Snapshot.Path
			source.Ref = response.Snapshot.Ref
			source.Commit = response.Snapshot.Commit
			source.Worktree = spec.Worktree
			source.Repository = response.Repository
		default:
			return nil, fmt.Errorf("unsupported source provider %q", source.Provider)
		}
		sources = append(sources, source)
	}
	return sources, nil
}

func resolveSourceProvider(inputPath, workspacePath string) (extensions.Extension, error) {
	for _, root := range []string{inputPath, workspacePath} {
		if strings.TrimSpace(root) == "" {
			continue
		}
		provider, err := extensions.Resolve(root, "git")
		if err == nil {
			return provider, nil
		}
	}
	return extensions.Resolve("", "git")
}

func canonicalSourceIdentity(value string) string {
	if strings.Contains(value, "://") || strings.HasPrefix(value, "git@") {
		return strings.TrimSpace(value)
	}
	if absolute, err := filepath.Abs(value); err == nil {
		return filepath.Clean(absolute)
	}
	return filepath.Clean(value)
}

func sourceCacheDir() string {
	if dir := strings.TrimSpace(os.Getenv("WEAVE_CACHE_DIR")); dir != "" {
		return dir
	}
	return filepath.Dir(okf.DefaultCacheDir())
}

func sourceTimeout(request contractv1.OperationRequest) time.Duration {
	if request.TimeoutMS > 0 {
		return time.Duration(request.TimeoutMS) * time.Millisecond
	}
	return 120 * time.Second
}

func validSourceAlias(alias string) bool {
	if alias == "" {
		return false
	}
	for index, char := range alias {
		switch {
		case char >= 'a' && char <= 'z':
		case char >= '0' && char <= '9':
		case char == '-' && index > 0:
		default:
			return false
		}
	}
	return true
}

func singleSourceRequest(request contractv1.OperationRequest, source resolvedSource) contractv1.OperationRequest {
	request.Path = source.Path
	request.Sources = nil
	if source.Provider != "" && source.Provider != "path" {
		request.Source = &contractv1.SourceContext{
			Alias:           source.Alias,
			Provider:        source.Provider,
			URI:             source.InputPath,
			Ref:             source.Ref,
			Commit:          source.Commit,
			SnapshotPath:    source.Path,
			RepositoryName:  source.Repository.Name,
			RepositoryTitle: source.Repository.Title,
			Description:     source.Repository.Description,
			Readme:          source.Repository.Readme,
			Host:            source.Repository.Host,
			Owner:           source.Repository.Owner,
			License:         source.Repository.License,
			Worktree:        source.Worktree,
		}
	} else {
		request.Source = nil
	}
	return request
}

func unwrap(value any) (any, *contractv1.FailureEnvelope) {
	switch envelope := value.(type) {
	case contractv1.SuccessEnvelope:
		return envelope.Result, nil
	case contractv1.FailureEnvelope:
		return nil, &envelope
	default:
		return nil, &contractv1.FailureEnvelope{
			OK: false,
			Error: contractv1.OperationError{
				Code:    contractv1.ErrInternal,
				Message: "internal operation error",
			},
		}
	}
}
