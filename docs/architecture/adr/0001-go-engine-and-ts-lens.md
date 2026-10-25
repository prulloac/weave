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

Proposed.

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
6. Provide both an in-process Go API and an `http.Handler`; the HTTP route surface remains
   compatible with the current mount.

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
- The renderer can be verified against golden HTML and the existing E2E assertions.
- The mount can be verified over HTTP.
- The lens keeps working while the server changes underneath it.

Porting the monolith in one step would make semantic drift and regressions hard to isolate.

## Consequences

### Positive

- Single-binary distribution, including lens assets.
- Reusable in any Go host process or alongside any language via HTTP.
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
| Sidecar-only TypeScript engine | Useful fallback for non-Go hosts, but does not give the host in-process reuse |
| Rewrite the lens in Go/WASM | Rejected: browser UI already works in TypeScript |

## Follow-Up Decisions

1. Go module path and repository layout (single module versus workspace).
2. Whether the default binary embeds the lens or loads it from disk.
3. Whether the HTTP JSON API ships in v1 or only the HTML routes and Go API.
4. Whether the Astro development site is kept, archived, or used only for lens development.
5. Long-term ownership of the TypeScript reference implementation after cutover.

## Related Documents

- [Architecture index](../index.md)
- [Contract layer](../layers/contract.md)
- [Engine layer](../layers/engine-go.md)
- [Render layer](../layers/render-go.md)
- [Lens layer](../layers/lens-ts.md)
- [Mount layer](../layers/mount-go.md)
- [CLI layer](../layers/cli-go.md)
- [Integration contracts](../integration/contracts.md)
- [Integration steps](../integration/steps.md)
- [Integration testing](../integration/testing.md)
