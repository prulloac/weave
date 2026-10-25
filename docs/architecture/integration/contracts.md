---
type: Integration Contract
title: "Integration Contracts"
description: "CLI-first integration contract: process I/O, operations envelopes, optional Go API, presentation DOM and filesystem rules."
tags:
  - "architecture"
  - "integration"
  - "cli"
  - "contract"
  - "operations"
generated:
  by: opencode/deepseek-v4.1-flash
  at: "2026-10-25T00:00:00Z"
status: draft
---

# Integration Contracts

This document defines every boundary a host project or another layer may depend on. Anything not
listed here is internal and may change.

**The primary integration contract is CLI I/O.** The bigger engine invokes `weave` as a
subprocess, passes arguments and optionally stdin, and reads stdout, stderr and the exit code.
HTTP is not an integration contract; it exists only to render the browser presentation.

There are four contracts:

1. CLI and operations I/O — primary.
2. Domain JSON — the payloads carried by the CLI.
3. Optional Go package API — for in-process Go hosts.
4. Presentation DOM and filesystem rules — for the browser UI and bundle input.

## 1. CLI and Operations I/O (Primary)

### One-shot commands

```sh
weave contract --json
weave parse <path> --json
weave graph <path> --json
weave find <path> <query> --json
weave list <path> [--type <t>] [--tag <t>] [--status <s>] [--limit <n>] --json
weave show <path> <id|file> --json
weave backlinks <path> <id|file> --json
weave path <path> <id|file> <id|file> --json
weave version --json
```

Guarantees:

- **stdout is machine-readable only** in `--json` mode: exactly one JSON envelope.
- **stderr is diagnostics** and never required for success.
- **Exit codes are stable** and documented below.
- **No network, port, daemon or authentication** is involved.
- Bundle paths are explicit arguments.
- Results are deterministic for identical inputs and binary version.
- No interactive prompts in machine mode.

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
    "code": "CONCEPT_NOT_FOUND",
    "message": "concept not found: does-not-exist",
    "details": {"target": "does-not-exist"}
  }
}
```

Rules:

- Exactly one of `result` or `error` is present.
- `id` is reserved for the deferred stdio mode and omitted in one-shot output.
- Unknown fields are ignored.
- The envelope is valid JSON even when the exit code is non-zero.
- `--json` is the only machine mode in v1. `--legacy-json` is deferred post-v1.

### Version discovery

`weave contract --json` returns the contract version, CLI version, binary version, operation
catalogue and error codes. Hosts call it once per binary version and fail fast on an unsupported
contract major.

### Stdio mode (deferred post-v1)

```sh
weave engine --stdio
```

This mode is **not shipped in v1**. When it lands:

- NDJSON request per line, NDJSON envelope per line.
- Requests are processed in order.
- stdout carries responses only; diagnostics go to stderr.
- Malformed input returns an `INVALID_REQUEST` envelope.
- EOF or a `shutdown` request exits the process.
- This is still process-local CLI I/O; there is no listener.

### Exit codes

| Code | Meaning |
| --- | --- |
| 0 | Success |
| 1 | Runtime error, parse failure, no matches, unknown concept, disconnected path |
| 2 | Usage error, invalid root, invalid request |
| 3 | Permission denied |
| 4 | Duplicate mount (presentation command) |
| 5 | Unknown mount port (presentation command) |

### Operation errors

| Code | Meaning |
| --- | --- |
| `INVALID_REQUEST` | Missing or unknown operation or field |
| `INVALID_ROOT` | Path missing or not a directory |
| `PERMISSION_DENIED` | Filesystem permission failure |
| `PARSE_ERROR` | Bundle cannot be parsed at all |
| `CONCEPT_NOT_FOUND` | Unknown concept target |
| `PATH_NOT_FOUND` | No route between concepts |
| `INTERNAL` | Unexpected failure |

The detailed request/response model lives in [Operations layer](../layers/operations.md).

## 2. Domain JSON (Payload Contract)

Domain payloads are defined in [Contract layer](../layers/contract.md):

- `Bundle`, `Concept`, `Link`, `IndexEntry`, `Validation`
- `Graph`, `SerializedGraph`
- `FindResult`, `ListResult`, `ShowResult`, `BacklinksResult`, `PathResult`
- `VersionInfo`, `ContractInfo`

Rules:

- JSON uses `snake_case`.
- Paths are bundle-relative POSIX paths; absolute roots appear only in `Bundle.root`.
- Concept ids never include `.md`.
- Arrays are ordered; consumers do not rely on object key order.
- Unknown JSON fields are ignored.
- Payloads are identical whether delivered by CLI or by the optional Go API.

## 3. Optional Go Package API

Only for hosts written in Go that want to avoid the process boundary.

```go
eng, err := engine.Open(ctx, root, engine.Options{})
defer eng.Close()

site, err := mount.NewSite(ctx, target, mount.Options{BasePath: "/weave/"})
defer site.Close()
```

Guarantees:

- `engine.Open` is the only expensive call; queries are read-only and concurrency-safe.
- Returned structures are immutable after `Open`.
- `mount.NewSite` is presentation only; it does not write the registry or install signals.
- `mount.Site.Handler` is safe for concurrent use and is not the integration API.
- The host owns lifecycle, auth, TLS and reverse-proxy concerns.

The Go API is not required for integration and may evolve within the module's semver policy. The
CLI contract is the compatibility promise.

## 4. Presentation DOM and Filesystem

### Presentation routes (not an integration API)

| Method | Path | Success | Failure | Content type |
| --- | --- | --- | --- | --- |
| GET | `/` | 200 | 500 | `text/html; charset=utf-8` |
| GET | `/lens` | 200 | 500 | `text/html; charset=utf-8` |
| GET | `/concept/{id}` | 200 | 404 | `text/html; charset=utf-8` |
| GET | `/assets/weave.css` | 200 | 404 | `text/css; charset=utf-8` |
| GET | `/assets/lens.js` | 200 | 404 | `text/javascript; charset=utf-8` |
| POST | `/__weave/shutdown` | 200 | 404/405 | `text/plain; charset=utf-8` |

These routes exist for a browser. A host must not treat them as the machine API.
`POST /__weave/shutdown` is available only when the standalone presentation enables it.

### DOM hooks

| Hook | Required | Meaning |
| --- | --- | --- |
| `data-testid="mount-nav"` | yes | Presentation header |
| `link[rel="stylesheet"][href$="/assets/weave.css"]` | yes | Theme stylesheet |
| `main.weave-main` | yes | Main content region |
| `#weave-lens-root` | lens only | Preact mount point |
| `#weave-graph` | lens only | Serialized graph payload |
| `data-testid="lens-index-entry"` | lens only | Anchor with `data-id` and `href` |
| `data-testid="force-graph-canvas"` | lens only | Canvas after hydration |
| `data-testid="legend"` | lens only | Legend after hydration |
| `data-testid="empty-message"` | lens only | Empty state |
| `data-testid="concept-page"` | optional | Concept article |
| `data-testid="not-found"` | optional | Presentation 404 |

Explorer, concept and lens DOM details are in [Render layer](../layers/render-go.md) and
[Lens layer](../layers/lens-ts.md). They are presentation contracts, not integration contracts.

### Filesystem and input

Input is an OKF v0.2 bundle directory.

| Rule | Guarantee |
| --- | --- |
| Input format | Markdown `.md` with YAML frontmatter |
| Required field | Non-empty `type` |
| Reserved names | `index.md`, `log.md` (never concepts) |
| Root version | Only root `index.md` may declare `okf_version: "0.2"` |
| Encoding | UTF-8; invalid files are reported and skipped |
| Paths | POSIX concept ids regardless of host OS |
| Symlinks | Files followed; directories one level with cycle guard |
| Writes | Never write to the target bundle |
| Cache | Optional, outside the target tree |

Host rules:

- Pass an absolute or resolvable target path.
- Do not mutate the target while a process is running; start a new process to observe changes.
- The host is responsible for file permissions and secret material in the bundle.
- The artifact layer used by the presentation is temporary and removed on close.

## Compatibility Matrix

| Component | Contract | CLI | Notes |
| --- | --- | --- | --- |
| Go engine v1 | v1 | n/a | Internal/optional API |
| Go operations v1 | v1 | `weave.cli/v1` | Primary machine surface |
| Go CLI v1 | v1 | `weave.cli/v1` | One-shot only; stdio deferred |
| Go render/mount v1 | v1 | n/a | Presentation only |
| Lens v1 | v1 | n/a | Presentation only |
| TS reference (frozen) | v1 | legacy (frozen) | Conformance reference until cutover |

## Related Documents

- [Contract layer](../layers/contract.md)
- [Operations layer](../layers/operations.md)
- [CLI layer](../layers/cli-go.md)
- [Engine layer](../layers/engine-go.md)
- [Render layer](../layers/render-go.md)
- [Lens layer](../layers/lens-ts.md)
- [Mount layer](../layers/mount-go.md)
- [Integration steps](steps.md)
- [Integration testing](testing.md)
