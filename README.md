# Weave

> To weave flat files into a living universe of knowledge.

Weave is a **local-first related-document database** for
[Open Knowledge Format (OKF)](#okf-bundles) bundles. It loads Markdown/YAML documents, builds
concept nodes and link edges, and answers keyword, BM25 and weighted fuzzy searches with top-K
results. Optional SQLite persistence, privacy directives, subprocess extensions and an MCP server
are part of the same CLI-first product.

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
- **Body compression**: text-readable bodies stored raw; binary blobs zstd by default, with per-row codecs, codec extensions and corruption recovery.
- **Permissive ingestion**: index non-OKF Markdown with deterministic synthetic metadata and warnings.
- **Incremental sync**: SHA-256 add/update/remove/skip with cascade reconciliation.
- **Privacy directives**: declared encrypt, tokenize and hash policies with key resolution.
- **Extensions**: `weave-<name>` enrichment during writes and reranking during search.
- **AI extension**: optional [`weave-ai`](https://github.com/prulloac/weave-ai) with Ollama,
  OpenAI-compatible and Gemini providers.
- **Deterministic** JSON envelopes for machine integration.
- **Calibrated ranking** against a labeled query corpus.
- **Reproducible benchmarks**: deterministic 100/1K/10K corpus, `cmd/bench` and committed baselines.

## Requirements

- Go 1.27+ to build from source; released binaries do not need Go.

## Install

User-level installation, no admin or sudo required. Linux and macOS:

```sh
curl -fsSL https://raw.githubusercontent.com/prulloac/weave/main/scripts/install.sh | sh
```

Windows PowerShell:

```powershell
irm https://raw.githubusercontent.com/prulloac/weave/main/scripts/install.ps1 | iex
```

The installer downloads the latest release, verifies its SHA-256 checksum and installs to
`~/.local/bin` (Linux/macOS) or `%LOCALAPPDATA%\Programs\weave\bin` (Windows), then adds that
directory to your user PATH.

For a managed or non-standard environment, choose a user-writable directory and skip the profile
edit:

```sh
curl -fsSL https://raw.githubusercontent.com/prulloac/weave/main/scripts/install.sh \
  | sh -s -- --bin-dir "$HOME/bin" --no-modify-path
```

Options: `--version <tag>`, `--bin-dir <dir>`, `--no-modify-path`; environment equivalents
`WEAVE_VERSION`, `WEAVE_BIN_DIR` and `WEAVE_NO_MODIFY_PATH=1`. The PowerShell script accepts
`-Version`, `-BinDir` and `-NoPath`.

`go install github.com/prulloac/weave/cmd/weave@latest` is also available for Go users, and every
release includes downloadable archives. The installer never requests administrator rights.

## Build

```sh
go build -o weave ./cmd/weave
```

Releases are produced by `.github/workflows/release.yml`: an annotated `v*` tag verifies the
repository, builds six static targets, writes sorted checksums and creates a draft GitHub Release.
`workflow_dispatch` performs a build-only dry run. See the
[release automation spec](https://github.com/prulloac/weave-docs/blob/main/specs/release-automation.md).

## Quickstart

```sh
weave ingest ~/notes --strict
weave search ~/notes "token rotation"
```

That is the two-command path from a clean install to first results. `weave config init` is optional
and writes the global defaults with a next-step hint.

## Usage

```sh
weave <command> [arguments] [flags]
```

Global flags:

- `--json` emits one `weave.cli/v1` envelope on stdout.
- `--quiet` prints errors only; `--verbose` adds bundle, source, database, cache and timing
  diagnostics to stderr. They are mutually exclusive.
- `--ascii` replaces `→` and `·` with ASCII; `NO_COLOR` is accepted even though Weave is monochrome
  by default. Progress lines appear only on a TTY and never with `--json`.
- `--no-cache` disables the parse cache, which is enabled by default.
- `--no-db` forces transient in-memory parsing; otherwise reads use `~/.weave/index.db`.
- `--db <file>` selects another database; it is mutually exclusive with `--no-db`.
- `--strict` / `--permissive` select ingest mode; ingest defaults to permissive.
- `--codec zstd|none|<name>` selects compression for binary blobs in ingest, rebuild and sync; default
  `zstd`. Text-readable content is always stored raw. LZ4 and other codecs require a
  `weave-codec-<name>` extension; `weave codecs list` reports installed codecs.
- `--assets` enables blob scanning; `--asset-types image/png,text/*` selects media types and is
  persisted for later sync/rebuild runs; `--no-assets` drops blob rows.
- `weave show <id> --out <file>` writes exact document or blob bytes; add `--force` to replace.
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
weave config paths --json         # config, DB, cache, keys and extensions
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
  it. XDG state migration is deferred (see the
  [configuration-path decision](https://github.com/prulloac/weave-docs/blob/main/architecture/adr/0002-config-paths-and-db-location.md)).
- `config show` and `config get` report every value with `source` and `origin`; `config paths`
  names the config, database, cache, key and extension locations.
- Secrets are never read from configuration; keys stay in `WEAVE_KEY_*` or `.weave/security.yaml`.
- Invalid or reserved keys exit `2`; unknown keys warn unless `--strict-config`.

Spec: [Configuration](https://github.com/prulloac/weave-docs/blob/main/specs/configuration.md).

### Shell completion

```sh
weave completion bash > ~/.local/share/bash-completion/completions/weave
weave completion zsh > "${fpath[1]}/_weave"
weave completion fish > ~/.config/fish/completions/weave.fish
```

The scripts are generated from the command table; `weave` with no arguments prints a short
start-here screen.

### Local-first guarantees

Weave has **no telemetry and makes no network calls by default**. Core parsing, indexing, search,
navigation, persistence and export are local.

Network access is opt-in and isolated to extensions you install:

- `weave-ai` calls the model endpoint you configure only when an AI enrichment or rerank is
  requested; documents are sent only when the `privacy.ai.send` directive opts in.
- Other `weave-<name>` extensions run as local subprocesses; a third-party extension may make its
  own network calls, so review it before installation.
- `weave config paths` lists the config, database, cache, key-search and extension paths.
- Privacy errors name the provider and key id when available; key material is never printed.

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

Spec: [Multiple Bundle Sources](https://github.com/prulloac/weave-docs/blob/main/specs/multi-bundle.md).

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

Spec: [Weave Audit](https://github.com/prulloac/weave-docs/blob/main/specs/weave-audit.md).

#### `weave relate`

Return the bounded graph neighborhood of a concept by depth and direction, as a deterministic
subgraph.

```sh
weave relate ~/notes concepts/okf-bundle --depth 2 --direction both --json
weave relate ~/notes concepts/okf-bundle --depth 1 --direction out
```

Spec: [Weave Relate](https://github.com/prulloac/weave-docs/blob/main/specs/weave-relate.md).

#### `weave explain`

Explain a search result: matched keywords, per-field fuzzy contributions, BM25 lexical evidence and
the hybrid score breakdown.

```sh
weave explain ~/notes "oauth tokn" --json
weave explain ~/notes oauth --id concepts/oauth-token-rotation --mode keyword
weave explain ~/notes "alpha beta" --mode bm25 --id concepts/alpha --json
```

Spec: [Weave Explain](https://github.com/prulloac/weave-docs/blob/main/specs/weave-explain.md).

#### `weave ingest`

Load a bundle into the default bundle-scoped database at `~/.weave/index.db`, or into `--db <file>`.
Ingest defaults to permissive; use `--strict` to reject invalid OKF documents. Text-readable bodies
are stored raw; binary blobs use zstd unless `--codec none|<name>` is given.

```sh
weave ingest ~/notes --permissive --json
weave ingest ~/notes --strict --json
weave ingest ~/notes --strict --db /tmp/notes.db --json
weave ingest ~/notes --strict --with-enricher keywords --json
```

Specs: [Storage & Sync](https://github.com/prulloac/weave-docs/blob/main/specs/storage-and-sync.md),
[Permissive Ingestion](https://github.com/prulloac/weave-docs/blob/main/specs/permissive-ingestion.md), [Compression](https://github.com/prulloac/weave-docs/blob/main/specs/compression.md).

#### `weave rebuild`

Re-index one bundle's rows without touching other bundles in the shared database. `--codec`
re-encodes binary blobs, so it is also the codec migration path; text-readable bodies stay raw.

```sh
weave rebuild ~/notes --strict --json
weave rebuild ~/notes --codec zstd --json
```

Specs: [Storage & Sync](https://github.com/prulloac/weave-docs/blob/main/specs/storage-and-sync.md), [Compression](https://github.com/prulloac/weave-docs/blob/main/specs/compression.md).

#### `weave sync`

Incrementally update one bundle from SHA-256 content hashes: add, update, remove and skip documents,
with cascading edge reconciliation. Updated binary documents keep their stored body codec unless
`--codec` is given; text-readable bodies stay raw.

```sh
weave sync ~/notes --json
weave sync ~/notes --strict --json
weave sync ~/notes --db /tmp/notes.db --json
weave sync ~/notes --codec zstd --json
weave sync ~/notes --with-enricher keywords --json
weave sync ~/notes --db ~/.weave/index.db --watch --json --events
```

`--watch` requires an indexed bundle and an explicit `--db`. The first sync prints one result, then
the process polls every 500 ms; with `--json --events`, later syncs are emitted as JSONL events.
SIGINT/SIGTERM exits cleanly.

Specs: [Storage & Sync](https://github.com/prulloac/weave-docs/blob/main/specs/storage-and-sync.md), [Watch Mode](https://github.com/prulloac/weave-docs/blob/main/specs/watch-mode.md).

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

Spec: [Bundle Export](https://github.com/prulloac/weave-docs/blob/main/specs/bundle-export.md).

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

Spec: [Privacy Directives](https://github.com/prulloac/weave-docs/blob/main/specs/privacy-directives.md).

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

Spec: [External Crypto Providers](https://github.com/prulloac/weave-docs/blob/main/specs/external-crypto-providers.md).

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

Spec: [Extension Subprocess IPC](https://github.com/prulloac/weave-docs/blob/main/specs/extension-ipc.md).

#### `weave-ai`

The reference AI extension lives in [`prulloac/weave-ai`](https://github.com/prulloac/weave-ai).
It enriches writes and reranks searches. Install it with
`go install github.com/prulloac/weave-ai@latest` or a release archive, put it on `$PATH` or the
bundle's `.weave/bin/`, then use it like any extension:

```sh
weave-ai --describe              # provider, endpoint and consent; never the key
weave ingest ~/notes --db ~/.weave/index.db --with-enricher ai
weave search ~/notes "token rotation" --rerank ai
```

- Providers: `ollama` (default, `http://127.0.0.1:11434`), `openai`, `gemini`.
- Config: `<bundle>/.weave/ai.yaml` or `WEAVE_AI_CONFIG`; env overrides `WEAVE_AI_*`.
- Core redacts encrypted/tokenized/hashed bodies before IPC; `privacy.ai.send: true` opts a
  document in to sending its body.
- `weave-ai --describe` prints the provider, model, endpoint and consent rule; it never prints the
  API key value.
- Model or provider failures degrade to core results with a warning.

Spec: [Weave AI Extension](https://github.com/prulloac/weave-docs/blob/main/specs/weave-ai-plugin.md).

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

Specs: [MCP Server](https://github.com/prulloac/weave-docs/blob/main/specs/mcp-server.md), [Watch Mode](https://github.com/prulloac/weave-docs/blob/main/specs/watch-mode.md),
[Multiple Bundle Sources](https://github.com/prulloac/weave-docs/blob/main/specs/multi-bundle.md).

#### `weave version`

Print the binary version and the contract it targets.

```sh
weave version
weave version --json
```

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
go test ./cmd/weave -run '^TestSmoke$'
go test ./engine/okf -run '^$' -fuzz FuzzExtractFrontmatter -fuzztime=5s
go build ./cmd/weave
go run ./cmd/docs-lint
./scripts/verify.sh
```

Project documentation (roadmap, backlog, specs, architecture, ADRs) lives in the sibling
[weave-docs](https://github.com/prulloac/weave-docs) repository; run its
[documentation checks](https://github.com/prulloac/weave-docs/blob/main/AGENTS.md) there.

Regenerate the contract goldens:

```sh
go run ./cmd/contract-export
```

Calibration corpus: `engine/search/testdata/calibration.json`.

Benchmarks:

```sh
go run ./benchmarks/corpus/gen -out benchmarks/corpus
go build -o /tmp/weave ./cmd/weave
go run ./cmd/bench -corpus benchmarks/corpus/10k -scenario all -weave /tmp/weave
```

See the [performance baseline](https://github.com/prulloac/weave-docs/blob/main/architecture/performance.md) and
[benchmark harness](benchmarks/README.md).

## Project Structure

```text
cmd/weave/              CLI
cmd/bench/              end-to-end benchmark harness
cmd/contract-export/    golden corpus generator
cmd/schema-diff/        additive-only schema check
cmd/release-package/    release archive and checksum builder
benchmarks/             corpus generator, registry, queries, baselines
contract/v1/            schemas, Go types, fixtures
engine/okf/             bundle parser, links, validation, cache
engine/graph/           nodes, edges, traversal
engine/search/          keyword, fuzzy and hybrid ranking
engine/audit/           bundle conformance, graph and index-quality findings
engine/relate/          bounded depth/direction graph neighborhoods
storage/sqlite/          bundle-scoped persistent storage and incremental sync
engine/privacy/          directive parsing, key resolution and crypto providers
engine/compression/      zstd body codec and per-row compression policy
engine/extensions/       weave-<name> discovery and weave.cli/v1 transport
release/                 release archives, checksums and build info
mcp/                    MCP tool descriptors, resources and in-process dispatch
watch/                  polling watcher over incremental sync
engine/nav/             list, show, backlinks, path
engine/operations/      transport-neutral envelopes
docs/                   user guides and BDD behavior scenarios
```

Project documentation (roadmap, backlog, specifications, architecture, ADRs, knowledge and style)
lives in the sibling `weave-docs` repository.

## Documentation

User documentation lives in this repository; project documentation lives in
[weave-docs](https://github.com/prulloac/weave-docs).

- [Documentation index](docs/README.md)
- [Specifications](https://github.com/prulloac/weave-docs/blob/main/specs/README.md)
- [Behavior scenarios](docs/bdd/README.md)
- [Architecture](https://github.com/prulloac/weave-docs/blob/main/architecture/index.md)
- [Architecture decisions](https://github.com/prulloac/weave-docs/blob/main/architecture/adr/README.md)
- [Roadmap](https://github.com/prulloac/weave-docs/blob/main/ROADMAP.md)
- [Backlog](https://github.com/prulloac/weave-docs/blob/main/backlog.md)

## License

Weave is released under the [MIT License](LICENSE).
