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
2. **Reference stays green.** The TypeScript engine keeps serving the product until the Go port
   passes the same fixtures.
3. **One layer at a time.** Each phase has an acceptance gate; later phases do not start while a
   gate is red.
4. **Behavior over bytes.** Golden JSON is compared semantically after normalization. Golden HTML
   is compared structurally; Playwright asserts user-visible behavior.
5. **No big-bang cutover.** The Go site runs beside the TypeScript site until all gates pass.
6. **Every phase is reversible.** Keep the previous binary or tag deployable.

## Phase 0 — Freeze the contract

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

## Phase 1 — Extract TypeScript layers (interim)

**Goal:** prove the package boundaries before changing language.

Deliverables:

- `@weave/engine` facade (`openWeave`) over `okf`, `graph`, `search`, `nav`.
- `@weave/render` for the HTML renderers.
- `@weave/mount` for the handler and artifacts.
- `@weave/cli` as a thin wrapper.
- No Astro or CLI imports from the engine.
- A boundary test that forbids upward imports.

Gate:

- Existing unit and E2E suites pass against the extracted packages.
- The boundary test fails on any engine import of render, mount, CLI or Astro.
- The CLI uses the facade and parses once per process.

Rollback: revert the extraction; behavior is unchanged.

## Phase 2 — Scaffold the Go module

**Goal:** a compiling Go module with contract types and the test harness.

Deliverables:

- `go.mod`, `golangci-lint` config, `gofmt`/`goimports` in CI.
- `contract/v1` Go types and schema validation tests.
- Golden fixture runner in Go that reads the same corpus as TypeScript.
- Empty package skeletons for `engine`, `render`, `mount`, `cmd/weave`.
- `Makefile` or `scripts/verify.sh` with the full verification pipeline.

Gate:

- `go build ./...` and `go test ./...` pass.
- The Go golden runner can read and validate the corpus (even if cases fail).
- CI runs Go lint, vet and tests on Linux, macOS and Windows.

Rollback: delete the Go module; no production behavior changes.

## Phase 3 — Port the engine

**Goal:** byte-equivalent domain output.

Order:

1. `okf/fs` + `okf/walk` (including symlink and cycle tests).
2. `okf/frontmatter` with `yaml.v3`.
3. `okf/links` with goldmark.
4. `okf/parse` + `okf/validate`.
5. `graph/build` + `graph/traverse`.
6. `search/build` + `search/query`.
7. `nav/*`.
8. `okf/cache` and concurrency.

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

## Phase 5 — Port the mount and CLI

**Goal:** feature parity for the standalone tool.

Deliverables:

- `mount.NewSite`, `Handler`, `Close`, artifacts, assets and registry.
- `cmd/weave` commands, flags, JSON output and exit codes.
- Standalone server with port fallback and optional shutdown.
- Embedded `http.Handler` recipe.

Gate:

- Route parity tests pass for every supported path.
- Artifact tests prove no writes to the source and correct cleanup.
- CLI exit codes and JSON output match the golden CLI fixtures.
- `weave mount`, `status`, `unmount`, `find`, `list`, `show`, `backlinks`, `path` all pass E2E.
- `go test -race ./...` passes.

Rollback: switch the `weave` bin back to the TypeScript implementation.

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

**Goal:** prove the Go site in production-like conditions, then retire the reference.

Deliverables:

- Side-by-side comparison: TypeScript site and Go site on the same fixture corpus.
- A normalized diff report for bundle, graph, search and navigation outputs.
- Migration note for hosts: Go module version, asset version, contract version.
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
- [ ] TypeScript golden exporter
- [ ] TypeScript layer extraction and boundary test
- [ ] Go module and CI
- [ ] Go filesystem and walker
- [ ] Go frontmatter
- [ ] Go links
- [ ] Go parse and validation
- [ ] Go graph
- [ ] Go search
- [ ] Go navigation
- [ ] Go renderer
- [ ] Go mount
- [ ] Go CLI
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

- The Go engine passes the full contract corpus and race suite.
- The Go renderer satisfies the DOM contract and zero-JS rules.
- The mount serves all routes and embeds the lens asset.
- The CLI is output- and exit-code-compatible.
- The lens package is independent and versioned.
- The TypeScript reference is archived at a tag.
- Documentation matches the shipped module and asset versions.

## Related Documents

- [Architecture index](../index.md)
- [Contract layer](../layers/contract.md)
- [Engine layer](../layers/engine-go.md)
- [Render layer](../layers/render-go.md)
- [Lens layer](../layers/lens-ts.md)
- [Mount layer](../layers/mount-go.md)
- [CLI layer](../layers/cli-go.md)
- [Integration contracts](contracts.md)
- [Integration testing](testing.md)
