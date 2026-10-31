# Weave

> To weave flat files into a living universe of knowledge.

Weave is a **local-first related-document database** for
[Open Knowledge Format (OKF)](#okf-bundles) bundles. The core loads Markdown/YAML documents, builds
concept nodes and link edges, and answers keyword and weighted fuzzy searches with top-K results.
v1.0 adds optional SQLite persistence, BM25 ranking, privacy directives, extensions and MCP.

Weave is CLI-first, local and offline. It has **no browser UI**.

## Features

- **Keyword search** over mandatory frontmatter keywords.
- **Weighted fuzzy search** over title, description and body.
- **Hybrid top-K results** with component scores and match metadata.
- **BM25 lexical ranking** through SQLite FTS5 with field weights, phrases and negative terms.
- **Calibrated hybrid ranking** using normalized BM25 plus fuzzy scores.
- **MCP tool harness** over operations, with stdio/TCP JSON-RPC transport, token auth and resources.
- **Graph navigation**: show, backlinks and shortest path.
- **Search explain**: per-token keyword and per-field fuzzy breakdowns.
- **Graph relate**: bounded depth/direction neighborhoods.
- **Bundle audit**: conformance, link, orphan and index-quality findings.
- **SQLite persistence**: bundle-scoped default database at `~/.weave/index.db`; `--no-db` opts out.
- **Body compression**: zstd by default, LZ4 opt-in, with per-row codecs and corruption recovery.
- **Permissive ingestion**: index non-OKF Markdown with deterministic synthetic metadata and warnings.
- **Incremental sync**: SHA-256 add/update/remove/skip with cascade reconciliation.
- **Privacy directives**: declared encrypt, tokenize and hash policies with key resolution.
- **Extensions**: `weave-<name>` enrichment during writes and reranking during search.
- **AI extension**: optional `weave-ai` with Ollama, OpenAI-compatible and Gemini providers.
- **Deterministic** JSON envelopes for machine integration.
- **Calibrated ranking** against a labeled query corpus.

## Requirements

- Go 1.27+

## Build

```sh
go build -o weave ./cmd/weave
```

Releases are produced by `.github/workflows/release.yml`: an annotated `v*` tag verifies the
repository, builds six static targets, writes sorted checksums and creates a draft GitHub Release.
`workflow_dispatch` performs a build-only dry run. See the
[release automation spec](docs/specs/release-automation.md).

## Usage

```sh
weave <command> [arguments] [flags]
```

Global flags:

- `--json` emits one `weave.cli/v1` envelope on stdout.
- `--no-cache` disables the parse cache, which is enabled by default.
- `--no-db` forces transient in-memory parsing; otherwise reads use `~/.weave/index.db`.
- `--db <file>` selects another database; it is mutually exclusive with `--no-db`.
- `--strict` / `--permissive` select ingest mode; ingest defaults to permissive.
- `--codec zstd|lz4|none` selects body compression for ingest, rebuild and sync; default `zstd`.
- `--with-enricher <name>` and `--rerank <name>` enable `weave-<name>` extensions; `--timeout`
  overrides the 5s default.
- `--source <alias=path>` adds another bundle source; repeatable on read and write commands.
- `--provider <name>` selects a `weave-crypto-<name>` provider for ingest/sync/show/audit/export.
- `--config <file>` selects a config file (`WEAVE_CONFIG` also works); `--no-config` ignores it;
  `--strict-config` rejects unknown keys. See [Configuration](#configuration).
- `--help` (or `-h`) prints the command summary; `--version` (or `-v`) prints the binary version.
- Diagnostics go to stderr.
- Exit codes: `0` success, `1` runtime error or no match, `2` usage or invalid parameters,
  `3` permission, privacy key or decryption failure.

### Configuration

Precedence: explicit CLI flag → `WEAVE_*` environment variable → config file → default.

```sh
weave config init                 # platform config dir, 0600; prints the next command
weave config show --json          # values, sources and origins
weave config get top_k --json     # one resolved key
weave config path --json
weave --config ./weave.yaml list ~/notes --no-db
```

```yaml
db: ~/.weave/index.db
cache_dir: ""            # empty uses the platform cache directory
codec: zstd
search_mode: hybrid
top_k: 10
ingest_mode: permissive
timeout: 5s
enricher: ""
rerank: ""
```

- Environment keys mirror the file: `WEAVE_DB`, `WEAVE_CACHE_DIR`, `WEAVE_CODEC`,
  `WEAVE_SEARCH_MODE`, `WEAVE_TOP_K`, `WEAVE_INGEST_MODE`, `WEAVE_TIMEOUT`, `WEAVE_ENRICHER`,
  `WEAVE_RERANK`; `WEAVE_CONFIG` selects the file.
- Config resolves through `os.UserConfigDir` (XDG on Linux, `%AppData%` on Windows, Application
  Support on macOS). The parse cache uses `os.UserCacheDir`.
- v1.0 keeps the database at `~/.weave/index.db`; use `--db`, `WEAVE_DB` or the `db` key to move
  it. XDG state migration is deferred (see
  [ADR 0004](docs/architecture/adr/0004-config-paths-and-db-location.md)).
- `config show` and `config get` report every value with `source` and `origin`.
- Secrets are never read from configuration; keys stay in `WEAVE_KEY_*` or `.weave/security.yaml`.
- Invalid or reserved keys exit `2`; unknown keys warn unless `--strict-config`.

Spec: [Configuration](docs/specs/configuration.md).

### Shipped commands

#### `weave contract`

Print the contract version, the CLI version and the operation/result pairs this binary supports.

```sh
weave contract
weave contract --json
```

#### `weave parse`

Parse an OKF bundle into concepts, validation state and the frontmatter index, then report the
concept count, detected `okf_version` and whether the bundle is valid.

```sh
weave parse ~/notes
weave parse ~/notes --json
weave parse ~/notes --no-cache
```

#### `weave graph`

Build the concept graph from Markdown links and `related` entries, and report node and edge counts.

```sh
weave graph ~/notes
weave graph ~/notes --json
```

#### `weave search`

Rank matching concepts with keyword, fuzzy, hybrid or BM25 lexical search. Keyword uses exact and
prefix matching with AND semantics; fuzzy tolerates typos; BM25 ranks multi-token lexical matches.
Persistent hybrid search combines normalized BM25 and fuzzy at 50/50 (default); `--no-db` retains
the legacy keyword + fuzzy fallback.

- `--mode hybrid|keyword|fuzzy|bm25` — default `hybrid`.
- `--top-k <n>` — default `10`, valid range `1..100`.
- `--rerank <name>` — optional `weave-<name>` reranker; failures fall back to core order.
- `--timeout <duration>` — extension timeout, default `5s`.
- Fuzzy field weights: **title 1.00, description 0.60, body 0.30**.
- BM25 requires persistent storage; `--no-db` with `--mode bm25` is an error.
- In `--json` mode an empty result exits `1`.

```sh
weave search ~/notes "token rotation"
weave search ~/notes "oauth tokn" --mode fuzzy --top-k 5
weave search ~/notes "security" --mode keyword --json
weave search ~/notes "alpha beta" --mode bm25 --json
weave search ~/notes "token rotation" --rerank keywords --timeout 2s --json
```

#### Multiple bundle sources

Add sources with a repeatable `--source <alias=path>`; the positional path is always the first
source. Aliases default to the directory name. Search, list and audit merge deterministically
(`score`, then source, then id), targets accept `alias:id`, and ambiguity is an error. Graph and
links stay bundle-scoped.

```sh
weave search ~/notes --source work=~/work/notes "oauth"
weave show ~/notes --source work=~/work/notes work:runbook
weave ingest ~/notes --source work=~/work/notes --db ~/.weave/index.db
```

Spec: [Multiple Bundle Sources](docs/specs/multi-bundle.md).

#### `weave list`

List concepts, optionally filtered by frontmatter `type`, `tags` and `status`. Filters are
repeatable for `--type` and `--tag`; `--limit` caps the output (default `20`).

```sh
weave list ~/notes
weave list ~/notes --type Concept --tag security
weave list ~/notes --status draft --limit 5 --json
```

#### `weave show`

Show one concept by bundle ID or file path, including type, status, tags, description, backlinks,
outgoing links and body. Encrypted bodies are redacted unless `--decrypt` resolves the key.

```sh
weave show ~/notes concepts/oauth-token-rotation
weave show ~/notes oauth-token-rotation.md --json
weave show ~/notes oauth-token-rotation --decrypt --json
```

#### `weave backlinks`

List the concepts that link to the given concept, showing the source ID and title.

```sh
weave backlinks ~/notes concepts/token-storage
weave backlinks ~/notes concepts/token-storage --json
```

#### `weave path`

Find the shortest path between two concepts with breadth-first search and print the hop sequence.

```sh
weave path ~/notes concepts/oauth-token-rotation concepts/token-storage
weave path ~/notes concepts/oauth-token-rotation concepts/token-storage --json
```

#### `weave audit`

Audit a bundle as a knowledge base: conformance failures, missing keywords, broken links, orphan
concepts and index gaps. Intended as a CI quality gate.

```sh
weave audit ~/notes --fail-on error --json
weave audit ~/notes --fail-on warning
weave audit ~/notes --verify-keys --json
```

Spec: [Weave Audit](docs/specs/weave-audit.md).

#### `weave relate`

Return the bounded graph neighborhood of a concept by depth and direction, as a deterministic
subgraph.

```sh
weave relate ~/notes concepts/okf-bundle --depth 2 --direction both --json
weave relate ~/notes concepts/okf-bundle --depth 1 --direction out
```

Spec: [Weave Relate](docs/specs/weave-relate.md).

#### `weave explain`

Explain a search result: matched keywords, per-field fuzzy contributions, BM25 lexical evidence and
the hybrid score breakdown.

```sh
weave explain ~/notes "oauth tokn" --json
weave explain ~/notes oauth --id concepts/oauth-token-rotation --mode keyword
weave explain ~/notes "alpha beta" --mode bm25 --id concepts/alpha --json
```

Spec: [Weave Explain](docs/specs/weave-explain.md).

#### `weave ingest`

Load a bundle into the default bundle-scoped database at `~/.weave/index.db`, or into `--db <file>`.
Ingest defaults to permissive; use `--strict` to reject invalid OKF documents. Bodies are compressed
with zstd unless `--codec lz4|none` is given.

```sh
weave ingest ~/notes --permissive --json
weave ingest ~/notes --strict --json
weave ingest ~/notes --strict --db /tmp/notes.db --json
weave ingest ~/notes --strict --codec lz4 --json
weave ingest ~/notes --strict --with-enricher keywords --json
```

Specs: [Storage & Sync](docs/specs/storage-and-sync.md),
[Permissive Ingestion](docs/specs/permissive-ingestion.md), [Compression](docs/specs/compression.md).

#### `weave rebuild`

Re-index one bundle's rows without touching other bundles in the shared database. `--codec`
re-encodes every body, so it is also the codec migration path.

```sh
weave rebuild ~/notes --strict --json
weave rebuild ~/notes --codec zstd --json
```

Specs: [Storage & Sync](docs/specs/storage-and-sync.md), [Compression](docs/specs/compression.md).

#### `weave sync`

Incrementally update one bundle from SHA-256 content hashes: add, update, remove and skip documents,
with cascading edge reconciliation. Updated documents keep their stored body codec unless `--codec`
is given.

```sh
weave sync ~/notes --json
weave sync ~/notes --strict --json
weave sync ~/notes --db /tmp/notes.db --json
weave sync ~/notes --codec lz4 --json
weave sync ~/notes --with-enricher keywords --json
weave sync ~/notes --db ~/.weave/index.db --watch --json --events
```

`--watch` requires an indexed bundle and an explicit `--db`. The first sync prints one result, then
the process polls every 500 ms; with `--json --events`, later syncs are emitted as JSONL events.
SIGINT/SIGTERM exits cleanly.

Specs: [Storage & Sync](docs/specs/storage-and-sync.md), [Watch Mode](docs/specs/watch-mode.md).

#### `weave export`

Reconstruct bundle files from the index.

```sh
weave export ~/notes --out ./export --format raw --yes --json
weave export ~/notes --out ./export --format encrypted --key-id export-2026 --json
```

- `raw` writes plaintext after confirmation (`--yes` for automation) and reports tokenized/hashed
  documents that cannot be reversed. Missing keys abort before writing.
- `encrypted` re-encrypts every body with `--key-id` and sets `privacy.encoded: true`; the result is a
  re-ingestable OKF bundle that never exposes plaintext.
- Targets may not overlap the source; non-empty targets require `--force`.
- `weave-export.json` is written last as the completion marker.

Spec: [Bundle Export](docs/specs/bundle-export.md).

#### Privacy directives

Declare per-document policies in frontmatter or protect inline spans with HTML comments. Weave
encrypts with AES-256-GCM, tokenizes with HMAC-SHA256 aliases or hashes normalized text; encrypted
documents are excluded from search, and tokenized/hashed content stays searchable through
deterministic aliases.

```markdown
---
type: runbook
keywords: [infra]
privacy:
  policy: encrypt
  provider: aes-256-gcm
  key_id: ops-prod-key
---
```

```markdown
Root token: <!-- weave:encrypt key_id="ops-prod-key" -->sec_live_994821a8f9<!-- /weave:encrypt -->
```

Keys resolve from `WEAVE_KEY_<KEY_ID>` then `.weave/security.yaml`; key material is never stored.

```sh
WEAVE_KEY_OPS_PROD_KEY=... weave ingest ~/notes --db ~/.weave/index.db --strict
weave show ~/notes secret --decrypt --json
weave audit ~/notes --verify-keys --json
```

Spec: [Privacy Directives](docs/specs/privacy-directives.md).

#### External crypto providers

The built-in `local` provider reads keys from `WEAVE_KEY_*` and `.weave/security.yaml`. External
providers are `weave-crypto-<name>` extensions selected by `.weave/crypto.yaml` or `--provider`:

```yaml
# .weave/crypto.yaml
default_provider: vault
providers:
  vault:
    binary: weave-crypto-vault
```

```sh
weave ingest ~/notes --db ~/.weave/index.db --provider vault
weave show ~/notes secret --db ~/.weave/index.db --decrypt --provider vault
weave audit ~/notes --db ~/.weave/index.db --verify-keys
```

Providers receive key ids only, return ciphertext and key versions, and are validated against their
`--capabilities` handshake. Failures abort writes with `PRIVACY_KEY_UNAVAILABLE` (exit `3`).

Spec: [External Crypto Providers](docs/specs/external-crypto-providers.md).

#### `weave extensions`

List executable `weave-<name>` extensions from `<path>/.weave/bin/` and `$PATH`. Enrichers run
during writes with `--with-enricher`, rerankers during search with `--rerank`; both speak
`weave.cli/v1` JSON over stdin/stdout.

```sh
weave extensions list ~/notes --json
weave ingest ~/notes --db ~/.weave/index.db --with-enricher keywords --json
weave search ~/notes "token rotation" --rerank keywords --json
```

- Bundle-local extensions win over `$PATH` entries with the same name.
- Enrichment failures add a warning and keep core results; writes still succeed.
- Rerank blends `0.5 * core + 0.5 * rerank` and falls back to core order on failure.
- Default timeout is `5s`; override with `--timeout`.

Spec: [Extension Subprocess IPC](docs/specs/extension-ipc.md).

#### `weave-ai`

The reference AI extension enriches writes and reranks searches. Build it into `$PATH` or the
bundle's `.weave/bin/`, then use it like any extension:

```sh
go build -o "$HOME/.local/bin/weave-ai" ./cmd/weave-ai
weave ingest ~/notes --db ~/.weave/index.db --with-enricher ai
weave search ~/notes "token rotation" --rerank ai
```

- Providers: `ollama` (default, `http://127.0.0.1:11434`), `openai`, `gemini`.
- Config: `<bundle>/.weave/ai.yaml` or `WEAVE_AI_CONFIG`; env overrides `WEAVE_AI_*`.
- Core redacts encrypted/tokenized/hashed bodies before IPC; `privacy.ai.send: true` opts a
  document in to sending its body.
- Model or provider failures degrade to core results with a warning.

Spec: [Weave AI Extension](docs/specs/weave-ai-plugin.md).

#### `weave mcp`

Serve read-only MCP tools and resources over stdio or TCP, using in-memory or persistent storage.
`--watch` keeps the persistent index current so tool and resource reads see edits without a restart.

```sh
weave mcp --bundle ~/notes --stdio
weave mcp --bundle ~/notes --listen 127.0.0.1:7331 --token secret
weave mcp --bundle ~/notes --db ~/.weave/index.db
weave mcp --bundle ~/notes --source work=~/work/notes --db ~/.weave/index.db --watch
```

- `--source` adds bundle sources; tools accept an optional `source` alias and resources expose
  `weave://source/<alias>/...`.
- Encrypted bodies are redacted by default; `--allow-sensitive` decrypts `weave_show` and concept
  resources without making encrypted documents searchable.
- Without `source`, tools merge sources and reject ambiguous targets.

Specs: [MCP Server](docs/specs/mcp-server.md), [Watch Mode](docs/specs/watch-mode.md),
[Multiple Bundle Sources](docs/specs/multi-bundle.md).

#### `weave version`

Print the binary version and the contract it targets.

```sh
weave version
weave version --json
```

### Retired commands

`weave mount`, `weave unmount` and `weave status` were retired with the move to a CLI-first,
local-first core. They are not planned for v1.0. See
[ADR 0003](docs/architecture/adr/0003-keyword-fuzzy-search-and-ui-retirement.md).

## OKF Bundles

Weave consumes OKF v0.2 bundles. The Weave profile requires:

```md
---
type: Concept
keywords:
  - authentication
  - oauth
  - token-rotation
title: "OAuth Token Rotation"
description: "How tokens rotate."
---

See [Token Storage](./token-storage.md).
```

- `type` and `keywords` are required.
- Missing keywords make the document invalid for keyword search, but it remains available for
  graph navigation and fuzzy search.
- Graph edges come from bundle-relative Markdown links and the `related` frontmatter array.

## Development

```sh
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/weave
go run ./cmd/docs-lint
./scripts/verify.sh
```

Regenerate the contract goldens:

```sh
go run ./cmd/contract-export
```

Calibration corpus: `engine/search/testdata/calibration.json`.

## Project Structure

```text
cmd/weave/              CLI
cmd/contract-export/    golden corpus generator
cmd/release-package/    release archive and checksum builder
cmd/weave-ai/           reference AI enrichment and rerank extension
contract/v1/            schemas, Go types, fixtures
engine/okf/             bundle parser, links, validation, cache
engine/graph/           nodes, edges, traversal
engine/search/          keyword, fuzzy and hybrid ranking
engine/audit/           bundle conformance, graph and index-quality findings
engine/relate/          bounded depth/direction graph neighborhoods
storage/sqlite/          bundle-scoped persistent storage and incremental sync
engine/privacy/          directive parsing, key resolution and crypto providers
engine/compression/      zstd and LZ4 body codecs
engine/extensions/       weave-<name> discovery and weave.cli/v1 transport
release/                 release archives, checksums and build info
mcp/                    MCP tool descriptors, resources and in-process dispatch
watch/                  polling watcher over incremental sync
engine/nav/             list, show, backlinks, path
engine/operations/      transport-neutral envelopes
docs/                   architecture, specs, knowledge
```

## Documentation

- [Roadmap](ROADMAP.md)
- [Architecture](docs/architecture/index.md)
- [Keyword + Fuzzy Search spec](docs/specs/keyword-fuzzy-search.md)
- [Weave Audit spec](docs/specs/weave-audit.md)
- [Weave Relate spec](docs/specs/weave-relate.md)
- [Weave Explain spec](docs/specs/weave-explain.md)
- [Storage & Sync spec](docs/specs/storage-and-sync.md)
- [Watch Mode spec](docs/specs/watch-mode.md)
- [Privacy Directives spec](docs/specs/privacy-directives.md)
- [Compression spec](docs/specs/compression.md)
- [Extension Subprocess IPC spec](docs/specs/extension-ipc.md)
- [Release Automation spec](docs/specs/release-automation.md)
- [Bundle Export spec](docs/specs/bundle-export.md)
- [Multiple Bundle Sources spec](docs/specs/multi-bundle.md)
- [Performance and Maintainability spec](docs/specs/optimization.md)
- [Configuration spec](docs/specs/configuration.md)
- [BM25 Ranking spec](docs/specs/bm25-ranking.md)
- [MCP Server spec](docs/specs/mcp-server.md)
- [MCP Sensitive Access spec](docs/specs/mcp-sensitive.md)
- [ADR 0003: search and UI retirement](docs/architecture/adr/0003-keyword-fuzzy-search-and-ui-retirement.md)
- [Backlog](docs/backlog.md)
