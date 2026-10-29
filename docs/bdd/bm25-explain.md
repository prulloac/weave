# BDD: BM25 Explain

Human-readable behavior cases for lexical and persistent-hybrid explanations.

**Spec reference:** [weave-explain.md](../specs/weave-explain.md)
**Go tests:** `cmd/weave/bm25_explain_bdd_test.go`

---

## Feature: Explain a BM25 Result

### Scenario: BM25 evidence matches the search score

- **Given** the BM25 bundle is ingested
- **When** I explain `both` for query `alpha beta` with `--mode bm25`
- **Then** `found` is `true`
- **And** `lexical.raw_score` and `lexical.score` equal the BM25 search hit
- **And** `rank` equals the hit's position in the BM25 top-K
- **And** `matched_fields` is not empty
- **And** token evidence includes `alpha` and `beta`

### Scenario: A known id outside top-K reports rank zero

- **Given** the BM25 bundle is ingested
- **When** I explain a known id with `--top-k 1` and that id ranks below first
- **Then** `found` is `true`
- **And** `rank` is `0`
- **And** `lexical.raw_score` matches the full BM25 search raw score

### Scenario: Unknown id is a concept-not-found failure

- **When** I explain an unknown id with `--mode bm25`
- **Then** the process exits `1`
- **And** the failure envelope reports `CONCEPT_NOT_FOUND`

### Scenario: Empty query has nothing to explain

- **When** I run BM25 explain with an empty query
- **Then** the process exits `1`

### Scenario: BM25 explain requires persistence

- **When** I run BM25 explain with `--no-db`
- **Then** the process exits `2`
- **And** the failure envelope reports `INVALID_REQUEST`

### Scenario: BM25 explain is deterministic

- **When** I run the same BM25 explanation twice
- **Then** both results are identical

### Scenario: Human output includes lexical evidence

- **When** I run BM25 explain without `--json`
- **Then** stdout contains the raw score, normalized score, matched fields and token contributions

---

## Feature: Explain Persistent Hybrid Ranking

### Scenario: Hybrid explanation includes BM25 and fuzzy components

- **Given** the BM25 bundle is ingested
- **And** I have the persistent hybrid search result for `alpha beta`
- **When** I explain the same id with `--mode hybrid`
- **Then** `lexical` is present and `lexical.score` matches the hybrid hit
- **And** `fuzzy` is present
- **And** `hybrid.lexical_weight` is `0.50`
- **And** `hybrid.fuzzy_weight` is `0.50`
- **And** `hybrid.total = 0.50 * lexical.score + 0.50 * fuzzy.score`
- **And** `rank` equals the hit's position in the hybrid top-K
