---
type: Layer Specification
title: "CLI Layer (Go)"
description: "Primary integration contract for search, parse, graph and navigation: envelopes, stdout/stderr, exit codes and top-K search flags."
tags:
  - "architecture"
  - "cli"
  - "go"
  - "integration"
  - "search"
generated:
  by: opencode/deepseek-v4.1-flash
  at: "2026-10-25T00:00:00Z"
status: draft
---

# CLI Layer (Go)

## Purpose

`cmd/weave` is the primary integration surface. It adapts argv/stdin/stdout to
[operations](operations.md) and owns:

- argument parsing,
- environment resolution,
- human and machine output,
- stdout/stderr separation,
- exit codes.

There is **no presentation command**. `mount`, `unmount`, `status`, `find` and `--legacy-json`
are gone or deferred.

## Command Surface

```sh
weave contract [--json]
weave parse <path> [--json] [--no-cache]
weave graph <path> [--json] [--no-cache]
weave search <path> <query> [--mode hybrid|keyword|fuzzy|bm25] [--top-k <n>] [--json] [--no-cache]
weave list <path> [--type <t>] [--tag <t>] [--status <s>] [--limit <n>] [--json] [--no-cache]
weave show <path> <id|file> [--json] [--no-cache]
weave backlinks <path> <id|file> [--json] [--no-cache]
weave path <path> <id|file> <id|file> [--json] [--no-cache]
weave audit <path> [--fail-on error|warning|none] [--verify-keys] [--json] [--no-cache]
weave relate <path> <id|file> [--depth <n>] [--direction out|in|both] [--json] [--no-cache]
weave explain <path> <query> [--id <id>] [--mode hybrid|keyword|fuzzy] [--top-k <n>] [--json] [--no-cache]
weave ingest <path> [--db <file>] [--strict|--permissive] [--json] [--no-cache]
weave rebuild <path> [--db <file>] [--strict|--permissive] [--json] [--no-cache]
weave sync <path> [--db <file>] [--strict|--permissive] [--json] [--no-cache]
weave version [--json]
weave --version | --help
```

## Machine Mode

`--json` emits exactly one `weave.cli/v1` envelope:

```json
{
  "contract": "weave.contract/v1",
  "cli": "weave.cli/v1",
  "ok": true,
  "result": {}
}
```

Rules:

- stdout contains only the envelope.
- stderr contains diagnostics and warnings.
- Failure envelopes are valid JSON and use the same contract.
- Exit codes:

| Code | Meaning |
| --- | --- |
| 0 | Success |
| 1 | Runtime error, no matches, unknown concept, no route |
| 2 | Usage error, invalid root, invalid search parameters |
| 3 | Permission denied |

- No network, daemon or interactive prompt.
- The parse cache is enabled by default; `--no-cache` disables it.

## Search Output

```sh
weave search <path> <query> --mode hybrid --top-k 10
```

- Mode defaults to `hybrid`; `--top-k` defaults to 10 and accepts 1..100.
- JSON responses always contain keyword, fuzzy and hybrid lists.
- Human output:
  - hybrid: `<total_score> <title> [<path>]`
  - keyword: `<score> <title> [<matched keywords>]`
  - fuzzy: `<score> <title> [<matched fields>]`
- Empty selected list exits 1 with `weave: no matches` on stderr, but still emits a valid
  success envelope in JSON mode.
- Invalid mode/top-K exits 2 with `INVALID_REQUEST`.

## Version Discovery

`weave contract --json` returns versions, operations and error codes. Hosts call it once per
binary and fail fast on an unsupported contract major.

## Deferred Modes

`weave engine --stdio` and any legacy output format remain outside v1.0. They are not part of the
current contract.

## Testing

- Golden human and envelope output.
- Process tests for every command, including stdout/stderr and exit codes.
- Search mode/top-K boundary tests.
- Cache hit, invalidation and `--no-cache` parity.
- Determinism tests.
- Contract discovery test.

## Related Documents

- [Operations layer](operations.md)
- [Engine layer](engine-go.md)
- [Keyword + Fuzzy Search spec](../../specs/keyword-fuzzy-search.md)
- [Integration contracts](../integration/contracts.md)
- [Integration testing](../integration/testing.md)
