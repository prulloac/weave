---
type: Migration Plan
title: "Integration Steps"
description: "Gated, dependency-ordered migration from the TypeScript monolith to the Go engine, renderer, mount and CLI with a TypeScript lens bundle."
tags:
  - "architecture"
  - "migration"
  - "plan"
  - "go"
  - "typescript"
generated:
  by: opencode/deepseek-v4.1-flash
  at: "2026-10-25T00:00:00Z"
status: draft
---

# Integration Steps

## Principles

1. **Contract first.** No implementation code is written until the contract schemas and golden
   fixtures exist.
2. **CLI is the integration boundary.** The CLI and operations envelopes are the compatibility
   promise; HTML is a presentation detail.
3. **Reference stays green.** The TypeScript engine keeps serving the product until the Go port
   passes the same fixtures.
4. **One layer at a time.** Each phase has an acceptance gate; later phases do not start while a
   gate is red.
5. **Behavior over bytes.** Golden JSON is compared semantically after normalization. Golden HTML
   is compared structurally; Playwright asserts user-visible behavior.
6. **No big-bang cutover.** The Go CLI and presentation run beside the TypeScript implementation
   until all gates pass.
7. **Every phase is reversible.** Keep the previous binary or tag deployable.

## Milestone Map

| Milestone | Phases | Outcome |
| --- | --- | --- |
| A — Integration | 0, 2, 3, 5A | Bigger engine uses `weave` over CLI |
| B — Presentation | 4, 5B, 6 | Browser site and lens reach parity |
| Cutover | 7 | TypeScript reference archived; Go is default |

Phase 1 is skipped by [ADR 0002](../adr/0002-v1-scope-layout-and-contract.md); the numbering is
kept for traceability.

## Phase 0 — Freeze the contract

**Status:** complete (schemas, generated types, golden corpus, exporter and validation tests).

**Goal:** make the observable behavior language-neutral.

Deliverables:

- `contract/v1/schema/*.schema.json` for bundle, concept, graph, search, navigation and errors.
- `contract/v1/types.go` and generated `types.ts`.
- Golden corpus under `contract/v1/golden` seeded from `tests/fixtures/okf-bundle` plus edge cases.
- A TypeScript exporter that writes normalized JSON for any fixture.
- A written list of semantic decisions (anchors, nested index, YAML subset, autolinks, symlinks).

Gate:

- The TypeScript implementation passes every golden fixture.
- Schemas validate both the golden outputs and the fixtures.
- Golden updates require review; CI never silently rewrites them.

Rollback: none needed; this phase is additive.

## Phase 1 — Freeze the reference and export goldens (not package extraction)

**Status:** replaces the earlier extraction plan per ADR 0002 D2. The TypeScript implementation
stays in its current layout.

**Goal:** keep the reference runnable and make its behavior exportable.

Deliverables:

- `contract:export` script that reads a fixture bundle and writes normalized golden JSON.
- A manifest mapping fixtures to expected bundle, graph, search and navigation outputs.
- Baseline notes recording the current TS test results.
- No new `@weave/*` packages, exports maps or package-boundary work.

Gate:

- `bun test` and the Playwright suite remain green.
- `bun run contract:export` is deterministic across repeated runs.
- Every fixture in the corpus has expected output.
- No new TypeScript package boundaries are introduced.

Rollback: remove the exporter; the reference is unchanged.

## Phase 2 — Scaffold the Go module

**Status:** complete (module, contract structs, schema round-trip tests, package skeletons, CI and
`scripts/verify.sh`).

**Goal:** a compiling Go module with contract types and the test harness.

Deliverables:

- `go.mod`, `golangci-lint` config, `gofmt`/`goimports` in CI.
- `contract/v1` Go types and schema validation tests.
- Golden fixture runner in Go that reads the same corpus as TypeScript.
- Empty package skeletons for `engine`, `engine/operations`, `render`, `mount`, `cmd/weave`.
- Operation catalogue and request/response types.
- `Makefile` or `scripts/verify.sh` with the full verification pipeline.

Gate:

- `go build ./...` and `go test ./...` pass.
- The Go golden runner can read and validate the corpus (even if cases fail).
- CI runs Go lint, vet and tests on Linux, macOS and Windows.

Rollback: delete the Go module; no production behavior changes.

## Phase 3 — Port the engine

**Status:** complete (parser, graph, BM25 search, navigation, operations, cache, conformance and
benchmarks).

**Goal:** byte-equivalent domain output.

Order:

1. `okf/fs` + `okf/walk` (including symlink and cycle tests).
2. `okf/frontmatter` with `yaml.v3`.
3. `okf/links` with goldmark.
4. `okf/parse` + `okf/validate`.
5. `graph/build` + `graph/traverse`.
6. `search/build` + `search/query`.
7. `nav/*`.
8. `operations` request/response dispatch, envelopes and error codes.
9. `okf/cache` and concurrency.

Deliverables:

- Go engine APIs as specified in [Engine layer](../layers/engine-go.md).
- Golden tests for every case.
- Benchmarks for parse and search.
- Error-injection and cancellation tests.

Gate:

- Every `Bundle`, `Graph`, `Search` and navigation golden matches the TypeScript reference after
  normalization.
- `go test -race ./...` passes.
- Benchmark stays within the agreed budget.
- Fuzz tests find no crashes or panics.

Rollback: the TypeScript engine remains the product; the Go packages are unused.

## Phase 4 — Port the renderer

**Goal:** the Go handler serves pages that satisfy the DOM contract.

Deliverables:

- `render` templates and goldmark pipeline.
- Golden HTML tests per fixture.
- Theme and `BasePath` support.
- XSS and escaping tests.
- Test-ID contract tests for explorer, concept, lens shell and 404.

Gate:

- All DOM contract assertions pass for Go-rendered HTML.
- `/` and `/concept/*` contain no scripts.
- `/lens` contains the payload and module script.
- Existing Playwright scenarios pass against a Go test server.
- Golden HTML diffs are reviewed and accepted.

Rollback: keep the TypeScript renderer behind a flag; the Go renderer is opt-in.

## Phase 5A — Port the CLI (Milestone A)

**Status:** complete (machine commands, envelopes, human output, exit codes, cache defaults and
process tests).

**Goal:** ship the machine integration contract.

Deliverables:

- `cmd/weave` machine commands: `contract`, `parse`, `graph`, `find`, `list`, `show`,
  `backlinks`, `path`, `version`.
- Envelope output (`--json`), stdout/stderr separation and exit codes.
- Parse cache enabled by default, `WEAVE_CACHE_DIR`, `--no-cache`.
- `weave parse --json` includes bodies; `graph`/query payloads do not.
- `weave contract --json` operation and version discovery.

Gate:

- Every CLI operation passes the process contract tests: valid envelope, empty stderr on success,
  documented exit code on failure, deterministic output.
- `weave contract --json` advertises the supported operations and versions.
- Cache tests pass: hit, invalidation, `--no-cache` parity, isolated temp directory.
- Conformance, race and fuzz gates pass.
- `go test -race ./...` passes.

Rollback: switch the `weave` bin back to the TypeScript implementation.

## Phase 5B — Port the presentation mount (Milestone B)

**Goal:** keep the browser presentation working on the Go handler.

Deliverables:

- `mount.NewSite`, `Handler`, `Close`, artifacts, assets and registry.
- Standalone `mount`, `unmount`, `status` commands with port fallback and optional shutdown.

Gate:

- Artifact tests prove no writes to the source and correct cleanup.
- `weave mount`, `status`, `unmount` pass the browser E2E suite.
- Route, asset and `BasePath` tests pass.
- `go test -race ./...` passes.

Rollback: keep the TypeScript presentation; it is independent of the CLI contract.

## Phase 6 — Lens asset pipeline and embedding

**Goal:** ship the lens as a versioned static asset inside the Go binary.

Deliverables:

- Standalone `lens/` package and build command.
- `dist/lens.js` plus `manifest.json` with contract version, hash and size.
- `//go:embed` in `mount/lensassets`.
- Cache-busted asset URL and ETag.
- Size budget in CI.

Gate:

- Lens unit and payload tests pass.
- Playwright lens scenarios pass against the embedded asset.
- Missing-asset fallback works (server index, no script, warning).
- No lens import of engine, parser, search, Node APIs or Astro.
- Contract compatibility check passes.

Rollback: serve the previous lens asset from the TypeScript build.

## Phase 7 — Dual run and cutover

**Goal:** prove the Go CLI and presentation in production-like conditions, then retire the
reference.

Deliverables:

- Side-by-side comparison: TypeScript CLI/presentation and Go CLI/presentation on the same
  fixture corpus.
- A normalized diff report for bundle, graph, search, navigation and CLI envelopes.
- Migration note for hosts: CLI contract version, Go module version, asset version, contract
  version.
- Tag the final TypeScript reference (`ts-reference-v1`) before deletion.
- Remove the TypeScript engine, renderers, mount and CLI, or archive them in a branch/tag.

Gate:

- Zero unexplained diffs across the corpus for at least one release candidate.
- E2E suite passes against the Go binary only.
- Performance and memory budgets met.
- Rollback plan documented and tested once.

Rollback: deploy the tagged TypeScript binary.

## Dependency-Ordered Checklist

- [ ] Contract schemas and golden corpus
- [ ] TypeScript golden exporter (Phase 1 is skipped; no package extraction)
- [ ] Go module and CI
- [ ] Go filesystem and walker
- [ ] Go frontmatter
- [ ] Go links
- [ ] Go parse and validation
- [ ] Go graph
- [ ] Go search
- [ ] Go navigation
- [ ] Go operations and envelopes
- [ ] Go CLI (one-shot envelope, cache default)
- [ ] Go renderer
- [ ] Go mount (presentation)
- [ ] Lens package and build
- [ ] Go embed and fallback
- [ ] Dual-run diff
- [ ] Cutover and archive

## Commit and Branch Strategy

- One branch per phase, stacked where practical (`feat/go-contract`, `feat/go-engine`,
  `feat/go-render`, `feat/go-mount`, `feat/lens-assets`).
- Atomic conventional commits per package:
  - `docs(contract): ...`
  - `test(engine): ...`
  - `feat(engine-go): ...`
  - `feat(render-go): ...`
  - `feat(mount-go): ...`
  - `build(lens): ...`
- Each phase updates `docs/backlog.md` and the relevant architecture doc status.
- No phase merges with a red gate.

## Definition of Done

### Milestone A — Integration

- The Go engine passes the full contract corpus and race suite.
- Every CLI operation emits a valid envelope, respects stdout/stderr rules and returns documented
  exit codes.
- `weave contract --json` is stable and tested.
- The parse cache is enabled by default, testable and invalidates correctly.
- `weave parse --json` includes bodies; query payloads do not.

### Milestone B — Presentation

- The Go renderer satisfies the DOM contract and zero-JS rules.
- The presentation mount serves all routes and embeds the lens asset.
- The lens package is independent and versioned.
- Browser E2E passes against the Go binary.

### Cutover

- The TypeScript reference is archived at a tag.
- Documentation matches the shipped module, CLI and asset versions.

The future MCP transport is explicitly out of scope for this plan; the operations layer is the
stable seam it will use.

## Post-v1 Backlog

Explicitly deferred after the first Go CLI release:

- `--legacy-json` output, if migration demand justifies it.
- `weave engine --stdio` NDJSON mode.
- The TCP MCP server, built on the operations layer.
- Any reload/watch semantics for stdio or MCP sessions.

None of these change the v1 CLI envelope contract or the engine semantics.

## Related Documents

- [Architecture index](../index.md)
- [ADR 0002](../adr/0002-v1-scope-layout-and-contract.md)
- [Contract layer](../layers/contract.md)
- [Engine layer](../layers/engine-go.md)
- [Operations layer](../layers/operations.md)
- [CLI layer](../layers/cli-go.md)
- [Render layer](../layers/render-go.md)
- [Lens layer](../layers/lens-ts.md)
- [Mount layer](../layers/mount-go.md)
- [Integration contracts](contracts.md)
- [Integration testing](testing.md)
