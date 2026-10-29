---
type: specification
title: "Multiple Bundle Sources Specification"
description: "Read and write several bundle roots in one invocation with aliases, merged search and unambiguous document targeting."
tags:
  - "specification"
  - "bundles"
  - "cli"
  - "v1"
generated:
  by: opencode/deepseek-v4.1-flash
  at: "2026-10-29T00:00:00Z"
status: draft
---

# Specification: Multiple Bundle Sources

Status: **Proposed**
Target: **v1.0**

## 1. Overview

Today every command takes exactly one bundle path. This slice lets one invocation read or write
several bundle roots as data sources. Sources are ordinary OKF bundles; Weave does not merge their
files, deduplicate documents or traverse links across roots.

```sh
weave search ~/notes --source ~/work/notes --source team=~/team/notes "oauth"
weave show ~/notes --source team=~/team/notes team:runbook
weave ingest ~/notes --source team=~/team/notes --db ~/.weave/index.db
```

Non-goals:

- Cross-bundle graph traversal or link resolution.
- Remote, network or git sources.
- Document deduplication across sources.
- Merging source files on export (see [Bundle Export](./bundle-export.md)).
- Concurrent per-source parsing in this slice.

## 2. Contracts

### 2.1 Source syntax

- The first positional path remains the primary source, preserving current behavior.
- `--source <path>` adds a source; repeatable.
- `--source <alias>=<path>` assigns an explicit alias.
- Aliases match `[a-z0-9][a-z0-9-]*`; duplicates and reserved prefixes (`contract`, `weave`) are
  `INVALID_REQUEST`.
- Without an alias, the source alias is the directory base name, lower-cased and sanitized; collisions
  get `-2`, `-3` suffixes in argument order.
- Relative paths resolve against the working directory; each root is normalized to an absolute path.
- The same root may appear twice only under different aliases; duplicate aliases are rejected.
- `--source` with `--no-db` parses each root in memory.
- Commands that write (`ingest`, `rebuild`, `sync`, `export`) accept `--source` too, but
  `export` exports exactly one source per invocation (see its spec).

### 2.2 CLI surface

```sh
weave parse   <path> [--source <alias=path> ...] [--json]
weave graph   <path> [--source ...] [--json]
weave search  <path> [--source ...] <query> [flags] [--json]
weave list    <path> [--source ...] [flags] [--json]
weave show    <path> [--source ...] <alias:id|id> [--json]
weave backlinks <path> [--source ...] <alias:id|id> [--json]
weave path    <path> [--source ...] <alias:id|id> <alias:id|id> [--json]
weave relate  <path> [--source ...] <alias:id|id> [flags] [--json]
weave explain <path> [--source ...] <query> [--id <alias:id>] [flags] [--json]
weave audit   <path> [--source ...] [flags] [--json]
weave ingest|rebuild|sync <path> [--source ...] [flags] [--json]
```

- `--source` is valid only on the commands above; otherwise exit `2`.
- `parse` and `graph` return a bundle-scoped result per source; with `--json` the result is a list of
  source payloads, preserving the single-source shape when only one source is present.
- Positional path and `--source` are combined; the positional source is first.

### 2.3 Result identity

- Every hit and result item gains an optional `source` field carrying the alias.
- `id` stays bundle-local; JSON consumers must key on `(source, id)`.
- Human output prefixes `alias:` when more than one source is active.
- `source` is omitted for single-source invocations, keeping existing goldens stable.

| Result type | Field |
| --- | --- |
| `KeywordHit`, `FuzzyHit`, `HybridHit`, `LexicalHit` | `source` |
| `ListResult`, `ShowResult` | `source` |
| `GraphNode`, `RelateResult` root | `source` |
| `AuditFinding` | `source` |
| `PathResult` | `source` |

### 2.4 Target resolution

- Targets accept `alias:id` or `alias:path.md`.
- An unqualified target resolves when exactly one source contains it.
- An unqualified target present in more than one source fails with `INVALID_REQUEST` and lists the
  candidate aliases.
- An unknown alias is `INVALID_REQUEST`.
- `path` requires both endpoints to resolve; endpoints in different sources are `INVALID_REQUEST`
  (no cross-source traversal).

## 3. Behavior

### 3.1 Search merge

1. Run the selected mode against each source with the requested top-K.
2. Keep each source's normalized scores as produced by that source.
3. Merge all hits into one list ordered by `(score desc, source asc, id asc)`.
4. Truncate to top-K.
5. The response includes only the dimension for the selected mode, as today.

- `--mode bm25` runs one FTS query per stored source and merges normalized lexical scores.
- Hybrid persistent search merges each source's hybrid list; core weights are unchanged.
- `explain --id` targets one source; without `--id` the top merged hit is explained and its source is
  reported.
- Partial failure (one unreadable source) aborts the command with `INVALID_ROOT`; no partial results.

### 3.2 List, graph and audit

- `list` merges entries ordered by `(source asc, id asc)` and applies `--limit` after merging.
- `graph` returns per-source nodes and edges; `dangling` is aggregated. No cross-source edges.
- `audit` runs per source, concatenates findings ordered by `(source asc, code asc, concept asc)`,
  sums the summary counts and sets `ok` from the combined findings. `path` and `fail_on` stay
  top-level; each finding carries `source`.

### 3.3 Writes

- `ingest` and `rebuild` write each source to the selected database; one failure aborts the whole
  transaction set and no source is partially committed.
- `sync` syncs each source; every source must already be indexed, otherwise `BUNDLE_NOT_INDEXED`
  names the missing alias.
- `--codec` and `--with-enricher` apply to every source.
- Reports aggregate counts across sources and add `sources: [{source, documents, warnings, errors}]`.
- Warnings and errors from extensions name both alias and document id.

### 3.4 MCP

- `weave mcp --bundle <path> [--source <alias=path> ...]`.
- Tools gain an optional `source` argument; omitted means search all and require unambiguous targets.
- Resources extend to `weave://source/<alias>/...` and keep existing URIs for the primary source.
- Tool result payloads follow the same `source` fields and ambiguity rules.

## 4. Data Flow

```text
CLI sources
  -> source resolver (alias, absolute root, dedupe)
  -> per-source engine or store load
  -> per-source operation
  -> merge (search/list/audit) or per-source result (parse/graph)
  -> contract payloads with optional source fields
```

## 5. Validation and Errors

| Condition | Code | Behavior |
| --- | --- | --- |
| Invalid or duplicate alias | `INVALID_REQUEST` | Exit `2` |
| Missing or unreadable source | `INVALID_ROOT` | Exit `1`, no partial results |
| Unqualified target in several sources | `INVALID_REQUEST` | Exit `2`, candidates listed |
| Cross-source `path` endpoints | `INVALID_REQUEST` | Exit `2` |
| Sync source not indexed | `BUNDLE_NOT_INDEXED` | Exit `1`, alias named |
| `--source` on a single-source command | `INVALID_REQUEST` | Exit `2` |
| Duplicate root with different aliases | `INVALID_REQUEST` | Exit `2` |

## 6. Testing

- Alias derivation, collision suffixes and validation.
- Two- and three-source search merge ordering, top-K truncation and BM25/hybrid parity.
- Ambiguous `show`/`relate` targets and `alias:id` resolution.
- No cross-source path or link resolution.
- List limit after merge; audit summary sums and finding ordering.
- Multi-source ingest/sync reports and failure atomicity.
- `--no-db` multi-source parity with persistent mode.
- Single-source output unchanged (goldens untouched).
- MCP multi-source tools and resources.
- Race suite and calibration unchanged.

## 7. Acceptance Criteria

- One invocation can read and write several bundle roots with stable aliases.
- Search and list merge deterministically; scores are not re-weighted across sources.
- Ambiguous targets fail with an explanatory error instead of picking a source silently.
- Graph and link semantics stay bundle-scoped.
- Single-source output is byte-identical to the current contract.
- The full verification gate stays green.

## Related Documents

- [Storage & Sync](./storage-and-sync.md)
- [Bundle Export](./bundle-export.md)
- [Keyword + Fuzzy Search](./keyword-fuzzy-search.md)
- [BM25 Ranking](./bm25-ranking.md)
- [MCP Server](./mcp-server.md)
- [ROADMAP](../../ROADMAP.md)
