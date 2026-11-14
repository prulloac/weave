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
	sqlitestore "github.com/prulloac/weave/storage/sqlite"
)

type resolvedSource struct {
	Alias              string
	Path               string
	InputPath          string
	Provider           string
	Ref                string
	Commit             string
	Worktree           bool
	Repository         extensions.SourceRepository
	Projects           []extensions.SourceProject
	Commits            []extensions.SourceCommit
	Documents          []extensions.SyntheticDocument
	Relations          []extensions.SyntheticRelation
	Artifacts          []extensions.SyntheticArtifact
	Metadata           map[string]any
	DocumentNamespaces []string
	ArtifactNamespaces []string
	RelationKinds      []string
	DefaultRelations   []string
	Warnings           []string
}

func (e *Executor) executeMulti(
	ctx context.Context,
	request contractv1.OperationRequest,
) any {
	var (
		sources []resolvedSource
		err     error
	)
	switch request.Op {
	case contractv1.OpIngest, contractv1.OpRebuild, contractv1.OpSync:
		if request.Permissive {
			sources, err = e.resolveWriteSources(ctx, request)
		} else {
			sources, err = e.resolveSources(ctx, request)
		}
	default:
		sources, err = e.resolveSources(ctx, request)
	}
	if err != nil {
		return failure(extensionErrorCode(err), err.Error(), nil)
	}
	if len(sources) == 0 && (request.Op == contractv1.OpIngest || request.Op == contractv1.OpRebuild || request.Op == contractv1.OpSync) {
		return failure(contractv1.ErrExtensionFailed, "all sources are degraded", nil)
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
			InputPath: canonicalSourceIdentity(inputPath),
			Provider:  strings.TrimSpace(spec.Provider),
			Ref:       strings.TrimSpace(spec.Ref),
		}
		switch source.Provider {
		case "", "path":
			source.Provider = "path"
		case "git":
			provider, manifest, err := resolveSourceProvider(ctx, inputPath, request.Path)
			if err != nil {
				return nil, err
			}
			sourceID := extensions.SingleSourceID(source.InputPath)
			if request.WorkspaceRoot != "" {
				sourceID = extensions.WorkspaceSourceID(request.WorkspaceRoot, alias, source.InputPath)
			}
			runner := extensions.Runner{Timeout: sourceTimeout(request)}
			response, err := runner.MaterializeSource(ctx, provider, extensions.SourceRequest{
				SourceID: sourceID,
				URI:      source.InputPath,
				Ref:      source.Ref,
				CacheDir: sourceCacheDir(),
				Options: extensions.SourceOptions{
					Worktree:        spec.Worktree,
					Submodules:      spec.Submodules,
					LFS:             spec.LFS,
					History:         spec.History,
					HistoryLimit:    spec.HistoryLimit,
					ProviderOptions: spec.ProviderOptions,
					Permissive:      request.Permissive,
				},
			})
			if err != nil {
				return nil, err
			}
			source.Path = response.Snapshot.Path
			source.Ref = response.Snapshot.Ref
			source.Commit = response.Snapshot.Commit
			source.Worktree = spec.Worktree
			source.Repository = response.Repository
			source.Projects = response.Projects
			source.Commits = response.Commits
			source.Documents = response.Documents
			source.Relations = response.Relations
			source.Artifacts = response.Artifacts
			source.Metadata = response.SourceMetadata
			source.DocumentNamespaces = manifest.DocumentNamespaces
			source.ArtifactNamespaces = manifest.ArtifactNamespaces
			source.RelationKinds = relationKindNames(manifest)
			source.DefaultRelations = relationDefaultNames(manifest)
			source.Warnings = response.Warnings
			for _, warning := range response.Warnings {
				e.warn("source %s: %s", alias, warning)
			}
		default:
			return nil, fmt.Errorf("unsupported source provider %q", source.Provider)
		}
		sources = append(sources, source)
	}
	return sources, nil
}

func (e *Executor) resolveWriteSources(ctx context.Context, request contractv1.OperationRequest) ([]resolvedSource, error) {
	sources := make([]resolvedSource, 0, len(request.Sources))
	for _, spec := range request.Sources {
		single := request
		single.Sources = []contractv1.SourceSpec{spec}
		resolved, err := e.resolveSources(ctx, single)
		if err != nil {
			e.warn("source %s: %v", spec.Alias, err)
			e.markSourceDegraded(ctx, request, spec)
			continue
		}
		sources = append(sources, resolved...)
	}
	return sources, nil
}

func (e *Executor) markSourceDegraded(ctx context.Context, request contractv1.OperationRequest, spec contractv1.SourceSpec) {
	dbPath, err := resolveDBPath(request.DB)
	if err != nil {
		return
	}
	store, err := sqlitestore.Open(dbPath)
	if err != nil {
		return
	}
	defer store.Close()
	if _, exists, _ := store.SourceByAlias(ctx, spec.Alias); exists {
		_ = store.SetSourceStatus(ctx, spec.Alias, sqlitestore.SourceDegraded)
		return
	}
	_ = store.UpsertSource(ctx, sqlitestore.SourceRecord{
		Alias:    spec.Alias,
		Path:     spec.Path,
		Status:   sqlitestore.SourceDegraded,
		Provider: spec.Provider,
	})
}

func resolveSourceProvider(ctx context.Context, inputPath, workspacePath string) (extensions.Extension, extensions.Manifest, error) {
	for _, root := range []string{inputPath, workspacePath} {
		if strings.TrimSpace(root) == "" {
			continue
		}
		provider, err := extensions.Resolve(root, "git")
		if err == nil {
			return validateSourceProvider(ctx, provider)
		}
	}
	provider, err := extensions.Resolve("", "git")
	if err != nil {
		return extensions.Extension{}, extensions.Manifest{}, err
	}
	return validateSourceProvider(ctx, provider)
}

func validateSourceProvider(ctx context.Context, provider extensions.Extension) (extensions.Extension, extensions.Manifest, error) {
	manifest, err := extensions.Describe(ctx, provider)
	if err != nil {
		return extensions.Extension{}, extensions.Manifest{}, err
	}
	if !manifest.SupportsSourceProvider("git") {
		return extensions.Extension{}, extensions.Manifest{}, fmt.Errorf("%w: %s does not declare the git source provider", extensions.ErrProtocol, provider.Name)
	}
	return provider, manifest, nil
}

func relationKindNames(manifest extensions.Manifest) []string {
	result := make([]string, 0, len(manifest.RelationKinds))
	for _, declaration := range manifest.RelationKinds {
		if name := strings.TrimSpace(declaration.Name); name != "" {
			result = append(result, name)
		}
	}
	return result
}

func relationDefaultNames(manifest extensions.Manifest) []string {
	result := make([]string, 0, len(manifest.RelationKinds))
	for _, declaration := range manifest.RelationKinds {
		if name := strings.TrimSpace(declaration.Name); name != "" && declaration.Default {
			result = append(result, name)
		}
	}
	return result
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
			Alias:              source.Alias,
			Provider:           source.Provider,
			URI:                source.InputPath,
			Ref:                source.Ref,
			Commit:             source.Commit,
			SnapshotPath:       source.Path,
			RepositoryName:     source.Repository.Name,
			RepositoryTitle:    source.Repository.Title,
			Description:        source.Repository.Description,
			Readme:             source.Repository.Readme,
			Host:               source.Repository.Host,
			Owner:              source.Repository.Owner,
			License:            source.Repository.License,
			HostKind:           source.Repository.HostKind,
			WebURL:             source.Repository.WebURL,
			CommitURL:          source.Repository.CommitURL,
			Worktree:           source.Worktree,
			Warnings:           source.Warnings,
			Projects:           contractSourceProjects(source.Projects),
			Commits:            contractSourceCommits(source.Commits),
			Documents:          contractSyntheticDocuments(source.Documents),
			Relations:          contractSyntheticRelations(source.Relations),
			Artifacts:          contractSyntheticArtifacts(source.Artifacts),
			Metadata:           source.Metadata,
			DocumentNamespaces: source.DocumentNamespaces,
			ArtifactNamespaces: source.ArtifactNamespaces,
			RelationKinds:      source.RelationKinds,
			DefaultRelations:   source.DefaultRelations,
		}
	} else {
		request.Source = nil
	}
	return request
}

func contractSourceProjects(values []extensions.SourceProject) []contractv1.SourceProject {
	if len(values) == 0 {
		return nil
	}
	result := make([]contractv1.SourceProject, 0, len(values))
	for _, value := range values {
		dependencies := make([]contractv1.SourceDependency, 0, len(value.Dependencies))
		for _, dependency := range value.Dependencies {
			dependencies = append(dependencies, contractv1.SourceDependency{
				Name: dependency.Name, Version: dependency.Version, Scope: dependency.Scope,
			})
		}
		result = append(result, contractv1.SourceProject{
			Path:         value.Path,
			Name:         value.Name,
			Description:  value.Description,
			Manifest:     value.Manifest,
			Ecosystem:    value.Ecosystem,
			Dependencies: dependencies,
		})
	}
	return result
}

func contractSourceCommits(values []extensions.SourceCommit) []contractv1.SourceCommit {
	if len(values) == 0 {
		return nil
	}
	result := make([]contractv1.SourceCommit, 0, len(values))
	for _, value := range values {
		result = append(result, contractv1.SourceCommit{
			SHA:     value.SHA,
			Parents: append([]string(nil), value.Parents...),
			Author:  value.Author,
			Date:    value.Date,
			Subject: value.Subject,
		})
	}
	return result
}

func contractSyntheticDocuments(values []extensions.SyntheticDocument) []contractv1.SyntheticDocument {
	if len(values) == 0 {
		return nil
	}
	result := make([]contractv1.SyntheticDocument, 0, len(values))
	for _, value := range values {
		var scope *string
		if value.Scope != nil {
			copied := *value.Scope
			scope = &copied
		}
		result = append(result, contractv1.SyntheticDocument{
			ID:          value.ID,
			Path:        value.Path,
			Type:        value.Type,
			ContentKind: value.ContentKind,
			MediaType:   value.MediaType,
			Title:       value.Title,
			Description: value.Description,
			Keywords:    append([]string(nil), value.Keywords...),
			Body:        value.Body,
			Metadata:    value.Metadata,
			Scope:       scope,
			Consumes:    append([]string(nil), value.Consumes...),
		})
	}
	return result
}

func contractSyntheticRelations(values []extensions.SyntheticRelation) []contractv1.SyntheticRelation {
	if len(values) == 0 {
		return nil
	}
	result := make([]contractv1.SyntheticRelation, 0, len(values))
	for _, value := range values {
		result = append(result, contractv1.SyntheticRelation{
			From:   value.From,
			To:     value.To,
			ToPath: value.ToPath,
			Kind:   contractv1.RelationKind(value.Kind),
			Raw:    value.Raw,
		})
	}
	return result
}

func contractSyntheticArtifacts(values []extensions.SyntheticArtifact) []contractv1.SyntheticArtifact {
	if len(values) == 0 {
		return nil
	}
	result := make([]contractv1.SyntheticArtifact, 0, len(values))
	for _, value := range values {
		result = append(result, contractv1.SyntheticArtifact{
			Path:      value.Path,
			MediaType: value.MediaType,
			Content:   value.Content,
		})
	}
	return result
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
