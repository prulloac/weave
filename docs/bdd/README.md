# Behavior Scenarios

Each document is a human-readable behavior contract. The Go test column names the executable
coverage; a scenario without a test is a documentation gap.

| Behavior | Spec | Go tests |
| --- | --- | --- |
| [AI plugin](ai-plugin.md) | [weave-ai-plugin.md](../specs/weave-ai-plugin.md) | `cmd/weave/ai_bdd_test.go`, `cmd/weave-ai/main_test.go` |
| [BM25 calibration](bm25-calibration.md) | [bm25-ranking.md](../specs/bm25-ranking.md) | `engine/search/bm25_calibration_test.go` |
| [BM25 explain](bm25-explain.md) | [weave-explain.md](../specs/weave-explain.md) | `cmd/weave/bm25_explain_bdd_test.go` |
| [BM25 ranking](bm25-ranking.md) | [bm25-ranking.md](../specs/bm25-ranking.md) | `storage/sqlite/fts.go`, `engine/search/hybrid_bm25.go` |
| [Bundle export](bundle-export.md) | [bundle-export.md](../specs/bundle-export.md) | `cmd/weave/export_bdd_test.go` |
| [Compression](compression.md) | [compression.md](../specs/compression.md) | `engine/compression/compression_test.go` |
| [Configuration](configuration.md) | [configuration.md](../specs/configuration.md) | `cmd/weave/config_bdd_test.go` |
| [Dry run](dryrun.md) | [configuration.md](../specs/configuration.md) | `cmd/weave/dryrun_bdd_test.go` |
| [Error contract](error-contract.md) | [configuration.md](../specs/configuration.md) | `cmd/weave/error_contract_bdd_test.go` |
| [Extension IPC](extension-ipc.md) | [extension-ipc.md](../specs/extension-ipc.md) | `engine/extensions/extensions_test.go`, `cmd/weave/extensions_bdd_test.go` |
| [External crypto](external-crypto.md) | [external-crypto-providers.md](../specs/external-crypto-providers.md) | `cmd/weave/crypto_provider_bdd_test.go` |
| [Feedback](feedback.md) | [configuration.md](../specs/configuration.md) | `cmd/weave/feedback_bdd_test.go` |
| [Help](help.md) | [configuration.md](../specs/configuration.md) | `cmd/weave/help_bdd_test.go` |
| [Incremental sync](incremental-sync.md) | [storage-and-sync.md](../specs/storage-and-sync.md) | `cmd/weave/keyword_store_bdd_test.go`, `cmd/weave/sync_bdd_test.go` |
| [Local-first](local-first.md) | [configuration.md](../specs/configuration.md) | `cmd/weave/config_bdd_test.go` |
| [Machine output](machine-output.md) | [configuration.md](../specs/configuration.md) | `cmd/weave/output_schema_bdd_test.go` |
| [MCP multi-source](mcp-multi-source.md) | [multi-bundle.md](../specs/multi-bundle.md) | `mcp/multi_test.go` |
| [MCP resources](mcp-resources.md) | [mcp-server.md](../specs/mcp-server.md) | `mcp/resource_test.go` |
| [MCP sensitive](mcp-sensitive.md) | [mcp-sensitive.md](../specs/mcp-sensitive.md) | `mcp/sensitive_test.go`, `cmd/weave/privacy_bdd_test.go` |
| [MCP server](mcp-server.md) | [mcp-server.md](../specs/mcp-server.md) | `mcp/server_test.go` |
| [MCP transport](mcp-transport.md) | [mcp-server.md](../specs/mcp-server.md) | `mcp/transport_test.go` |
| [Multi-bundle](multi-bundle.md) | [multi-bundle.md](../specs/multi-bundle.md) | `cmd/weave/multi_bundle_bdd_test.go` |
| [Node graph engine](node-graph-engine.md) | [node-graph-engine.md](../specs/node-graph-engine.md) | `engine/conformance_test.go`, `engine/graph` tests |
| [Onboarding](onboarding.md) | [configuration.md](../specs/configuration.md) | `cmd/weave/help_bdd_test.go`, `cmd/weave/feedback_bdd_test.go` |
| [Permissive ingestion](permissive-ingestion.md) | [permissive-ingestion.md](../specs/permissive-ingestion.md) | `cmd/weave/permissive_bdd_test.go` |
| [Pluggable scoring](extension-scoring.md) | [extension-scoring.md](../specs/extension-scoring.md) | planned: `cmd/weave/scoring_bdd_test.go`, `engine/extensions/scoring_test.go` |
| [Privacy directives](privacy-directives.md) | [privacy-directives.md](../specs/privacy-directives.md) | `cmd/weave/privacy_bdd_test.go`, `engine/privacy/privacy_test.go` |
| [Release automation](release-automation.md) | [release-automation.md](../specs/release-automation.md) | `release/release_test.go`, `cmd/weave/release_bdd_test.go` |
| [Release readiness](release-readiness.md) | [release-readiness.md](../specs/release-readiness.md) | `cmd/docs-lint/main_test.go`, `cmd/weave/smoke_test.go`, `release/release_test.go` |
| [Storage and sync](storage-and-sync.md) | [storage-and-sync.md](../specs/storage-and-sync.md) | `cmd/weave/storage_bdd_test.go` |
| [Sync watch](sync-watch.md) | [watch-mode.md](../specs/watch-mode.md) | `cmd/weave/sync_watch_bdd_test.go` |
| [Watch mode](watch-mode.md) | [watch-mode.md](../specs/watch-mode.md) | `watch/watcher_test.go` |
| [Weave explain](weave-explain.md) | [weave-explain.md](../specs/weave-explain.md) | `cmd/weave/explain_bdd_test.go` |
