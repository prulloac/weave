---
type: behavior
title: "Pluggable Scoring Behavior Scenarios"
description: "Behavior cases for custom scorer extensions, document context, blend weights and safe fallback."
tags:
  - "behavior"
  - "extensions"
  - "scoring"
  - "ranking"
  - "post-ga"
generated:
  by: opencode/deepseek-v4.1-flash
  at: "2026-10-31T00:00:00Z"
status: stable
---

# Behavior Scenarios: Pluggable Scoring

Specification: [extension-scoring.md](https://github.com/prulloac/weave-docs/blob/main/specs/extension-scoring.md)

Go tests: `cmd/weave/scoring_bdd_test.go`, `engine/extensions/scoring_test.go`.

These scenarios describe observable behavior. They do not prescribe implementation details.

## Feature: Scorer discovery

### Scenario: A bundle-local scorer is preferred

- **Given** a scorer named `semantic` in `<bundle>/.weave/bin/` and another on `$PATH`
- **When** I run `weave search <bundle> <query> --scorer semantic --json`
- **Then** the bundle-local executable is invoked
- **And** the response reports the scorer name and weight

### Scenario: A missing scorer fails before search

- **Given** no executable named `weave-semantic`
- **When** I run `weave search <bundle> <query> --scorer semantic --json`
- **Then** the command exits `2`
- **And** the error code is `EXTENSION_NOT_FOUND`
- **And** no search result is emitted

## Feature: Scorer request context

### Scenario: The scorer receives candidates with safe context

- **Given** an indexed bundle with titles, descriptions and keywords
- **When** a scorer is invoked
- **Then** the request contains the query, mode and `candidate_k`
- **And** each candidate has its ID, path, title, description, type, status and keywords
- **And** each candidate has keyword, lexical, fuzzy and core scores
- **And** the request contains no raw frontmatter or file body

### Scenario: Snippets are bounded and masked

- **Given** a long document and an encrypted document excluded from the index
- **When** a scorer is invoked
- **Then** the snippet is at most 2 KiB
- **And** the snippet matches the masked search snippet
- **And** encrypted plaintext is absent from the request

### Scenario: Keywords are bounded

- **Given** a document with more than 64 declared and inferred keywords
- **When** a scorer is invoked
- **Then** the candidate keyword list contains at most 64 entries

## Feature: Candidate selection

### Scenario: Candidate K broadens the candidate set

- **Given** an indexed bundle with at least 200 matching documents
- **When** I run search with `--candidate-k 100 --top-k 10`
- **Then** the scorer receives at most 100 candidates
- **And** the final response contains at most 10 hits

### Scenario: Candidate K must cover Top K

- **Given** `--top-k 50`
- **When** I pass `--candidate-k 20`
- **Then** the command exits `2`
- **And** the hint explains that candidate K must be at least top K

### Scenario: Candidate K is capped

- **Given** a search request
- **When** I pass `--candidate-k 101`
- **Then** the command exits `2`

## Feature: Blend weights

### Scenario: Weight zero returns the core result without invoking the scorer

- **Given** a scorer that would reverse the ranking
- **When** I run search with `--scorer-weight 0`
- **Then** the scorer is not invoked
- **And** the result is identical to a search without `--scorer`

### Scenario: Weight one ranks by the scorer alone

- **Given** a scorer that assigns scores in reverse core order
- **When** I run search with `--scorer-weight 1`
- **Then** the final order is the reverse of the core order within the candidate set
- **And** every hit has a `scorer_score`

### Scenario: Weight half blends both scores

- **Given** known core and scorer scores
- **When** I run search with `--scorer-weight 0.5`
- **Then** each final score equals `0.5 * core + 0.5 * scorer`
- **And** ties break by ID ascending

### Scenario: An invalid weight is a usage error

- **Given** a search request
- **When** I pass `--scorer-weight 1.5` or `--scorer-weight -0.1`
- **Then** the command exits `2`

## Feature: Validation and fallback

### Scenario: A missing candidate falls back

- **Given** a scorer response that omits one candidate
- **When** I run the search
- **Then** the command exits `0`
- **And** the core results are unchanged
- **And** one warning is written to stderr

### Scenario: An extra candidate falls back

- **Given** a scorer response that adds an unknown candidate
- **When** I run the search
- **Then** the command exits `0`
- **And** the core results are unchanged
- **And** one warning is written to stderr

### Scenario: A duplicate candidate falls back

- **Given** a scorer response that repeats a candidate ID
- **When** I run the search
- **Then** the command exits `0`
- **And** the core results are unchanged
- **And** one warning is written to stderr

### Scenario: An invalid score falls back

- **Given** a scorer response with `NaN`, infinity or a non-numeric score
- **When** I run the search
- **Then** the command exits `0`
- **And** the core results are unchanged

### Scenario: A timeout falls back

- **Given** a scorer that sleeps longer than the configured timeout
- **When** I run the search with `--timeout 50ms`
- **Then** the command exits `0`
- **And** the core results are unchanged
- **And** one timeout warning is written to stderr

### Scenario: A malformed response falls back

- **Given** a scorer that prints invalid JSON or the wrong protocol version
- **When** I run the search
- **Then** the command exits `0`
- **And** the core results are unchanged

### Scenario: Oversized frames fall back

- **Given** a scorer response larger than 1 MiB
- **When** I run the search
- **Then** the command exits `0`
- **And** the core results are unchanged
- **And** the warning names the frame limit

## Feature: Evidence and compatibility

### Scenario: Search exposes the scorer score

- **Given** a successful scorer invocation
- **When** I run search with `--json`
- **Then** each hit includes `scorer_score`
- **And** the response includes scorer name, weight and candidate count

### Scenario: Explain shows core, scorer and final scores

- **Given** a successful scorer invocation
- **When** I run `weave explain --scorer <name> --id <id> --json`
- **Then** the result includes `core_score`, `scorer_score` and `final_score`
- **And** the existing core evidence is unchanged

### Scenario: Default output is unchanged

- **Given** a search without `--scorer`
- **When** I run it against the release goldens
- **Then** the output is byte-identical to the current release
- **And** no `scorer_score` field is emitted

### Scenario: The existing reranker is unchanged

- **Given** a search with `--rerank <name>`
- **When** I compare it with the shipped reranker behavior
- **Then** the request shape, fixed 0.5 blend and fallback are unchanged

### Scenario: Contract changes are additive

- **Given** the updated contract schemas
- **When** I run the schema diff
- **Then** every addition is optional
- **And** existing consumers validate without the new fields
