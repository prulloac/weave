---
type: specification
title: "OKF Bundle Parser Specification"
description: "Contract for parsing OKF v0.2 bundles into validated Go contract types with mandatory type and keywords, extracted graph edges and cache support."
tags:
  - "okf"
  - "parser"
  - "go"
  - "specification"
generated:
  by: opencode/deepseek-v4.1-flash
  at: "2026-10-26T00:00:00Z"
status: stable
---

# Specification: OKF Bundle Parser

## 1. Overview

Pure, offline Go package that turns an OKF v0.2 bundle into validated `contractv1.Bundle`
structures. It is the foundation for the graph, keyword/fuzzy search and CLI.

The Weave profile requires both `type` and `keywords` in frontmatter. Unknown keys/types, broken
links and missing optional fields are tolerated.

## 2. Package Structure

```text
engine/okf/
├── fs.go            # FileSystem interface + OS adapter
├── parse.go         # walk, concurrency, parsing, assembly, validation
├── frontmatter.go   # delimiter splitting, yaml.v3 decode, keyword extraction
├── links.go         # source-preserving Markdown link scanner and resolution
└── cache.go         # optional per-file cache
```

The public data model lives in `contract/v1/types.go`.

## 3. Contracts

### 3.1 Concept

`contractv1.OkfConcept`:

- `id`: bundle-relative path minus `.md`
- `path`: bundle-relative path including `.md`
- `type`: required non-empty string
- `keywords`: required non-empty list of non-empty strings
- `title`, `description`, `tags`, `status`: optional
- `frontmatter`: raw parsed map, unknown keys preserved
- `body`: text after the closing delimiter
- `links`: source-preserving links with resolution metadata

### 3.2 Bundle

`contractv1.Bundle`:

- `root`: absolute bundle root
- `okf_version`: only from the root `index.md`
- `concepts`: sorted by id
- `index`: root index entries
- `validation`: bundle validity, per-concept validation and warnings

### 3.3 Validation

`contractv1.ConceptValidation` includes `has_frontmatter`, `has_type`, `has_keywords` and
`warnings`. A bundle is valid only when every concept has all three.

Missing keywords:

- parse and keep the concept,
- mark `has_keywords: false`,
- exclude it from the keyword index,
- keep it available for graph navigation and fuzzy search.

### 3.4 API

```go
func ParseBundle(ctx context.Context, root string, options Options) (*contractv1.Bundle, error)

type Options struct {
    Cache       bool
    CacheDir    string
    Concurrency int
    FS          FileSystem
    OnCacheHit  func(path string)
}
```

## 4. Data Flow

1. Resolve the absolute root.
2. Walk deterministically, skipping `.git` and `node_modules`.
3. Follow one level of directory symlinks with a realpath cycle guard.
4. Read files concurrently with a bounded worker pool (default 32).
5. Split frontmatter with `---` delimiters.
6. Decode YAML with `yaml.v3`; normalize maps and preserve unknown keys.
7. Extract keywords and validate `type`/`keywords`.
8. Extract links with the source-preserving scanner.
9. Resolve link targets; broken targets remain unresolved.
10. Assemble concepts, index, validation and warnings deterministically.
11. Optionally save the per-file cache.

## 5. Link Semantics

- Inline, titled, angle-bracket, full-reference, collapsed-reference and shortcut-reference links
  are supported.
- Images are skipped.
- Links inside fenced code and inline code are ignored.
- Autolinks are excluded in v1.
- `#fragment` and `?query` are stripped before resolution.
- Leading `/` is bundle-root-relative; otherwise relative to the concept directory.
- External schemes are never resolved.
- `raw` preserves the exact Markdown source.

## 6. Cache

- Opt-in; CLI enables it by default.
- Key: absolute path plus `(mtime, size)` and parser version.
- Value: per-file outcome (concept, validation, index data).
- Writes are best-effort and concurrency-safe.
- Cache failures never fail a parse.

## 7. Edge Cases

- Missing frontmatter: exclude concept, record `missing frontmatter`.
- Missing `type`: include concept, `has_type: false`.
- Missing keywords: include concept, `has_keywords: false`.
- Broken links: keep as unresolved edges and warnings.
- Invalid UTF-8: per-file parse error, skipped.
- Empty bundle: valid, warning emitted.
- Unsupported `okf_version`: warning, best-effort parse.
- Root vs nested `index.md`: only the root sets index/version.
- Symlink cycles: guarded by realpath visited-set.

## 8. Performance

Go benchmark on 1,000 generated files: ~47 ms parse. The 10k-file target remains 500 ms on a local
SSD.

## 9. Testing

- Contract golden conformance (`engine/conformance_test.go`).
- Cache hit/invalidation tests.
- Race suite.
- Frontmatter/link fuzz tests (planned).

## Related Concepts

- Implements [OKF Bundle](../knowledge/concepts/okf-bundle.md)
- Feeds [Node Graph Engine](./node-graph-engine.md)
- Feeds [Keyword + Fuzzy Search](./keyword-fuzzy-search.md)
- Consumed by the Go CLI
