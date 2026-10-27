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
weave contract --json
weave parse <path> --json
weave graph <path> --json
weave search <path> <query> --mode hybrid --top-k 10 --json
weave list <path> --type Concept --tag security --json
weave show <path> <id|file> --json
weave backlinks <path> <id|file> --json
weave path <path> <from> <to> --json
weave version --json
```

- `--json` emits one `weave.cli/v1` envelope on stdout.
- diagnostics go to stderr.
- exit codes: `0` success, `1` runtime/no match, `2` usage/invalid parameters, `3` permission.
- the parse cache is enabled by default; `--no-cache` disables it.

### Search modes

```sh
weave search ~/notes "oauth tokn" --mode fuzzy --top-k 5
weave search ~/notes "security" --mode keyword
weave search ~/notes "token rotation" --mode hybrid   # default
```

Fuzzy field weights: **title 1.00, description 0.60, body 0.30**. Hybrid combines keyword and
fuzzy scores at 50/50 (calibrated).

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
