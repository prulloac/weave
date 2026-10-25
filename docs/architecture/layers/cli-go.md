---
type: Layer Specification
title: "CLI Layer (Go)"
description: "Primary integration contract: command surface, machine envelopes, stdout/stderr rules, exit codes and version discovery."
tags:
  - "architecture"
  - "cli"
  - "go"
  - "integration"
generated:
  by: opencode/deepseek-v4.1-flash
  at: "2026-10-25T00:00:00Z"
status: draft
---

# CLI Layer (Go)

## Purpose

`cmd/weave` is the **primary integration surface** for projects that cannot or do not want to link
the Go engine. It is a thin adapter over the [operations layer](operations.md):

```text
argv / env / stdin  →  operations.Request  →  engine
stdout / stderr / exit code  ←  operations.Response
```

It owns process-specific concerns:

- argument and flag parsing,
- environment resolution,
- human and machine output,
- stdout/stderr separation,
- exit codes,
- signal handling,
- the default mount registry location,
- the shutdown endpoint opt-in.

It owns no domain logic.

## Layout

```text
cmd/weave/
├── main.go
├── commands/
│   ├── contract.go
│   ├── parse.go
│   ├── graph.go
│   ├── find.go
│   ├── list.go
│   ├── show.go
│   ├── backlinks.go
│   ├── path.go
│   ├── engine.go        # post-v1 --stdio loop
│   ├── mount.go         # presentation only
│   ├── unmount.go
│   ├── status.go
│   ├── version.go
│   └── help.go
└── output/
    ├── human.go
    ├── envelope.go
    └── legacy.go          # post-v1
```

`mount`, `unmount` and `status` drive the presentation server. They are not part of the machine
integration contract.

## Command Surface

```sh
# Machine operations (primary integration)
weave contract [--json]
weave parse <path> [--json]
weave graph <path> [--json]
weave find <path> <query> [--limit <n>] [--json]
weave list <path> [--type <t>] [--tag <t>] [--status <s>] [--limit <n>] [--json]
weave show <path> <id|file> [--json]
weave backlinks <path> <id|file> [--json]
weave path <path> <id|file> <id|file> [--json]
weave version [--json]

# Deferred post-v1 (not in the first Go CLI release)
# weave engine --stdio
# --legacy-json

# Presentation (human browser UI; not an integration API)
weave mount <path> [--port <port>] [--build-only]
weave unmount <port>
weave status [--prune]

weave --version | --help
```

`weave parse --json` includes each concept's `body` by default. `graph`, `find`, `list`,
`backlinks` and `path` omit bodies; `show` includes the selected concept's body. A `--no-body`
flag is deferred post-v1.

## Output Modes

| Mode | Selected by | stdout | stderr |
| --- | --- | --- | --- |
| Human | default | human text | diagnostics |
| Envelope JSON | `--json` | one JSON envelope | diagnostics |

v1 has exactly these two modes. Legacy JSON and NDJSON stdio are deferred; see
[Deferred Modes](#deferred-modes-post-v1).

### Envelope JSON

`--json` emits exactly one JSON document with a trailing newline:

```json
{
  "contract": "weave.contract/v1",
  "cli": "weave.cli/v1",
  "ok": true,
  "result": []
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

- stdout is **only** the envelope. No logs, warnings, colors, banners or progress bars.
- Warnings and human diagnostics go to stderr, prefixed with `weave:`.
- A failed operation still emits a valid envelope on stdout in `--json` mode, and exits non-zero.
- `--json` is the only machine mode in v1.
- In human mode, stdout is the requested text and errors go to stderr.

### Deferred Modes (Post-v1)

These are explicitly **not** part of the first Go CLI release:

- `--legacy-json`: the pre-Go result-shaped output. It may be added later if migration demand
  justifies it, and would be documented as deprecated from the start.
- `weave engine --stdio`: NDJSON request/response over stdin/stdout for hosts that issue many
  operations. It may be added after the one-shot CLI contract is stable.

Neither mode affects the v1 envelope contract. The operations layer already models the requests
and responses, so adding them later does not change engine semantics. The future MCP server uses
the same seam.

## Exit Codes

| Code | Meaning |
| --- | --- |
| 0 | Success |
| 1 | Runtime error, parse failure, no matches, unknown concept, disconnected path |
| 2 | Usage error, invalid root, missing path, invalid number, invalid request |
| 3 | Permission denied |
| 4 | Duplicate mount on the same target and port |
| 5 | Unknown mount port |

The operations layer returns error codes; the CLI maps them to these exit codes.

## Version Discovery

```sh
weave contract --json
```

```json
{
  "contract": "weave.contract/v1",
  "cli": "weave.cli/v1",
  "binary_version": "0.1.0",
  "operations": ["contract", "parse", "graph", "find", "list", "show", "backlinks", "path", "version"],
  "errors": ["INVALID_REQUEST", "CONCEPT_NOT_FOUND", "PATH_NOT_FOUND", "PARSE_ERROR", "INTERNAL"]
}
```

A host should:

1. Run `weave contract --json` once per binary version.
2. Cache the supported operations and contract version.
3. Fail fast if the contract major is unsupported.
4. Prefer `--json` and the operations envelope over screen-scraping human output.

## Input and State

- Bundle paths are explicit arguments, never implicit environment state.
- `weave` does not read a project config file in v1.
- The parse cache is **enabled by default** for machine commands. `WEAVE_CACHE_DIR` sets its
  location; `--no-cache` disables it for one invocation. It never changes results.
- Commands are stateless and may be run concurrently. They do not share a daemon.
- No command prompts interactively in machine mode. If a prompt would be required, the command
  fails with `INVALID_REQUEST` and an explanatory stderr message.

## Environment

| Setting | Source | Default |
| --- | --- | --- |
| Registry directory | `WEAVE_REGISTRY_DIR` | `os.TempDir()/weave` |
| Parse cache directory | `WEAVE_CACHE_DIR` | `os.TempDir()/weave-okf-cache` (enabled by default) |
| Default port (mount only) | `WEAVE_PORT` | `4318` |
| Lens asset | embedded | embedded `dist/lens.js` |

Environment is resolved once in `main.go` and passed down explicitly. Lower layers do not read
environment variables.

## Presentation Commands

`mount`, `unmount` and `status` are human-facing:

1. Validate the path and warn when it is not a Git repository.
2. Create the presentation site.
3. Bind a listener; if the requested port is busy, bind the next free port.
4. Register the active mount.
5. Install `SIGINT`/`SIGTERM` handlers.
6. Print the URL and concept count.
7. Serve until shutdown; remove the registry entry and artifacts on exit.

`--build-only` parses the bundle and builds the graph but does not bind a port or write the
registry.

These commands are intentionally excluded from the machine integration contract. A host that
needs machine access uses the operations commands above, not the mount.

## Signals and Shutdown (Presentation)

- `SIGINT`/`SIGTERM` trigger a graceful HTTP shutdown and `Site.Close()`.
- `weave unmount <port>` calls the presentation shutdown endpoint when enabled, waits briefly,
  then falls back to registry cleanup.
- Signal handling is installed by the CLI only; embedded presentation sites are managed by their
  host.

## Help and Errors

- `weave --help` prints the command list.
- Unknown commands return exit code 2 with a short hint on stderr.
- Human errors are `weave: <message>` on stderr.
- Machine errors are a valid envelope on stdout plus the same message on stderr.
- No stack traces in normal operation; set `WEAVE_DEBUG=1` for verbose diagnostics on stderr.

## Testing

1. Golden tests for human and envelope output.
2. Argument parser table tests for every flag and error path.
3. Process tests with `exec.Command` against a built binary:
   - stdout/stderr separation,
   - exit codes,
   - envelope validity,
   - no color/progress in machine mode,
   - version and contract discovery.
4. JSON Schema validation of every envelope.
5. Determinism tests: repeated invocations produce identical envelopes, with and without cache.
6. Cache tests: hit on the second run, invalidation after a file change, `--no-cache` parity,
   isolated temporary cache directories.
7. Presentation command tests are separate and may use HTTP.
8. Post-v1: legacy-output and NDJSON tests are added when those modes ship.

## Related Documents

- [ADR 0002](../adr/0002-v1-scope-layout-and-contract.md)
- [Operations layer](operations.md)
- [Engine layer](engine-go.md)
- [Mount layer](mount-go.md)
- [Integration contracts](../integration/contracts.md)
- [Integration steps](../integration/steps.md)
- [Integration testing](../integration/testing.md)
