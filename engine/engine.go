package engine

import (
	"context"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine/graph"
	"github.com/prulloac/weave/engine/nav"
	"github.com/prulloac/weave/engine/okf"
	"github.com/prulloac/weave/engine/privacy"
	"github.com/prulloac/weave/engine/search"
)

// Options controls engine construction.
type Options struct {
	Cache       bool
	CacheDir    string
	Concurrency int
	Permissive  bool
	FS          okf.FileSystem
	OnCacheHit  func(path string)
}

// Engine is an immutable parsed bundle with graph and search views.
type Engine struct {
	root   string
	bundle *contractv1.Bundle
	graph  *graph.Graph
	search *search.Index
}

// Open parses a bundle and builds its graph and search index once.
func Open(ctx context.Context, root string, options Options) (*Engine, error) {
	bundle, err := okf.ParseBundle(ctx, root, okf.Options{
		Cache:       options.Cache,
		CacheDir:    options.CacheDir,
		Concurrency: options.Concurrency,
		Permissive:  options.Permissive,
		FS:          options.FS,
		OnCacheHit:  options.OnCacheHit,
	})
	if err != nil {
		return nil, err
	}

	return FromBundle(bundle), nil
}

// FromBundle builds an engine from an already parsed bundle.
func FromBundle(bundle *contractv1.Bundle) *Engine {
	nodeGraph := graph.Build(bundle)
	searchIndex := search.Build(bundle, nodeGraph)
	return &Engine{
		root:   bundle.Root,
		bundle: bundle,
		graph:  nodeGraph,
		search: searchIndex,
	}
}

func (e *Engine) Root() string               { return e.root }
func (e *Engine) Bundle() *contractv1.Bundle { return e.bundle }
func (e *Engine) Graph() *graph.Graph        { return e.graph }
func (e *Engine) SearchIndex() *search.Index { return e.search }

func (e *Engine) Search(query string, mode contractv1.SearchMode, topK int) (contractv1.SearchResponse, error) {
	return search.Search(e.search, query, mode, topK)
}

func (e *Engine) Explain(
	query string,
	mode contractv1.SearchMode,
	topK int,
	target string,
) (contractv1.ExplainResult, error) {
	return search.Explain(e.search, query, mode, topK, target)
}

func (e *Engine) List(filters nav.ListFilters) []contractv1.ListResult {
	return nav.List(e.bundle, filters)
}

func (e *Engine) Show(target string) (*contractv1.ShowResult, bool) {
	return nav.Show(e.bundle, e.graph, target)
}

// ShowOptions controls redaction and decryption for one concept view.
type ShowOptions struct {
	Decrypt  bool
	Provider string
}

// ShowWithOptions returns a concept view with its privacy state applied.
func (e *Engine) ShowWithOptions(
	target string,
	options ShowOptions,
) (*contractv1.ShowResult, bool, error) {
	result, ok := nav.Show(e.bundle, e.graph, target)
	if !ok {
		return nil, false, nil
	}
	concept := e.conceptByID(result.ID)
	if concept == nil {
		return result, true, nil
	}
	result.Privacy = privacy.StateFor(*concept)
	if result.Privacy == nil {
		return result, true, nil
	}
	if result.Privacy.Redacted {
		result.Body = ""
	}
	if !options.Decrypt {
		return result, true, nil
	}
	providerName := options.Provider
	if providerName == "" && concept.Privacy != nil {
		providerName = concept.Privacy.ProviderName
	}
	provider, err := privacy.ResolveProvider(e.root, providerName)
	if err != nil {
		return nil, true, err
	}
	if provider != nil {
		defer provider.Close()
	}
	body, err := privacy.DecryptBodyWith(context.Background(), e.root, concept.Body, concept.Privacy, nil, provider)
	if err != nil {
		return nil, true, err
	}
	result.Body = body
	if result.Privacy.Policy == contractv1.PrivacyEncrypt {
		result.Privacy.Decrypted = true
	}
	return result, true, nil
}

func (e *Engine) conceptByID(id string) *contractv1.OkfConcept {
	for index := range e.bundle.Concepts {
		if e.bundle.Concepts[index].ID == id {
			return &e.bundle.Concepts[index]
		}
	}
	return nil
}

func (e *Engine) Backlinks(target string) (*contractv1.BacklinksResult, bool) {
	return nav.Backlinks(e.bundle, e.graph, target)
}

func (e *Engine) Path(from, to string) (*contractv1.PathResult, bool) {
	return nav.Path(e.bundle, e.graph, from, to)
}

// Close releases engine resources. The in-memory engine has none.
func (e *Engine) Close() error { return nil }
