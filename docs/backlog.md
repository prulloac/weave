# Weave Backlog

Tracked work queue for the Weave vision: "To weave flat files into a living universe of knowledge."

**v1.0 includes every tier below.** Persistence, privacy, extensions and MCP are v1.0 scope, not a
later platform; the tiers and waves are internal delivery order.

Status legend:

- `backlog` — not started
- `in-progress` — actively worked
- `red` — tests written, failing (BDD/TDD phase)
- `green` — tests passing
- `review` — PR submitted, awaiting review/merge
- `done` — merged

Pipeline phase legend (per SDD -> BDD -> TDD):

- `-` — not applicable / not started
- `x` — complete

## Tier 1 — Foundation

| Feature | Spec | PR | Spec | BDD | TDD | Status |
| --- | --- | --- | --- | --- | --- | --- |
| OKF Bundle Parser | [okf-bundle-parser.md](specs/okf-bundle-parser.md) | [#1](https://github.com/prulloac/weave/pull/1) | x | x | x | done |

## Tier 2 — Engine

| Feature | Spec | PR | Spec | BDD | TDD | Status |
| --- | --- | --- | --- | --- | --- | --- |
| Node Graph Engine | [node-graph-engine.md](specs/node-graph-engine.md) | - | x | x | x | green |
| Keyword + Fuzzy Search (core v1) | [keyword-fuzzy-search.md](specs/keyword-fuzzy-search.md) | - | x | x | x | green |
| Weave CLI Search & Navigate | [keyword-fuzzy-search.md](specs/keyword-fuzzy-search.md) | - | x | x | x | green |

## Tier 3 — Search core

| Feature | Spec | PR | Spec | BDD | TDD | Status |
| --- | --- | --- | --- | --- | --- | --- |
| Mandatory keywords, fuzzy scoring, hybrid top-K and calibration | [keyword-fuzzy-search.md](specs/keyword-fuzzy-search.md) | - | x | x | x | green |
| Remove `find`; add `weave search` | [keyword-fuzzy-search.md](specs/keyword-fuzzy-search.md) | - | x | x | x | green |
| Calibration corpus and ranking baselines | [keyword-fuzzy-search.md](specs/keyword-fuzzy-search.md) | - | x | x | x | green |
| `weave audit` bundle quality gate | [weave-audit.md](specs/weave-audit.md) | - | x | x | x | green |
| `weave relate` graph neighborhood | [weave-relate.md](specs/weave-relate.md) | - | x | - | - | backlog |
| `weave explain` ranking breakdown | [weave-explain.md](specs/weave-explain.md) | - | x | - | - | backlog |
| Introspection contract types (audit/relate/explain) | [weave-audit.md](specs/weave-audit.md) | - | x | - | - | backlog |

## Tier 4 — Portability

| Feature | Spec | PR | Spec | BDD | TDD | Status |
| --- | --- | --- | --- | --- | --- | --- |
| Go engine + CLI base | [architecture/index.md](architecture/index.md) | - | x | - | - | done |
| Roadmap: current capabilities and v1.0 scope | [ROADMAP.md](../ROADMAP.md) | - | x | - | - | done |

## Tier 5 — v1.0 Platform

| Feature | Spec | PR | Spec | BDD | TDD | Status |
| --- | --- | --- | --- | --- | --- | --- |
| Persistent SQLite storage | [storage-and-sync.md](specs/storage-and-sync.md) | - | x | - | - | backlog |
| Incremental sync | [storage-and-sync.md](specs/storage-and-sync.md) | - | x | - | - | backlog |
| Watch mode | [watch-mode.md](specs/watch-mode.md) | - | x | - | - | backlog |
| Permissive Markdown ingestion | [permissive-ingestion.md](specs/permissive-ingestion.md) | - | x | - | - | backlog |
| BM25 lexical ranking | [bm25-ranking.md](specs/bm25-ranking.md) | - | x | - | - | backlog |
| BM25 calibration | [bm25-ranking.md](specs/bm25-ranking.md) | - | x | - | - | backlog |
| Memory/body compression | [compression.md](specs/compression.md) | - | x | - | - | backlog |
| Privacy directives | [privacy-and-extensions.md](specs/privacy-and-extensions.md) | - | x | - | - | backlog |
| External crypto providers | [external-crypto-providers.md](specs/external-crypto-providers.md) | - | x | - | - | backlog |
| Extension subprocess IPC | [privacy-and-extensions.md](specs/privacy-and-extensions.md) | - | x | - | - | backlog |
| Enrichment extension | [privacy-and-extensions.md](specs/privacy-and-extensions.md) | - | x | - | - | backlog |
| Rerank extension | [privacy-and-extensions.md](specs/privacy-and-extensions.md) | - | x | - | - | backlog |
| AI enrichment/reranking plugin | [weave-ai-plugin.md](specs/weave-ai-plugin.md) | - | x | - | - | backlog |

## Tier 6 — MCP

| Feature | Spec | PR | Spec | BDD | TDD | Status |
| --- | --- | --- | --- | --- | --- | --- |
| MCP in-memory over operations | [mcp-server.md](specs/mcp-server.md) | - | x | - | - | backlog |
| MCP persistent + watch reload | [mcp-server.md](specs/mcp-server.md) | - | x | - | - | backlog |
| Sensitive MCP access policies | [mcp-server.md](specs/mcp-server.md) | - | x | - | - | backlog |

## Cross-cutting

- [x] Knowledge corpus: vision + related concepts (`docs/knowledge/`)
- [x] okf-generator skill aligned to OKF v0.2
- [x] spec-writer skill actor convention aligned to OKF v0.2
- [x] Go test infrastructure: unit, conformance, calibration, CLI process tests
- [x] CI workflow: contract export diff, `go test -race`, vet, gofmt, build, docs lint
- [x] `scripts/verify.sh` full local gate
- [x] CONTRIBUTING.md: development process, worktrees, stacked PRs, PR template
- [x] PR template: `.github/PULL_REQUEST_TEMPLATE.md`
- [x] Contract v1 schemas, generated types, golden corpus and exporter (Phase 0)
- [x] Go module scaffold, contract structs, schema round-trip tests and CI (Phase 2)
- [x] Go engine port: parser, graph, search, navigation and operations (Phase 3)
- [x] Go machine CLI: commands, envelopes, exit codes and cache defaults (Phase 5A / Milestone A)
- [x] ADR 0003: keyword + fuzzy search
- [x] Keyword + Fuzzy Search specification
- [x] Search calibration corpus and baseline report
- [x] v1.0 storage/sync and privacy/extension specifications placed under `docs/specs/`
- [x] Extended roadmap integrated into `ROADMAP.md`

## Dependency Order

The full feature DAG lives in [ROADMAP.md § Feature Dependency Map](../ROADMAP.md#feature-dependency-map).
The authoritative feature-to-spec mapping is
[ROADMAP.md § Feature Register](../ROADMAP.md#feature-register); every undelivered feature must have
one row here and one row there.

Execution waves:

1. Shipped: parser, graph, search, CLI, calibration.
2. Parallel: introspection contract types, `audit`, `relate`, `explain`, permissive ingestion.
3. Storage: SQLite.
4. Parallel: sync, BM25, compression, MCP in-memory.
5. Watch and privacy directives.
6. Extension runner, enrichment, rerank, sensitive MCP.
7. AI plugin, external crypto, MCP persistent.

Critical path: `contract -> parser/graph/search -> storage -> sync -> BM25 -> privacy -> extension runner -> MCP persistent`.

## Maintenance

Keep this file updated as pipeline phases advance. Checkboxes in the phase columns are toggled per feature as they complete.
