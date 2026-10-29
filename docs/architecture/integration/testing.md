---
type: Test Strategy
title: "Integration Testing"
description: "Go unit, conformance, calibration, cache, CLI process and race tests for the related-document database."
tags:
  - "architecture"
  - "testing"
  - "conformance"
  - "calibration"
  - "go"
generated:
  by: opencode/deepseek-v4.1-flash
  at: "2026-10-25T00:00:00Z"
status: draft
---

# Integration Testing

## Strategy

| Level | Scope | Tooling | Gate |
| --- | --- | --- | --- |
| 1 | Contract schemas | `go test ./contract/v1` | Goldens validate and round-trip |
| 2 | Engine unit/race | `go test -race ./...` | No races; package tests pass |
| 3 | Golden conformance | `engine/conformance_test.go` | Bundle, graph and search match goldens |
| 4 | Calibration | `engine/search/calibration_test.go` | NDCG@K and MRR@K meet baseline |
| 5 | CLI process | `cmd/weave/main_test.go` | Envelopes, stderr, exit codes, cache |
| 6 | Performance | Go benchmarks | Budgets documented |

No browser, HTTP or Playwright tests exist.

## Conformance Corpus

Location: `contract/v1/golden`.

- Fixtures include mandatory keywords.
- Each case exports bundle, graph and search expectations.
- Search cases declare query, mode and top-K.
- Navigation cases cover list, show, backlinks and path.
- `find` cases are gone.

Commands:

```sh
go run ./cmd/contract-export
go test ./contract/v1
go test ./engine/...
```

## Calibration

Location: `engine/search/testdata/calibration.json`.

Cases cover:

- exact keywords,
- keyword prefixes,
- multi-keyword AND queries,
- typos,
- title vs description vs body weighting,
- missing-keyword documents,
- empty queries.

Metrics:

- NDCG@K (primary)
- MRR@K
- precision@K (reported)
- recall@K (reported)

CI fails if the averages drop below the committed baseline. Current baseline:
**NDCG@10 >= 0.75, MRR@10 >= 0.70**. Measured at implementation time: NDCG@10 ≈ 0.974,
MRR@10 = 1.0.

## CLI Tests

- Every command: `contract`, `parse`, `graph`, `search`, `list`, `show`, `backlinks`, `path`,
  `version`, `audit`, `relate`, `explain`, `ingest`, `rebuild`, `sync`.
- Behavior suites: `cmd/weave/audit_bdd_test.go` covers `weave audit` report codes, ordering,
  `--fail-on`, human output, invalid roots and bundle immutability. `cmd/weave/relate_bdd_test.go`
  covers relate depth, direction, cycles, related edges, ordering and human output.
  `cmd/weave/explain_bdd_test.go` covers explain score parity, rank, modes and human output.
  `cmd/weave/storage_bdd_test.go` covers default-DB ingest, bundle isolation, `--db`/`--no-db`,
  persistence without sources and storage error paths. `cmd/weave/permissive_bdd_test.go` covers
  permissive ingest, synthetic documents, search inclusion and permissive audit.
  `cmd/weave/sync_bdd_test.go` covers hash skip/update/add/remove/rename, cascade and sync errors.
- Envelope validity and stdout purity.
- Human output.
- Exit codes: 0/1/2/3.
- Search modes and top-K boundaries.
- Empty results still emit a success envelope but exit 1.
- Cache default, `--no-cache`, hit and invalidation.
- Determinism.

## Race and Cache

- `go test -race ./...` is required in CI and `scripts/verify.sh`.
- Cache tests cover hit, invalidation, concurrency safety and no-cache parity.

## Performance

Targets for 10,000 documents:

| Query | Target |
| --- | --- |
| Keyword top-10 | < 20 ms |
| Fuzzy top-10 | < 50 ms |
| Hybrid top-10 | < 50 ms |

Benchmarks live in `engine/bench_test.go`.

## CI

`go` job:

- regenerate goldens and fail on drift,
- `go test -race ./...`,
- `go vet ./...`,
- gofmt check,
- `go build ./cmd/weave`.

`docs` job: markdownlint.

## Related Documents

- [ADR 0003](../adr/0003-keyword-fuzzy-search-and-ui-retirement.md)
- [Keyword + Fuzzy Search spec](../../specs/keyword-fuzzy-search.md)
- [Contract layer](../layers/contract.md)
- [Engine layer](../layers/engine-go.md)
- [CLI layer](../layers/cli-go.md)
- [Integration contracts](contracts.md)
