---
type: Architecture Decision Record
title: "ADR 0002: v1 Scope, Repository Layout and Contract Decisions"
description: "Approved decisions for the Go module layout, migration sequencing, v1 milestone scope, contract generation, CLI cache and parse payload."
tags:
  - "architecture"
  - "adr"
  - "go"
  - "contract"
  - "migration"
  - "scope"
generated:
  by: opencode/deepseek-v4.1-flash
  at: "2026-10-25T00:00:00Z"
status: accepted
---

# ADR 0002: v1 Scope, Repository Layout and Contract Decisions

## Status

Superseded in part by [ADR 0003](0003-keyword-fuzzy-search-and-ui-retirement.md): the UI and
presentation milestones are cancelled, and the contract v1 it describes has been replaced in
place. The repository, module and CLI-first decisions remain valid.

## Context

The architecture split is documented and the TypeScript baseline is green:

- 115 unit tests pass.
- 80 Playwright scenarios pass.
- The parser, graph, search, CLI and mount are working.
- The Go port has not started.

The next step is Phase 0 (contract schemas and golden fixtures). Before writing code, six
decisions had to be fixed so the repository layout, milestones and CLI behavior do not drift
mid-migration.

## Decision Summary

| ID | Decision | Blocks |
| --- | --- | --- |
| D1 | Single Go module at the repository root | Phase 0 paths, Phase 2 imports |
| D2 | Skip the interim TypeScript layer extraction | Phase 1 sequencing |
| D3 | Milestone A is contract + engine + CLI; presentation follows in B | Milestone gates |
| D4 | JSON Schema is the source of truth; Go structs are hand-written and schema-tested | Phase 0 artifacts |
| D5 | CLI parse cache is enabled by default, with `--no-cache` | CLI behavior and tests |
| D6 | `weave parse` includes concept bodies by default | Contract and payload size |

## D1 — Single Go module at the repository root

### Intent

Keep the migration mechanically simple. A single module avoids workspace files, replace
directives and nested module boundaries while the TypeScript and Go trees coexist. The module
path should be stable because it becomes the import prefix for every Go package.

### Decision

- `go.mod` lives at the repository root.
- Module path: `github.com/prulloac/weave`.
- Go packages live at the root:

```text
contract/v1/
engine/
engine/operations/
render/
mount/
cmd/weave/
```

- The existing TypeScript tree (`src/`, `cli/`, `tests/`, `lens/` during migration) remains in
  place. Go tooling ignores non-Go directories.
- No nested `go.mod` files and no Go workspace in v1.
- The lens remains a nested npm package, not a Go module.

### Effects on later decisions

- D4 paths are fixed: schemas live at `contract/v1/schema`.
- Phase 2 imports use `github.com/prulloac/weave/contract/v1` and sibling paths.
- CI can run one `go test ./...` and one `go build ./cmd/weave`.
- The TypeScript reference can stay in the same repository until cutover without a module split.
- A future extraction to a separate module or repo remains possible but is not planned.

### Expected outcomes

- `go build ./cmd/weave` produces the binary from a clean checkout.
- `go test ./...` discovers Go packages only.
- Import paths are stable for hosts and future MCP code.
- No `replace` directives or workspace files are needed.

### Acceptance

- `go list ./...` lists the expected packages.
- `go test ./...` and `go build ./cmd/weave` pass in CI on Linux, macOS and Windows.
- No nested `go.mod` exists.
- The TypeScript build and tests are unaffected.

## D2 — Skip the interim TypeScript layer extraction

### Intent

Avoid churn in a codebase that is about to be replaced. The CLI is the integration boundary, so
there is no external consumer of an extracted `@weave/engine` package. The TypeScript code needs
to stay runnable as the reference implementation, not become a new package layout.

### Decision

- Do not extract `@weave/engine`, `@weave/render`, `@weave/mount` or `@weave/cli`.
- Keep the TypeScript implementation in place and frozen except for:
  - the golden exporter,
  - bug fixes required to keep the reference green,
  - documentation updates.
- Add a `contract:export` script that reads fixtures and writes normalized golden JSON.
- Phase 1 becomes "freeze and export", not "extract packages".

### Effects on later decisions

- Phase 2 can start immediately after Phase 0; there is no package-boundary work in between.
- No TypeScript package versioning, exports map or publish pipeline is needed.
- Conformance compares the frozen TS implementation to the Go port through JSON files, not
  through a shared package.
- If a TypeScript consumer appears later, extraction can be reconsidered without blocking v1.

### Expected outcomes

- The TS suite stays green with minimal changes.
- The golden exporter is the only new TS surface.
- Migration effort shifts from refactoring TS to porting Go.
- The reference remains a single, understandable codebase until cutover.

### Acceptance

- `bun test` and `bunx playwright test` remain green.
- `bun run contract:export` produces deterministic golden output for every fixture.
- No new TypeScript package boundaries or exports maps are introduced.
- The Go port can be validated entirely through goldens.

## D3 — Milestone A is contract + engine + CLI; presentation follows in B

### Intent

Deliver the integration value first. The bigger engine needs CLI operations, not a browser site.
Splitting the work reduces risk and gives an earlier feedback loop on the contract.

### Decision

- **Milestone A (v1 integration):**
  - contract schemas and golden corpus,
  - Go engine,
  - operations layer,
  - machine CLI commands and envelopes,
  - conformance, race, fuzz and CLI process tests.
- **Milestone B (presentation):**
  - Go renderer,
  - presentation mount,
  - lens asset package and embedding,
  - browser E2E and visual checks.
- Cutover happens after both milestones pass their gates.

### Effects on later decisions

- Phase 3 (engine) and Phase 5A (CLI) are prioritized.
- Phase 4 (render), Phase 5B (mount) and Phase 6 (lens) are later but not optional.
- The CLI contract can be consumed by the bigger engine before the presentation is ready.
- The lens and renderer continue to target contract v1, so Milestone B does not reopen engine
  semantics.

### Expected outcomes

- The bigger engine integrates through `weave` while the UI is still being ported.
- Contract and CLI issues are found early, when they are cheapest to fix.
- Presentation work does not block the integration roadmap.
- No partial release exposes a broken browser UI.

### Acceptance

- Milestone A: every CLI operation passes the process contract, conformance and race gates.
- Milestone B: DOM contract, zero-JS, lens E2E and asset embedding pass.
- Cutover only after both milestones and the dual-run diff.

## D4 — JSON Schema is the source of truth

### Intent

Prevent drift between Go, TypeScript and future transports. The contract must be readable without
reading an implementation.

### Decision

- JSON Schema 2020-12 files under `contract/v1/schema` are authoritative.
- TypeScript types for the lens are generated from the schemas.
- Go structs are hand-written with explicit `json` tags and validated against the schemas in CI.
- The domain wire format stays `snake_case`.
- Generated TypeScript types are checked in and regenerated in CI; a stale generated file fails
  the build.
- No Go code generation is required in v1. Hand-written structs keep the toolchain small; schema
  tests catch drift.

### Effects on later decisions

- Phase 0 must produce schemas before Go structs.
- Adding a field means schema first, then Go struct, then generated TS type, then golden update.
- Removing or renaming a field requires a contract major.
- The lens consumes generated types, which keeps its payload assumptions aligned with Go.
- Future MCP tool schemas can be derived from the same operation definitions.

### Expected outcomes

- One reviewable diff per contract change.
- Go and TypeScript cannot silently diverge on field names or types.
- Hosts can validate payloads without language-specific SDKs.
- The operations layer has a stable wire model.

### Acceptance

- Every golden JSON document validates against its schema.
- Go structs marshal to schema-valid JSON.
- Generated TypeScript types match the schemas.
- CI fails on a stale generated file or a schema-invalid response.

## D5 — CLI parse cache is enabled by default

### Intent

Reduce repeated parse cost when a host invokes `weave` many times. The bigger engine may call
`find`, `show` and `path` in sequence; reparsing the whole bundle each time is wasted work.

### Decision

- The parse cache is enabled by default for machine CLI commands.
- Cache location: `WEAVE_CACHE_DIR`, defaulting to the OS temp directory.
- `--no-cache` disables it for one invocation.
- The cache is keyed by absolute path plus `(mtime, size)` and a parser-version constant.
- Cache writes are best-effort; failures never fail a command.
- The cache never changes results. Output is identical with and without it.
- Presentation mounts do not use the cache; they parse once per site.

### Effects on later decisions

- CLI tests must isolate the cache with a temporary directory or `--no-cache`.
- Determinism tests run both cached and uncached to prove output equality.
- The engine cache API remains opt-in; the CLI chooses the default.
- Future stdio mode can reuse a parsed engine rather than the file cache.

### Expected outcomes

- Second and subsequent CLI invocations are faster on unchanged bundles.
- Stale entries invalidate on file change.
- No cache files are written into the bundle.
- Concurrent CLI processes are safe.

### Acceptance

- A cache-hit test observes a hit on the second run.
- Changing a file invalidates that entry.
- `--no-cache` yields identical output.
- Cache tests use an isolated temp directory.
- No writes occur inside the target bundle.

## D6 — `weave parse` includes concept bodies

### Intent

Make `parse` a complete, faithful view of the bundle. A host should not need a second command to
retrieve concept content, and goldens should represent the full domain model.

### Decision

- `weave parse` includes `body` for every concept by default.
- `graph`, `find`, `list`, `backlinks` and `path` do not include bodies, except `show`, which is
  the explicit body view.
- A `--no-body` flag is deferred post-v1 if large bundles make the payload impractical.
- The contract keeps `body` as a required concept field.
- Hosts should redirect large payloads to a file rather than rely on terminal buffering.

### Effects on later decisions

- `parse` output size is proportional to the bundle, not just metadata.
- Golden `bundle.json` files contain bodies; diffs include content changes.
- A future streaming or `--no-body` mode must preserve the contract field semantics.
- The CLI remains a one-shot JSON emitter; no pagination in v1.

### Expected outcomes

- Hosts can build their own index or cache from one `parse` call.
- `show` remains the focused concept view.
- No ambiguity about missing body fields.
- Large-bundle ergonomics can be improved later without breaking the contract.

### Acceptance

- `weave parse --json` includes non-empty bodies for concepts with content.
- `graph --json` does not include bodies.
- `show --json` includes the body.
- Goldens cover body content and escaping.

## Milestone Map

| Milestone | Phases | Outcome |
| --- | --- | --- |
| A — Integration | 0 (contract), 2 (Go scaffold), 3 (engine/operations), 5A (CLI) | Bigger engine can use `weave` over CLI |
| B — Presentation | 4 (render), 5B (mount), 6 (lens assets) | Browser site and lens reach parity |
| Cutover | 7 | TypeScript reference archived; Go is default |

Phase 1 is skipped by D2. The numbering is kept for traceability with
[Integration steps](../integration/steps.md).

## Trade-offs Accepted

- The v1 CLI output is a breaking change from the legacy TypeScript `--json` shapes. `--legacy-json`
  is deferred, so early adopters must use the envelope.
- Skipping the TS extraction means the frozen TS tree keeps its current layout until cutover.
- Hand-written Go structs can drift from schemas; schema tests are the guard.
- Body-inclusive `parse` output can be large; `--no-body` is deferred.
- Milestone B delays the browser presentation; the CLI integration is not blocked by it.

## Remaining Open Decisions

These are intentionally deferred and do not block Phase 0:

1. Whether the default binary embeds the lens or loads it from disk (Milestone B).
2. Whether the Astro site is kept for lens development, archived, or removed (Milestone B).
3. When the TypeScript reference is tagged and archived (cutover).
4. When `--legacy-json` is introduced and how long it is supported.
5. When `weave engine --stdio` is introduced and whether it needs reload semantics.
6. MCP transport details: bind address, authentication, tool naming.
7. The exact `--no-body` design and whether large outputs need a file-writing flag.

## Related Documents

- [Architecture index](../index.md)
- [ADR 0001](0001-go-engine-and-ts-lens.md)
- [Contract layer](../layers/contract.md)
- [Operations layer](../layers/operations.md)
- [CLI layer](../layers/cli-go.md)
- [Integration contracts](../integration/contracts.md)
- [Integration steps](../integration/steps.md)
- [Integration testing](../integration/testing.md)
