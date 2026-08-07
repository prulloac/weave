---
type: specification
title: "Weave CLI Mount Specification"
description: "Contract for the weave CLI: a single-command, pollution-free mount of a local repository that serves the static graph and search engines."
tags:
  - "cli"
  - "bun"
  - "specification"
generated:
  by: opencode/big-pickle
  at: "2026-08-06T00:00:00Z"
status: draft
---

# Specification: Weave CLI Mount

## 1. Overview

Portable, offline CLI that mounts Weave over any local directory or Git repository with a single command. It wraps the target in a temporary symlink layer, runs the [OKF Bundle Parser](./okf-bundle-parser.md), and starts a local static server exposing the node graph and search index. The underlying repository is never modified: all artifacts are ephemeral and removed on unmount or error.

## 2. Module Structure & Execution Context

- **Entry Point:** `cli/index.ts` with `bin` mapping in `package.json` (`weave`).
- **No browser JS at build time; CLI runs under Bun.** Uses native `Bun.argv`, `Bun.serve`, and `Bun.shell`.
- **Location rationale:** the CLI lives at the repo root in `cli/` rather than `src/`, because `src/` is reserved for the Astro site. The CLI is a standalone runtime tool, not site code; this keeps Astro's `src/` pure and makes the CLI independently publishable.
- **Shebang:** `cli/index.ts` starts with `#!/usr/bin/env bun`; Bun executes `.ts` directly, so no compile step is needed for the `bin` mapping.
- **Commands:**
  - `weave mount <path>` — build graph + index and serve locally.
  - `weave unmount <port>` — stop the server and clean up temporary artifacts.
  - `weave status` — report mounted instances.
  - `weave --version` / `weave --help`.

**Files:**
- `cli/index.ts` — argument parsing + dispatch.
- `cli/mount.ts` — mount lifecycle.
- `cli/artifacts.ts` — temporary symlink layer creation/teardown.
- `cli/server.ts` — local static server.
- `cli/status.ts` — mount registry queries.

The CLI imports the shared parser from `src/lib/okf/parser.ts`; no other `src/` coupling is allowed.

## 3. Command Contracts

### 3.1 `weave mount <path>`

```typescript
export interface MountOptions {
  /** Target directory or git repository root */
  path: string;
  /** Port to serve on; default 4318 */
  port?: number;
  /** Do not start the server; build only */
  buildOnly?: boolean;
}

export interface MountResult {
  port: number;
  /** Absolute path of the temp symlink layer */
  artifactDir: string;
  /** Number of parsed concepts */
  conceptCount: number;
  /** Milliseconds spent building graph + index */
  buildMs: number;
  /** URL to open, e.g. http://localhost:4318 */
  url: string;
}
```

### 3.2 `weave unmount <port>`

```typescript
export interface UnmountResult {
  port: number;
  removedArtifacts: string[];
}
```

### 3.3 `weave status`

```typescript
export interface MountStatus {
  active: MountEntry[];
}
export interface MountEntry {
  port: number;
  target: string;
  startedAt: string;
  url: string;
}
```

## 4. Data Flow & Dependencies

- Validate `path` exists and is a directory; warn (not fail) if not a Git repository.
- Create temporary artifact dir via `Bun.mkdtemp` (system temp, e.g. `$TMPDIR/weave-<pid>-<rand>`).
- Materialize a **temporary symlink layer**: symlink the target's `.md`/`.yaml` files (or the whole tree) into the artifact dir; no copies, no writes into the target.
- Run `parseBundle(artifactDir)` from [OKF Bundle Parser](./okf-bundle-parser.md).
- Build node graph + search index in memory (build-time metadata validation).
- Serve on localhost via `Bun.serve`; render Astro pages from the graph data.
- On `SIGINT`, `unmount`, or fatal error: close server, `rm -rf` artifact dir, leave target untouched.
- Implements contract from [Git Repository Mount](../knowledge/concepts/git-repository-mount.md).

## 5. Edge Cases & Errors

- **Nonexistent path:** exit code 2 with clear message; no artifacts created.
- **Permission denied on target:** exit code 3; read-only targets still mountable for serving.
- **Port in use:** pick next free port, print it; never fail on port collision.
- **Already mounted same path:** `status` shows it; `mount` reuses or errors with exit code 4 (explicit choice documented).
- **Crash/kill mid-mount:** orphan artifact dirs cleaned by `status --prune`; never touches target.
- **Unmount of unknown port:** exit code 5, no-op.
- **Empty bundle:** mounts, serves empty graph with notice, exit 0.
- **Offline guarantee:** no network calls anywhere in mount/parse/serve path; assertable in tests.
- **Pollution guarantee:** target `mtime`/`git status` unchanged after mount+unmount; assertable in tests.

## Related Concepts

- Depends on [OKF Bundle Parser](./okf-bundle-parser.md)
- Runs [Node Graph Engine](../knowledge/concepts/node-graph-engine.md)
- Serves [Search Index](../knowledge/concepts/search-index.md)
- Implements [Git Repository Mount](../knowledge/concepts/git-repository-mount.md)
