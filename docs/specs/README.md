# Specifications

Every specification owns one behavioral contract. Delivery status lives in the
[roadmap](../../ROADMAP.md) and [backlog](../backlog.md); the `Status` column below is document
maturity only.

| Spec | Status | BDD | Register |
| --- | --- | --- | --- |
| [OKF Bundle Parser](okf-bundle-parser.md) | stable | [Node graph](../bdd/node-graph-engine.md) | foundation |
| [Node Graph Engine](node-graph-engine.md) | stable | [Node graph](../bdd/node-graph-engine.md) | foundation |
| [Keyword + Fuzzy Search](keyword-fuzzy-search.md) | stable | — | foundation |
| [Weave Audit](weave-audit.md) | stable | — | 1, 3 |
| [Weave Relate](weave-relate.md) | stable | — | 1, 2 |
| [Weave Explain](weave-explain.md) | stable | [Explain](../bdd/weave-explain.md), [BM25 explain](../bdd/bm25-explain.md) | 1, 4, 21 |
| [Permissive Ingestion](permissive-ingestion.md) | stable | [Permissive ingestion](../bdd/permissive-ingestion.md) | 5 |
| [Storage & Sync](storage-and-sync.md) | stable | [Storage](../bdd/storage-and-sync.md), [Incremental sync](../bdd/incremental-sync.md) | 6, 7 |
| [Watch Mode](watch-mode.md) | stable | [Watch mode](../bdd/watch-mode.md), [Sync watch](../bdd/sync-watch.md) | 8, 24 |
| [BM25 Ranking](bm25-ranking.md) | stable | [BM25](../bdd/bm25-ranking.md), [Calibration](../bdd/bm25-calibration.md) | 9, 10, 21 |
| [Compression](compression.md) | stable | [Compression](../bdd/compression.md) | 11 |
| [Privacy Directives](privacy-directives.md) | stable | [Privacy](../bdd/privacy-directives.md) | 12 |
| [Extension Subprocess IPC](extension-ipc.md) | stable | [Extension IPC](../bdd/extension-ipc.md) | 13, 14, 15 |
| [External Crypto Providers](external-crypto-providers.md) | stable | [External crypto](../bdd/external-crypto.md) | 17 |
| [Weave AI Extension](weave-ai-plugin.md) | stable | [AI plugin](../bdd/ai-plugin.md) | 16 |
| [MCP Server](mcp-server.md) | stable | [MCP server](../bdd/mcp-server.md), [Transport](../bdd/mcp-transport.md), [Resources](../bdd/mcp-resources.md) | 19, 20, 22, 23, 24 |
| [MCP Sensitive Access](mcp-sensitive.md) | stable | [MCP sensitive](../bdd/mcp-sensitive.md) | 18 |
| [Multiple Bundle Sources](multi-bundle.md) | stable | [Multi-bundle](../bdd/multi-bundle.md), [MCP multi-source](../bdd/mcp-multi-source.md) | 27, 28 |
| [Release Automation](release-automation.md) | stable | [Release automation](../bdd/release-automation.md) | 25 |
| [Bundle Export](bundle-export.md) | stable | [Bundle export](../bdd/bundle-export.md) | 26 |
| [Configuration](configuration.md) | stable | [Configuration](../bdd/configuration.md), [Machine output](../bdd/machine-output.md), [Help](../bdd/help.md), [Feedback](../bdd/feedback.md), [Onboarding](../bdd/onboarding.md), [Local-first](../bdd/local-first.md), [Error contract](../bdd/error-contract.md), [Dry run](../bdd/dryrun.md) | 30 |
| [Optimization Program](optimization.md) | draft | — | 29 (stretch) |
| [Release Readiness](release-readiness.md) | stable | [Release readiness](../bdd/release-readiness.md) | release gate |

## Ownership rules

- A spec is owned by the register rows listed above. `foundation` rows predate the register and are
  covered by their behavior tests and calibration baselines.
- `—` means the spec is covered by unit, conformance or calibration tests rather than a separate BDD
  document.
- A spec may be owned by several register rows when the behavior is genuinely shared. The first
  listed spec in a register row owns the contract.
- Behavior scenarios live in [`docs/bdd`](../bdd/README.md); decisions live in
  [`docs/architecture/adr`](../architecture/adr/README.md).
