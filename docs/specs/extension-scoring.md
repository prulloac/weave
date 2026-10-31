---
type: specification
title: "Pluggable Scoring and Custom Ranking Specification"
description: "Post-GA extension contract for scoring search candidates with document context and configurable blend weights."
tags:
  - "specification"
  - "extensions"
  - "scoring"
  - "ranking"
  - "post-ga"
generated:
  by: opencode/deepseek-v4.1-flash
  at: "2026-10-31T00:00:00Z"
status: draft
---

# Specification: Pluggable Scoring and Custom Ranking

## 1. Overview

This is a **post-GA** feature. The shipped v1.0 release does not depend on it and the default ranking
does not change.

Weave already supports a lightweight reranker: `weave search --rerank <name>` sends the query and
the current top-K candidate IDs and scores to a `weave-<name>` process, then blends the returned
score at a fixed `0.5 / 0.5` weight. That contract is intentionally narrow: the extension cannot see
document context, cannot broaden the candidate set and cannot choose the blend.

This specification adds a larger **scorer** contract:

- the core retrieves a bounded candidate set from an existing mode;
- the scorer receives the query plus safe document context for every candidate;
- the scorer returns one score per candidate;
- the operator chooses the blend weight, including full replacement of the core score;
- the final result keeps the existing hit shapes and adds an optional `scorer_score`.

The core still owns retrieval, validation, privacy and output. A scorer controls the ordering of a
bounded candidate set, not the index or the candidate-generation algorithm.

Non-goals:

- Custom tokenizers, parsers, indexes or retrieval engines.
- Adding candidates outside the core candidate set.
- Extension writes to the bundle or database.
- Long-lived scorer processes, streaming or extension-managed caches.
- Remote scorer hosting, multi-tenant execution, sandboxing or signature verification.
- A Go plugin ABI or a stable Go `Scorer` interface.
- Changing default ranking, weights, calibration or `--rerank`.
- MCP scorer wiring in the first slice.

## 2. Compatibility

- `weave.cli/v1` remains authoritative. New message types, request fields, response fields and hit
  fields are additive.
- The shipped `--rerank` path is unchanged: same request shape, fixed `0.5 / 0.5` blend, same
  fallback behavior.
- `--scorer` is a new opt-in path. A search without `--scorer` is byte-for-byte compatible with the
  current output.
- Old consumers ignore the optional `scorer_score` field.
- Existing contract goldens and calibration baselines must remain unchanged when `--scorer` is
  absent.
- A scorer extension is an executable named `weave-<name>` discovered by the existing rules in
  [`extension-ipc.md`](./extension-ipc.md).

## 3. Contracts

### 3.1 Discovery and invocation

- Discovery, naming, executable checks, environment and working-directory inheritance are identical
  to the extension IPC contract.
- One process per search invocation, one JSON request on stdin, one JSON response on stdout.
- Stderr is diagnostic only and never parsed.
- Invocations are sequential. A scorer is not called concurrently for one search.
- Default timeout is **10 seconds** for a scorer. `--timeout` overrides it.
- Request frame limit is **8 MiB**; response frame limit is **1 MiB**. Exceeding either is
  `EXTENSION_PROTOCOL`.
- The scorer never receives write access through Weave; it must not modify the bundle or database.

### 3.2 Scorer request

```json
{
  "v": "weave.cli/v1",
  "type": "scorer_request",
  "query": "zero cgo database",
  "mode": "hybrid",
  "candidate_k": 100,
  "candidates": [
    {
      "id": "docs/adr-004.md",
      "path": "docs/adr-004.md",
      "title": "Database Storage Selection",
      "description": "We decided to adopt pure-Go SQLite.",
      "type": "adr",
      "status": "accepted",
      "keywords": ["database", "storage", "sqlite"],
      "snippet": "We decided to adopt pure-Go SQLite for the local index.",
      "scores": {
        "keyword": 0.82,
        "lexical": 0.82,
        "fuzzy": 0.45,
        "core": 0.82
      }
    }
  ]
}
```

Fields:

| Field | Required | Meaning |
| --- | --- | --- |
| `v` | yes | `weave.cli/v1` |
| `type` | yes | `scorer_request` |
| `query` | yes | Original query string |
| `mode` | yes | `keyword`, `fuzzy`, `hybrid` or `bm25` |
| `candidate_k` | yes | Number of candidates in this request |
| `candidates` | yes | Exactly `candidate_k` entries, ordered by core rank |
| `candidates[].id` | yes | Stable document ID |
| `candidates[].path` | yes | Bundle-relative path |
| `candidates[].title` | no | Title, omitted when empty |
| `candidates[].description` | no | Description, omitted when empty |
| `candidates[].type` | no | Frontmatter `type`, omitted when empty |
| `candidates[].status` | no | Frontmatter `status`, omitted when empty |
| `candidates[].keywords` | no | Declared and inferred keywords, bounded to 64 |
| `candidates[].snippet` | no | Best-effort masked snippet, bounded to 2 KiB |
| `candidates[].scores` | yes | Normalized core dimensions and the selected mode score |

`core` is the score the selected mode uses for ranking. Unavailable dimensions are `0`.

The request deliberately excludes raw frontmatter, file bodies, privacy metadata, keys and
encrypted content. A scorer that needs more context must read the bundle itself; that is outside
the contract.

### 3.3 Scorer response

```json
{
  "v": "weave.cli/v1",
  "ranked_candidates": [
    { "id": "docs/adr-004.md", "score": 0.95 },
    { "id": "docs/adr-003.md", "score": 0.40 }
  ]
}
```

Fields:

| Field | Required | Meaning |
| --- | --- | --- |
| `v` | yes | `weave.cli/v1` |
| `ranked_candidates` | yes | Exactly one entry per request candidate |
| `ranked_candidates[].id` | yes | A candidate ID from the request |
| `ranked_candidates[].score` | yes | Finite number in `[0,1]` |

Rules:

- Every request candidate must appear exactly once. Additions, omissions and duplicates are
  `EXTENSION_PROTOCOL`.
- `score` is clamped to `[0,1]`. `NaN` and infinities are `EXTENSION_PROTOCOL`.
- Unknown response fields are ignored.
- An unknown `v` is `EXTENSION_PROTOCOL`.
- Ordering in the response is irrelevant; the core sorts by final score and ID.

### 3.4 CLI

```sh
weave search <path> <query> --scorer <name> [--scorer-weight <0..1>] [--candidate-k <n>] [--timeout <d>] --json
weave explain <path> <query> --scorer <name> [--scorer-weight <0..1>] [--candidate-k <n>] [--timeout <d>] --json
```

- `--scorer` names one extension. The name follows the extension naming rules.
- `--scorer-weight` defaults to `0.5`, accepts `0` through `1` inclusive, and is rounded to two
  decimal places for display only.
- `--candidate-k` defaults to `100`, accepts `1` through `1000`, and must be at least `--top-k`.
- `--scorer` and `--rerank` are mutually exclusive.
- `--scorer-weight` and `--candidate-k` require `--scorer`.
- `--timeout` applies to the scorer invocation.
- A missing or invalid scorer fails before search output with `EXTENSION_NOT_FOUND`, exit `2`.
- A scorer failure, timeout or protocol violation keeps the core results unchanged, emits one
  stderr warning and exits `0`.
- JSON stdout remains one `weave.cli/v1` envelope. A successful response adds:

```json
{
  "scorer": {
    "name": "semantic",
    "weight": 0.8,
    "candidate_k": 200
  }
}
```

and each hit may carry `scorer_score`.

### 3.5 Explain

`weave explain` accepts the same scorer flags. When a scorer is active, the result includes:

- `core_score`: the score from the selected mode;
- `scorer_score`: the score returned by the extension;
- `final_score`: the blended score;
- `scorer`: name, weight and candidate count.

The existing keyword, lexical, fuzzy and hybrid evidence remains unchanged. A scorer adds one
`explain_scorer` block rather than replacing core evidence.

### 3.6 Contract additions

The following optional fields are additive to `weave.contract/v1`:

| Type | Field | Meaning |
| --- | --- | --- |
| `KeywordHit`, `FuzzyHit`, `LexicalHit`, `HybridHit` | `scorer_score` | Extension score, omitted without `--scorer` |
| `SearchResponse` | `scorer` | Scorer name, weight and candidate count |
| `ExplainResult` | `explain_scorer` | Scorer score and blend evidence |

Existing fields, ordering and exit codes remain unchanged when the feature is absent.

## 4. Behavior

### 4.1 Candidate selection

1. Run the core retrieval for the selected mode with `candidate_k` as the candidate limit.
2. Build the scorer request from the first `candidate_k` results.
3. Invoke the scorer.
4. Validate the response against the candidate set.
5. Blend scores and sort.
6. Truncate to `top_k` for the response.

`candidate_k` broadens recall within the selected core mode. It does not add documents that the
mode did not retrieve. A custom retrieval extension is a separate future contract.

### 4.2 Context and privacy

- Only searchable candidates are scored. Encrypted documents excluded from the index are never
  sent.
- `snippet` is the same masked snippet the search response would expose. Plaintext from encrypted
  content is never sent.
- Tokenized and hashed content is sent in its stored searchable form.
- `keywords` contains declared and inferred keywords. Raw frontmatter is not sent.
- The scorer inherits the process environment and working directory, as existing extensions do.
  Operators are responsible for what their scorer reads.

### 4.3 Blending and ordering

```text
final = (1 - scorer_weight) * core_score + scorer_weight * scorer_score
```

- `scorer_weight = 0` skips the extension entirely and returns the core result.
- `scorer_weight = 1` ranks only by the scorer score within the candidate set.
- `0 < scorer_weight < 1` blends both scores.
- Scores are rounded with the existing score rounding.
- Sort by final score descending, then ID ascending.
- The selected mode's hit shape and component scores are preserved.
- `scorer_score` is copied to every hit that participated in the scorer request.

### 4.4 Failure and fallback

- Missing extension: fail fast with `EXTENSION_NOT_FOUND`, exit `2`.
- Timeout, non-zero exit, malformed JSON, wrong protocol version, missing candidate, duplicate
  candidate, extra candidate, invalid score or oversized frame: keep the core results unchanged,
  emit one warning on stderr and exit `0`.
- A warning never changes the JSON envelope or the exit code.
- A failed scorer is not retried in the same search.

### 4.5 Determinism and limits

- The same bundle, query, mode, scorer and weights must produce the same candidate set and order.
- Extensions must be deterministic for the same request if reproducible ranking is required.
- `candidate_k` is capped at `1000`; `top_k` remains capped at `100`.
- The request is capped at `8 MiB`; the response at `1 MiB`.
- The scorer is invoked once per search.
- No scorer runs for `--mode keyword --scorer-weight 0` or any other weight-zero request.

## 5. Data Flow

```text
CLI search --scorer --scorer-weight --candidate-k
  -> engine/search              (core candidates, up to candidate_k)
  -> engine/extensions runner   (build context, invoke, validate)
  -> engine/operations          (blend, sort, truncate)
  -> contract/v1                (optional scorer_score and scorer metadata)
  -> CLI/MCP response
```

`weave explain --scorer` follows the same path but returns score evidence instead of a full hit
list.

## 6. Validation and Errors

| Condition | Behavior |
| --- | --- |
| `--scorer` and `--rerank` together | Usage error, exit `2` |
| `--scorer-weight` outside `[0,1]` | Usage error, exit `2` |
| `--candidate-k` below `--top-k` | Usage error, exit `2` |
| `--candidate-k` above `1000` | Usage error, exit `2` |
| Missing scorer executable | `EXTENSION_NOT_FOUND`, exit `2` |
| Scorer exits non-zero | Core results unchanged, warning, exit `0` |
| Scorer times out | Core results unchanged, warning, exit `0` |
| Response missing a candidate | Core results unchanged, warning, exit `0` |
| Response adds or duplicates a candidate | Core results unchanged, warning, exit `0` |
| Response score is `NaN`, infinite or non-numeric | Core results unchanged, warning, exit `0` |
| Request or response exceeds frame limit | Core results unchanged, warning, exit `0` |
| Encrypted document in candidate set | Not possible; index excludes it |
| `scorer_weight = 0` | Scorer not invoked; core result returned |
| `scorer_weight = 1` | Final order is the scorer order within the candidate set |
| Extension failure during `explain` | Core evidence returned with warning, exit `0` |

## 7. Testing

- Unit tests for request construction, context bounds, privacy fields, score validation, weights
  `0`, `0.5` and `1`, candidate coverage and frame limits.
- CLI process tests with a deterministic fixture scorer that reverses the candidate order.
- A test proving `--scorer-weight 0` does not invoke the extension.
- A test proving `--scorer-weight 1` ignores the core score for ordering.
- A test proving a missing or malformed scorer response falls back to the exact core output.
- A test proving encrypted content is absent from the scorer request.
- A test proving `--rerank` output is unchanged.
- A test proving `explain` reports core, scorer and final scores.
- Contract schema and golden updates are additive; old goldens remain valid.
- Calibration stays green without `--scorer`; scorer fixtures are deterministic and do not enter
  the labeled corpus.
- Race coverage for concurrent CLI invocations; no shared scorer process state.
- Performance benchmark at `candidate_k = 100`, `500` and `1000` after implementation.

## 8. Acceptance Criteria

1. `weave search --scorer` invokes a discovered `weave-<name>` with the documented context.
2. `--scorer-weight 0` returns the exact core result without invoking the extension.
3. `--scorer-weight 1` orders the candidate set only by scorer scores.
4. `--candidate-k` controls the candidate set and the final response is truncated to `top_k`.
5. The scorer response must cover exactly the request set; every violation falls back safely.
6. Encrypted content and raw frontmatter never appear in the scorer request.
7. `scorer_score` and scorer metadata are additive and absent without `--scorer`.
8. `weave explain --scorer` reports core, scorer and final scores.
9. Existing `--rerank`, default search output, contract goldens and calibration remain unchanged.
10. A missing scorer exits `2`; a failed scorer exits `0` with core results and one warning.
11. CLI help, README, spec index and backlog describe the post-GA feature.
12. The feature is not part of the v1.0 release gate.

## 9. Delivery Slices

| Slice | Scope |
| --- | --- |
| S1 | Contract types, scorer request/response validation, CLI flags and fallback |
| S2 | Document context, snippets, privacy filtering and `candidate_k` |
| S3 | Blend weights, `scorer_score`, response metadata and `explain` |
| S4 | BDD scenarios, CLI process tests, fixtures and performance benchmark |
| S5 | Documentation and post-GA release notes |

Each slice is a separate stacked PR. S1 must not change default output or `--rerank`.

## 10. Open Questions

- Should MCP expose `scorer` in `weave_search` after the CLI slice, or keep scoring CLI-only?
- Should a future retrieval extension be allowed to propose candidate filters, or is core-mode
  candidate generation the permanent boundary?
- Should scorers be allowed to return an optional explanation string, or is numeric evidence
  sufficient?

## 11. Related Documents

- [Extension Subprocess IPC](./extension-ipc.md) — discovery, transport and the shipped reranker
- [Keyword + Fuzzy Search](./keyword-fuzzy-search.md) — core keyword/fuzzy ranking
- [BM25 Ranking](./bm25-ranking.md) — lexical ranking and normalization
- [Weave Explain](./weave-explain.md) — score evidence
- [Privacy Directives](./privacy-directives.md) — masked snippets and encrypted content
- [Optimization Program](./optimization.md) — post-GA performance work
- [Roadmap](../../ROADMAP.md) — post-GA feature register
