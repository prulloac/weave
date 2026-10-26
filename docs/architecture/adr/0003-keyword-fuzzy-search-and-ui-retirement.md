---
type: Architecture Decision Record
title: "ADR 0003: Keyword + Fuzzy Search and UI Retirement"
description: "Redefines Weave as an in-memory related-document database with mandatory keywords, weighted fuzzy search, top-K results and no UI."
tags:
  - "architecture"
  - "adr"
  - "search"
  - "fuzzy"
  - "keywords"
  - "scope"
generated:
  by: opencode/deepseek-v4.1-flash
  at: "2026-10-25T00:00:00Z"
status: accepted
---

# ADR 0003: Keyword + Fuzzy Search and UI Retirement

## Status

Accepted on 2026-10-25. This ADR overrides the UI and search decisions in ADR 0001 and ADR 0002
where they conflict. The Go engine, contract, operations and CLI remain the foundation.

## Context

The base Go engine is complete: it parses OKF bundles, builds nodes and edges, indexes documents,
navigates the graph and exposes a CLI contract.

The product purpose is changing:

- Weave is **not** a visualization product.
- Weave is an **in-memory related-document database**.
- Documents are related by Markdown links, as before.
- Search has **two dimensions**: mandatory frontmatter keywords and weighted fuzzy matching.
- Every search returns **top-K results**.
- The browser, mount, renderer and lens are no longer product goals.

## Decision Summary

| ID | Decision |
| --- | --- |
| D1 | Add a mandatory `keywords` frontmatter field |
| D2 | Parse documents missing keywords, mark them invalid, exclude them from keyword search |
| D3 | Return keyword, fuzzy and hybrid top-K result sets |
| D4 | Override contract v1 in place; no v2 |
| D5 | Remove `find`; add `weave search` |
| D6 | Cancel and remove the UI/presentation work |
| D7 | Keyword matching is exact token plus prefix boost; synonyms later |
| D8 | Fuzzy scoring weights title > description > body |
| D9 | Calibrate against a labeled query corpus with ranking metrics |
| D10 | Keep the engine in-memory; no external database |
| D11 | Default top-K is 10, configurable up to 100 |

## D1 — Mandatory `keywords` frontmatter

### Intent

Give every document an explicit, human-curated retrieval vocabulary that is independent of the
body text. Keywords are the deterministic dimension of search.

### Decision

```yaml
keywords:
  - authentication
  - oauth
  - token-rotation
```

- `keywords` is a required frontmatter field.
- It must be a non-empty list of non-empty strings.
- Missing or invalid keywords make the document invalid for the Weave profile.
- Keywords are preserved as written and indexed case-insensitively.
- `type` remains required; Weave now adds a second required field.

### Effects

- The contract v1 `Concept` and `GraphNode` shapes gain `keywords`.
- Validation gains `has_keywords` and a missing-keywords warning/error.
- Existing bundles without keywords are no longer fully valid.
- Search indexes and result shapes change.

### Expected outcomes

- Keyword search is predictable and independent of prose.
- Authors control the retrieval vocabulary.
- Documents missing keywords are visible as validation failures, not silently unsearchable.

### Acceptance

- Missing or empty keywords produce a validation entry with `has_keywords: false`.
- A bundle containing such a document has `validation.valid: false`.
- The document remains available for graph navigation and fuzzy search.
- The document never appears in keyword results.

## D2 — Parse but exclude from keyword search

### Intent

Keep diagnostics, graph traversal and fuzzy search working while making the keyword-dimension
failure explicit.

### Decision

- Documents with missing/invalid keywords are parsed and retained.
- They are excluded from the keyword index.
- They remain in the fuzzy index and graph.
- `validation.valid` is false and the validation entry records the problem.

### Effects

- CLI search can still find the document fuzzily.
- Operators can query the bundle to find documents needing keywords.
- Keyword results never contain a document without keywords.

### Expected outcomes

- No silent exclusion from the whole engine for one missing field.
- Clear remediation path for bundle authors.

### Acceptance

- A keyword query never returns a keywords-missing document.
- A fuzzy query can return it.
- Validation exposes the missing field.

## D3 — Keyword, fuzzy and hybrid result sets

### Intent

Expose both search dimensions explicitly while also providing one ranked list for ordinary use.

### Decision

A search response contains:

1. `keyword`: top-K by keyword match.
2. `fuzzy`: top-K by weighted fuzzy similarity.
3. `hybrid`: top-K by a normalized combination of both.

Default CLI mode is `hybrid`; `--mode keyword` and `--mode fuzzy` return one dimension.

### Effects

- Search result types grow component scores and matched metadata.
- Top-K applies per dimension and to the hybrid list independently.
- A document may appear in all three lists.

### Expected outcomes

- Callers can audit why a document ranked.
- Hybrid is the ergonomic default; component modes are available for precision tasks.
- Calibration can tune each dimension separately.

### Acceptance

- Each response list has at most K entries.
- Hybrid scores are derived from the component scores.
- Component lists are stable and deterministic.

## D4 — Override contract v1 in place

### Intent

Avoid a parallel compatibility layer for a product that has no external consumers yet.

### Decision

- The current `contract/v1` is replaced with the new document and search model.
- The previous v1 goldens and schemas are overwritten.
- No v2 directory and no backward-compatibility mode.
- The CLI envelope stays `weave.cli/v1`; the domain contract inside it changes.

### Effects

- Existing fixtures must add keywords.
- Old search goldens are replaced by the new search corpus.
- The old TypeScript implementation is no longer contract-compatible and is retired rather than
  updated.

### Expected outcomes

- One contract, one engine, no dual maintenance.
- Cleaner repository after UI and old search cleanup.

### Acceptance

- `contract/v1` describes keywords and the new search results.
- All goldens validate against the new v1.
- No code path claims the old v1 search contract.

## D5 — Remove `find`, add `weave search`

### Intent

Use one clear verb for the new two-dimensional search.

### Decision

- Remove the `find` operation, result type, CLI command and tests.
- Add `weave search <path> <query>`.
- Flags: `--mode hybrid|keyword|fuzzy`, `--top-k <n>`, `--json`, `--no-cache`.
- `--top-k` defaults to 10 and is capped at 100.
- Human output shows the hybrid list by default; component modes show their own list.

### Effects

- Operations catalogue and CLI help change.
- The old BM25 `FindResult` type is removed.
- `list`, `show`, `backlinks`, `path` remain unchanged except for keywords in payloads.

### Expected outcomes

- One search command with explicit modes.
- No ambiguity between BM25 “find” and fuzzy “search”.

### Acceptance

- `weave search` passes contract and CLI tests.
- `weave find` is gone.
- `weave contract --json` lists `search`, not `find`.

## D6 — Cancel and remove the UI work

### Intent

Focus the product on the in-memory search and navigation engine.

### Decision

- Remove the browser presentation, mount, renderer, lens, Astro site and Preact components from
  the roadmap.
- Delete the associated code, tests, dependencies and documentation in a dedicated cleanup.
- Keep the Go engine and CLI as the only product surfaces.
- The lens asset pipeline and `mount`/`unmount`/`status` commands are not implemented in the new
  product.

### Effects

- No Astro, Tailwind, Preact or Playwright UI work.
- No `/lens`, `/concept`, explorer or mount server.
- `package.json`, CI and docs shrink accordingly.
- The old TypeScript engine/search is retired with the UI because it is no longer the reference.

### Expected outcomes

- A Go-only engine and CLI repository.
- No presentation maintenance cost.
- Smaller dependency and CI surface.

### Acceptance

- No production package imports Astro, Preact or UI renderers.
- No UI tests remain.
- `weave mount`, `unmount` and `status` are removed from help and code.
- The Go CLI and contract tests remain green.

## D7 — Keyword matching semantics

### Decision

- Tokenize keywords and queries with the same Unicode letter/number tokenizer.
- Case-insensitive.
- Every query token must match at least one keyword token (AND semantics).
- Exact token match scores `1.0`; a token prefix match scores `0.75`.
- Keyword score is the average of the best per-query-token scores.
- Duplicate normalized keywords are indexed once.
- Synonyms and stemming are deferred.

### Expected outcomes

- Exact keywords rank first.
- Partial keywords still work.
- Multi-keyword queries are precise.

### Acceptance

- Exact and prefix cases are covered by the calibration corpus.
- A document without keywords never appears in keyword results.

## D8 — Fuzzy field weights

### Decision

Fuzzy search runs over three fields with these starting weights:

| Field | Weight |
| --- | --- |
| `title` | 1.00 |
| `description` | 0.60 |
| `body` | 0.30 |

- Normalize the weighted field score by the sum of weights (`1.90`) so the result is `[0,1]`.
- Per field, compute the best token similarity for each query token, then average across query
  tokens.
- Token similarity is the maximum of normalized Levenshtein similarity and trigram Dice
  similarity.
- Include a field in `matched_fields` when its contribution exceeds the configured threshold.
- Snippets come from the body match window when the body contributes; otherwise omit.

### Effects

- Title matches dominate, description is secondary, body is supporting evidence.
- Typos, partial words and word-order changes are handled.
- The weights are starting values, not permanent; calibration can adjust them.

### Expected outcomes

- A title match outranks an equivalent body match.
- Fuzzy search tolerates common typos.
- Component scores are explainable.

### Acceptance

- Weighted ranking tests pin title > description > body for equivalent matches.
- Fuzzy results degrade gracefully for short and long documents.
- Scores are stable and normalized.

## D9 — Calibration corpus and metrics

### Decision

- Add a labeled query corpus under `engine/search/testdata`.
- Each case contains a query, mode, K and expected ranked ids with relevance grades.
- Metrics: NDCG@K, MRR@K, precision@K, recall@K.
- Commit a baseline; CI fails when the primary metric drops below the baseline.
- Tune field weights, fuzzy threshold and hybrid weights only through this corpus.

### Expected outcomes

- “Calibrated” becomes measurable.
- Weight changes are reviewable.
- Ranking regressions are caught before release.

### Acceptance

- The corpus covers exact keywords, prefixes, typos, partial phrases, multi-token queries,
  missing-keyword documents and empty results.
- A calibration report is generated by a Go test or command.
- The baseline is documented in the spec.

## D10 — In-memory database engine

### Decision

- Load the bundle into memory once per process.
- Build:
  - document catalog,
  - graph adjacency,
  - keyword inverted index,
  - fuzzy token/trigram index.
- No external database, no persistence layer, no daemon in v1.
- The CLI is the integration boundary.

### Expected outcomes

- Fast top-K queries without operational dependencies.
- Deterministic results.
- The same store can later back an MCP server without changing search semantics.

## D11 — Top-K defaults

### Decision

- Default `K = 10`.
- Maximum `K = 100`.
- `K` applies independently to keyword, fuzzy and hybrid lists.
- Invalid `K` is an `INVALID_REQUEST`.

### Expected outcomes

- Predictable CLI output size.
- Avoid huge payloads by default.

## Consequences

### Positive

- The product has a clear search identity.
- Keywords give authors deterministic retrieval control.
- Weighted fuzzy scoring with a calibration corpus makes ranking defensible.
- Removing UI reduces maintenance and dependencies.

### Negative / risks

- Breaking change to the contract and fixtures.
- Existing bundles need keywords.
- Fuzzy scoring needs real calibration to avoid overfitting the corpus.
- Removing UI and the TS reference is a large cleanup.
- Keyword AND semantics may be too strict for exploratory queries; `--mode fuzzy` remains available.

## Follow-Up Work

1. Write the detailed search spec (`docs/specs/keyword-fuzzy-search.md`).
2. Replace contract v1 schemas, types and goldens.
3. Implement the in-memory store, keyword index, fuzzy index and hybrid ranking.
4. Replace `find` with `search` in operations and CLI.
5. Build the calibration corpus and baseline report.
6. Remove UI, mount, renderer, lens, Astro, Preact and UI tests.
7. Retire the old TypeScript engine/search with the UI cleanup.
8. Update architecture docs and backlog to remove the UI roadmaps.

## Related Documents

- [Architecture index](../index.md)
- [ADR 0001](0001-go-engine-and-ts-lens.md)
- [ADR 0002](0002-v1-scope-layout-and-contract.md)
- [Keyword + Fuzzy Search spec](../../specs/keyword-fuzzy-search.md)
- [Weave backlog](../../backlog.md)
