# BDD: BM25 Lexical Ranking

Human-readable behavior cases for FTS5 BM25 ranking.

**Spec reference:** [bm25-ranking.md](../specs/bm25-ranking.md)
**Go tests:** `cmd/weave/bm25_bdd_test.go`

This slice covers `--mode bm25` and FTS indexing. Hybrid rebase, BM25 explain and calibration are
the follow-up calibration slice.

---

## Feature: FTS Indexing

### Scenario: Ingest indexes valid and synthetic documents

- **Given** a permissive bundle with valid and synthetic Markdown documents
- **When** I ingest it into the default database
- **And** I run `weave search <bundle> alpha --mode bm25 --json`
- **Then** the result contains `title-alpha` and `synthetic`
- **And** every lexical hit has a positive `raw_score` and a normalized `score` in `[0,1]`

### Scenario: Sync updates the FTS index

- **Given** the BM25 bundle is ingested
- **When** I change `beta-only` so its body contains `alpha`
- **And** I run `weave sync <bundle> --permissive --json`
- **And** I run BM25 search for `alpha`
- **Then** `beta-only` appears in the results

---

## Feature: Multi-Token Lexical Ranking

### Scenario: Documents matching a subset of query terms are included

- **Given** the BM25 bundle is ingested
- **When** I run `weave search <bundle> "alpha beta" --mode bm25 --json`
- **Then** `alpha-only`, `beta-only` and `both` all appear
- **And** exact keyword AND semantics are not applied

### Scenario: Title matches outweigh body matches

- **Given** the BM25 bundle is ingested
- **When** I run BM25 search for `alpha`
- **Then** `title-alpha` ranks above `body-alpha`

### Scenario: Phrase queries are exact

- **Given** documents containing `red green blue` and `green red`
- **When** I run BM25 search for `"red green"`
- **Then** `phrase` appears
- **And** `phrase-reverse` does not

### Scenario: Negative terms are excluded

- **When** I run BM25 search for `alpha -beta`
- **Then** `alpha-only` and `title-alpha` appear
- **And** `both` and `beta-only` do not

### Scenario: Top-K caps lexical results

- **When** I run BM25 search for `alpha --top-k 1 --json`
- **Then** exactly one lexical result is returned

### Scenario: Repeated BM25 searches are deterministic

- **When** I run the same BM25 search twice
- **Then** the lexical result lists are identical

---

## Feature: BM25 Errors

### Scenario: BM25 requires persistent storage

- **When** I run `weave search <bundle> alpha --mode bm25 --no-db --json`
- **Then** the process exits `2`
- **And** the failure envelope reports `INVALID_REQUEST`

### Scenario: Empty query has no lexical results

- **When** I run BM25 search with an empty query
- **Then** the process exits `1`
