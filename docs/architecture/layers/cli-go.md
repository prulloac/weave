---
type: Layer Specification
title: "CLI Layer (Go)"
description: "Standalone weave command surface, flags, exit codes, JSON output, configuration and signal handling."
tags:
  - "architecture"
  - "cli"
  - "go"
  - "dx"
generated:
  by: opencode/deepseek-v4.1-flash
  at: "2026-10-25T00:00:00Z"
status: draft
---

# CLI Layer (Go)

## Purpose

`cmd/weave` is a thin adapter over the engine, render and mount packages. It owns everything
process-specific:

- argument parsing,
- stdout/stderr formatting,
- exit codes,
- signal handling,
- the default mount registry location,
- the shutdown endpoint opt-in.

Business logic stays in the lower layers.

## Layout

```text
cmd/weave/
├── main.go
├── commands/
│   ├── mount.go
│   ├── unmount.go
│   ├── status.go
│   ├── find.go
│   ├── list.go
│   ├── show.go
│   ├── backlinks.go
│   ├── path.go
│   ├── version.go
│   └── help.go
└── output/
    ├── text.go
    └── json.go
```

## Command Surface

```text
weave mount <path> [--port <port>] [--build-only]
weave unmount <port>
weave status [--prune]
weave find <path> <query> [--limit <n>] [--json]
weave list <path> [--type <t>] [--tag <t>] [--status <s>] [--limit <n>] [--json]
weave show <path> <id|file> [--json]
weave backlinks <path> <id|file> [--json]
weave path <path> <id|file> <id|file> [--json]
weave --version | --help
```

Command behavior must remain compatible with the current TypeScript CLI.

## Exit Codes

| Code | Meaning |
| --- | --- |
| 0 | Success |
| 1 | Runtime error, no matches, unknown concept, disconnected path |
| 2 | Usage error, missing path, invalid number |
| 3 | Permission denied |
| 4 | Duplicate mount on the same target and port |
| 5 | Unknown mount port |

Errors are printed to stderr as `weave: <message>`.

## JSON Output

- `--json` emits a single JSON document with a trailing newline.
- Field names follow `contract/v1` (`snake_case`).
- Ordering is stable: find by score/id, list/show/backlinks/path by id.
- No ANSI colors, progress bars or warnings on stdout in JSON mode.
- Warnings go to stderr and must not corrupt the JSON document.
- Exit codes are unchanged by `--json`.

## Mount Lifecycle

1. Validate the path and warn when it is not a Git repository.
2. Create the site through `mount.NewSite`.
3. Bind a listener; if the requested port is busy, bind the next free port.
4. Register the active mount.
5. Install signal handlers for `SIGINT` and `SIGTERM`.
6. Print:
   - `weave: mounted <path> at <url>`
   - `weave: <n> concepts in <ms>ms`
7. Serve until shutdown; on exit remove the registry entry and artifact root.

`--build-only` performs parse and graph construction but does not bind a port or write the
registry. It prints the concept count and timing.

## Configuration

| Setting | Source | Default |
| --- | --- | --- |
| Registry directory | `WEAVE_REGISTRY_DIR` | `os.TempDir()/weave` |
| Parse cache directory | `WEAVE_CACHE_DIR` | `os.TempDir()/weave-okf-cache` |
| Default port | flag or `WEAVE_PORT` | `4318` |
| Lens asset | embedded | embedded `dist/lens.js` |

Configuration is resolved once in `main.go` and passed down explicitly. Lower layers must not
read environment variables.

## Signals and Shutdown

- `SIGINT`/`SIGTERM` trigger a graceful `http.Server.Shutdown` with a timeout, then `Site.Close()`.
- `weave unmount <port>` calls the shutdown endpoint when enabled, waits briefly for the process
  to exit, then falls back to registry cleanup.
- Signal handling must be installed by the CLI only; embedded sites are managed by the host.

## Version

- `weave --version` prints `weave <semver>` plus the contract version:
  `weave 0.1.0 (contract 1)`.
- The version is injected at build time with `-ldflags "-X main.version=..."`.
- Development builds print `weave dev (contract 1)`.

## Help and Discoverability

- `weave --help` prints the command list above.
- Unknown commands return exit code 2 with a short hint.
- `weave status` prints a tab-separated table and `no active mounts` when empty.

## Testing

1. Golden CLI output tests for text and JSON modes.
2. Argument parser table tests for every flag and error path.
3. Exit-code tests using `go test` with `exec.Command` against a built binary.
4. Integration tests mounting the fixture bundle and running every command.
5. Registry and unmount lifecycle tests.
6. Cross-platform smoke tests for path handling.
7. Determinism test: repeated invocations produce identical output.

## Related Documents

- [Mount layer](mount-go.md)
- [Engine layer](engine-go.md)
- [Integration contracts](../integration/contracts.md)
- [Integration testing](../integration/testing.md)
