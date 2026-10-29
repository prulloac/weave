---
type: specification
title: "Extension Subprocess IPC Specification"
description: "Git-style weave-<name> subprocess protocol for enrichment and reranking over weave.cli/v1."
tags:
  - "specification"
  - "extensions"
  - "ipc"
  - "v1"
generated:
  by: opencode/deepseek-v4.1-flash
  at: "2026-10-28T00:00:00Z"
status: draft
---

# Specification: Extension Subprocess IPC

Status: **Implemented** (discovery, `weave.cli/v1` runner, enrichment and reranking).
Target: **v1.0**

## 1. Overview

Weave extensions are standalone executables named `weave-<name>`. Core starts one process per
invocation, writes one JSON request to stdin, reads one JSON response from stdout and waits for exit.
This slice ships the runner plus the two reference consumers: **enrichment during writes** and
**reranking during search**.

Extension IPC is the shared transport for later crypto providers; that contract remains in
[External Crypto Providers](./external-crypto-providers.md) and is out of scope here.

Non-goals:

- Long-lived extension processes or daemons.
- Extension writes to the bundle or index; all persistence stays in core transactions.
- Automatic extension discovery beyond `.weave/bin/` and `$PATH`.
- Sandboxing, signature verification or network policy.
- AI plugin packaging and external crypto providers.

## 2. Contracts

### 2.1 Discovery

- Valid names match `[a-z0-9][a-z0-9-]*`; anything else is `EXTENSION_NOT_FOUND`.
- Resolution order:
  1. `<bundle-root>/.weave/bin/weave-<name>`
  2. `$PATH` entries named `weave-<name>`
- The file must be executable; a non-executable match is skipped.
- A bundle-local extension wins over a `$PATH` extension with the same name.
- Discovery never follows symlinks outside the bundle or PATH entry.

`weave extensions list [<path>] --json` returns:

```json
{
  "extensions": [
    {"name": "keywords", "path": "/notes/.weave/bin/weave-keywords", "source": "bundle"},
    {"name": "rerank", "path": "/usr/local/bin/weave-rerank", "source": "path"}
  ]
}
```

- Sorted by name, then source (`bundle` before `path`).
- `path` is optional; without it only `$PATH` is scanned.

### 2.2 Transport

- One request object per process invocation, newline-terminated on stdin.
- One response object on stdout, optionally followed by whitespace.
- Stderr is captured as diagnostics and never parsed.
- Requests and responses are bounded to 1 MiB; larger output is `EXTENSION_PROTOCOL`.
- Every envelope carries `"v": "weave.cli/v1"`.
- Unknown response fields are ignored; unknown `v` is `EXTENSION_PROTOCOL`.
- Default timeout is 5 seconds, configurable with `--timeout` (Go duration, e.g. `750ms`, `10s`).
- Invocations are sequential and deterministic; no extension concurrency.
- The child inherits the process environment and working directory.

### 2.3 Enrichment

Request:

```json
{
  "v": "weave.cli/v1",
  "type": "enrichment_request",
  "doc_id": "docs/adr-004.md",
  "title": "Database Storage Selection",
  "frontmatter": {"type": "adr", "keywords": ["database", "storage"]},
  "body": "We decided to adopt pure-Go SQLite..."
}
```

- `body` is truncated to 64 KiB.
- `doc_id`, `title` and `frontmatter` are always present; `frontmatter` may be empty.

Response:

```json
{
  "v": "weave.cli/v1",
  "inferred_keywords": ["sqlite", "cgo-free", "wal"],
  "custom_metadata": {"summary": "Selection of pure-Go SQLite."}
}
```

- At most 32 inferred keywords are accepted; each is trimmed and must be 1–128 bytes.
- Invalid entries are dropped; a response with no valid keywords and no metadata is still valid.
- Inferred keywords are appended to the document's stored keywords, after declared keywords,
  de-duplicated case-insensitively. Declared keywords always win.
- `custom_metadata` is stored in the `enrichments` table keyed by bundle, document and enricher.
- Re-running the same enricher replaces its stored enrichment and inferred keywords.
- A failed, timed-out or malformed response keeps the previous enrichment and adds one warning.

### 2.4 Reranking

Request:

```json
{
  "v": "weave.cli/v1",
  "type": "rerank_request",
  "query": "zero cgo database",
  "candidates": [
    {"id": "docs/adr-004.md", "keyword_score": 0.82, "lexical_score": 0.82, "fuzzy_score": 0.45}
  ]
}
```

- Candidates are the current top-K of the selected search mode.
- Scores are normalized to `[0,1]`; unavailable dimensions are `0`.

Response:

```json
{
  "v": "weave.cli/v1",
  "ranked_candidates": [
    {"id": "docs/adr-004.md", "rerank_score": 0.95}
  ]
}
```

- The response must contain every candidate id exactly once, with no additions or duplicates.
- `rerank_score` is clamped to `[0,1]`; non-numeric values are `EXTENSION_PROTOCOL`.
- Final score is `0.5 * core_score + 0.5 * rerank_score`, where `core_score` is the selected mode's
  score (`keyword`, `fuzzy`, `hybrid` or normalized `bm25`).
- Results are sorted by final score descending, then id ascending, and rounded like other scores.
- The response reuses the existing hit shapes and adds optional `rerank_score` per hit.
- A failed, timed-out or malformed response returns the unchanged core results plus a stderr warning.

### 2.5 CLI

```sh
weave extensions list [<path>] --json
weave ingest  <path> --with-enricher <name> [--timeout 5s] --json
weave rebuild <path> --with-enricher <name> [--timeout 5s] --json
weave sync    <path> --with-enricher <name> [--timeout 5s] --json
weave search  <path> <query> --rerank <name> [--timeout 5s] --json
```

- `--with-enricher` and `--rerank` accept one extension name.
- A missing or invalid name fails before any write with `EXTENSION_NOT_FOUND`, exit `2`.
- `--timeout` requires a positive duration; invalid values exit `2`.
- `--timeout` without `--with-enricher` or `--rerank` is a usage error.
- Enrichment warnings are counted in `IngestReport.warnings` or `SyncReport.warnings`; search
  warnings only go to stderr and do not change the exit code.
- JSON stdout stays a single valid envelope; diagnostics go to stderr.

## 3. Behavior

### 3.1 Write flow

1. Parse the bundle.
2. Resolve the enricher; fail fast when missing.
3. For each concept, invoke the enricher with the bounded request.
4. Merge valid inferred keywords and collect custom metadata.
5. Write documents, merged keywords, enrichments and edges in one transaction.
6. On extension failure, keep the previous enrichment for that document and count one warning.

### 3.2 Search flow

1. Run the core search for the requested mode and top-K.
2. Build rerank candidates from the result list.
3. Invoke the reranker.
4. Validate the response covers the candidate set exactly.
5. Blend scores, sort and return the same hit shapes with `rerank_score` set.

### 3.3 Storage

The `enrichments` table already exists:

```sql
CREATE TABLE enrichments (
    bundle_root TEXT NOT NULL,
    doc_id TEXT NOT NULL,
    enricher_id TEXT NOT NULL,
    data_json TEXT NOT NULL,
    PRIMARY KEY (bundle_root, doc_id, enricher_id)
);
```

- `data_json` stores `{"name": "<enricher>", "response": {...}}` exactly as returned.
- Enrichments cascade-delete with their document.
- `LoadBundle` does not re-run enrichments; merged keywords are persisted on the document row.
- Removing `--with-enricher` leaves previously stored enrichments and keywords in place; `rebuild`
  without an enricher removes both.

## 4. Data Flow

```text
CLI --with-enricher
  -> engine/extensions runner  (discover, invoke, validate)
  -> operations                (merge keywords, collect metadata)
  -> storage/sqlite            (documents.keywords_json, enrichments rows)
  -> engine/search             (inferred keywords are searchable)

CLI search --rerank
  -> engine/search             (core top-K)
  -> engine/extensions runner
  -> operations                (validate, blend, sort)
  -> CLI/MCP response          (rerank_score fields)
```

## 5. Validation and Errors

| Condition | Code | Behavior |
| --- | --- | --- |
| Unknown or invalid extension name | `EXTENSION_NOT_FOUND` | Exit `2` before writes |
| Non-executable or missing binary | `EXTENSION_NOT_FOUND` | Exit `2` before writes |
| Timeout | `EXTENSION_TIMEOUT` | Warning; fallback to core behavior |
| Non-zero exit | `EXTENSION_FAILED` | Warning; fallback |
| Malformed JSON, wrong `v`, oversized output | `EXTENSION_PROTOCOL` | Warning; fallback |
| Rerank misses/duplicates/adds candidates | `EXTENSION_PROTOCOL` | Warning; unchanged results |
| Invalid timeout | `INVALID_REQUEST` | Exit `2` |

- Extension diagnostics include the extension name, a short reason and the exit status; they never
  include request bodies or document content.
- A failing extension never aborts a write or a search.

## 6. Testing

- Discovery order: bundle-local wins over PATH; non-executable skipped; invalid names rejected.
- `weave extensions list` output and ordering.
- Enrichment merges keywords, dedupes, caps at 32 and persists metadata.
- Enrichment failure/timeout/malformed response keeps previous data and adds one warning.
- Rerank reorders, blends at 0.5/0.5, sets `rerank_score` and preserves hit shapes.
- Rerank failure returns the unchanged core ordering.
- Timeout flag validation and exit codes.
- No enrichment invocation on read-only commands.
- Determinism across repeated runs with the same fixture extension.
- Full gate: `go test ./...`, `go test -race ./...`, `go vet ./...`, `./scripts/verify.sh`,
  docs-lint, markdownlint; calibration unchanged.

Fixture extensions live in `cmd/weave/testdata/extensions/` and are shell-free Go test binaries or
checked-in scripts with explicit shebangs.

## 7. Acceptance Criteria

- Extensions resolve from `.weave/bin/` before `$PATH` and are listed deterministically.
- Enrichment adds searchable inferred keywords and persists custom metadata.
- Reranking blends core and extension scores deterministically and annotates hits.
- Missing extensions fail fast; runtime extension failures degrade to core behavior with warnings.
- Extensions never receive index paths or write access through the protocol.
- The full verification gate stays green and ranking calibration is unchanged.

## Related Documents

- [Privacy & Extensions](./privacy-and-extensions.md)
- [External Crypto Providers](./external-crypto-providers.md)
- [Weave AI Extension](./weave-ai-plugin.md)
- [Storage & Sync](./storage-and-sync.md)
- [Keyword + Fuzzy Search](./keyword-fuzzy-search.md)
- [BM25 Ranking](./bm25-ranking.md)
- [ROADMAP](../../ROADMAP.md)
