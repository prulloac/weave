# BDD: Weave Explain

Human-readable behavior cases for `weave explain`.

**Spec reference:** [weave-explain.md](../specs/weave-explain.md)
**Go tests:** `cmd/weave/explain_bdd_test.go`

---

## Feature: Explain a Hybrid Search Result

As a knowledge worker, I want to know why a document ranked where it did so that I can trust and tune the ranking.

### Scenario: Explain the rank-1 hybrid result by default

- **Given** a bundle containing OAuth, token-storage and unrelated concepts
- **When** I run `weave explain <bundle> oauth --json`
- **Then** the result is `found: true`
- **And** the explained id is the rank-1 hybrid result from `weave search`
- **And** `rank` is `1`
- **And** the mode is `hybrid`

### Scenario: Explain a specific hybrid result

- **Given** the same bundle
- **When** I run `weave explain <bundle> "oauth tokn" --id concepts/oauth --json`
- **Then** the keyword, fuzzy and hybrid scores equal the matching `weave search` component scores
- **And** `rank` equals the document's 1-based position in the hybrid top-K
- **And** the keyword evidence contains an exact match for `oauth`
- **And** the keyword evidence contains no match for `tokn`
- **And** the fuzzy evidence contains at least one `tokn` token match

### Scenario: Repeat explanations are deterministic

- **Given** a hybrid explanation request
- **When** I run it twice
- **Then** both JSON results are identical

---

## Feature: Explain Individual Search Modes

As a ranking maintainer, I want each mode's score isolated so that I can debug keyword and fuzzy behavior separately.

### Scenario: Keyword mode explains exact and prefix matches

- **Given** a bundle with the keyword `oauth` and the keyword `security`
- **When** I explain query `oauth` in keyword mode
- **Then** the keyword score is `1.0`
- **And** the token evidence reports an `exact` match
- **When** I explain query `sec` in keyword mode
- **Then** the keyword score is `0.75`
- **And** the token evidence reports a `prefix` match to `security`

### Scenario: Keyword mode reports a no-match query token

- **Given** a known concept whose keyword `oauth` matches, while `missing` matches no keyword
- **When** I explain query `oauth missing` in keyword mode with that concept selected
- **Then** the result is `found: true`
- **And** `rank` is `0` because the strict-AND search produces no candidate
- **And** the token evidence reports `none` for `missing`

### Scenario: Fuzzy mode explains a typo

- **Given** a bundle containing the token `Token` in several fields
- **When** I explain query `tokn` in fuzzy mode for the token-storage concept
- **Then** the fuzzy score equals the `weave search` fuzzy score
- **And** the fuzzy token evidence includes a match to `token`
- **And** the similarity is greater than zero

---

## Feature: Handle Known and Unknown IDs

### Scenario: Known id outside top-K reports rank zero

- **Given** query `token` where `concepts/token` ranks below `concepts/oauth`
- **When** I explain `concepts/token` with `--top-k 1`
- **Then** `found` is `true`
- **And** `rank` is `0`
- **And** the component scores still match the full search scores for that id

### Scenario: Unknown id is a concept-not-found failure

- **Given** a bundle without `concepts/missing`
- **When** I run `weave explain <bundle> oauth --id concepts/missing --json`
- **Then** the process exits `1`
- **And** the failure envelope reports `CONCEPT_NOT_FOUND`

### Scenario: Empty query has nothing to explain

- **Given** an empty query
- **When** I run `weave explain <bundle> "" --json`
- **Then** the process exits `1`

### Scenario: Query with no matches has nothing to explain

- **Given** a query that matches no concept
- **When** I run `weave explain <bundle> zzzz --json`
- **Then** the process exits `1`

---

## Feature: Human Output

As a CLI user, I want a readable score breakdown.

### Scenario: Human output shows components and rank

- **Given** the rank-1 hybrid result for query `oauth`
- **When** I run `weave explain <bundle> oauth` without `--json`
- **Then** stdout contains the query, mode, result id and rank
- **And** stdout contains keyword, fuzzy and hybrid sections
