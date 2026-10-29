# BDD: MCP stdio/TCP Transport

Human-readable behavior cases for the M2/M3 JSON-RPC transport.

**Spec reference:** [mcp-server.md](../specs/mcp-server.md)
**Go tests:** `mcp/transport_test.go`

Both transports use newline-delimited JSON-RPC 2.0.

---

## Feature: Initialize and Discover Tools

### Scenario: Stdio initialize advertises tools

- **Given** an in-process MCP server over the standard fixture bundle
- **When** I send `initialize` with a valid token
- **And** I send `tools/list`
- **Then** initialize returns the protocol version, `tools` capability and server info
- **And** tools list contains all eight Weave tools

### Scenario: Initialized notification has no response

- **Given** an initialized stdio session
- **When** I send `notifications/initialized`
- **And** I send `tools/list`
- **Then** only the tools/list response is emitted

### Scenario: Calls before initialize are rejected

- **When** I send `tools/list` before `initialize`
- **Then** the response is JSON-RPC `-32002 NOT_INITIALIZED`

---

## Feature: Tool Calls

### Scenario: Tool call returns structured content

- **Given** an initialized session
- **When** I call `weave_search` with `graph`
- **Then** `isError` is `false`
- **And** `structuredContent` is a SearchResponse
- **And** the text content is the same JSON payload

### Scenario: Tool failure is a tool error, not a protocol crash

- **Given** an initialized session
- **When** I call `weave_show` for a missing concept
- **Then** the JSON-RPC response has no protocol error
- **And** the result has `isError: true`
- **And** `structuredContent.code` is `CONCEPT_NOT_FOUND`

---

## Feature: Protocol Errors

### Scenario: Malformed JSON returns a parse error

- **When** I send an invalid JSON line
- **Then** the response is JSON-RPC `-32700`
- **And** the connection stays usable

### Scenario: Unknown method returns method-not-found

- **When** I send `weave_magic`
- **Then** the response is JSON-RPC `-32601`

### Scenario: Ping returns an empty result

- **When** I send `ping`
- **Then** the response has an empty result

---

## Feature: Authentication

### Scenario: Missing token is unauthorized

- **Given** a server configured with a token
- **When** I initialize without a token
- **Then** the response is JSON-RPC `-32001` with `UNAUTHORIZED`

### Scenario: Wrong token is unauthorized

- **Given** a server configured with a token
- **When** I initialize with the wrong token
- **Then** the response is JSON-RPC `-32001` with `UNAUTHORIZED`

---

## Feature: TCP Transport

### Scenario: TCP initialize and tools/list

- **Given** a TCP listener on an ephemeral loopback port
- **When** a client connects and initializes
- **Then** the client can list tools and call `weave_search`

### Scenario: Concurrent TCP clients

- **Given** two TCP clients connected to the same server
- **When** both initialize and call `weave_search`
- **Then** both receive the same result payload
