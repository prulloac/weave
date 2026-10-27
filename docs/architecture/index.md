---
okf_version: "0.2"
---

# Weave Architecture

Weave is an **in-memory related-document database**. It loads OKF bundles, builds concept nodes
and link edges, and answers keyword and weighted fuzzy searches with top-K results.

**The UI is retired.** Browser visualization, renderer, mount, lens and Astro are cancelled by
[ADR 0003](adr/0003-keyword-fuzzy-search-and-ui-retirement.md). The Go engine and CLI are the only
product surfaces.

## New Purpose

- Load a bundle into memory.
- Build nodes and edges that relate documents.
- Search by two dimensions:
  - mandatory frontmatter `keywords`,
  - weighted fuzzy matching over title, description and body.
- Return top-K keyword, fuzzy and hybrid results.
- Navigate the graph with `show`, `backlinks` and `path`.

The detailed contract and algorithms are in
[Keyword + Fuzzy Search](../specs/keyword-fuzzy-search.md).

## Target Topology

```text
┌───────────────────────────────────────────────────────────────┐
│                        Host project                           │
│         subprocess · stdin/stdout · exit codes                │
└───────────────────────────────┬───────────────────────────────┘
                                │ CLI I/O
┌───────────────────────────────▼───────────────────────────────┐
│  cmd/weave + engine/operations                                │
│  search · parse · graph · show · backlinks · path · version   │
├───────────────────────────────────────────────────────────────┤
│  engine/search                                                │
│  documents · keyword index · fuzzy index · trigram index      │
├───────────────────────────────────────────────────────────────┤
│  engine/graph + engine/okf                                    │
│  nodes · edges · traversal · bundle parser · cache            │
├───────────────────────────────────────────────────────────────┤
│  contract/v1                                                  │
│  schemas · Go types · search results · goldens                │
└───────────────────────────────────────────────────────────────┘
```

## Layers

| Layer | Language | Package | Responsibility |
| --- | --- | --- | --- |
| Contract | JSON Schema + Go | `contract/v1` | Documents, keywords, search results, envelopes, goldens |
| OKF | Go | `engine/okf` | Parse bundles, frontmatter, links, validation, cache |
| Graph | Go | `engine/graph` | Nodes, edges, degrees, traversal |
| Search | Go | `engine/search` | In-memory catalog, keyword index, fuzzy index, ranking |
| Operations | Go | `engine/operations` | Transport-neutral search/navigation requests |
| CLI | Go | `cmd/weave` | Argv, stdout/stderr, envelopes, exit codes |

## Integration Surfaces

| Surface | Audience | Status | Contract |
| --- | --- | --- | --- |
| CLI processes | Any language or runtime | Primary | argv, stdout/stderr, exit codes, `weave.cli/v1` |
| Go package API | Go hosts | Optional | Engine/store packages |
| TCP MCP server | Future agent integrations | Later | MCP tools over the operations layer |
| Browser presentation | — | **Retired** | Removed by ADR 0003 |

A host that is not written in Go must use the CLI. A Go host may use the package API, but the CLI
remains the compatibility promise.

## Milestones

| Milestone | Scope | Status |
| --- | --- | --- |
| A — Base engine | contract, parser, graph, operations, machine CLI | Complete |
| B — Search v1 | mandatory keywords, weighted fuzzy search, hybrid top-K, calibration, CLI `search` | Complete |
| C — Cleanup | remove UI, old search and TypeScript implementation; finalize v1 docs | Complete |
| D — MCP | future transport over operations | Later |

## Dependency Rules

1. `contract/v1` has no internal dependencies.
2. `engine/okf` and `engine/graph` may import `contract/v1`; they must not import `search`,
   `operations`, `cmd`, or any UI code.
3. `engine/search` may import `engine/okf`, `engine/graph` and `contract/v1`.
4. `engine/operations` may import the engine, search and navigation packages and `contract/v1`; it
   must not import `cmd`.
5. `cmd/weave` is the only package allowed to read process arguments and environment variables,
   set exit codes, or write output.
6. No package may import Astro, Preact, renderers, mount code or lens assets.
7. The integration boundary is `cmd/weave` plus the operations contract.
8. Every layer boundary is covered by a contract test.

## Key Decisions

| Decision | Choice |
| --- | --- |
| Engine language | Go |
| Product surface | Go CLI; no UI |
| Document identity | Bundle-relative path minus `.md` |
| Required frontmatter | `type` and `keywords` |
| Keyword search | Exact token + prefix boost; AND semantics |
| Fuzzy fields | Title 1.00, description 0.60, body 0.30 |
| Fuzzy similarity | Normalized Levenshtein and trigram Dice, max |
| Search output | Keyword, fuzzy and hybrid top-K lists |
| Default mode | Hybrid |
| Default top-K | 10; max 100 |
| Contract version | New v1 replaces the previous v1 in place |
| CLI operation | `search` replaces `find` |
| Calibration | Labeled corpus with NDCG@K and MRR@K baselines |
| Storage | In-memory; no external database |
| Integration | CLI first; Go package optional; MCP later |
| Presentation | Cancelled and removed |

## Documents

| Document | Purpose |
| --- | --- |
| [ADR 0003](adr/0003-keyword-fuzzy-search-and-ui-retirement.md) | New purpose, keywords, fuzzy search and UI retirement |
| [Keyword + Fuzzy Search spec](../specs/keyword-fuzzy-search.md) | Detailed data model, algorithms, CLI and calibration |
| [Contract layer](layers/contract.md) | Documents, keywords, graph and search contract |
| [Engine layer](layers/engine-go.md) | Parser, graph, search and navigation |
| [Operations layer](layers/operations.md) | Transport-neutral requests and envelopes |
| [CLI layer](layers/cli-go.md) | CLI commands, output modes and exit codes |
| [Integration contracts](integration/contracts.md) | CLI, operations and filesystem contracts |
| [Integration steps](integration/steps.md) | Completed search v1 implementation phases |
| [Integration testing](integration/testing.md) | Conformance and test strategy |

## Reading Order

1. ADR 0003 and this index.
2. Keyword + Fuzzy Search spec.
3. Contract layer.
4. Engine and store/search implementation.
5. Operations and CLI.
6. Integration contracts and testing.
7. Historical ADRs only if needed.

## Non-Goals

- Any browser UI, renderer, mount, lens, Astro or Preact work.
- Synonyms, stemming or embeddings in v1.
- Cross-bundle federated search.
- Persistent indexes or a query daemon.
- An HTTP integration API.
- Natural-language question answering.
- MCP transport in the current milestone.

## Status

The new v1 is implemented. The Go engine parses mandatory keywords, builds keyword and fuzzy
indexes, returns keyword/fuzzy/hybrid top-K results, and exposes the `search` CLI command.
Calibration passes its baseline. The UI, TypeScript implementation, Astro, Preact, Playwright and
presentation dependencies have been removed. The remaining deferred work is stdio and MCP.
