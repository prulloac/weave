---
type: Layer Specification
title: "Operations Layer"
description: "Transport-neutral search, parse, graph and navigation operations shared by the CLI and future transports."
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

The operations layer defines what Weave can do independently of transport. The CLI uses it today;
a future MCP server can reuse it without changing engine semantics.

```text
engine/operations
   ↑            ↑
cmd/weave     future weave mcp
(argv)        (TCP, MCP tools)
```

## Request Model

```go
type OperationRequest struct {
    ID     string
    Op     OperationName
    Path   string
    Query  string
    Mode   SearchMode   // keyword | fuzzy | hybrid
    TopK   *int         // 1..100, default 10
    Target string
    From   string
    To     string
    Types  []string
    Tags   []string
    Status string
    Limit  *int
}
```

Rules:

- `op` is required.
- Bundle operations require `path`.
- `search` requires `query`; `mode` defaults to `hybrid`; `top_k` defaults to 10.
- Unknown fields are ignored.
- Requests are stateless; a one-shot process may use the parse cache.

## Operation Catalogue

| Op | Required fields | Result |
| --- | --- | --- |
| `contract` | — | `ContractInfo` |
| `parse` | `path` | `Bundle` |
| `graph` | `path` | `Graph` |
| `search` | `path`, `query` | `SearchResponse` |
| `list` | `path` | `ListResults` |
| `show` | `path`, `target` | `ShowResult` |
| `backlinks` | `path`, `target` | `BacklinksResult` |
| `path` | `path`, `from`, `to` | `PathResult` |
| `version` | — | `VersionInfo` |

`find` is removed.

## Response Envelope

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
    "message": "concept not found: missing",
    "details": {"target": "missing"}
  }
}
```

- Exactly one of `result` or `error` is present.
- Error codes are transport-neutral; the CLI maps them to exit codes.
- `search` validation errors (bad mode or top-K) return `INVALID_REQUEST`.

## Volume and One-Shot Behavior

v1 ships one process per operation. The optional stdio mode remains deferred post-v1. The
operations model is stable enough to add it without changing semantics.

## Error Codes

| Code | Meaning | CLI exit |
| --- | --- | --- |
| `INVALID_REQUEST` | Missing/unknown op or invalid search parameters | 2 |
| `INVALID_ROOT` | Path missing or not a directory | 2 |
| `PERMISSION_DENIED` | Filesystem permission failure | 3 |
| `PARSE_ERROR` | Bundle cannot be parsed | 1 |
| `CONCEPT_NOT_FOUND` | Unknown concept target | 1 |
| `PATH_NOT_FOUND` | No route between concepts | 1 |
| `INTERNAL` | Unexpected failure | 1 |

## Testing

- Dispatch tests for every operation.
- Search mode/top-K validation tests.
- Envelope success/failure tests.
- CLI process tests reuse the same fixtures.
- Future MCP contract tests reuse the same request/response model.

## Related Documents

- [Contract layer](contract.md)
- [Engine layer](engine-go.md)
- [CLI layer](cli-go.md)
- [Keyword + Fuzzy Search spec](../../specs/keyword-fuzzy-search.md)
- [Integration contracts](../integration/contracts.md)
