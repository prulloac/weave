package engine

import (
	"context"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine/graph"
	"github.com/prulloac/weave/engine/nav"
	"github.com/prulloac/weave/engine/okf"
	"github.com/prulloac/weave/engine/search"
)

// Options controls engine construction.
type Options struct {
	Cache       bool
	CacheDir    string
	Concurrency int
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
		FS:          options.FS,
		OnCacheHit:  options.OnCacheHit,
	})
	if err != nil {
		return nil, err
	}

	nodeGraph := graph.Build(bundle)
	searchIndex := search.Build(bundle, nodeGraph)

	return &Engine{
		root:   bundle.Root,
		bundle: bundle,
		graph:  nodeGraph,
		search: searchIndex,
	}, nil
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

func (e *Engine) Backlinks(target string) (*contractv1.BacklinksResult, bool) {
	return nav.Backlinks(e.bundle, e.graph, target)
}

func (e *Engine) Path(from, to string) (*contractv1.PathResult, bool) {
	return nav.Path(e.bundle, e.graph, from, to)
}

// Close releases engine resources. The in-memory engine has none.
func (e *Engine) Close() error { return nil }
