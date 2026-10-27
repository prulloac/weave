---
type: specification
title: "BM25 Lexical Ranking Specification"
description: "v1.0 SQLite FTS5 lexical ranking, field weights, hybrid normalization, explain diagnostics and calibration."
tags:
  - "specification"
  - "search"
  - "bm25"
  - "sqlite"
  - "v1"
generated:
  by: opencode/deepseek-v4.1-flash
  at: "2026-10-27T00:00:00Z"
status: draft
---

# Specification: BM25 Lexical Ranking

## 1. Overview

The shipped core uses exact/prefix keyword matching with strict AND semantics. v1.0 adds **BM25
lexical ranking** through SQLite FTS5 so multi-token queries do not drop documents that match only
some terms.

```sh
weave search <path> <query> --mode bm25 --json
weave search <path> <query> --mode hybrid --json
weave explain <path> <query> --mode bm25 --id <id> --json
```

## 2. Goals and Non-Goals

### Goals

- Rank multi-token queries by relevance instead of strict AND.
- Match FTS5 BM25 semantics deterministically.
- Normalize lexical scores for hybrid ranking.
- Keep exact keyword search available.
- Extend calibration to cover BM25.

### Non-Goals

- Semantic/vector search.
- Cross-document query expansion.
- Fuzzy matching inside SQLite BM25 (fuzzy remains the separate dimension).
- Replacing exact keyword search.

## 3. Modes

| Mode | Semantics |
| --- | --- |
| `exact` | Exact/prefix keyword matching, AND across query tokens |
| `bm25` | SQLite FTS5 BM25 lexical ranking |
| `fuzzy` | Weighted Levenshtein + trigram similarity |
| `hybrid` | Normalized BM25 + fuzzy |

- `keyword` is an alias for `exact` during the v1.0 transition; new documentation prefers `exact`.
- Default mode remains `hybrid`.
- `--mode bm25` is additive; it does not change existing `keyword` or `fuzzy` results.

## 4. FTS5 Index

```sql
CREATE VIRTUAL TABLE fts_documents USING fts5 (
    id UNINDEXED,
    title,
    description,
    keywords,
    body,
    tokenize = 'porter unicode61'
);
```

- One row per indexed document.
- `id` is the concept id.
- Keywords are indexed as a single field.
- Body is the stripped Markdown body.
- Synthetic permissive documents index their synthesized title/description/body.

### Field weights

Initial FTS5 column weights:

| Field | Weight |
| --- | --- |
| `title` | 10 |
| `keywords` | 8 |
| `description` | 4 |
| `body` | 1 |

These weights are calibration parameters, separate from fuzzy field weights.

## 5. Query Semantics

- Tokenize with FTS5 `porter unicode61`.
- Default query is a term union with coordination: a document may match any query term.
- Ranking uses `bm25(fts_documents, 10.0, 8.0, 4.0, 1.0)`.
- Quoted phrases are exact phrase queries.
- Negative terms (`-term`) are excluded.
- Empty/whitespace queries return no results.
- `top_k` defaults to 10; max 100.

## 6. Score Normalization

BM25 scores are unbounded. For hybrid ranking:

```text
max_score = max(bm25 raw scores in the candidate set)
bm25_norm = max_score > 0 ? bm25_raw / max_score : 0
```

- Normalization is per query.
- `bm25_norm` is in `[0,1]`.
- Ties break by id ascending.
- The component result includes both `raw_score` and normalized `score`.

## 7. Hybrid Ranking

```text
total = a * bm25_norm + b * fuzzy_score + c * rerank_score
```

- Initial `a = 0.50`, `b = 0.50`, `c = 0` (no extension).
- When a reranker extension is active, `c` is configured by the extension contract.
- Hybrid candidates are the union of the BM25 top-K and fuzzy top-K.
- Sort by `total` descending, then BM25 descending, then fuzzy descending, then id.
- Exact keyword mode remains available for deterministic facet-like retrieval.

## 8. Explain Integration

`weave explain --mode bm25` reports:

- raw BM25 score,
- normalized score,
- per-query-term contributions,
- matched fields,
- document/field frequencies used by FTS5 where available.

`weave explain --mode hybrid` includes BM25, fuzzy, and hybrid components.

## 9. Calibration

Extend `engine/search/testdata/calibration.json` with:

- multi-token queries where only some terms match,
- phrase queries,
- title-vs-body lexical weighting,
- permissive synthetic documents,
- exact-vs-bm25 comparisons.

Baseline metrics:

- NDCG@10 and MRR@10 must be recorded for BM25 and hybrid.
- Hybrid must not regress the existing calibration baseline.
- Changes to field weights require a calibration diff.

## 10. Migration and Compatibility

- Exact/prefix keyword search remains available as `--mode exact`.
- `keyword` remains an alias during the v1.0 transition.
- The v1 contract adds mode `bm25`, optional raw lexical scores and per-term diagnostics.
- Existing goldens are retained until the BM25 cutover; BM25 gets its own calibration corpus.

## 11. Testing

- FTS5 index creation and migration.
- Multi-token OR/coordination ranking.
- Phrase and negative-term queries.
- Field-weight ordering.
- Normalization edge cases (all zero scores, one candidate, ties).
- Explain contributions.
- Calibration baseline.
- Permissive synthetic documents.

## 12. Acceptance Criteria

- Multi-token queries no longer drop documents that match a subset of terms.
- BM25 ranking is deterministic and reproducible.
- Hybrid normalization is bounded and calibrated.
- `explain` exposes lexical contributions.
- Exact mode preserves existing behavior.
- Calibration meets or exceeds the committed baseline.

## Related Documents

- [Keyword + Fuzzy Search](./keyword-fuzzy-search.md)
- [Storage & Sync](./storage-and-sync.md)
- [Weave Explain](./weave-explain.md)
- [Permissive Ingestion](./permissive-ingestion.md)
- [ROADMAP](../../ROADMAP.md)
