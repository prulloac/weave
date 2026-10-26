---
type: Migration Plan
title: "Integration Steps"
description: "Implementation plan for the new v1: mandatory keywords, weighted fuzzy search, hybrid top-K, calibration and UI removal."
tags:
  - "architecture"
  - "migration"
  - "search"
  - "keywords"
  - "calibration"
generated:
  by: opencode/deepseek-v4.1-flash
  at: "2026-10-25T00:00:00Z"
status: draft
---

# Integration Steps

## Principles

1. **Contract first.** Schemas and fixtures define the search model.
2. **CLI is the integration boundary.** No UI, HTTP or daemon.
3. **Calibration is a gate.** Ranking changes must be measured.
4. **One implementation.** Go is the engine; the old TypeScript implementation is removed.
5. **Deterministic output.** Same inputs produce the same top-K order.

## Phase 0 — Contract v1 rewrite (complete)

- Add mandatory `keywords` to concepts and graph nodes.
- Add `has_keywords` to validation.
- Replace `FindResult` with keyword/fuzzy/hybrid search types.
- Replace `find` with `search` in operations.
- Regenerate fixtures and goldens with keywords.
- Remove `SerializedGraph` and UI-only types.

Gate: `go test ./contract/v1` passes.

## Phase 1 — Engine search (complete)

- Keyword index with exact/prefix scoring and AND semantics.
- Fuzzy field/trigram indexes.
- Levenshtein + trigram Dice similarity.
- Field weights `title 1.00`, `description 0.60`, `body 0.30`.
- Hybrid combination and top-K.
- Missing-keywords exclusion from keyword search.
- Cache and concurrency safety.

Gate: conformance and race tests pass.

## Phase 2 — Calibration (complete)

- Labeled corpus in `engine/search/testdata/calibration.json`.
- NDCG@K, MRR@K, precision and recall.
- Committed baseline: NDCG@10 >= 0.75, MRR@10 >= 0.70.
- Test fails below baseline.

Gate: `TestCalibration` passes.

## Phase 3 — CLI (complete)

- Remove `find`; add `weave search`.
- `--mode` and `--top-k`.
- Envelopes, human output and exit codes.
- Update contract discovery.
- Process tests.

Gate: `cmd/weave` tests and `scripts/verify.sh` pass.

## Phase 4 — UI and legacy cleanup (complete)

- Remove Astro, Preact, Tailwind, Playwright and public assets.
- Remove the TypeScript engine, CLI, renderers, mount, lens and UI tests.
- Remove UI specifications and BDD files.
- Remove `mount`, `unmount`, `status` and `find` commands.
- Simplify `package.json`, CI and `scripts/verify.sh`.
- Update README, CONTRIBUTING and architecture docs.

Gate: `./scripts/verify.sh` passes on a Go-only tree.

## Post-v1 Backlog

- `weave engine --stdio`.
- MCP transport over operations.
- Synonyms and stemming.
- `--no-body` for large parse payloads.

## Definition of Done

- Every concept has keywords or is explicitly invalid.
- Search returns deterministic keyword, fuzzy and hybrid top-K lists.
- Calibration meets the baseline.
- No UI or TypeScript implementation remains.
- `go test -race ./...`, `go vet ./...`, `gofmt`, contract export and docs lint pass.

## Related Documents

- [ADR 0003](../adr/0003-keyword-fuzzy-search-and-ui-retirement.md)
- [Keyword + Fuzzy Search spec](../../specs/keyword-fuzzy-search.md)
- [Contract layer](../layers/contract.md)
- [Engine layer](../layers/engine-go.md)
- [Operations layer](../layers/operations.md)
- [CLI layer](../layers/cli-go.md)
- [Integration contracts](contracts.md)
- [Integration testing](testing.md)
