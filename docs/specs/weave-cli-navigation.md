---
type: specification
title: "Weave CLI Find & Navigate Specification"
description: "Stateless, grep-style CLI commands for finding, searching and navigating OKF bundles from the terminal, consuming the Node Graph Engine and Search Index."
tags:
  - "cli"
  - "search"
  - "navigation"
  - "specification"
generated:
  by: opencode/big-pickle
  at: "2026-08-06T00:00:00Z"
status: draft
---

# Specification: Weave CLI Find & Navigate

## 1. Overview

Stateless, grep-style terminal commands that let developers, engineers and AI agents find, search and navigate OKF bundles without a server. Every invocation re-parses the target bundle and rebuilds the graph + search index in memory, prints human-readable output (or stable `--json` for scripts and agents), and exits. No daemon, no persisted state, no pollution of the target.

This is the technical companion to the Visualization Lens (Tier 3): the CLI serves precise **micro-navigation** for developers, engineers and AI agents; the Lens serves the **macro-view** for regular and casual users. The two surfaces reuse the same engine contracts and are not rivals.

## 2. Module Structure & Execution Context

- **Entry Point:** extends the existing CLI in `cli/` — dispatch is added to `cli/index.ts`; new command implementations live in `cli/find.ts`, `cli/show.ts`, and `cli/path.ts`.
- **Runtime:** Bun. Uses `Bun.argv` and native stdout. Shebang follows the existing `cli/index.ts` convention.
- **Stateless:** each run calls `parseBundle` → `buildGraph` → `buildSearchIndex` fresh. Nothing is cached or written.
- **Reuse, don't re-invent:** consumes the [Node Graph Engine](./node-graph-engine.md) and [Search Index](./search-index.md) contracts; no new engine logic.
- **Commands:**
  - `weave find <query>` — full-text ranked search.
  - `weave show <id|path>` — render a concept: metadata, body, outgoing links, backlinks.
  - `weave backlinks <id|path>` — list incoming edges for a concept.
  - `weave path <from> <to>` — shortest path between two concepts.
  - Existing `weave --version` / `weave --help` apply globally.

**Files:**

- `cli/find.ts` — `weave find` implementation.
- `cli/show.ts` — `weave show` + `weave backlinks` implementation.
- `cli/path.ts` — `weave path` implementation.
- `cli/resolve.ts` — shared id-or-path input resolution for the target directory and concept arguments.

The CLI imports the shared engines from `src/lib/okf/`, `src/lib/graph/`, and `src/lib/search/`; no other `src/` coupling is allowed.

## 3. Command Contracts

### 3.1 `weave find <query>`

```text
weave find <path> <query> [--limit 20] [--json]
```

```typescript
export interface FindResult {
  id: string;
  title: string;
  path: string;
  type: string;
  score: number;
  snippet?: string;
}
```

- Human output: one line per result, `score title [path]` (highest score first).
- `--limit N` — cap results (default 20).
- `--json` — emit `FindResult[]` with stable key order.
- Backed by `search()` from [Search Index](./search-index.md).

### 3.2 `weave show <id|path>`

```text
weave show <path> <id|path> [--json]
```

```typescript
export interface ShowResult {
  id: string;
  path: string;
  type: string;
  title?: string;
  description?: string;
  tags?: string[];
  status?: 'draft' | 'stable' | 'deprecated';
  body: string;
  links: Array<{ raw: string; target: string; resolvesInBundle: boolean }>;
  backlinks: string[];
}
```

- Human output: frontmatter metadata, backlink count, outgoing links, then body.
- `--json` — emit `ShowResult`.

### 3.3 `weave backlinks <id|path>`

```text
weave backlinks <path> <id|path> [--json]
```

```typescript
export interface BacklinksResult {
  id: string;
  backlinks: Array<{ from: string; title: string }>;
}
```

- Backed by `incoming()` from [Node Graph Engine](./node-graph-engine.md).
- `--json` — emit `BacklinksResult`.

### 3.4 `weave path <from> <to>`

```text
weave path <path> <from> <to> [--json]
```

```typescript
export interface PathResult {
  from: string;
  to: string;
  path: string[] | null;
}
```

- Backed by `findPath()` from [Node Graph Engine](./node-graph-engine.md).
- `path: null` when no route exists (exit code 1).
- `--json` — emit `PathResult`.

## 4. Data Flow & Dependencies

- `<path>` target must be a directory; resolve with the same semantics as `weave mount` (exit 2 on missing, 3 on permission denied).
- Each invocation: `parseBundle` → `buildGraph` → `buildSearchIndex`, then run the command's query against the in-memory structures.
- **Id-or-path resolution:** accept a concept id (`concepts/okf-bundle`), a bundle-relative path (`concepts/okf-bundle.md`), or an absolute path; normalize to a concept id before lookup.
- **Exit codes:** `0` success with results; `1` no matches / concept not found / no route; `2` target path error; `3` permission denied. Matches the [Weave CLI Mount](./weave-cli.md) convention.
- **`--json` guarantee:** valid JSON, deterministic field order, machine-parseable for scripts and AI agents.
- **Offline guarantee:** no network calls anywhere in the invocation path.

## 5. Edge Cases & Errors

- **Empty query** (`find ""`): exit 1 with a message, no results.
- **Unknown concept** (`show`/`backlinks` with an id that is not in the bundle): exit 1, "concept not found".
- **No route** (`path` between disconnected nodes): `path: null`, exit 1.
- **Empty bundle:** `find` returns nothing (exit 1); `show`/`backlinks` exit 1.
- **Stateless determinism:** identical invocations produce identical output; ordering stable for `--json`.
- **Performance budget:** a full stateless invocation (parse + graph + index + query) on a 10k-file bundle completes in under 200 ms on a local SSD, preserving the vision's "milliseconds" feel.

## Related Concepts

- Depends on [OKF Bundle Parser](./okf-bundle-parser.md)
- Depends on [Node Graph Engine](./node-graph-engine.md)
- Depends on [Search Index](./search-index.md)
- Extends [Weave CLI Mount](./weave-cli.md)
- Complements [Visualization Lens](../knowledge/concepts/node-graph-engine.md) (Tier 3)
- Implements [Weave Vision](../knowledge/concepts/weave-vision.md) — "never leaves terminal"
