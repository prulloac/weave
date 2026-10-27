---
type: specification
title: "Watch Mode Specification"
description: "V2 filesystem watch mode for incremental sync and live MCP reload."
tags:
  - "specification"
  - "watch"
  - "sync"
  - "v2"
generated:
  by: opencode/deepseek-v4.1-flash
  at: "2026-10-27T00:00:00Z"
status: draft
---

# Specification: Watch Mode

## 1. Overview

Watch mode keeps a v2 persistent index current while files change. It is built on the incremental
sync engine and is used by both `weave sync --watch` and `weave mcp --watch`.

```sh
weave sync <path> --watch --json
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

- `--watch` requires a persistent database (`--db` or default `.weave/index.db`).
- Initial sync runs before watching.
- The process stays alive until SIGINT/SIGTERM.
- Reindex operations are serialized.
- Queries read the last committed snapshot; sync uses transactions.
- `--json` with `--watch` emits one initial sync result and JSONL status events on stdout only if
  `--events` is set; default diagnostics go to stderr.

## 4. Event Handling

- Use `fsnotify` where supported; fall back to polling when the backend is unavailable.
- Watch the bundle root recursively.
- Ignore `.git`, `node_modules`, the database directory and temporary files.
- Debounce at 200 ms; coalesce events per path.
- Treat editor atomic saves (rename/create) as modification.
- Handle delete by pruning the document and cascading edges.
- Handle rename as delete + create.
- Ignore files outside the bundle root.
- Cap pending event queue; drop duplicates before syncing.

## 5. Sync Semantics

Watch mode delegates to the v2 sync workflow:

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

- Filesystem watcher failure: log and fall back to polling; if polling is unavailable, exit `1`.
- Database busy/locked: retry with bounded backoff.
- Parse error: log, keep previous snapshot, continue.
- Permission error: log, continue; `--fail-fast` exits `3`.
- Shutdown: stop watcher, finish in-flight sync, close DB.

## 7. Output

Initial result:

```json
{"mode":"watch","documents":42,"added":0,"updated":0,"removed":0,"errors":0}
```

Optional event:

```json
{"event":"sync","at":"2026-10-27T12:00:00Z","added":1,"updated":2,"removed":0,"errors":0}
```

## 8. Testing

- Create/modify/delete/rename.
- Burst edits and debounce.
- Atomic-save pattern.
- Ignored directories.
- Parse-error retention.
- Shutdown during sync.
- Polling fallback.
- Concurrent queries during reindex.
- Race suite.

## 9. Acceptance Criteria

- Watch mode keeps the index current without full reindexes.
- No document is lost on transient parse errors.
- Queries remain consistent with committed snapshots.
- Shutdown is clean.
- Ignored directories and temp files never trigger sync.
- `--watch` refuses to run without persistent storage.

## Related Documents

- [V2 Storage & Sync](./v2-storage-and-sync.md)
- [MCP Server](./mcp-server.md)
- [Permissive Ingestion](./permissive-ingestion.md)
- [ROADMAP](../../ROADMAP.md)
