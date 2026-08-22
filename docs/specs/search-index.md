---
type: specification
title: "Search Index Specification"
description: "Contract for an offline, keyboard-driven full-text index built alongside the node graph from a parsed OKF bundle."
tags:
  - "search"
  - "index"
  - "offline"
  - "specification"
generated:
  by: opencode/big-pickle
  at: "2026-08-06T00:00:00Z"
status: draft
---

# Specification: Search Index

## 1. Overview

Pure, offline, synchronous TypeScript module that turns a validated `ParsedBundle` (from [OKF Bundle Parser](./okf-bundle-parser.md)) — and optionally the [Node Graph](./node-graph-engine.md) — into an in-memory, queryable full-text index. It powers instant, keyboard-driven querying of Markdown and YAML content without a browser, keeping the exploration flow inside the terminal, exactly as the [Search Index](../knowledge/concepts/search-index.md) concept promises.

A hand-rolled inverted index with a Markdown-aware tokenizer and **BM25** ranking keeps the dependency tree empty and the build static, matching the parser's zero-dependency ethos.

## 2. Module Structure & Execution Context

- **Library Root:** `src/lib/search/`
- **No hydration / no client JS.** Runs in the Astro server runtime and CLI only. Zero browser bundle.
- **Synchronous and dependency-free.** Builds from an in-memory bundle; no third-party search library, no network. Native `Map`/`Set` postings.
- **Runtime:** Bun-first; plain TypeScript, no Node polyfills required.

**Files:**

- `src/lib/search/types.ts` — index and result interfaces.
- `src/lib/search/tokenize.ts` — Markdown-aware tokenizer (`tokenize`, `stripMarkdown`).
- `src/lib/search/build.ts` — `buildSearchIndex()`: document assembly + inverted postings.
- `src/lib/search/query.ts` — `search()`: matching, scoring, snippet extraction.

## 3. Component Contracts

### 3.1 SearchIndex (opaque)

```typescript
export interface SearchIndex {
  /* Opaque — consumed only through the query API */
}
```

### 3.2 SearchResult

```typescript
export interface SearchResult {
  /** Concept id matched */
  id: string;
  /** Display title */
  title: string;
  /** Bundle-relative path, e.g. "concepts/okf-bundle.md" */
  path: string;
  /** Concept type */
  type: string;
  /** Composite relevance score; higher is better */
  score: number;
  /** Field contributing the highest weight */
  bestField: 'title' | 'tags' | 'description' | 'body' | 'path' | 'type' | 'frontmatter';
  /** Excerpt around the best body match, when a body term hits */
  snippet?: string;
}
```

### 3.3 Build & Query API

```typescript
export function buildSearchIndex(bundle: ParsedBundle, graph?: NodeGraph): SearchIndex;
export function search(index: SearchIndex, query: string, options?: SearchOptions): SearchResult[];
export function indexSize(index: SearchIndex): number;
export function tokenize(text: string): string[];
export function stripMarkdown(body: string): string;
```

### 3.4 SearchOptions

```typescript
export interface SearchOptions {
  /** Max results returned; default 20 */
  limit?: number;
  /** Fields to restrict matching to; default all fields */
  fields?: Field[];
  /** Minimum score threshold; default 0 */
  minScore?: number;
}

export type Field = 'title' | 'tags' | 'description' | 'body' | 'path' | 'type' | 'frontmatter';
```

## 4. Data Flow & Dependencies

### 4.1 Indexing

- Input: `ParsedBundle` from [OKF Bundle Parser](./okf-bundle-parser.md); optional `NodeGraph` from [Node Graph Engine](./node-graph-engine.md).
- Per concept, assemble one search document indexing:
  - `title`, `description`, `tags`, `type`, `id` and `path` from concept metadata;
  - `body` — Markdown content stripped of link syntax, inline formatting, and code fences via `stripMarkdown`;
  - `frontmatter` — stringified values of the raw frontmatter record.
- `tokenize`: lowercase, Unicode-aware split on non-alphanumeric boundaries, prefix-tolerant (substring/prefix matching for keyboard-driven type-ahead).
- Build an inverted index: normalized token → set of matching document ids, per field.

### 4.2 Querying

- Tokenize the query; a document is a candidate when it matches **every** query token (AND semantics) across the restricted or default field set.
- Score each candidate with **per-field BM25**, combined as a weighted sum in descending field weight order: `title` > `tags` > `description` > `body` > `type`/`path`/`frontmatter`.
  - Constants: `k1 = 1.2`, `b = 0.75`.
  - `IDF(t) = ln((N − df(t) + 0.5) / (df(t) + 0.5) + 1)` where `N` is the number of documents in the bundle and `df(t)` the document frequency of token `t` (computed bundle-wide at build time, per field).
  - Per field: `score_f(d) = Σ_t IDF(t) · tf(t,d,f) · (k1 + 1) / (tf(t,d,f) + k1 · (1 − b + b · |d_f| / avgdl_f))` — standard BM25 term-frequency saturation and document-length normalization against the field's average length.
  - Total: `score(d) = Σ_f w_f · score_f(d)`; weights are fixed constants, documented in `query.ts`.
- Optional graph boost: when a `NodeGraph` was provided at build time, add a degree bonus (`inDegree + outDegree`) so well-connected hub concepts surface first — the "living universe" ranking signal.
- Snippet: when a body term matches, emit a ~120-character window around the first occurrence; otherwise omit.
- Results sorted by `score` descending, then `id` ascending for determinism, capped at `limit`.

### 4.3 Consumers

- Explorer: render results (and per-result `data-testid` hooks for behavior tests) for keyboard-driven queries.
- CLI mount ([Weave CLI Mount](./weave-cli.md)): build the index in memory alongside the graph at mount time; expose querying through the local server without network.
- Implements contract from [Search Index](../knowledge/concepts/search-index.md).

## 5. Performance, Edge Cases & Errors

- **Performance:** index build for a 10k-document bundle adds < 50 ms on a local SSD; single-token and multi-token queries return in < 5 ms on 10k documents. Combined with parser and graph build, stays inside the vision's "milliseconds" budget.
- **Empty query / whitespace-only query:** returns `[]`, no error.
- **Empty bundle:** empty index, `indexSize()` returns 0; `search` returns `[]`.
- **No matches:** returns `[]` (no partial-score fallback).
- **Markdown-heavy bodies:** link text and inline code are indexed as their visible text, not their raw syntax.
- **Unicode and diacritics:** tokenizer normalizes case and splits on non-alphanumeric boundaries; diacritics are preserved (no lossy folding) unless a future normalization policy overrides this.
- **Missing optional fields (title, description, tags):** indexed as empty strings; document still searchable by body/path/type.
- **Duplicate tokens in a document:** term frequency feeds BM25 but saturates via `k1`, so repeating the same token never linearly inflates rank.
- **Determinism:** result ordering is stable across builds for identical inputs.

## Related Concepts

- Depends on [OKF Bundle Parser](./okf-bundle-parser.md)
- Indexes [OKF Bundle](../knowledge/concepts/okf-bundle.md)
- Ranks with [Node Graph Engine](./node-graph-engine.md)
- Runs inside [Git Repository Mount](../knowledge/concepts/git-repository-mount.md)
- Implements [Search Index](../knowledge/concepts/search-index.md)
