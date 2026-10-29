---
type: Layer Specification
title: "Engine Layer (Go)"
description: "Go bundle parser, graph, in-memory keyword/fuzzy search, navigation and cache for the related-document database."
tags:
  - "architecture"
  - "engine"
  - "go"
  - "parser"
  - "search"
  - "fuzzy"
generated:
  by: opencode/deepseek-v4.1-flash
  at: "2026-10-25T00:00:00Z"
status: draft
---

# Engine Layer (Go)

## Purpose

The engine turns an OKF bundle into an in-memory related-document database:

```text
filesystem -> Bundle -> Graph -> Search Index -> navigation/search results
```

It is headless, deterministic and transport-free. The CLI is the only product surface; the Go API
is an optional in-process convenience.

## Package Layout

```text
engine/
├── engine.go            # Open, Bundle, Graph, Search, navigation methods
├── okf/
│   ├── fs.go            # FileSystem adapter
│   ├── parse.go         # walk, parse, validate, concurrency
│   ├── frontmatter.go   # YAML frontmatter and keyword extraction
│   ├── links.go         # source-preserving link scanner
│   └── cache.go         # optional per-file cache
├── graph/
│   ├── build.go         # nodes, edges, dangling links
│   └── traverse.go      # BFS, neighbors, orphans
├── search/
│   ├── build.go         # documents, keyword postings, field/trigram indexes
│   ├── query.go         # keyword, fuzzy and hybrid top-K
│   ├── similarity.go    # Levenshtein, trigram Dice, rounding
│   └── tokenize.go      # tokenization and markdown stripping
├── nav/
│   └── nav.go           # normalize, list, show, backlinks, path
├── privacy/             # directives, key resolution and crypto providers
├── compression/         # zstd and LZ4 body codecs
├── extensions/          # weave-<name> discovery and transport
└── operations/          # transport-neutral envelopes
```

## Public API

```go
type Options struct {
    Cache       bool
    CacheDir    string
    Concurrency int
    FS          okf.FileSystem
    OnCacheHit  func(path string)
}

func Open(ctx context.Context, root string, options Options) (*Engine, error)

func (e *Engine) Bundle() *contractv1.Bundle
func (e *Engine) Graph() *graph.Graph
func (e *Engine) SearchIndex() *search.Index
func (e *Engine) Search(query string, mode contractv1.SearchMode, topK int) (contractv1.SearchResponse, error)
func (e *Engine) List(filters nav.ListFilters) []contractv1.ListResult
func (e *Engine) Show(target string) (*contractv1.ShowResult, bool)
func (e *Engine) Backlinks(target string) (*contractv1.BacklinksResult, bool)
func (e *Engine) Path(from, to string) (*contractv1.PathResult, bool)
```

## Parsing

- Walk deterministically, skipping `.git` and `node_modules`.
- Follow one level of directory symlinks with a realpath cycle guard.
- Read concurrently with a bounded worker pool (default 32).
- Decode UTF-8 strictly; invalid files become per-file parse errors.
- Extract frontmatter with `yaml.v3`; preserve unknown keys.
- Required fields: non-empty `type` and a non-empty `keywords` list.
- Missing/invalid keywords make the document invalid, but it remains available for graph and
  fuzzy search and is excluded from the keyword index.
- Extract links with a source-preserving scanner so `raw` keeps the exact Markdown form.
- Resolve fragments/queries away before graph resolution.
- Record broken links as unresolved edges and validation warnings.
- Cache per-file outcomes keyed by absolute path, mtime, size and parser version.

## Search

The full contract is in [Keyword + Fuzzy Search](../../specs/keyword-fuzzy-search.md).

- **Keyword index:** normalized keyword token -> documents.
- **Fuzzy index:** per-field token postings and trigram index.
- **Keyword score:** exact `1.0`, prefix `0.75`, AND across query tokens.
- **Fuzzy score:** max of normalized Levenshtein and trigram Dice per token; field weights
  `title 1.00`, `description 0.60`, `body 0.30`; normalized by `1.90`.
- **Hybrid score:** `0.50 * keyword + 0.50 * fuzzy`.
- **Top-K:** default 10, maximum 100.
- Results contain keyword, fuzzy and hybrid lists with component scores and match metadata.
- Ranking is deterministic with id tie-breaks.
- Calibration lives in `engine/search/testdata/calibration.json` and is guarded by
  `TestCalibration`.

## Graph

- One node per concept, keyed by id, with keywords, type, title, status and tags.
- Edges from resolved Markdown links (`link`) and frontmatter `related` (`related`).
- Deduplicate identical edges and compute in/out degree.
- Keep dangling/external targets per source concept.
- BFS shortest path, neighbors and orphan detection.

## Cache and Concurrency

- Cache is opt-in at the engine API and enabled by the CLI.
- Cache writes are best-effort and concurrency-safe.
- Cache failures never fail a parse.
- Parsing is deterministic regardless of worker scheduling.
- Queries are read-only and safe for concurrent use after `Open`.

## Performance

Go benchmark on 1,000 generated files (local x86_64): parse ~47 ms. Search latency is measured by
the calibration/benchmark suite; the 10k-file budgets in the spec remain the release gate.

## Testing

- Contract schema round-trip tests.
- Golden conformance for bundle, graph and search.
- Calibration corpus with NDCG@K and MRR@K baselines.
- Cache hit/invalidation and race tests.
- CLI process tests.
- Fuzz tests for frontmatter, links and ids (planned).

## Related Documents

- [Keyword + Fuzzy Search spec](../../specs/keyword-fuzzy-search.md)
- [Contract layer](contract.md)
- [Operations layer](operations.md)
- [CLI layer](cli-go.md)
- [ADR 0003](../adr/0003-keyword-fuzzy-search-and-ui-retirement.md)
- [Integration testing](../integration/testing.md)
