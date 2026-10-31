---
type: proposal
title: "Maintainability Hardening for Weave"
description: "Enforced CI analysis, responsibility-based splits of the storage and CLI hotspots, a shared CLI test harness, executable dependency rules, documentation debt and one error taxonomy per boundary."
tags:
  - proposal
  - maintainability
  - go
  - architecture
  - optimization
generated:
  by: subagent
  at: "2026-10-31T02:38:07Z"
status: draft
---

# Proposal: Maintainability Hardening for Weave

## Context and goals

The [optimization specification](../../specs/optimization.md) already states Weave's maintainability
targets: a 1,200-line file cap, responsibility-based splits, one CLI test harness, one error table
per boundary, package comments and an executable dependency-direction check. The
[research summary](./research-maintainability.md) shows that most of these are policy without
enforcement. This proposal turns that gap into seven workstreams that can ship as stacked PRs.

Goal: cheaper change, unchanged behavior. The
CLI contract (`weave.cli/v1`), exit codes, JSON goldens, BDD suites, calibration corpus and SQLite
schema are fixed points. A refactor succeeds only when `./scripts/verify.sh`, the racer, goldens,
behavior suites and `TestCalibration` pass unchanged.

## Evidence

| Signal | Verified state on 2026-10-31 |
| --- | --- |
| Size | 14,271 non-test Go LOC, 10,607 test LOC, 21 packages, 44 test files. |
| Hotspots | `storage/sqlite/store.go` 1,641 lines / 46 functions; `cmd/weave/main.go` 1,308 / 30; `engine/operations/operations.go` 1,136 / 23. Only the first two exceed 1,200. |
| Function size | `Executor.Execute` is 259 lines (94–352) with 10 sequential `request.Op ==` checks and a second 10-case switch. `parseFlags` 244, `export` 220, `run` 217, `printHuman` 171, `buildRequest` 156. Nine non-test functions exceed 120 lines. |
| Test duplication | 21 CLI test files; four invokers (`invoke`, `invokeRaw`, `invokePersistent`, `runBinary`); six typed JSON wrappers (`runStorageJSON`, `runAuditJSON`, `runSyncJSON`, `runRelateJSON`, `runExport`, `runExplainJSON`); 94 `decodeEnvelope` calls; identical `extensionRaw` and `compressionRaw` SQL helpers. |
| Package docs | 7 of 21 packages lack a package comment, not 9 as reported: `cmd/contract-export`, `cmd/docs-lint`, `cmd/weave`, `engine/graph`, `engine/nav`, `engine/okf`, `engine/search`. `engine/doc.go` still says the implementation "lands in Phase 3". |
| Error mapping | `contract/v1` defines 17 codes, but `operationErrors` advertises 16: `BUNDLE_NOT_INDEXED` is missing. Five rerank/enrichment sites map every extension failure to `EXTENSION_NOT_FOUND`, so `EXTENSION_TIMEOUT`, `EXTENSION_FAILED` and `EXTENSION_PROTOCOL` are advertised but never emitted. CLI exit decisions are spread over `exitCodeFor`, `successExitCode` and `isEmptyResult`. |
| CI | `.github/workflows/ci.yml` checks tidy, regenerates goldens, runs `go test -race`, `go vet`, gofmt, build and docs-lint. There is no staticcheck/golangci-lint, govulncheck, coverage, size cap, package-comment or dependency check. The gofmt glob covers only `cmd contract engine`, omitting `storage`, `mcp`, `release` and `watch`. |
| Coverage | CLI-driven `-coverpkg=./...` coverage is 68.5%. Package-local coverage is misleading: `engine/graph` and `engine/nav` report 0%, `engine/operations` 13.2%, because they are exercised through `cmd/weave`. |

## Proposed workstreams

| ID | Scope and why | Effort | Risk | Depends | Exit criteria |
| --- | --- | --- | --- | --- | --- |
| M1 | CI analysis gate. Add `.golangci.yml` v2 (`version: "2"`, `linters.default: none`; enable `govet`, `staticcheck` with `checks: ["all"]` so ST1000 runs, `errcheck`, `ineffassign`, `unused`, `misspell`, `revive`), pin the linter version, extend gofmt to every Go package, add `govulncheck` via a `go.mod` `tool` directive in default (non-JSON) format, and publish unit plus CLI-process coverage artifacts with a trend delta against `main`. Wire all of it into `ci.yml` and `scripts/verify.sh`. No analysis runs beyond vet and gofmt. | 1–2 d | Low; first run may surface a backlog. | None | Seeded unused variable, missing package comment and known vulnerable module each fail the job; coverage artifacts upload; no percentage gate. |
| M2 | Split `storage/sqlite/store.go` by responsibility: `store.go` (types, open/close/path), `schema.go` (schemaVersion, createSchema, migrations), `write.go` (Ingest/Rebuild/Sync, writeBundle, insert*, collectEdges), `read.go` (HasBundle, LoadBundle, scanConcept, conceptsOf), `fts.go` (populateFTS, SearchBM25, ExplainBM25, query builders), `enrichment.go` (load/merge/insert enrichment). One file mixes six concerns. | 2–3 d | Medium: transaction and SQL ordering. | M1 | No storage file above 600 lines; schema v3 and SQL statements unchanged; SQLite tests, storage BDD and goldens unchanged; race green. |
| M3 | Split `cmd/weave/main.go` into `flags.go`, `request.go`, `output.go`, `help.go`, `policy.go`; split `engine/operations/operations.go` into `execute.go`, `write.go`, `search.go`, `explain.go`, `errors.go`, `envelope.go`. Replace the `Execute` if-chain with a `map[OperationName]handler` plus validation middleware, deriving descriptors from the same table. God functions obscure ordering. | 3–4 d | Medium: exit ordering and output bytes. | M4 | `run` below 120 lines, `main.go` below 300, `Execute` below 60, every touched file below 450; help, envelopes and human output byte-identical; goldens/BDD/race green. |
| M4 | One `cmd/weave/harness_test.go` exposing `Invoke`, `InvokePersistent`, `InvokeBinary`, `OpenDB` and a generic `Decode[T]`. Replace four invokers, six typed wrappers and the two raw-DB helpers. Keep it test-only, in package `main`, so no production binary grows. Duplication hides failures. | 1–2 d | Low–medium: temp HOME/cache differences. | None; precedes M3 | One harness provides every invocation mode; no per-suite invoker, decoder or raw-DB helper remains; 94 call sites behave identically; test count and pass status unchanged; no non-test file. |
| M5 | Executable dependency direction. Add a small `internal/archcheck` package and `TestDependencyDirection` that runs `go list -deps -json ./...` from the module root, compares edges to a declared map (`contract` at the base; `engine` packages above it; `storage` above engine; `operations` above both; `mcp` only operations; `cmd`/`watch` at the edge) and rejects `storage/sqlite -> cmd`, `storage/sqlite -> mcp` and `mcp -> cmd`. Include a failing fixture. The rule is prose. | 1 d | Low. | M1 | Repository run passes; fixture with `storage/sqlite -> cmd/weave` fails; no new module dependency. |
| M6 | Documentation debt. Add package comments to the seven packages, write ADR 0004 recording the line cap, lint set, coverage trend and dependency test as accepted policy, refresh `docs/architecture/layers/*`, and remove the stale "Phase 3" text from `engine/doc.go`. Docs have drifted. | 1–2 d | Low. | M1, M2, M3 | ST1000 reports zero; 21/21 packages documented; docs-lint and markdownlint pass for touched docs; ADR accepted by the owner; no stale phase claims. |
| M7 | Error taxonomy consolidation. Add one operations table mapping sentinels to code/message/details, one CLI table mapping each code to exit status plus success/empty policy, and have MCP reuse the same codes. Include all 17 codes and fix the five extension sites that currently collapse timeout/failure/protocol into `EXTENSION_NOT_FOUND`. Codes leak inconsistently. | 2 d | Medium: codes are observable. | M3 | 17/17 codes covered; each maps to one exit; `weave contract --json` lists 17; timeout/failure/protocol cases assert their codes. Output changes need a spec addendum and goldens. |

## Sequencing

Ship in five stacked-PR waves: **wave 0** M1 + M5, which adds guardrails without
touching behavior; **wave 1** M4, because the harness must exist before files move; **wave 2** M2;
**wave 3** M3 then M7; **wave 4** M6, after file names settle. Draft ADR 0004 during M1 and accept it
before M2 starts. Freeze hotspot files during their wave; land each move as an atomic conventional
commit through `gh stack`.

## Metrics

| Type | Metric | Baseline | Target |
| --- | --- | --- | --- |
| Leading | Non-test files above 1,200 lines | 2 | 0 |
| Leading | Longest function (`Execute`) | 259 | < 60 after M3 |
| Leading | `Execute` operation branches | 10 `if` + 10 `case` | 0 branch chain |
| Leading | Packages with a comment | 14/21 | 21/21 |
| Leading | Ad-hoc CLI invokers / typed decoders | 4 / 6 | 0 / 0 (harness only) |
| Leading | Codes covered by one mapping table test | 0/17 | 17/17 |
| Leading | Lint and vuln findings | unmeasured | 0 |
| Leading | CLI process coverage | 68.5% | non-decreasing |
| Lagging | Review lead time, revert rate, escaped regressions | unmeasured | improve over two quarters |
| Lagging | Files touched to add one operation | 4–6 | 2 |

Avoid metric gaming: coverage stays a trend, never a percentage gate; line counts are paired with a
"no pass-through wrapper" review check; `//nolint` requires an issue link; deleting or weakening
goldens/BDD/calibration is forbidden; and a sample of splits is reviewed for ownership of state, not
just file size.

## Risks and non-goals

Risks: refactor churn (stacked PRs, frozen hotspots), mechanical splitting (wrapper review), longer CI
(cache, one job, non-blocking coverage), and observable error-code changes (spec addendum plus
goldens). Non-goals: ranking/weights/calibration, schema/storage semantics, contract v2, UI/daemon/
runtime dependencies, MCP redesign, and replacing golden/BDD tests with unit tests.

## Acceptance criteria

1. CI fails on lint, vulnerability, package-comment, line-cap and dependency-direction violations.
2. `storage/sqlite/store.go` and `cmd/weave/main.go` are below 1,200 lines, with `Execute` table-driven.
3. One shared CLI harness replaces duplicate invokers, decoders and raw-DB helpers.
4. All 21 packages have comments, and ADR 0004 is accepted.
5. All 17 error codes have one mapping and one test per boundary.
6. `./scripts/verify.sh`, `go test -race ./...`, goldens, BDD and calibration remain green.
7. No production behavior changes except the explicitly documented error-code corrections.

## References

- [Maintainability research](./research-maintainability.md)
- [Optimization specification](../../specs/optimization.md)
- [Architecture index](../index.md)
- [ADR 0003](../adr/0003-keyword-fuzzy-search-and-ui-retirement.md)
- [CI workflow](../../../.github/workflows/ci.yml)
- [golangci-lint v2 configuration](https://golangci-lint.run/docs/configuration/file/) (accessed 2026-10-31)
- [Staticcheck checks](https://staticcheck.dev/docs/checks/) and [options](https://staticcheck.dev/docs/configuration/options/) (accessed 2026-10-31)
- [govulncheck](https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck) (exit-code behavior verified 2026-10-31)
- [Coverage for integration tests](https://go.dev/doc/build-cover) (accessed 2026-10-31)
- [Google Go Style: Best Practices](https://google.github.io/styleguide/go/best-practices) (accessed 2026-10-31)
- [Architecture Decision Records](https://adr.github.io/) (accessed 2026-10-31)
