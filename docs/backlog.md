# Weave Backlog

Tracked work queue for the Weave vision: "To weave flat files into a living universe of knowledge."

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
| Keyword + Fuzzy Search (new v1) | [keyword-fuzzy-search.md](specs/keyword-fuzzy-search.md) | - | x | x | x | green |
| Weave CLI Search & Navigate | [keyword-fuzzy-search.md](specs/keyword-fuzzy-search.md) | - | x | x | x | green |

## Tier 3 — Search v1

| Feature | Spec | PR | Spec | BDD | TDD | Status |
| --- | --- | --- | --- | --- | --- | --- |
| Mandatory keywords, fuzzy scoring, hybrid top-K and calibration | [keyword-fuzzy-search.md](specs/keyword-fuzzy-search.md) | - | x | x | x | green |
| Remove `find`; add `weave search` | [keyword-fuzzy-search.md](specs/keyword-fuzzy-search.md) | - | x | x | x | green |
| Calibration corpus and ranking baselines | [keyword-fuzzy-search.md](specs/keyword-fuzzy-search.md) | - | x | x | x | green |
| `weave audit` bundle quality gate | [weave-audit.md](specs/weave-audit.md) | - | x | - | - | backlog |
| `weave relate` graph neighborhood | [weave-relate.md](specs/weave-relate.md) | - | x | - | - | backlog |
| `weave explain` ranking breakdown | [weave-explain.md](specs/weave-explain.md) | - | x | - | - | backlog |

## Tier 4 — Portability

| Feature | Spec | PR | Spec | BDD | TDD | Status |
| --- | --- | --- | --- | --- | --- | --- |
| Go engine + CLI base | [architecture/index.md](architecture/index.md) | - | x | - | - | done |
| MCP server over operations | [mcp-server.md](specs/mcp-server.md) | - | x | - | - | backlog |
| Roadmap: current capabilities and MCP path | [ROADMAP.md](../ROADMAP.md) | - | x | - | - | done |

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

## Maintenance

Keep this file updated as pipeline phases advance. Checkboxes in the phase columns are toggled per feature as they complete.
