---
okf_version: "0.2"
---

# Weave Architecture

Target architecture and migration plan for splitting Weave into reusable layers, with the
headless engine, renderer, mount server and CLI implemented in **Go**, and the browser lens kept
as **TypeScript-built assets**.

**Integration is CLI-first.** The bigger engine talks to Weave by invoking the `weave` binary and
reading stdout/stderr/exit codes. The Go package API is available for in-process Go hosts. The
HTTP mount is a **presentation adapter for humans**, not the integration surface. A TCP MCP
server is a later transport adapter over the same operations.

## Target Topology

```text
┌───────────────────────────────────────────────────────────────┐
│                        Host project                           │
│         subprocess · stdin/stdout · exit codes                │
└───────────────────────────────┬───────────────────────────────┘
                                │ CLI I/O (primary)
┌───────────────────────────────▼───────────────────────────────┐
│  cmd/weave + engine/operations                                │
│  commands · JSON envelopes · exit codes                       │
├───────────────────────────────────────────────────────────────┤
│  weave/engine                                                 │
│  okf parser · graph · BM25 search · navigation · cache        │
├───────────────────────────────────────────────────────────────┤
│  weave/contract                                               │
│  JSON schemas · Go types · versioning · golden fixtures       │
└───────────────────────────────┬───────────────────────────────┘
                                │ optional in-process / UI
┌───────────────────────────────▼───────────────────────────────┐
│  weave/render + weave/mount (presentation only)               │
│  HTML pages · lens shell · artifacts · local browser server   │
└───────────────────────────────┬───────────────────────────────┘
                                │ embeds at build time
┌───────────────────────────────▼───────────────────────────────┐
│  lens (TypeScript / Preact)                                   │
│  canvas force graph · legend · DOM bootstrap · no engine code │
└───────────────────────────────────────────────────────────────┘
```

## Layers

| Layer | Language | Package | Responsibility | Depends on |
| --- | --- | --- | --- | --- |
| Contract | JSON Schema + Go | `weave/contract` | Domain types, versions, golden fixtures | — |
| Engine | Go | `weave/engine` | Parse OKF, build graph, search, navigate | `contract` |
| Operations | Go | `weave/engine/operations` | Transport-neutral request/response operations | `engine`, `contract` |
| CLI | Go | `cmd/weave` | Argv/env/stdin/stdout adapters, exit codes | `operations` |
| Render | Go | `weave/render` | Markdown to HTML, page shells, theme | `contract`, engine view types |
| Lens | TypeScript | `lens` (standalone) | Browser canvas UI, asset bundle | Graph payload contract only |
| Mount | Go | `weave/mount` | Browser presentation: artifacts, routes, assets | `engine`, `render`, lens assets |

## Integration Surfaces

| Surface | Audience | Status | Contract |
| --- | --- | --- | --- |
| CLI processes | Any language or runtime | Primary now | argv, stdin/stdout/stderr, exit codes, `weave.cli/v1` |
| Operations NDJSON | Hosts issuing many requests | Deferred post-v1 | Same envelope, process-local stdio |
| Go package API | Go hosts | Optional | Engine and operations packages |
| Browser presentation | Humans | Presentation only | DOM hooks and HTML routes |
| TCP MCP server | Future agent integrations | Later | MCP tools over the operations layer |

A host that is not written in Go must use the CLI. A Go host may use the package API, but the
CLI remains the compatibility promise.

## Milestones

| Milestone | Phases | Outcome |
| --- | --- | --- |
| A — Integration | 0 contract, 2 Go scaffold, 3 engine/operations, 5A CLI | Bigger engine can use `weave` over CLI |
| B — Presentation | 4 render, 5B mount, 6 lens assets | Browser site and lens reach parity |
| Cutover | 7 | TypeScript reference archived; Go is default |

Phase 1 (TypeScript package extraction) is skipped. The numbering is retained for traceability;
see [ADR 0002](adr/0002-v1-scope-layout-and-contract.md) and
[Integration steps](integration/steps.md).

## Dependency Rules

1. `contract` has no internal dependencies and no runtime dependencies.
2. `engine` may import `contract`; it must not import `render`, `mount`, `cmd`, or any UI code.
3. `operations` may import `engine` and `contract`; it must not import `render`, `mount` or `cmd`.
4. `cmd/weave` is the only package allowed to parse process arguments, read environment
   variables, set exit codes, install signal handlers, or write the mount registry by default.
5. `render` and `mount` are presentation adapters. They may import `engine` and `contract`; they
   must not import `cmd` or `operations`.
6. The lens bundle must not import the engine, parser, search index, operations, or Node APIs. It
   consumes only the serialized graph payload.
7. The integration boundary is `cmd/weave` plus the operations contract. No host integration may
   depend on HTML routes except for embedding the human-facing UI.
8. Every layer boundary is covered by a contract test; no layer depends on another layer's
   internal file layout.

## Key Decisions

| Decision | Choice |
| --- | --- |
| Engine language | Go |
| Render language | Go (`html/template` + goldmark) |
| Lens language | TypeScript / Preact, built to static assets |
| Binary model | Single Go binary with embedded lens assets |
| **Integration model** | **CLI I/O (argv/stdout/stderr/exit codes) first** |
| Go library | Optional in-process API for Go hosts; not required for integration |
| HTTP | Presentation only: browser UI, lens shell, local pages |
| Future transport | TCP MCP adapter over the same operations, added later |
| CLI output contract | `weave.cli/v1` envelope only in v1 |
| Deferred CLI modes | `--legacy-json` and `weave engine --stdio` ship after v1 |
| Operations | One typed operation set shared by CLI and future MCP |
| Contract | JSON Schema v1 with cross-language golden fixtures |
| Reference implementation | TypeScript until the Go port passes the full conformance suite |
| YAML | `gopkg.in/yaml.v3` in Go; required subset fixed by fixtures |
| Markdown | `goldmark` with GFM; raw HTML disabled |
| Search | Custom BM25 port, not a search engine dependency |
| **Repository layout** | **Single Go module at the repo root** (`github.com/prulloac/weave`) |
| **Migration sequencing** | **Skip TS package extraction**; freeze the reference and export goldens |
| **v1 scope** | **Milestone A: contract + engine + CLI.** Milestone B: render + mount + lens |
| **Contract source** | **JSON Schema is authoritative**; generated TS types; hand-written, schema-tested Go structs |
| **CLI cache** | **Enabled by default**; `WEAVE_CACHE_DIR`, `--no-cache` |
| **`weave parse` payload** | **Includes concept bodies**; `--no-body` deferred post-v1 |

## Documents

| Document | Purpose |
| --- | --- |
| [ADR 0001](adr/0001-go-engine-and-ts-lens.md) | Why Go for the engine/render/mount and TypeScript for the lens |
| [ADR 0002](adr/0002-v1-scope-layout-and-contract.md) | Approved v1 layout, sequencing, scope, contract generation, cache and parse payload |
| [Contract layer](layers/contract.md) | Language-neutral domain model, schemas, semantics and versioning |
| [Engine layer](layers/engine-go.md) | Go parser, graph, search, navigation, cache and FS adapter |
| [Operations layer](layers/operations.md) | Transport-neutral requests, responses and error model |
| [CLI layer](layers/cli-go.md) | Primary integration contract: argv, stdin/stdout, envelopes, exit codes |
| [Render layer](layers/render-go.md) | Go templates, Markdown rendering, theme, zero-JS routes |
| [Lens layer](layers/lens-ts.md) | TypeScript asset bundle, DOM contract and build pipeline |
| [Mount layer](layers/mount-go.md) | Browser presentation adapter: routes, artifacts, assets, registry |
| [Integration contracts](integration/contracts.md) | CLI, operations, domain, asset and filesystem contracts |
| [Integration steps](integration/steps.md) | Phased, gated migration from the TypeScript monolith |
| [Integration testing](integration/testing.md) | Conformance corpus, test pyramid, CI and acceptance gates |

## Reading Order

1. Decision records and this index.
2. Contract layer.
3. Operations layer.
4. CLI layer and integration contracts.
5. Engine layer.
6. Render, lens and mount layers (presentation only).
7. Integration steps.
8. Integration testing.

## Non-Goals

- Porting the Preact lens to Go.
- Maintaining two production engines indefinitely.
- Live reload or file watching inside mounts.
- A plugin system or dynamic loading of engines.
- Changing the OKF v0.2 input format or the bundle authoring experience.
- Rewriting the Astro development site as part of the migration.
- An HTTP or TCP integration API in v1. HTTP exists only to render the browser UI.
- A long-running daemon. The optional stdio mode is still process-local CLI I/O.

## Status

Proposal, `draft`. **Phase 0 is complete:** contract v1 schemas, generated TypeScript types, the
golden corpus and the exporter exist under `contract/v1`. **Phase 2 scaffold is complete:** the Go
module, contract structs, schema round-trip tests, package skeletons and CI/verify entrypoints
exist. No engine logic has been written yet. The current TypeScript implementation remains the
reference until the conformance suite described in
[Integration testing](integration/testing.md) passes against the Go port.
