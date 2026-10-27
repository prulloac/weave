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
- **Graph navigation**: show, backlinks and shortest path.
- **Bundle audit**: conformance, link, orphan and index-quality findings.
- **In-memory** indexes; v1.0 adds optional SQLite persistence.
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

Rank matching concepts with keyword, fuzzy or hybrid search. Keyword uses exact and prefix matching
with AND semantics; fuzzy tolerates typos; hybrid combines both scores at 50/50 (default).

- `--mode hybrid|keyword|fuzzy` — default `hybrid`.
- `--top-k <n>` — default `10`, valid range `1..100`.
- Fuzzy field weights: **title 1.00, description 0.60, body 0.30**.
- In `--json` mode an empty result exits `1`.

```sh
weave search ~/notes "token rotation"
weave search ~/notes "oauth tokn" --mode fuzzy --top-k 5
weave search ~/notes "security" --mode keyword --json
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

#### `weave version`

Print the binary version and the contract it targets.

```sh
weave version
weave version --json
```

### Planned commands

> **Not yet shipped.** The commands below are specified for v1.0 but are yet to be shipped. They are
> not available in the current binary. Flags that depend on v1.0 features, such as
> `weave show --decrypt` and `weave search --rerank`, are also not yet shipped.

#### `weave relate`

Return the bounded graph neighborhood of a concept by depth and direction, as a deterministic
subgraph.

```sh
weave relate ~/notes concepts/okf-bundle --depth 2 --direction both --json
```

Spec: [Weave Relate](docs/specs/weave-relate.md).

#### `weave explain`

Explain a search result: matched keywords, per-field fuzzy contributions and the hybrid score
breakdown.

```sh
weave explain ~/notes "oauth tokn" --mode hybrid --json
```

Spec: [Weave Explain](docs/specs/weave-explain.md).

#### `weave ingest`

Load a bundle into persistent SQLite storage, optionally permissively for non-OKF Markdown trees.

```sh
weave ingest ~/notes --permissive --json
```

Spec: [Storage & Sync](docs/specs/storage-and-sync.md),
[Permissive Ingestion](docs/specs/permissive-ingestion.md).

#### `weave sync`

Incrementally update the persistent index from content hashes, with optional watch mode and
enrichment or crypto extensions.

```sh
weave sync ~/notes --json
weave sync ~/notes --watch
```

Specs: [Storage & Sync](docs/specs/storage-and-sync.md), [Watch Mode](docs/specs/watch-mode.md),
[Privacy & Extensions](docs/specs/privacy-and-extensions.md).

#### `weave rebuild`

Re-encode the persistent index, for example after a compression codec change.

```sh
weave rebuild ~/notes --json
```

Spec: [Compression](docs/specs/compression.md).

#### `weave mcp`

Start the read-only MCP server over the operations layer, in-memory or backed by persistent storage.

```sh
weave mcp --bundle ~/notes
weave mcp --bundle ~/notes --db .weave/index.db --watch
```

Spec: [MCP Server](docs/specs/mcp-server.md).

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
engine/nav/             list, show, backlinks, path
engine/operations/      transport-neutral envelopes
docs/                   architecture, specs, knowledge
```

## Documentation

- [Roadmap](ROADMAP.md)
- [Architecture](docs/architecture/index.md)
- [Keyword + Fuzzy Search spec](docs/specs/keyword-fuzzy-search.md)
- [ADR 0003: search and UI retirement](docs/architecture/adr/0003-keyword-fuzzy-search-and-ui-retirement.md)
- [Backlog](docs/backlog.md)
