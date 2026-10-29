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
status: stable
---

# Specification: Weave MCP Server

Status: **M1–M3 tools/transport/auth and M4 core resources implemented**. Watch reload, sensitive
access and remaining polish are pending.

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

## 6. Resources

Resources are read-only and use the same initialize/auth gate as tools.

### 6.1 Static resources

| URI | Payload |
| --- | --- |
| `weave://bundle/index` | `IndexEntry[]` |
| `weave://bundle/stats` | bundle counts and validation summary |

### 6.2 Resource templates

| URI template | Payload |
| --- | --- |
| `weave://concept/{id}` | `ShowResult` |
| `weave://graph/neighborhood/{id}` | `RelateResult` |

- `{id}` may contain slash-separated path segments and is URL-unescaped before lookup.
- Resource contents are JSON text with `mimeType: application/json`.
- Resources never accept a bundle path.
- Unknown resources return JSON-RPC `-32602` with data code `RESOURCE_NOT_FOUND`.
- Calls before initialize return `-32002 NOT_INITIALIZED`.

### 6.3 Methods

- `resources/list` returns `{"resources": [...]}`.
- `resources/templates/list` returns `{"resourceTemplates": [...]}`.
- `resources/read` takes `{"uri": "..."}` and returns `{"contents": [...]}`.
- `initialize` advertises `capabilities.resources`.

## 7. Transport and Protocol

### 7.1 Framing

- Stdio and TCP both use **one JSON-RPC 2.0 object per line** (newline-delimited JSON).
- Stdio reads stdin and writes stdout; diagnostics go to stderr.
- TCP accepts concurrent connections; each connection has its own initialize state.
- Default maximum frame size is **1 MiB**. Oversized frames return a parse error and close the
  connection.
- Stdio exits cleanly on EOF. TCP connections close on EOF or write failure.
- TLS termination is the responsibility of the operator/reverse proxy.

### 7.2 Methods

```text
initialize
notifications/initialized
tools/list
tools/call
ping
```

`initialize` request parameters:

```json
{
  "protocolVersion": "2025-06-18",
  "capabilities": {},
  "clientInfo": {"name": "test", "version": "1"},
  "_meta": {"token": "secret"}
}
```

`initialize` result:

```json
{
  "protocolVersion": "2025-06-18",
  "capabilities": {"tools": {}},
  "serverInfo": {"name": "weave", "version": "dev"}
}
```

- `notifications/initialized` emits no response.
- `tools/list` returns `{"tools": [...]}`.
- `tools/call` takes `{"name": "<tool>", "arguments": {...}}`.
- Success result:

```json
{
  "content": [{"type": "text", "text": "<JSON result>"}],
  "structuredContent": {},
  "isError": false
}
```

- Tool failures use `isError: true` and `structuredContent: {"code": "...", "message": "..."}`.
- Unknown methods return JSON-RPC `-32601`; invalid parameters `-32602`; malformed JSON `-32700`.
- Calls before `initialize` return `-32002 NOT_INITIALIZED`.
- `ping` returns an empty result.

### 7.3 Authentication

- TCP default bind is `127.0.0.1:7331`.
- A token is required for any non-loopback bind.
- When a token is configured, `initialize._meta.token` must match using constant-time comparison.
- A missing/invalid token returns JSON-RPC `-32001` with data code `UNAUTHORIZED` and closes the
  connection.
- Stdio may also require a token if configured.

### 7.4 CLI

```sh
weave mcp --bundle <path> [--db <file>] [--top-k-default 10] [--allow-sensitive]
weave mcp --bundle <path> --stdio
weave mcp --bundle <path> --listen 127.0.0.1:7331 [--token <token>]
```

- `--stdio` and `--listen` are mutually exclusive.
- `--watch` remains part of the watch-mode slice.
- Non-loopback `--listen` without `--token` is a usage error.

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

### M1 — MCP schema and in-process harness (implemented)

- Define tool descriptors and JSON schemas from operations.
- Build an in-process MCP request/response harness.
- Conformance tests for tool inputs/outputs.
- No network yet.

### M2 — Local stdio (implemented)

- `weave mcp --stdio` for local single-client use.
- Reuse the same tool descriptors and error mapping.
- Useful for early agent integrations before TCP.

### M3 — TCP MCP server (implemented)

- `weave mcp --bundle <path> --listen 127.0.0.1:7331 --token <token>`.
- Optional `--db` binding to persistent storage.
- Token auth, lifecycle, concurrency and reconnect tests.
- MCP client integration tests.
- Documentation for Claude/agent clients.

### M4 — Resources and polish (resources implemented)

- Bundle/index/stats resources.
- Concept and graph-neighborhood resources.
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
