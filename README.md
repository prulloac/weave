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
- **Permissive ingestion**: index non-OKF Markdown with deterministic synthetic metadata and warnings.
- **Incremental sync**: SHA-256 add/update/remove/skip with cascade reconciliation.
- **Deterministic** JSON envelopes for machine integration.
- **Calibrated ranking** against a labeled query corpus.

## Requirements

- Go 1.27+

## Build

```sh
go build -o weave ./cmd/weave
```

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
- `--help` (or `-h`) prints the command summary; `--version` (or `-v`) prints the binary version.
- Diagnostics go to stderr.
- Exit codes: `0` success, `1` runtime error or no match, `2` usage or invalid parameters,
  `3` permission denied.

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
- Fuzzy field weights: **title 1.00, description 0.60, body 0.30**.
- BM25 requires persistent storage; `--no-db` with `--mode bm25` is an error.
- In `--json` mode an empty result exits `1`.

```sh
weave search ~/notes "token rotation"
weave search ~/notes "oauth tokn" --mode fuzzy --top-k 5
weave search ~/notes "security" --mode keyword --json
weave search ~/notes "alpha beta" --mode bm25 --json
```

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
outgoing links and body.

```sh
weave show ~/notes concepts/oauth-token-rotation
weave show ~/notes oauth-token-rotation.md --json
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
Ingest defaults to permissive; use `--strict` to reject invalid OKF documents.

```sh
weave ingest ~/notes --permissive --json
weave ingest ~/notes --strict --json
weave ingest ~/notes --strict --db /tmp/notes.db --json
```

Specs: [Storage & Sync](docs/specs/storage-and-sync.md),
[Permissive Ingestion](docs/specs/permissive-ingestion.md).

#### `weave rebuild`

Re-index one bundle's rows without touching other bundles in the shared database.

```sh
weave rebuild ~/notes --strict --json
```

Spec: [Storage & Sync](docs/specs/storage-and-sync.md).

#### `weave sync`

Incrementally update one bundle from SHA-256 content hashes: add, update, remove and skip documents,
with cascading edge reconciliation.

```sh
weave sync ~/notes --json
weave sync ~/notes --strict --json
weave sync ~/notes --db /tmp/notes.db --json
```

Spec: [Storage & Sync](docs/specs/storage-and-sync.md). Watch mode is the next slice.

#### `weave mcp`

Serve read-only MCP tools over stdio or TCP, using in-memory or persistent storage.

```sh
weave mcp --bundle ~/notes --stdio
weave mcp --bundle ~/notes --listen 127.0.0.1:7331 --token secret
weave mcp --bundle ~/notes --db ~/.weave/index.db
```

Spec: [MCP Server](docs/specs/mcp-server.md).

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
contract/v1/            schemas, Go types, fixtures
engine/okf/             bundle parser, links, validation, cache
engine/graph/           nodes, edges, traversal
engine/search/          keyword, fuzzy and hybrid ranking
engine/audit/           bundle conformance, graph and index-quality findings
engine/relate/          bounded depth/direction graph neighborhoods
storage/sqlite/          bundle-scoped persistent storage and incremental sync
mcp/                    MCP tool descriptors and in-process dispatch
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
- [BM25 Ranking spec](docs/specs/bm25-ranking.md)
- [MCP Server spec](docs/specs/mcp-server.md)
- [ADR 0003: search and UI retirement](docs/architecture/adr/0003-keyword-fuzzy-search-and-ui-retirement.md)
- [Backlog](docs/backlog.md)
