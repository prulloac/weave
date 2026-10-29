# BDD: BM25 Calibration

Human-readable behavior cases for BM25 hybrid ranking and calibration.

**Spec reference:** [bm25-ranking.md](../specs/bm25-ranking.md)
**Go tests:** `cmd/weave/bm25_calibration_bdd_test.go`, `engine/search/bm25_calibration_test.go`

BM25 explain evidence is a follow-up explain slice.

---

## Feature: Persistent Hybrid Ranking

### Scenario: Hybrid uses normalized BM25 plus fuzzy

- **Given** the BM25 bundle is ingested into the default database
- **When** I run `weave search <bundle> "alpha beta" --mode hybrid --json`
- **Then** at least one hybrid hit has `lexical_score > 0`
- **And** every hybrid hit satisfies `total_score = 0.5 * lexical_score + 0.5 * fuzzy_score`
- **And** hybrid hits do not use keyword scores

### Scenario: Hybrid is deterministic

- **Given** the same ingested bundle
- **When** I run the same hybrid search twice
- **Then** the hybrid result lists are identical

### Scenario: `--no-db` keeps the legacy hybrid fallback

- **Given** a bundle that has not been indexed
- **When** I run `weave search <bundle> alpha --mode hybrid --no-db --json`
- **Then** the search succeeds
- **And** hybrid hits use keyword and fuzzy scores, with `lexical_score` absent or zero

---

## Feature: BM25 Calibration Corpus

### Scenario: BM25 cases meet their baseline

- **Given** the calibration corpus includes multi-token, field-weighting and phrase BM25 cases
- **When** I run the BM25 calibration suite
- **Then** NDCG@10 and MRR@10 meet the recorded BM25 baseline

### Scenario: Hybrid cases meet their baseline

- **Given** the calibration corpus includes typo and graph hybrid cases
- **When** I run the hybrid calibration suite
- **Then** NDCG@10 and MRR@10 meet the recorded hybrid baseline

### Scenario: Calibration is reproducible

- **Given** the calibration corpus is unchanged
- **When** I run the calibration suite twice
- **Then** the measured metrics are identical
