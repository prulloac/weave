---
okf_version: "0.2"
---

# Weave Architecture

Target architecture and migration plan for splitting Weave into reusable layers, with the
headless engine, renderer, mount server and CLI implemented in **Go**, and the browser lens kept
as **TypeScript-built assets**.

## Target Topology

```text
┌───────────────────────────────────────────────────────────────┐
│                        Host project                           │
│            Go service · Go CLI · embedded static site         │
└───────────────────────────────┬───────────────────────────────┘
                                │ imports / http.Handler
┌───────────────────────────────▼───────────────────────────────┐
│  weave/mount                                                  │
│  artifact layer · route dispatch · assets · registry · CLI    │
├───────────────────────────────────────────────────────────────┤
│  weave/render                                                 │
│  html/template shells · goldmark · theme · lens shell         │
├───────────────────────────────────────────────────────────────┤
│  weave/engine                                                 │
│  okf parser · graph · BM25 search · navigation · cache        │
├───────────────────────────────────────────────────────────────┤
│  weave/contract                                               │
│  JSON schemas · Go types · versioning · golden fixtures       │
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
| Render | Go | `weave/render` | Markdown to HTML, page shells, theme | `contract`, `engine/graph` types |
| Lens | TypeScript | `lens` (standalone) | Browser canvas UI, asset bundle | Graph payload contract only |
| Mount | Go | `weave/mount` | Artifacts, HTTP routes, static assets, registry | `engine`, `render`, lens assets |
| CLI | Go | `cmd/weave` | Argument parsing, exit codes, signals | `mount`, `engine` |

## Dependency Rules

1. `contract` has no internal dependencies and no runtime dependencies.
2. `engine` may import `contract`; it must not import `render`, `mount`, `cmd`, or any UI code.
3. `render` may import `contract` and graph view types; it must not import `mount` or `cmd`.
4. `mount` may import `engine`, `render`, and embedded lens assets; it must not import `cmd`.
5. `cmd/weave` is the only package allowed to parse process arguments, set exit codes, install
   signal handlers, or write the mount registry by default.
6. The lens bundle must not import the engine, parser, search index, or Node APIs. It consumes
   only the serialized graph payload.
7. Every layer boundary is covered by a contract test; no layer depends on another layer's
   internal file layout.

## Key Decisions

| Decision | Choice |
| --- | --- |
| Engine language | Go |
| Render language | Go (`html/template` + goldmark) |
| Lens language | TypeScript / Preact, built to static assets |
| Binary model | Single Go binary with embedded lens assets |
| Integration model | In-process Go package; optional `http.Handler` |
| Contract | JSON Schema v1 with cross-language golden fixtures |
| Reference implementation | TypeScript until the Go port passes the full conformance suite |
| YAML | `gopkg.in/yaml.v3` in Go; required subset fixed by fixtures |
| Markdown | `goldmark` with GFM; raw HTML disabled |
| Search | Custom BM25 port, not a search engine dependency |

## Documents

| Document | Purpose |
| --- | --- |
| [ADR 0001](adr/0001-go-engine-and-ts-lens.md) | Why Go for the engine/render/mount and TypeScript for the lens |
| [Contract layer](layers/contract.md) | Language-neutral domain model, schemas, semantics and versioning |
| [Engine layer](layers/engine-go.md) | Go parser, graph, search, navigation, cache and FS adapter |
| [Render layer](layers/render-go.md) | Go templates, Markdown rendering, theme, zero-JS routes |
| [Lens layer](layers/lens-ts.md) | TypeScript asset bundle, DOM contract and build pipeline |
| [Mount layer](layers/mount-go.md) | `http.Handler`, artifacts, routes, registry and embedding |
| [CLI layer](layers/cli-go.md) | Command surface, flags, exit codes and signal handling |
| [Integration contracts](integration/contracts.md) | Go API, HTTP API, DOM, asset and filesystem contracts |
| [Integration steps](integration/steps.md) | Phased, gated migration from the TypeScript monolith |
| [Integration testing](integration/testing.md) | Conformance corpus, test pyramid, CI and acceptance gates |

## Reading Order

1. Decision record and this index.
2. Contract layer.
3. Integration contracts.
4. Engine, render and lens layer docs.
5. Mount and CLI layer docs.
6. Integration steps.
7. Integration testing.

## Non-Goals

- Porting the Preact lens to Go.
- Maintaining two production engines indefinitely.
- Live reload or file watching inside mounts.
- A plugin system or dynamic loading of engines.
- Changing the OKF v0.2 input format or the bundle authoring experience.
- Rewriting the Astro development site as part of the migration.

## Status

Proposal, `draft`. No production code has been migrated yet. The current TypeScript
implementation remains the reference until the conformance suite described in
[Integration testing](integration/testing.md) passes against the Go port.
