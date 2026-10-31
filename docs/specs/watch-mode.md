---
type: specification
title: "Watch Mode Specification"
description: "v1.0 filesystem watch mode for incremental sync and live MCP reload."
tags:
  - "specification"
  - "watch"
  - "sync"
  - "v1"
generated:
  by: opencode/deepseek-v4.1-flash
  at: "2026-10-27T00:00:00Z"
status: stable
---

# Specification: Watch Mode

Status: **Polling watcher, `weave mcp --watch` and `weave sync --watch` implemented**. `fsnotify`
event delivery and `--fail-fast` remain follow-ups.

## 1. Overview

Watch mode keeps a v1.0 persistent index current while files change. It is built on the incremental
sync engine and is used by both `weave sync --watch` and `weave mcp --watch`.

```sh
weave sync <path> --watch --db <file> [--events] [--json]
weave mcp --bundle <path> --db .weave/index.db --watch
```

## 2. Goals and Non-Goals

### Goals

- React to create, modify, rename and delete events.
- Reuse hash-based incremental sync rather than full reindexing.
- Coalesce bursts of filesystem events.
- Keep serving queries during reindex.
- Expose failures without stopping the process.

### Non-Goals

- Distributed sync.
- Watching remote filesystems.
- Editing files.
- Guaranteeing sub-second propagation.
- Running without a persistent SQLite index.

## 3. Process Contract

- `--watch` requires an explicit `--db` path and an already indexed bundle. An unindexed bundle
  returns `BUNDLE_NOT_INDEXED`; run `weave ingest` first.
- Initial sync runs before watching.
- The process stays alive until SIGINT/SIGTERM.
- Reindex operations are serialized.
- Queries read the last committed snapshot; sync uses transactions.
- MCP tools and resources read the persistent bundle per request, so a successful watch sync is
  immediately visible without restarting the server.
- `--json` with `--watch` emits one initial sync result and JSONL status events on stdout only if
  `--events` is set; default diagnostics go to stderr.

## 4. Event Handling

- Polling with SHA-256 snapshots is the delivered mechanism; `fsnotify` event optimization is a
  follow-up and no event backend fallback is required today.
- Poll interval defaults to 500 ms; tests use a shorter interval.
- Watch the bundle root recursively.
- Ignore `.git`, `node_modules`, `.weave`, other dot-directories and dot-files, editor swap files and
  files ending in `~`, `.tmp` or `.swp`.
- Only Markdown files participate in snapshots and sync; other files never trigger a sync.
- Coalesce bursts by polling: sync runs only when the snapshot differs from the last synced one.
- Treat editor atomic saves as modification.
- Handle delete by pruning the document and cascading edges.
- Handle rename as delete + create.
- Ignore files outside the bundle root.

## 5. Sync Semantics

Watch mode delegates to the v1.0 sync workflow:

1. Hash changed files with SHA-256.
2. Skip unchanged hashes.
3. Re-parse changed files.
4. Replace outgoing edges.
5. Reconcile dangling incoming edges.
6. Update FTS5 rows.
7. Commit in one transaction.
8. Emit a sync summary.

If a parse fails, keep the previous document version and report the error; do not remove the last
good content.

## 6. Error Handling

- Snapshot read failure: report through `OnError` and keep polling; the loop does not exit.
- Sync failure: report through `OnError`, retain the last good index, and retry on the next tick.
- Parse error: sync keeps the previous document version and reports the error.
- Shutdown: stop polling, finish any in-flight sync, and let the caller close the database.
- `--fail-fast` and database-backoff retries remain deferred with the `fsnotify` slice.

## 7. Output

Initial sync result is passed to `OnSync` as a `contractv1.SyncReport`:

```json
{"db":"/home/u/.weave/index.db","bundle":"/notes","mode":"strict","documents":3,"added":0,"updated":0,"removed":0,"skipped":3,"warnings":0,"errors":0}
```

With `--json --events`, one initial `weave.cli/v1` success envelope is printed, then JSONL events:

```json
{"event":"sync","at":"2026-10-30T12:00:00Z","added":1,"updated":2,"removed":0,"errors":0}
```

Without `--events`, later syncs are silent in JSON mode. Human mode prints one summary per sync.
`--fail-fast` exit codes remain deferred; `weave mcp --watch` writes watch diagnostics to stderr.

## 8. Testing

- Initial sync before polling.
- Modify and delete propagation.
- Burst edits and ignored directories.
- Clean shutdown under cancellation.
- MCP reload visibility and concurrent queries.
- Unindexed bundle rejection.
- Race suite.
- Rename/atomic-save, parse-error retention and polling fallback are pending the `fsnotify` slice.

## 9. Acceptance Criteria

- Watch mode keeps the index current without full reindexes.
- No document is lost on transient parse errors.
- Queries remain consistent with committed snapshots.
- Shutdown is clean.
- Ignored directories and temp files never trigger sync.
- `--watch` refuses to run without persistent storage.

## Related Documents

- [Storage & Sync](./storage-and-sync.md)
- [MCP Server](./mcp-server.md)
- [Permissive Ingestion](./permissive-ingestion.md)
- [ROADMAP](../../ROADMAP.md)
