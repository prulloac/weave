---
okf_version: "0.2"
---

# Weave Architecture

Weave is a **local-first related-document database**. The core loads OKF bundles, builds concept
nodes and link edges, and answers keyword and weighted fuzzy searches with top-K results. v1.0 adds
optional SQLite persistence, BM25 ranking, privacy directives, extensions and MCP.

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
| Operations | Go | `engine/operations` | Transport-neutral ingest, rebuild, search, navigation, audit, relate and explain requests |
| CLI | Go | `cmd/weave` | Argv, stdout/stderr, envelopes, exit codes |

## Integration Surfaces

| Surface | Audience | Status | Contract |
| --- | --- | --- | --- |
| CLI processes | Any language or runtime | Primary | argv, stdout/stderr, exit codes, `weave.cli/v1` |
| Go package API | Go hosts | Optional | Engine/store packages |
| MCP tool harness | Go hosts, MCP clients | M1–M3 available | `mcp.Server` + stdio/TCP transport |
| TCP MCP server | MCP clients | Available | newline JSON-RPC over TCP |
| Browser presentation | — | **Retired** | Removed by ADR 0003 |

A host that is not written in Go must use the CLI. A Go host may use the package API, but the CLI
remains the compatibility promise.

## Milestones

| Milestone | Scope | Status |
| --- | --- | --- |
| A — Base engine | contract, parser, graph, operations, machine CLI | Complete |
| B — Search v1 | mandatory keywords, weighted fuzzy search, hybrid top-K, calibration, CLI `search` | Complete |
| C — Cleanup | remove UI, old search and TypeScript implementation; finalize v1 docs | Complete |
| D — Introspection | `weave audit`, `weave relate`, `weave explain` | Complete |
| E — Storage & sync | SQLite persistence, incremental sync, watch | Complete |
| F — Privacy & extensions | Explicit privacy directives and subprocess plugins | Directives, IPC, enrichment and rerank complete; crypto providers planned |
| G — MCP | MCP tools over operations | M1–M4 complete; sensitive/polish next |

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
| Graph neighborhood | v1.0 `weave relate`, bounded BFS depth 0..5 across out/in/both |
| Explainability | v1.0 `weave explain`, keyword/fuzzy/BM25 evidence and hybrid breakdown |
| Ingestion | Strict OKF for parse/graph/search; permissive mode for `ingest`/`rebuild`/`audit` |
| Default mode | Hybrid |
| Default top-K | 10; max 100 |
| Contract version | New v1 replaces the previous v1 in place |
| CLI operation | `search` replaces `find` |
| Calibration | Labeled corpus with NDCG@K and MRR@K baselines |
| Storage | Bundle-scoped SQLite default at `~/.weave/index.db`; `--no-db` in-memory |
| Ranking | Persistent hybrid BM25 + fuzzy; `--no-db` legacy keyword + fuzzy |
| Privacy | v1.0 explicit encrypt/tokenize/hash directives |
| Extensions | v1.0 subprocess IPC over `weave.cli/v1` |
| Integration | CLI first; Go package optional; MCP M1 harness shipped; TCP pending |
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
| [Weave Audit spec](../../docs/specs/weave-audit.md) | Bundle and graph quality gate |
| [Weave Relate spec](../../docs/specs/weave-relate.md) | Graph neighborhood command |
| [Weave Explain spec](../../docs/specs/weave-explain.md) | Ranking breakdown command |
| [Permissive Ingestion spec](../../docs/specs/permissive-ingestion.md) | Markdown-tree ingestion with warnings |
| [Storage & Sync spec](../../docs/specs/storage-and-sync.md) | Persistent SQLite and incremental sync |
| [Watch Mode spec](../../docs/specs/watch-mode.md) | Live filesystem sync |
| [BM25 Ranking spec](../../docs/specs/bm25-ranking.md) | FTS5 lexical ranking |
| [Compression spec](../../docs/specs/compression.md) | Body codec contract (zstd default, LZ4 opt-in) |
| [Privacy Directives spec](../../docs/specs/privacy-directives.md) | Directive parser, keys, storage and search contract |
| [Privacy & Extensions spec](../../docs/specs/privacy-and-extensions.md) | Extension IPC and architecture summary |
| [Extension Subprocess IPC spec](../../docs/specs/extension-ipc.md) | `weave-<name>` discovery, transport, enrichment and rerank |
| [Release Automation spec](../../docs/specs/release-automation.md) | Tag-driven verified GitHub Releases |
| [Bundle Export spec](../../docs/specs/bundle-export.md) | Raw and encrypted bundle reconstruction |
| [Multiple Bundle Sources spec](../../docs/specs/multi-bundle.md) | Aliased multi-root reads and writes |
| [Optimization spec](../../docs/specs/optimization.md) | Benchmarks, budgets and maintainability targets |
| [Configuration spec](../../docs/specs/configuration.md) | Global YAML config and `WEAVE_*` variables |
| [External Crypto Providers spec](../../docs/specs/external-crypto-providers.md) | Age, Vault and KMS provider contract |
| [Weave AI Extension spec](../../docs/specs/weave-ai-plugin.md) | Optional enrichment and reranking plugin |
| [MCP Server spec](../../docs/specs/mcp-server.md) | TCP MCP roadmap and contract |
| [MCP Sensitive Access spec](../../docs/specs/mcp-sensitive.md) | Redacted defaults and opt-in decryption |
| [Roadmap](../../ROADMAP.md) | Current capabilities and roadmap |

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
- Synonyms, stemming or embeddings in the core engine.
- Cross-bundle federated search.
- A query daemon separate from the CLI.
- An HTTP integration API.
- Natural-language question answering.
- Remote multi-tenant MCP hosting.

## Status

The core v1 engine plus `weave audit`, `weave relate`, `weave explain`, SQLite storage, permissive
ingestion, incremental sync and BM25 lexical ranking are implemented. The Go engine parses
mandatory keywords, builds keyword and fuzzy indexes, returns keyword/fuzzy/hybrid top-K results,
exposes the `search` CLI command, audits bundles for conformance and graph quality, returns bounded
graph neighborhoods and explains score components. Bundles are persisted by default in a
bundle-scoped SQLite database at `~/.weave/index.db`; `--no-db` uses the transient parser.
Permissive ingestion synthesizes metadata for dirty Markdown, `weave sync` reconciles changed files
by SHA-256 hash, and persistent hybrid search combines normalized BM25 with fuzzy scores.
Calibration passes its baseline. The UI and old TypeScript stack are removed.

The remaining v1.0 scope — external crypto providers, the AI plugin, MCP sensitive access and
`weave sync --watch` — is specified but not implemented. MCP tools, stdio/TCP transport, auth,
resources and watch reload are available, privacy directives ship with encrypted, tokenized and
hashed storage, bodies use zstd by default, and `weave-<name>` extensions enrich writes and rerank
searches.
