---
type: Architecture Decision Record
title: "ADR 0001: Go Engine and Render, TypeScript Lens"
description: "Decision to port the headless engine, renderer, mount and CLI to Go while keeping the browser lens as a TypeScript asset bundle."
tags:
  - "architecture"
  - "adr"
  - "go"
  - "typescript"
  - "layering"
generated:
  by: opencode/deepseek-v4.1-flash
  at: "2026-10-25T00:00:00Z"
status: draft
---

# ADR 0001: Go Engine and Render, TypeScript Lens

## Status

Superseded in part by [ADR 0003](0003-keyword-fuzzy-search-and-ui-retirement.md): the UI, renderer,
lens and mount decisions below are historical and no longer guide new work.

## Context

Weave currently ships as a TypeScript/Bun/Astro monolith:

- The domain core (`src/lib/okf`, `graph`, `search`, `lens/layout`) is pure TypeScript.
- The render layer is TypeScript string templating (`markdown`, `concept-render`, `explorer-render`,
  `lens-shell`, `site-render`).
- The mount server, artifact layer, registry and CLI are Bun/Node TypeScript.
- The browser lens is Preact plus a canvas force simulation.

The project must become reusable inside a larger product. That product needs a stable engine that
can be embedded, shipped as a single binary, and operated without a Node/Bun runtime. The
browser lens does not need to leave TypeScript.

Measured baseline before this decision: a 10k-file bundle parses in roughly 410 ms
(~0.04 ms/file) on WSL ext4 with bounded-concurrency reads. Performance alone does not justify a
rewrite; distribution, runtime fit, and embedding do.

## Decision

1. Implement the **headless engine**, **renderer**, **mount server** and **CLI** in **Go**.
2. Keep the **lens** in **TypeScript/Preact** and ship it as static browser assets embedded into
   the Go binary.
3. Define a language-neutral **contract v1** (JSON schemas plus golden fixtures) before porting.
4. Keep the TypeScript implementation as the **reference implementation** until the Go port
   passes the full conformance suite.
5. Split the repository into the layers described in [the architecture index](../index.md).
6. Make **CLI I/O the primary integration contract**: argv, stdin/stdout/stderr, exit codes and
   versioned JSON envelopes.
7. Keep a thin **operations layer** so the CLI and a future TCP MCP server share the same
   request/response model.
8. Keep the Go package API as an optional in-process convenience, not the required integration
   path.
9. Treat the HTTP mount as a **presentation adapter for the browser UI**; it is not an
   integration API.

## Rationale

### Why Go for the engine and renderer

- Weave's workload is filesystem traversal, text parsing, graph building, BM25 search, path
  finding, HTML templating, static asset serving, and CLI ergonomics. This is a strong fit for Go.
- A single static binary with `//go:embed` assets simplifies distribution and operational
  packaging.
- `net/http`, `html/template`, `context`, and cross-compilation are first-class and stable.
- Bounded goroutines map cleanly to the existing 32-way concurrent file reads.
- `gopkg.in/yaml.v3` and `goldmark` replace the hand-rolled YAML subset and the link regexes with
  maintained parsers.
- Go is easier for a larger team to contribute to than Rust for this kind of I/O-heavy code.

### Why CLI-first integration

The larger engine needs a narrow, process-boundary contract that works from any language and can
later be wrapped by an MCP transport without redesigning the domain:

- A subprocess boundary avoids embedding a Go runtime into a non-Go host.
- stdout/stderr and exit codes are stable, testable and observable.
- One-shot commands are stateless; a future stdio mode would still be process-local, not a daemon.
- The operations model maps directly to future MCP tools, so the transport changes but the
  semantics do not.
- HTTP remains available only where it is inherent: serving the browser presentation.

### Why not Rust

Rust would be a reasonable choice if the host product were written in Rust, or if the engine
needed WASM, a C ABI, or hard latency and memory guarantees. None of those are current
requirements. Rust would raise implementation and iteration cost without changing the product
outcome.

### Why keep the lens in TypeScript

- The lens is browser code: DOM bootstrap, canvas rendering, mouse hit-testing, Preact.
- Preact and the existing canvas simulation are already implemented and tested.
- Porting the UI to Go would require a browser target and would discard working code.
- The lens already consumes a serializable graph payload; the contract is the only coupling.

### Why split before porting

A layer split reduces the port to independent, testable surfaces:

- The engine can be ported and verified against golden JSON.
- The CLI can be verified as a process: stdout, stderr, exit codes and envelopes.
- The renderer and mount can be verified as a presentation surface with golden HTML and E2E.
- The lens keeps working while the server changes underneath it.

Porting the monolith in one step would make semantic drift and regressions hard to isolate.

## Consequences

### Positive

- Single-binary distribution, including lens assets.
- Reusable from any language through a process boundary, without an HTTP API.
- A future MCP transport can reuse the operations layer instead of reimplementing behavior.
- Maintained YAML and Markdown libraries instead of hand-rolled parsers.
- Clear ownership and dependency direction between layers.
- Contract tests make future ports (including a possible Rust engine) tractable.

### Negative

- Two-language build and CI matrix.
- A lens asset pipeline must be versioned and embedded.
- Semantic differences between the hand-rolled YAML subset and `yaml.v3` must be pinned by
  fixtures.
- Go HTML templates will not match the TypeScript string output byte-for-byte; E2E assertions
  must target structure and behavior, while golden HTML tests normalize whitespace.
- Temporary duplication during migration.

### Risks and mitigations

| Risk | Mitigation |
| --- | --- |
| Parser or link semantics drift | Contract v1 golden fixtures; TS exporter; property tests |
| YAML superset changes behavior | Document the required subset; fixtures assert it; unknown keys preserved |
| Renderer diverges from DOM contract | Test-ID and route contract table; Playwright against the Go server |
| Windows symlink/hardlink differences | Artifact tests on Windows CI; hardlink fallback |
| Lens asset incompatibility | `weave.contract/v1` compatibility matrix; lens ignores unknown fields |
| Duplicate engines during migration | Time-boxed dual-run; cutover gate; archive the TS reference at a tag |

## Alternatives Considered

| Alternative | Verdict |
| --- | --- |
| Keep TypeScript and publish `@weave/engine` | Best if the host stays in Node/Bun; rejected because the host needs a non-Node deployment |
| Port everything to Rust | Rejected: higher cost, no current hard requirement |
| Go engine only, keep TS render/mount | Rejected: SSR would cross a process boundary or keep a Node runtime |
| WebAssembly engine | Rejected: adds bundling and I/O constraints without a current use case |
| HTTP JSON API as the primary integration surface | Rejected: couples the host to a server, port and lifecycle; CLI I/O is simpler and the MCP transport comes later |
| TCP service now | Rejected for v1: premature; the operations layer preserves the option |
| Rewrite the lens in Go/WASM | Rejected: browser UI already works in TypeScript |

Resolved during review: `--legacy-json` and `weave engine --stdio` are **deferred until after v1**.
The v1 machine contract is the one-shot `--json` envelope only. No legacy output mode and no
long-lived stdio mode ship in the first Go CLI.

Resolved by [ADR 0002](0002-v1-scope-layout-and-contract.md): the Go module layout, migration
sequencing, v1 milestone scope, contract generation, CLI cache default and `weave parse` payload.

## Follow-Up Decisions

1. Whether the default binary embeds the lens or loads it from disk.
2. Whether the Astro development site is kept, archived, or used only for lens development.
3. Long-term ownership of the TypeScript reference implementation after cutover.
4. When to introduce `--legacy-json` and how long to keep it.
5. When to introduce `weave engine --stdio` and whether it needs a reload operation.
6. The MCP transport details (bind address, authentication, tool naming) when that work starts.
7. The `--no-body` design for large `weave parse` payloads.

## Related Documents

- [Architecture index](../index.md)
- [ADR 0002](0002-v1-scope-layout-and-contract.md)
- [Contract layer](../layers/contract.md)
- [Engine layer](../layers/engine-go.md)
- [Operations layer](../layers/operations.md)
- [CLI layer](../layers/cli-go.md)
- [ADR 0003](0003-keyword-fuzzy-search-and-ui-retirement.md)
- [Keyword + Fuzzy Search spec](../../specs/keyword-fuzzy-search.md)
- [Integration contracts](../integration/contracts.md)
- [Integration steps](../integration/steps.md)
- [Integration testing](../integration/testing.md)
