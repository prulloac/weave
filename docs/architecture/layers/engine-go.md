---
type: Layer Specification
title: "Engine Layer (Go)"
description: "Go package layout, public API, parsing pipeline, parity requirements and testing strategy for the headless Weave engine."
tags:
  - "architecture"
  - "engine"
  - "go"
  - "parser"
  - "search"
generated:
  by: opencode/deepseek-v4.1-flash
  at: "2026-10-25T00:00:00Z"
status: draft
---

# Engine Layer (Go)

## Purpose

The engine turns an OKF v0.2 bundle directory into an immutable, queryable in-memory model:

```text
filesystem → Bundle → Graph → SearchIndex → navigation results
```

It is headless and deterministic. It does not render HTML, serve HTTP, read process arguments,
speak a transport protocol, or know about the lens. The public integration surface is the
[CLI layer](cli-go.md); the engine API is an optional in-process convenience for Go hosts.

## Package Layout

```text
engine/
├── engine.go            # public facade: Open, accessors, query methods
├── options.go           # Options, defaults
├── errors.go            # typed errors and codes
├── okf/
│   ├── fs.go            # FileSystem interface + OS adapter
│   ├── walk.go          # deterministic tree walk, symlink guard
│   ├── frontmatter.go   # yaml.v3 decoding + normalization
│   ├── links.go         # source-preserving Markdown link scanner and resolution
│   ├── parse.go         # per-file and bundle assembly
│   ├── validate.go      # conformance validation
│   └── cache.go         # optional per-file parse cache
├── graph/
│   ├── build.go
│   ├── traverse.go
│   └── types.go
├── search/
│   ├── build.go
│   ├── query.go
│   ├── tokenize.go
│   └── types.go
└── nav/
    ├── find.go
    ├── list.go
    ├── show.go
    ├── backlinks.go
    └── path.go
```

`contract/v1` is a sibling package and the only shared dependency.

## Public API

```go
package engine

type Options struct {
    // Cache enables the per-file parse cache. Default false.
    Cache bool
    // CacheDir overrides the cache location. Empty means WEAVE_CACHE_DIR or os.TempDir.
    CacheDir string
    // Concurrency bounds concurrent file reads. Default 32.
    Concurrency int
    // FS overrides the filesystem adapter. Default is the OS adapter.
    FS okf.FileSystem
}

type Engine struct {
    // unexported fields
}

func Open(ctx context.Context, root string, opts Options) (*Engine, error)

func (e *Engine) Root() string
func (e *Engine) Bundle() *contract.Bundle
func (e *Engine) Graph() *graph.Graph
func (e *Engine) SearchIndex() *search.Index

func (e *Engine) Find(query string, opts search.Options) []contract.FindResult
func (e *Engine) List(filters nav.ListFilters) []contract.ListResult
func (e *Engine) Show(target string) (*contract.ShowResult, bool)
func (e *Engine) Backlinks(target string) (*contract.BacklinksResult, bool)
func (e *Engine) Path(from, to string) (*contract.PathResult, bool)

func (e *Engine) Close() error
```

Design rules:

- `Open` performs the full parse, graph build and index build once. Query methods are read-only
  and safe for concurrent use after `Open` returns.
- `Open` honors `context.Context` cancellation during walking and reading.
- The engine never caches across roots implicitly. Reopening is the only way to observe changes.
- The host decides whether to call `Open` again after files change.
- All exported result types come from `contract/v1`; the engine's internal graph and index types
  are not part of the public API.
- The CLI reaches the engine through the [operations layer](operations.md). The engine API is
  not the compatibility promise; the CLI contract is.
- If the engine API is used directly, it is stable within the module's semver policy but is not
  a cross-language contract.

## Parsing Pipeline

1. **Walk** the bundle root deterministically.
2. **Read** files with bounded concurrency.
3. **Extract frontmatter** with `yaml.v3`.
4. **Extract links** with the source-preserving Markdown scanner.
5. **Resolve** link targets against the final concept set.
6. **Validate** per concept and per bundle.
7. **Build graph** from resolved links and the `related` frontmatter array.
8. **Build search index** from concept fields plus graph degree.
9. **Return** the immutable model.

The order matters because link resolution needs the complete concept set, and the search degree
boost needs the graph.

## Filesystem Adapter

```go
package okf

type DirEntry struct {
    Name  string
    IsDir bool
    IsLink bool
}

type FileInfo struct {
    Size    int64
    ModTime time.Time
    IsDir   bool
    IsFile  bool
}

type FileSystem interface {
    Walk(ctx context.Context, root string) ([]string, error)
    ReadFile(ctx context.Context, path string) ([]byte, error)
    Stat(ctx context.Context, path string) (FileInfo, error)
    RealPath(ctx context.Context, path string) (string, error)
}
```

The OS adapter wraps `os`, `filepath` and `io/fs`. Tests use an in-memory adapter for
deterministic fixtures and error injection. The adapter must:

- return POSIX-style relative paths to the caller,
- skip `.git` and `node_modules`,
- follow one level of directory symlinks,
- guard cycles with resolved real paths.

## Frontmatter and YAML

Go uses `gopkg.in/yaml.v3` and accepts the full YAML 1.2 document, which is a superset of the
current TypeScript subset. The contract fixtures define the required observable subset:

- scalars, booleans, numbers, null,
- quoted and unquoted strings, URLs with fragments,
- flow and block sequences,
- nested maps,
- lists of mappings (`verified`),
- comments.

Normalization rules:

| Frontmatter value | Contract field |
| --- | --- |
| `type` must be a non-empty string | `Concept.Type` |
| `title`, `description` | stringified when present |
| `tags` string or list | `[]string` |
| `status` one of `draft`, `stable`, `deprecated` | `Concept.Status`; other values ignored |
| everything else | preserved in `Concept.Frontmatter` |

Do not reject unknown keys or unknown types. Do not require `metadata.id`.

## Link Extraction

Links are extracted with a small source-preserving scanner rather than the goldmark AST. Goldmark
is reserved for HTML rendering in the presentation layer. The scanner is used because the
contract's `raw` field must preserve the exact source form (`[x](<b.md>)`, `[x](b.md "title")`,
`[x][ref]`, `[x][]`, `[x]`), which inline AST nodes do not expose.

The scanner:

- strips fenced code blocks and inline code spans,
- collects reference definitions and resolves full, collapsed and shortcut references,
- handles inline, titled and angle-bracket destinations,
- skips image embeds,
- excludes autolinks in v1,
- returns source slices in document order.

This behavior is pinned by the `anchors`, `reference-links` and `code-links` golden cases.

Resolution:

```go
func Resolve(rawTarget, conceptDir string) (resolved string, external bool)
```

1. If the target has a URL scheme, mark external.
2. Strip `#fragment` and `?query`.
3. If the remainder is empty, leave unresolved.
4. Leading `/` means bundle root; otherwise clean `path.Join(conceptDir, target)`.
5. Convert to POSIX separators and trim a trailing `.md` only for graph node ids, not for
   `resolved`.

`resolves_in_bundle` is set only after the full concept set is known.

## Graph

- One node per concept, keyed by id.
- Edges from resolved links (`source: link`) and `related` frontmatter entries
  (`source: related`).
- Deduplicate identical `(from, to, source)` edges.
- Compute `in_degree` and `out_degree` during assembly.
- Broken and external links go to `dangling`, keyed by source id.
- Serialize nodes and edges in id order for deterministic output and lens payloads.

Navigation:

- `findPath` is BFS over directed edges, returning the shortest hop sequence.
- `neighbors`, `connectedComponents` and `orphanNodes` are optional helpers for hosts and tests.

## Search

Port the current BM25 semantics exactly; see [Contract layer](contract.md) for constants.

Implementation notes:

- Build an inverted index per field: `map[Field]map[token][]docIndex`.
- Store term frequency per document and field.
- Store field lengths and averages.
- Apply a prefix match (`strings.HasPrefix(indexToken, queryToken)`).
- Require every query token to match across the selected fields.
- Sort by score descending, then id ascending.
- Keep snippets only for body matches.
- Use `unicode`-aware splitting equivalent to `[^\p{L}\p{N}]+`.
- Keep the same field weights and BM25 constants; do not upgrade to a different ranking model in
  v1.

## Cache

The cache is opt-in and mirrors the current TypeScript design:

- Key: bundle root hash.
- Entry: absolute file path plus `(mtime, size)`.
- Value: per-file parse outcome (concept, validation, index result).
- Version constant invalidates all entries on parser changes.
- Writes are best-effort and atomic (`.tmp` plus rename).
- Cache failures never fail a parse.

## Errors

```go
type Error struct {
    Code  string // e.g. PARSE_ERROR, INVALID_ROOT, CACHE_ERROR
    Path  string
    Err   error
}
```

- `Open` returns an error for an unreadable root or a cancelled context.
- Per-file failures are recorded in `Bundle.Validation`, not returned as a fatal error.
- CLI exit codes and HTTP statuses are mapped by the outer layers, never by the engine.

## Performance Targets

| Metric | Target |
| --- | --- |
| 10k small files | Under 500 ms on local SSD, no cache |
| Per file | ~0.04 ms or better |
| Memory | O(total parsed content); bodies retained unless a future option says otherwise |
| Concurrency | 32 reads by default, configurable |
| Determinism | Identical inputs produce byte-identical JSON after normalization |

Benchmarks live in `engine/okf/bench_test.go` and `engine/search/bench_test.go` with generated
bundles. CI runs a smoke benchmark with a generous ceiling to catch order-of-magnitude
regressions.

## Testing

1. **Unit tests** per package, table-driven.
2. **Golden tests** against `contract/v1/golden` for `Bundle`, `Graph`, `Search` and nav results.
3. **Filesystem adapter tests** including symlink cycles and Windows hardlink fallback.
4. **Fuzz tests** for frontmatter, link targets and path normalization.
5. **Race tests** with `go test -race ./...`.
6. **Benchmarks** for parse and search.

The engine port is complete only when it passes every golden fixture and the race suite.

## Operations Boundary

The engine exposes typed data and query methods. The [operations layer](operations.md) wraps them
into transport-neutral requests and envelopes. Do not put argv parsing, exit codes, HTTP statuses,
NDJSON framing or MCP types in the engine.

## Parity Map from TypeScript

| TypeScript | Go target |
| --- | --- |
| `src/lib/okf/frontmatter.ts` | `engine/okf/frontmatter.go` |
| `src/lib/okf/links.ts` | `engine/okf/links.go` |
| `src/lib/okf/parser.ts` | `engine/okf/parse.go`, `walk.go` |
| `src/lib/okf/validate.ts` | `engine/okf/validate.go` |
| `src/lib/okf/cache.ts` | `engine/okf/cache.go` |
| `src/lib/graph/build.ts` | `engine/graph/build.go` |
| `src/lib/graph/traverse.ts` | `engine/graph/traverse.go` |
| `src/lib/search/*` | `engine/search/*` |
| `cli/find.ts`, `list.ts`, `show.ts`, `path.ts` | `engine/nav/*` |
| `cli/resolve.ts` | `engine/nav/resolve.go` |

## Related Documents

- [Contract layer](contract.md)
- [Operations layer](operations.md)
- [CLI layer](cli-go.md)
- [Render layer](render-go.md)
- [Mount layer](mount-go.md)
- [Integration contracts](../integration/contracts.md)
- [Integration testing](../integration/testing.md)
