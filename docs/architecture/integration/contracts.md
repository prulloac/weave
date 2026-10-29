---
type: Integration Contract
title: "Integration Contracts"
description: "CLI-first contracts for documents with mandatory keywords, graph navigation and keyword/fuzzy/hybrid search."
tags:
  - "architecture"
  - "integration"
  - "cli"
  - "search"
generated:
  by: opencode/deepseek-v4.1-flash
  at: "2026-10-25T00:00:00Z"
status: draft
---

# Integration Contracts

The primary integration contract is **CLI I/O**. There is no HTTP, DOM or presentation contract.
The UI is retired by [ADR 0003](../adr/0003-keyword-fuzzy-search-and-ui-retirement.md).

## 1. CLI Contract

### Commands

```sh
weave contract --json
weave ingest <path> [--db <file>] [--strict|--permissive] --json
weave rebuild <path> [--db <file>] [--strict|--permissive] --json
weave parse <path> --json
weave graph <path> --json
weave search <path> <query> [--mode hybrid|keyword|fuzzy] [--top-k <n>] --json
weave list <path> [--type <t>] [--tag <t>] [--status <s>] [--limit <n>] --json
weave show <path> <id|file> --json
weave backlinks <path> <id|file> --json
weave path <path> <id|file> <id|file> --json
weave version --json
```

Guarantees:

- stdout is one JSON envelope in `--json` mode.
- stderr is diagnostics.
- Exit codes: 0 success, 1 runtime/no match, 2 usage/invalid parameters, 3 permission.
- No network, daemon or UI.
- Bundle reads use `~/.weave/index.db` by default; `--no-db` forces in-memory parsing and
  `--db <file>` selects another database.
- An unindexed bundle returns `BUNDLE_NOT_INDEXED`; missing databases are `INVALID_REQUEST`.
- Parse cache enabled by default; `--no-cache` disables it.

### Envelope

Success:

```json
{
  "contract": "weave.contract/v1",
  "cli": "weave.cli/v1",
  "ok": true,
  "result": {}
}
```

Failure:

```json
{
  "contract": "weave.contract/v1",
  "cli": "weave.cli/v1",
  "ok": false,
  "error": {
    "code": "INVALID_REQUEST",
    "message": "top_k must be between 1 and 100",
    "details": {"top_k": 0}
  }
}
```

## 2. Document Contract

Concept documents require:

- `type`: non-empty string.
- `keywords`: non-empty list of non-empty strings.

Missing keywords produce `has_keywords: false` and `validation.valid: false`. The document remains
available for graph navigation and fuzzy search, but never appears in keyword results.

See [Contract layer](../layers/contract.md) for full fields.

## 3. Search Contract

`search` returns:

```json
{
  "query": "oauth tokn",
  "mode": "hybrid",
  "top_k": 10,
  "keyword": [],
  "fuzzy": [],
  "hybrid": []
}
```

- Top-K default 10; valid range 1..100.
- Keyword, fuzzy and hybrid lists are independently capped.
- Keyword exact = 1.0, prefix = 0.75, AND across query tokens.
- Fuzzy weights: title 1.00, description 0.60, body 0.30.
- Hybrid = `0.50 * keyword + 0.50 * fuzzy` (calibrated).
- Scores are deterministic and rounded to four decimals.
- `find` is removed.

## 4. Go Package API

Optional for Go hosts:

```go
eng, err := engine.Open(ctx, root, engine.Options{})
response, err := eng.Search("oauth", contractv1.SearchHybrid, 10)
```

Guarantees:

- `Open` parses once; queries are read-only and concurrency-safe.
- Search returns the same `SearchResponse` as the CLI.
- The CLI remains the compatibility promise; the Go API follows module semver.

## 5. Filesystem Contract

| Rule | Guarantee |
| --- | --- |
| Input | OKF v0.2 bundle directory |
| Required | `type` and `keywords` |
| Reserved | `index.md`, `log.md` |
| Encoding | UTF-8; invalid files reported and skipped |
| Symlinks | Files followed; directories one level with cycle guard |
| Writes | Never write into the target bundle |
| Cache | Optional, outside the target tree |
| Database | Default `~/.weave/index.db`, bundle-scoped; never written into the source bundle |

## Compatibility

| Component | Contract | CLI |
| --- | --- | --- |
| Go engine v1 | v1 | n/a |
| Go operations v1 | v1 | `weave.cli/v1` |
| Go CLI v1 | v1 | `weave.cli/v1` |
| Old TypeScript/UI | retired | removed |

## Related Documents

- [ADR 0003](../adr/0003-keyword-fuzzy-search-and-ui-retirement.md)
- [Contract layer](../layers/contract.md)
- [Operations layer](../layers/operations.md)
- [CLI layer](../layers/cli-go.md)
- [Engine layer](../layers/engine-go.md)
- [Keyword + Fuzzy Search spec](../../specs/keyword-fuzzy-search.md)
- [Integration testing](testing.md)
