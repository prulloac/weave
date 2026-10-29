---
type: specification
title: "Weave MCP Server Specification"
description: "Roadmap and contract for exposing Weave's search, relate, explain and audit operations as an MCP server over TCP."
tags:
  - "specification"
  - "mcp"
  - "server"
  - "transport"
  - "roadmap"
generated:
  by: opencode/deepseek-v4.1-flash
  at: "2026-10-26T00:00:00Z"
status: draft
---

# Specification: Weave MCP Server

## 1. Overview

The MCP server exposes Weave to agents through the Model Context Protocol. It wraps the existing
transport-neutral operations layer; it does not reimplement parsing, graph or search.

```text
engine/operations
   ├── cmd/weave     (CLI, current)
   └── cmd/weave-mcp (MCP, roadmap)
```

The first target is a **TCP MCP server** started explicitly by the operator. Stdio may be added as
a local convenience, but it is not the primary plan. The server can wrap either the in-memory
engine or the persistent store; the operations and tool schemas stay the same.

## 2. Goals and Non-Goals

### Goals

- Expose search, relate, explain, audit, show, backlinks and path as MCP tools.
- Load one bundle once and serve many read-only calls.
- Keep the same scoring, validation and determinism as the CLI.
- Authenticate TCP clients with a token.
- Default to localhost-only binding.
- Return structured JSON results with stable error codes.

### Non-Goals

- Remote multi-tenant hosting.
- Write operations or bundle mutation.
- Arbitrary filesystem access per request.
- An HTTP REST API.
- A UI.
- Embeddings or semantic search.

## 3. Process Model

```sh
weave mcp --bundle <path> [--db <file>] [--watch] [--listen 127.0.0.1:7331] [--token <token>] [--top-k-default 10] [--allow-sensitive]
```

- `--bundle` is required and fixed for the process lifetime.
- `--listen` defaults to `127.0.0.1:7331`.
- `--token` is required for non-loopback binding. For loopback, a token is optional but recommended.
- `--db` uses the persistent SQLite store; without it, the in-memory engine is used.
- `--watch` enables watch-based reload.
- `--allow-sensitive` is required to expose encrypted/tokenized fields in tool results.
- The server loads the bundle once at startup and exits on SIGINT/SIGTERM.
- No request may choose a different bundle path.

## 4. M1 In-Process Tool Contract

M1 delivers the tool layer without networking. It is the contract that M2 stdio and M3 TCP reuse.

```go
type Options struct {
    Bundle       string
    DB           string // empty means in-memory
    TopKDefault  int
    AllowSensitive bool
}

type Server struct { /* ... */ }

func New(executor *operations.Executor, options Options) (*Server, error)
func (s *Server) Tools() []ToolDescriptor
func (s *Server) Call(ctx context.Context, name string, arguments map[string]any) (any, error)
```

- `Bundle` is fixed at construction; tools never take a path.
- `DB` empty selects in-memory operation; otherwise the persistent store is used.
- `TopKDefault` defaults to `10`; tool `top_k` has the same valid range `1..100` as search.
- `Call` builds the equivalent `OperationRequest` and returns the same payload as the CLI
  envelope’s `result`.
- Operation failures become MCP errors with the same stable code and message:

```json
{
  "code": "INVALID_REQUEST",
  "message": "top_k must be between 1 and 100"
}
```

- Unknown tools, missing required arguments and wrong argument types return `INVALID_REQUEST`.
- `weave_audit` is successful even when the report contains findings.
- `weave_contract` returns the operations contract.
- M1 has no sockets, tokens, resources or watch mode.

## 5. MCP Tools

| Tool | Input | Result |
| --- | --- | --- |
| `weave_search` | `query`, `mode?`, `top_k?` | `SearchResponse` |
| `weave_explain` | `query`, `id?`, `mode?`, `top_k?` | `ExplainResult` |
| `weave_relate` | `id`, `depth?`, `direction?` | `RelateResult` |
| `weave_audit` | `fail_on?` | `AuditReport` |
| `weave_show` | `id` | `ShowResult` |
| `weave_backlinks` | `id` | `BacklinksResult` |
| `weave_path` | `from`, `to` | `PathResult` |
| `weave_contract` | — | `ContractInfo` |

Rules:

- Inputs use the same names and semantics as the operations contract.
- Results are the same payloads as the CLI JSON envelope `result`.
- Errors map to MCP tool errors with `INVALID_REQUEST`, `CONCEPT_NOT_FOUND`, `PATH_NOT_FOUND`,
  `INTERNAL`, etc.
- Tools are read-only.
- `weave_audit` returning findings is a successful tool call; the report contains its own `ok`.
- `top_k` defaults to the process default and is capped at 100.

## 6. Resources (optional, later)

- `weave://bundle/index` — root index entries.
- `weave://bundle/stats` — bundle counts and validation summary.
- `weave://concept/{id}` — concept metadata and body.
- `weave://graph/neighborhood/{id}` — relate output.

Resources are read-only and may be deferred until tools are stable.

## 7. Transport and Protocol

- MCP over TCP, JSON-RPC framed per the MCP specification.
- One bundle per server process.
- The server advertises capabilities and tool schemas at initialize.
- Requests are processed concurrently with read-only engine access.
- The server rejects unknown methods and malformed frames without crashing.
- Frame size and request count limits are configurable.
- TLS termination is the responsibility of the operator/reverse proxy in v1; a future option may add
  native TLS.

## 8. Security

- Bind to `127.0.0.1` by default.
- Require a bearer token for any non-loopback bind.
- Constant-time token comparison.
- No arbitrary paths, shell commands or writes.
- Limit tool output size; `top_k` and `relate` depth are bounded.
- Log connection and tool metadata, never document bodies by default.
- Encrypted fields require `--allow-sensitive`; tokenized/hashed fields follow the privacy spec.
- Graceful shutdown on SIGINT/SIGTERM.

## 9. Roadmap

### M0 — Operations stabilization

- Implement `weave audit`, `weave relate` and `weave explain`.
- Add contract types, operations, CLI and tests.
- Freeze the operation schemas used by MCP.

### M1 — MCP schema and in-process harness

- Define tool descriptors and JSON schemas from operations.
- Build an in-process MCP request/response harness.
- Conformance tests for tool inputs/outputs.
- No network yet.

### M2 — Local stdio (optional)

- `weave mcp --stdio` for local single-client use.
- Reuse the same tool descriptors and error mapping.
- Useful for early agent integrations before TCP.

### M3 — TCP MCP server

- `weave mcp --bundle <path> --listen 127.0.0.1:7331 --token <token>`.
- Optional `--db`/`--watch` binding to persistent storage.
- Token auth, lifecycle, concurrency and reconnect tests.
- MCP client integration tests.
- Documentation for Claude/agent clients.

### M4 — Resources and polish

- Bundle/index/stats resources.
- Rate/output limits.
- Metrics/logging.
- Packaging and service examples.

## 10. Testing

- Tool schema snapshots.
- In-process dispatch tests.
- Auth and non-loopback guard tests.
- MCP initialize/list-tools/call-tool tests.
- Concurrent calls with `-race`.
- Graceful shutdown and reconnect tests.
- Result equivalence with CLI operations.

## 11. Acceptance Criteria

- All tools return the same payloads as the CLI operations.
- Unknown tools and invalid inputs return structured MCP errors.
- A non-loopback bind without a token is refused.
- The server is read-only and bundle-bound.
- Conformance and race tests pass.

## Related Documents

- [Operations layer](../architecture/layers/operations.md)
- [CLI layer](../architecture/layers/cli-go.md)
- [Keyword + Fuzzy Search spec](./keyword-fuzzy-search.md)
- [Storage & Sync](./storage-and-sync.md)
- [Privacy & Extensions](./privacy-and-extensions.md)
- [Weave Audit spec](./weave-audit.md)
- [Weave Relate spec](./weave-relate.md)
- [Weave Explain spec](./weave-explain.md)
- [ROADMAP](../../ROADMAP.md)
