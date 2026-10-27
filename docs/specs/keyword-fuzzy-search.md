---
type: specification
title: "Keyword + Fuzzy Search Specification"
description: "Contract and algorithms for Weave's in-memory core: mandatory keywords, weighted fuzzy search, hybrid top-K results and evaluation."
tags:
  - "search"
  - "fuzzy"
  - "keywords"
  - "specification"
  - "database"
generated:
  by: opencode/deepseek-v4.1-flash
  at: "2026-10-25T00:00:00Z"
status: stable
---

# Specification: Keyword + Fuzzy Search

## 1. Overview

Weave loads an OKF bundle into memory as a document database:

- each concept is a node,
- Markdown links and `related` entries are edges,
- each document carries mandatory `keywords`,
- search runs over two dimensions: keyword matching and weighted fuzzy matching,
- every search returns top-K results.

The UI is out of scope. The Go CLI is the integration surface.

## 2. Goals and Non-Goals

### Goals

- Deterministic keyword retrieval based on author-curated frontmatter.
- Fuzzy retrieval that tolerates typos and partial words.
- Title weighted above description, description above body.
- Explicit component scores and matched metadata.
- Configurable top-K, default 10.
- Measurable calibration against a labeled query corpus.
- In-memory operation without requiring an external database.

### Non-Goals

- Browser UI, mount, renderer, lens or Astro.
- Synonyms, stemming or embeddings in the core engine.
- Cross-bundle federated search.
- Persistent indexes in this in-memory engine; v1.0 persistence is specified separately.
- Natural-language question answering.
- MCP transport in this milestone.

## 3. Document Model

### 3.1 Keywords

`keywords` is a required frontmatter field:

```yaml
keywords:
  - authentication
  - oauth
  - token-rotation
```

Rules:

- Must be a non-empty list of non-empty strings.
- Preserved verbatim in the contract `keywords` field.
- Indexed case-insensitively after Unicode-aware tokenization.
- Duplicate normalized keywords are indexed once.
- Missing/empty/invalid keywords make the document invalid for the Weave profile.

### 3.2 Validation

`ConceptValidation` gains:

```json
{
  "id": "concepts/example",
  "has_frontmatter": true,
  "has_type": true,
  "has_keywords": false,
  "warnings": ["missing keywords"]
}
```

Bundle `valid` requires `has_frontmatter && has_type && has_keywords` for every concept.

A document with missing keywords:

- remains in `Bundle.concepts`,
- remains in the graph,
- remains in the fuzzy index,
- is excluded from the keyword index,
- can still appear in fuzzy and hybrid results.

### 3.3 Graph

Nodes gain `keywords`. Edges are unchanged: `link` and `related`.

## 4. Normalization and Tokenization

All fields use the same tokenizer:

1. Lowercase.
2. Split on any Unicode non-letter/non-number run.
3. Drop empty tokens.

The body is passed through `StripMarkdown` before tokenization, preserving link labels, removing
fences/emphasis/heading markers.

Keyword tokens are additionally normalized by collapsing duplicate tokens per document.

## 5. Index Structures

The in-memory store contains:

```text
documents[id] -> {id, title, description, body, keywords, type, path}
graph         -> nodes, outgoing, incoming, dangling
keywordIndex  -> normalizedKeyword -> []documentID
fuzzyIndex    -> field -> normalizedToken -> []documentID
trigramIndex  -> field -> trigram -> []normalizedToken
```

- The keyword index excludes documents without valid keywords.
- The fuzzy index includes all documents.
- The trigram index accelerates fuzzy token candidate lookup.
- All postings are sorted by document id for deterministic iteration.

## 6. Keyword Search

### 6.1 Matching

Given a query:

1. Tokenize the query.
2. For each query token, find keyword tokens that are equal or start with it.
3. Every query token must match at least one keyword token (AND semantics).
4. For each document, take the best match per query token:
   - exact token: `1.0`
   - prefix token: `0.75`
5. `keyword_score` is the average of the per-token best scores, in `[0,1]`.
6. Sort by score descending, then document id ascending.
7. Return at most K results.

`matched_keywords` lists the original keyword values that matched, deduplicated and sorted.

### 6.2 Missing Keywords

Documents without valid keywords are never returned by keyword search.

## 7. Fuzzy Search

### 7.1 Token Similarity

For normalized tokens `a` and `b`:

```text
lev = 1 - levenshtein(a, b) / max(len(a), len(b))
tri = 2 * |trigrams(a) ∩ trigrams(b)| / (|trigrams(a)| + |trigrams(b)|)
similarity = max(lev, tri)
```

- Trigrams are computed on `#token#` so boundaries matter.
- Tokens shorter than three characters use exact/prefix comparison only.
- A similarity below `fuzzyThreshold` contributes zero. Starting threshold: `0.55`.
- The threshold is calibrated, not permanent.

### 7.2 Field Score

For each field (`title`, `description`, `body`):

1. For each query token, find the best similarity against any field token.
2. Average those best similarities across query tokens.
3. Store the field score in `[0,1]`.

This means a field matches well only when every query token has a reasonable match.

### 7.3 Field Weights

| Field | Weight |
| --- | --- |
| `title` | 1.00 |
| `description` | 0.60 |
| `body` | 0.30 |

```text
weighted = (titleScore * 1.00 + descriptionScore * 0.60 + bodyScore * 0.30) / 1.90
```

`fuzzy_score` is `weighted`, in `[0,1]`.

`matched_fields` contains fields whose weighted contribution exceeds `fieldThreshold`.
Starting value: `0.05`.

### 7.4 Snippets

If the body contributes, produce a snippet around the earliest body match using a ±40/120
character window. Otherwise omit `snippet`.

### 7.5 Sorting

Sort by `fuzzy_score` descending, then id ascending. Return at most K.

## 8. Hybrid Search

### 8.1 Component Scores

- Keyword score: `0` when the document has no keyword match or no valid keywords.
- Fuzzy score: `0..1` as above.

### 8.2 Combination

Starting formula:

```text
total_score = 0.50 * keyword_score + 0.50 * fuzzy_score
```

The 50/50 weights are calibration parameters.

### 8.3 Candidate Set

The hybrid list is computed from the union of the keyword top-K and fuzzy top-K candidate sets.
This is an explicit performance trade-off for the in-memory engine. If a document is outside both
component top-K lists, it is not considered for the hybrid list.

### 8.4 Sorting

Sort by `total_score` descending, then `fuzzy_score` descending, then `keyword_score` descending,
then id ascending. Return at most K.

## 9. Search Response

```json
{
  "query": "oauth tokn",
  "mode": "hybrid",
  "top_k": 10,
  "keyword": [
    {
      "id": "concepts/oauth-token-rotation",
      "title": "OAuth Token Rotation",
      "path": "concepts/oauth-token-rotation.md",
      "type": "Concept",
      "keywords": ["oauth", "tokens"],
      "matched_keywords": ["oauth", "tokens"],
      "score": 1.0
    }
  ],
  "fuzzy": [
    {
      "id": "concepts/oauth-token-rotation",
      "title": "OAuth Token Rotation",
      "path": "concepts/oauth-token-rotation.md",
      "type": "Concept",
      "score": 0.87,
      "matched_fields": ["title", "description"],
      "snippet": "…"
    }
  ],
  "hybrid": [
    {
      "id": "concepts/oauth-token-rotation",
      "title": "OAuth Token Rotation",
      "path": "concepts/oauth-token-rotation.md",
      "type": "Concept",
      "keyword_score": 1.0,
      "fuzzy_score": 0.87,
      "total_score": 0.935,
      "matched_keywords": ["oauth", "tokens"],
      "matched_fields": ["title", "description"],
      "snippet": "…"
    }
  ]
}
```

### 9.1 Top-K

- `top_k` defaults to 10.
- Allowed range: 1..100.
- Out-of-range values return `INVALID_REQUEST`.
- Each list is capped independently.

### 9.2 Modes

- `--mode hybrid` (default): human output uses `hybrid`.
- `--mode keyword`: human output uses `keyword`.
- `--mode fuzzy`: human output uses `fuzzy`.
- JSON responses always include all three lists.

## 10. CLI Contract

```sh
weave search <path> <query> [--mode hybrid|keyword|fuzzy] [--top-k <n>] [--json] [--no-cache]
```

- Removes `weave find`.
- `--json` emits one `weave.cli/v1` envelope.
- Human output:
  - hybrid: `<total_score> <title> [<path>]`
  - keyword: `<score> <title> [<matched keywords>]`
  - fuzzy: `<score> <title> [<matched fields>]`
- Empty results exit `1` with a stderr message, still emitting a valid envelope in JSON mode.
- Invalid input exits `2`.
- Keyword/fuzzy/hybrid results are deterministic.

Operations:

```text
search   fields: path, query, mode, top_k
```

`find` is removed from the operation catalogue.

## 11. Calibration

### 11.1 Corpus

Location: `engine/search/testdata/calibration.json`.

```json
{
  "version": 1,
  "cases": [
    {
      "name": "exact-keyword",
      "query": "oauth",
      "mode": "keyword",
      "top_k": 10,
      "expected": [
        {"id": "concepts/oauth-token-rotation", "grade": 3},
        {"id": "concepts/oauth-scopes", "grade": 2}
      ]
    }
  ]
}
```

Cases must cover:

- exact keywords,
- keyword prefixes,
- multi-keyword AND queries,
- typos,
- partial phrases,
- title vs description vs body weighting,
- missing-keyword documents,
- empty results.

### 11.2 Metrics

- NDCG@K (primary)
- MRR@K
- precision@K
- recall@K

### 11.3 Baseline

- The first implementation records measured baselines: NDCG@10 ≈ 0.974, MRR@10 = 1.0.
- CI fails if NDCG@K or MRR@K drops below the committed baseline (0.75 / 0.70).
- Weight/threshold changes require a calibration diff in the PR.

### 11.4 Tunable Parameters

- `fuzzyThreshold` (start `0.55`)
- `fieldThreshold` (start `0.05`)
- field weights `1.00 / 0.60 / 0.30`
- hybrid weights `0.50 / 0.50`

## 12. Performance

Targets for 10,000 documents on a local SSD:

| Query | Target |
| --- | --- |
| Keyword top-10 | < 20 ms |
| Fuzzy top-10 | < 50 ms |
| Hybrid top-10 | < 50 ms |
| Initial load | documented baseline; no regression per release |

The store is immutable after load. Queries do not mutate indexes.

## 13. Testing

1. Unit tests for tokenization, Levenshtein, trigram Dice and field weighting.
2. Keyword exact/prefix/AND tests.
3. Fuzzy typo/partial/ordering tests.
4. Hybrid ranking and tie-break tests.
5. Top-K boundary and invalid-K tests.
6. Missing-keywords exclusion tests.
7. Calibration corpus and metrics.
8. CLI envelope, mode, exit-code and stdout/stderr tests.
9. Race tests for concurrent reads.
10. Benchmarks for keyword, fuzzy and hybrid at 1k and 10k documents.

## 14. Migration and Cleanup

- Replace contract v1 schemas, types and goldens with the new model.
- Add `keywords` to every fixture.
- Replace `find` golden cases with `search` cases.
- Remove UI, renderer, mount, lens, Astro, Preact and UI tests.
- Retire the old TypeScript engine/search with the UI cleanup.
- Update architecture docs and backlog to remove the UI roadmap.

## 15. Acceptance Criteria

- Every document in the conformance corpus has a `keywords` field.
- Missing keywords produce `has_keywords: false` and `validation.valid: false`.
- Keyword results exclude invalid-keyword documents.
- Fuzzy results can include them.
- Title-weighted fuzzy ranking beats description and body for equivalent matches.
- Search returns deterministic top-K lists in all three modes.
- CLI exposes `search` and no longer exposes `find`.
- Calibration metrics meet or exceed the committed baseline.
- UI code, tests and dependencies are gone.
- `go test -race ./...` and the calibration suite pass.

## Related Documents

- [BM25 Ranking (v1.0)](./bm25-ranking.md)
- [Permissive Ingestion (v1.0)](./permissive-ingestion.md)
- [Weave Explain](./weave-explain.md)
- [ROADMAP](../../ROADMAP.md)
