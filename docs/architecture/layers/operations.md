---
type: Layer Specification
title: "Operations Layer"
description: "Transport-neutral request/response operations shared by the CLI today and a future TCP MCP server."
tags:
  - "architecture"
  - "operations"
  - "cli"
  - "mcp"
  - "go"
generated:
  by: opencode/deepseek-v4.1-flash
  at: "2026-10-25T00:00:00Z"
status: draft
---

# Operations Layer

## Purpose

The operations layer defines what Weave can do, independent of how a caller asks. It exists so the
CLI and a future TCP MCP server share one request/response model, one error model and one set of
semantics.

```text
engine/operations
   ↑            ↑
cmd/weave     future weave mcp
(argv)        (TCP, MCP tools)
```

No HTTP handler and no transport types appear in this package.

## Package Layout

```text
engine/operations/
├── request.go       # Request, Op, path/query/filter fields
├── response.go      # Response envelope, per-op result types
├── errors.go        # operation error codes
├── execute.go       # dispatch and validation
└── descriptors.go   # machine-readable operation catalogue
```

## Request Model

```go
type Op string

const (
    OpContract  Op = "contract"
    OpParse     Op = "parse"
    OpGraph     Op = "graph"
    OpFind      Op = "find"
    OpList      Op = "list"
    OpShow      Op = "show"
    OpBacklinks Op = "backlinks"
    OpPath      Op = "path"
    OpVersion   Op = "version"
)

type Request struct {
    ID      string   `json:"id,omitempty"`      // caller-supplied, echoed back
    Op      Op       `json:"op"`
    Path    string   `json:"path,omitempty"`    // bundle root for bundle operations
    Query   string   `json:"query,omitempty"`   // find
    Target  string   `json:"target,omitempty"`  // show, backlinks
    From    string   `json:"from,omitempty"`    // path
    To      string   `json:"to,omitempty"`      // path
    Types   []string `json:"types,omitempty"`   // list
    Tags    []string `json:"tags,omitempty"`    // list
    Status  string   `json:"status,omitempty"`  // list
    Limit   int      `json:"limit,omitempty"`   // find, list
    Fields  []string `json:"fields,omitempty"`  // find
}
```

Rules:

- `op` is required and must be one of the catalogue values.
- `path` is required for every bundle operation.
- Unknown fields are ignored for forward compatibility.
- No operation mutates the bundle. `parse` and `graph` build an in-memory view; `find`, `list`,
  `show`, `backlinks` and `path` query it.
- Requests are stateless. A one-shot process may use the on-disk cache; no request depends on a
  prior request.
- Callers must not send secrets in request fields; paths and ids may appear in diagnostics.

## Response Model

Success:

```json
{
  "id": "req-1",
  "contract": "weave.contract/v1",
  "cli": "weave.cli/v1",
  "ok": true,
  "result": {}
}
```

Failure:

```json
{
  "id": "req-1",
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

- `contract` and `cli` versions are always present.
- `id` is echoed only when supplied.
- `ok` is always present in machine mode.
- Exactly one of `result` or `error` is present.
- Result payloads use the domain contract; see [Contract layer](contract.md).
- Error codes are stable and transport-neutral. HTTP statuses and CLI exit codes are mapped by
  the adapters.

## Operation Catalogue

| Op | Purpose | Required fields | Result |
| --- | --- | --- | --- |
| `contract` | Describe contract, CLI and operations | — | `ContractInfo` |
| `parse` | Parse a bundle | `path` | `Bundle` |
| `graph` | Build the graph | `path` | `Graph` |
| `find` | Rank full-text search results | `path`, `query` | `FindResult[]` |
| `list` | Filter concepts by metadata | `path` | `ListResult[]` |
| `show` | Show a concept with links/backlinks | `path`, `target` | `ShowResult` |
| `backlinks` | Incoming edges for a concept | `path`, `target` | `BacklinksResult` |
| `path` | Shortest directed path | `path`, `from`, `to` | `PathResult` |
| `version` | Binary and contract versions | — | `VersionInfo` |

Optional fields are the filters and limits in the request model. Invalid combinations return
`INVALID_REQUEST` before any filesystem work.

## Contract and Version Discovery

`contract` returns machine-readable metadata so a host can negotiate without parsing `--help`:

```json
{
  "contract": "weave.contract/v1",
  "cli": "weave.cli/v1",
  "binary_version": "0.1.0",
  "operations": [
    {"op": "parse", "fields": ["path"], "result": "Bundle"},
    {"op": "find", "fields": ["path", "query", "limit", "fields"], "result": "FindResult[]"}
  ],
  "errors": ["INVALID_REQUEST", "CONCEPT_NOT_FOUND", "PATH_NOT_FOUND", "PARSE_ERROR", "INTERNAL"]
}
```

A host should call `contract` once, cache the result for the binary version, and fail fast on an
unsupported contract major.

## One-Shot vs Stdio

### One-shot

v1 ships one-shot only. The default integration mode is one process per operation:

```sh
weave find /notes "graph" --json
```

Characteristics:

- No daemon, no port, no network.
- Fresh process state; optional on-disk cache may accelerate repeated calls.
- Simple error handling through exit codes and stdout.
- Startup cost is a Go process start plus parse; acceptable for build-time and interactive use.

### Stdio (deferred post-v1)

This mode is designed but **not shipped in v1**. After the one-shot CLI stabilizes, a long-lived
process may read NDJSON requests from stdin and write NDJSON responses to stdout:

```sh
weave engine --stdio
```

```json
{"id":"1","op":"find","path":"/notes","query":"graph"}
{"id":"1","contract":"weave.contract/v1","cli":"weave.cli/v1","ok":true,"result":[]}
```

Rules:

- One JSON object per line; no pretty printing.
- stdout carries only responses; diagnostics go to stderr.
- Requests are processed in order. Concurrency is a future option.
- EOF or a `shutdown` request exits the process.
- Still no network listener; this is process-local CLI I/O.
- The stdio mode may reuse a parsed engine across requests, keyed by `path` and refreshed when
  the bundle changes or on an explicit `reload` operation (future).

## Error Codes

| Code | Meaning | CLI exit | HTTP (presentation only) |
| --- | --- | --- | --- |
| `INVALID_REQUEST` | Missing/unknown operation or field | 2 | 400 |
| `INVALID_ROOT` | Path missing or not a directory | 2 | 404 |
| `PERMISSION_DENIED` | Filesystem permission failure | 3 | 403 |
| `PARSE_ERROR` | Bundle cannot be parsed at all | 1 | 422 |
| `CONCEPT_NOT_FOUND` | Unknown concept target | 1 | 404 |
| `PATH_NOT_FOUND` | No route between concepts | 1 | 404 |
| `INTERNAL` | Unexpected failure | 1 | 500 |

The operations package returns codes; adapters own the exit-code and HTTP mappings.

## Future MCP Mapping

When the TCP MCP server is added:

- each operation becomes an MCP tool with the same name,
- the request struct defines the tool input schema,
- the response envelope defines the tool result,
- `contract` becomes the discovery call,
- the transport adds framing, authentication and lifecycle only.

No engine, render or operation semantics should need to change for MCP.

## Testing

1. Table tests for every operation and validation error.
2. Golden request/response fixtures shared with the CLI tests.
3. Determinism tests: identical requests produce identical envelopes.
4. Version negotiation: unsupported contract major returns `INVALID_REQUEST`.
5. Future MCP adapter contract tests reuse the same fixtures.
6. Post-v1: stdio tests cover NDJSON framing, ordering, malformed lines, EOF and stdout purity.

## Related Documents

- [ADR 0002](../adr/0002-v1-scope-layout-and-contract.md)
- [Contract layer](contract.md)
- [Engine layer](engine-go.md)
- [CLI layer](cli-go.md)
- [Integration contracts](../integration/contracts.md)
- [Integration steps](../integration/steps.md)
- [Integration testing](../integration/testing.md)
