# Behavior Scenarios

Each document is a human-readable behavior contract. The executable coverage column names the test
or check; a scenario without one is a documentation gap.

| Behavior | Spec | Executable coverage |
| --- | --- | --- |
| [AI extension core behavior](ai-plugin.md) | [weave-ai-plugin.md](https://github.com/prulloac/weave-docs/blob/main/specs/weave-ai-plugin.md) | `cmd/weave/ai_bdd_test.go`, `engine/privacy/redact_test.go`, `engine/extensions/schema_test.go` |
| [BM25 calibration](bm25-calibration.md) | [bm25-ranking.md](https://github.com/prulloac/weave-docs/blob/main/specs/bm25-ranking.md) | `engine/search/bm25_calibration_test.go` |
| [BM25 explain](bm25-explain.md) | [weave-explain.md](https://github.com/prulloac/weave-docs/blob/main/specs/weave-explain.md) | `cmd/weave/bm25_explain_bdd_test.go` |
| [BM25 ranking](bm25-ranking.md) | [bm25-ranking.md](https://github.com/prulloac/weave-docs/blob/main/specs/bm25-ranking.md) | `storage/sqlite/fts.go`, `engine/search/hybrid_bm25.go` |
| [Blob assets](blob-assets.md) | [blob-assets.md](https://github.com/prulloac/weave-docs/blob/main/specs/blob-assets.md) | `cmd/weave/blob_assets_bdd_test.go`, `engine/assets/assets_test.go`, `storage/sqlite/schema_v7_test.go`, `mcp/blob_test.go` |
| [Bundle export](bundle-export.md) | [bundle-export.md](https://github.com/prulloac/weave-docs/blob/main/specs/bundle-export.md) | `cmd/weave/export_bdd_test.go` |
| [Codec extensions](codec-extensions.md) | [codec-extensions.md](https://github.com/prulloac/weave-docs/blob/main/specs/codec-extensions.md) | `cmd/weave/codecs_bdd_test.go`, `engine/codecs/codecs_test.go`, `storage/sqlite/schema_v7_test.go` |
| [Compression](compression.md) | [compression.md](https://github.com/prulloac/weave-docs/blob/main/specs/compression.md) | `cmd/weave/compression_bdd_test.go`, `storage/sqlite/compression_test.go` |
| [Configuration](configuration.md) | [configuration.md](https://github.com/prulloac/weave-docs/blob/main/specs/configuration.md) | `cmd/weave/config_bdd_test.go` |
| [Dry run](dryrun.md) | [configuration.md](https://github.com/prulloac/weave-docs/blob/main/specs/configuration.md) | `cmd/weave/dryrun_bdd_test.go` |
| [Embeddings](embeddings.md) | [embeddings.md](https://github.com/prulloac/weave-docs/blob/main/specs/embeddings.md) | `cmd/weave/embeddings_bdd_test.go`, `storage/sqlite/embeddings_test.go`, `storage/sqlite/vec_test.go` |
| [Error contract](error-contract.md) | [configuration.md](https://github.com/prulloac/weave-docs/blob/main/specs/configuration.md) | `cmd/weave/error_contract_bdd_test.go` |
| [Extension IPC](extension-ipc.md) | [extension-ipc.md](https://github.com/prulloac/weave-docs/blob/main/specs/extension-ipc.md) | `engine/extensions/extensions_test.go`, `cmd/weave/extensions_bdd_test.go` |
| [Extension output boundary](extension-output-boundary.md) | [extension-output-boundary.md](https://github.com/prulloac/weave-docs/blob/main/specs/extension-output-boundary.md) | planned: `cmd/weave/extension_boundary_bdd_test.go`, `engine/operations/extension_documents_test.go`, `engine/extensions/boundary_test.go`, `mcp/workspace_test.go` |
| [Extension data](extension-data.md) | [extension-data.md](https://github.com/prulloac/weave-docs/blob/main/specs/extension-data.md) | `cmd/weave/extension_data_bdd_test.go`, `cmd/weave/export_bdd_test.go`, `engine/extensions/data_test.go`, `engine/operations/extension_data_test.go`, `storage/sqlite/extension_data_test.go`, `mcp/extension_data_test.go` |
| [External crypto](external-crypto.md) | [external-crypto-providers.md](https://github.com/prulloac/weave-docs/blob/main/specs/external-crypto-providers.md) | `cmd/weave/crypto_provider_bdd_test.go` |
| [Feedback](feedback.md) | [configuration.md](https://github.com/prulloac/weave-docs/blob/main/specs/configuration.md) | `cmd/weave/feedback_bdd_test.go` |
| [Help](help.md) | [configuration.md](https://github.com/prulloac/weave-docs/blob/main/specs/configuration.md) | `cmd/weave/help_bdd_test.go` |
| [Incremental sync](incremental-sync.md) | [storage-and-sync.md](https://github.com/prulloac/weave-docs/blob/main/specs/storage-and-sync.md) | `cmd/weave/keyword_store_bdd_test.go`, `cmd/weave/sync_bdd_test.go` |
| [Local-first](local-first.md) | [configuration.md](https://github.com/prulloac/weave-docs/blob/main/specs/configuration.md) | `cmd/weave/config_bdd_test.go` |
| [Machine output](machine-output.md) | [configuration.md](https://github.com/prulloac/weave-docs/blob/main/specs/configuration.md) | `cmd/weave/output_schema_bdd_test.go` |
| [MCP multi-source](mcp-multi-source.md) | [multi-bundle.md](https://github.com/prulloac/weave-docs/blob/main/specs/multi-bundle.md) | `mcp/multi_test.go` |
| [MCP resources](mcp-resources.md) | [mcp-server.md](https://github.com/prulloac/weave-docs/blob/main/specs/mcp-server.md) | `mcp/resource_test.go` |
| [MCP sensitive](mcp-sensitive.md) | [mcp-sensitive.md](https://github.com/prulloac/weave-docs/blob/main/specs/mcp-sensitive.md) | `mcp/sensitive_test.go`, `cmd/weave/privacy_bdd_test.go` |
| [MCP server](mcp-server.md) | [mcp-server.md](https://github.com/prulloac/weave-docs/blob/main/specs/mcp-server.md) | `mcp/server_test.go` |
| [MCP transport](mcp-transport.md) | [mcp-server.md](https://github.com/prulloac/weave-docs/blob/main/specs/mcp-server.md) | `mcp/transport_test.go` |
| [Multi-bundle](multi-bundle.md) | [multi-bundle.md](https://github.com/prulloac/weave-docs/blob/main/specs/multi-bundle.md) | `cmd/weave/multi_bundle_bdd_test.go` |
| [Node graph engine](node-graph-engine.md) | [node-graph-engine.md](https://github.com/prulloac/weave-docs/blob/main/specs/node-graph-engine.md) | `engine/conformance_test.go`, `engine/graph` tests |
| [Onboarding](onboarding.md) | [configuration.md](https://github.com/prulloac/weave-docs/blob/main/specs/configuration.md) | `cmd/weave/help_bdd_test.go`, `cmd/weave/feedback_bdd_test.go` |
| [Permissive ingestion](permissive-ingestion.md) | [permissive-ingestion.md](https://github.com/prulloac/weave-docs/blob/main/specs/permissive-ingestion.md) | `cmd/weave/permissive_bdd_test.go` |
| [Pluggable scoring](extension-scoring.md) | [extension-scoring.md](https://github.com/prulloac/weave-docs/blob/main/specs/extension-scoring.md) | `cmd/weave/scoring_bdd_test.go`, `engine/extensions/scoring_test.go` |
| [Privacy directives](privacy-directives.md) | [privacy-directives.md](https://github.com/prulloac/weave-docs/blob/main/specs/privacy-directives.md) | `cmd/weave/privacy_bdd_test.go`, `engine/privacy/privacy_test.go` |
| [Release automation](release-automation.md) | [release-automation.md](https://github.com/prulloac/weave-docs/blob/main/specs/release-automation.md) | `release/release_test.go`, `cmd/weave/release_bdd_test.go` |
| [Release readiness](release-readiness.md) | [release-readiness.md](https://github.com/prulloac/weave-docs/blob/main/specs/release-readiness.md) | `cmd/docs-lint/main_test.go`, `cmd/weave/smoke_test.go`, `release/release_test.go`, `weave-docs/scripts/check-links.py`, `Docs` workflow |
| [Storage and sync](storage-and-sync.md) | [storage-and-sync.md](https://github.com/prulloac/weave-docs/blob/main/specs/storage-and-sync.md) | `cmd/weave/storage_bdd_test.go` |
| [Sync watch](sync-watch.md) | [watch-mode.md](https://github.com/prulloac/weave-docs/blob/main/specs/watch-mode.md) | `cmd/weave/sync_watch_bdd_test.go` |
| [Watch mode](watch-mode.md) | [watch-mode.md](https://github.com/prulloac/weave-docs/blob/main/specs/watch-mode.md) | `watch/watcher_test.go` |
| [Weave explain](weave-explain.md) | [weave-explain.md](https://github.com/prulloac/weave-docs/blob/main/specs/weave-explain.md) | `cmd/weave/explain_bdd_test.go` |
| [Weave image](weave-image.md) | [weave-image.md](https://github.com/prulloac/weave-docs/blob/main/specs/weave-image.md) | `cmd/weave/blob_assets_bdd_test.go`, `weave-image/main_test.go` |
| [Weave code](weave-code.md) | [weave-code.md](https://github.com/prulloac/weave-docs/blob/main/specs/weave-code.md) | `cmd/weave/weave_code_bdd_test.go`, `cmd/weave/code_relations_bdd_test.go`, `engine/assets/code_test.go`, `engine/graph/relations_test.go`, `engine/extensions/project_test.go`, `storage/sqlite/relations_test.go`; extension tests live in `weave-code` |
| [Weave git source](weave-git.md) | [weave-git.md](https://github.com/prulloac/weave-docs/blob/main/specs/weave-git.md) | `cmd/weave/weave_git_bdd_test.go`, `engine/extensions/source_test.go`, `engine/operations/source_test.go`, `storage/sqlite/schema_v9_test.go`; provider tests live in `weave-git` |
| [Workspace and source registry](workspace.md) | [workspace.md](https://github.com/prulloac/weave-docs/blob/main/specs/workspace.md) | `cmd/weave/workspace_bdd_test.go`, `storage/sqlite/schema_v8_test.go`, `mcp/workspace_test.go` |

Extension-side protocol and provider scenarios live in
[`weave-ai`](https://github.com/prulloac/weave-ai/blob/main/docs/bdd/ai-plugin.md). Image provider,
OCR and installer scenarios live in
[`weave-image`](https://github.com/prulloac/weave-image/blob/main/docs/bdd/image.md); codec
scenarios live in
[`weave-codec-lz4`](https://github.com/prulloac/weave-codec-lz4/blob/main/docs/bdd/codec.md).
