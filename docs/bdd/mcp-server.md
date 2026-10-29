# BDD: MCP Tool Harness

Human-readable behavior cases for the M1 in-process MCP tool layer.

**Spec reference:** [mcp-server.md](../specs/mcp-server.md)
**Go tests:** `mcp/server_test.go`

TCP, stdio, auth, resources and watch mode are transport-slice features.

---

## Feature: Tool Discovery

### Scenario: All MCP tools are advertised

- **Given** an in-process MCP server bound to a bundle
- **When** I list its tools
- **Then** the names are `weave_search`, `weave_explain`, `weave_relate`, `weave_audit`,
  `weave_show`, `weave_backlinks`, `weave_path` and `weave_contract`
- **And** each descriptor has an input schema
- **And** required fields match the tool contract

### Scenario: Search requires a query

- **Given** the tool descriptors
- **When** I inspect `weave_search`
- **Then** `query` is required and `mode`, `top_k` are optional

### Scenario: Path requires both endpoints

- **When** I inspect `weave_path`
- **Then** `from` and `to` are required

---

## Feature: Tool Dispatch Matches Operations

### Scenario: Search matches the operations layer

- **Given** an in-process server over the standard fixture bundle
- **When** I call `weave_search` with `graph`
- **Then** the result equals the direct search operation payload
- **And** the default top-K is used

### Scenario: Each read tool matches operations

- **Given** an in-process server over the standard fixture bundle
- **When** I call each read tool
- **Then** each result equals the corresponding operations payload

### Scenario: Audit findings are a successful call

- **Given** a bundle with audit findings
- **When** I call `weave_audit`
- **Then** the call succeeds
- **And** the result contains the findings report

### Scenario: Contract discovery is exposed

- **When** I call `weave_contract`
- **Then** the result contains the operation descriptors

---

## Feature: Validation and Errors

### Scenario: Unknown tool is rejected

- **When** I call `weave_unknown`
- **Then** the call fails with `INVALID_REQUEST`

### Scenario: Missing required argument is rejected

- **When** I call `weave_search` without `query`
- **Then** the call fails with `INVALID_REQUEST`

### Scenario: Wrong argument type is rejected

- **When** I call `weave_search` with a string `top_k`
- **Then** the call fails with `INVALID_REQUEST`

### Scenario: Invalid mode is rejected

- **When** I call `weave_search` with `mode: "semantic"`
- **Then** the call fails with `INVALID_REQUEST`

### Scenario: Unknown concept maps to CONCEPT_NOT_FOUND

- **When** I call `weave_show` for a missing concept
- **Then** the call fails with `CONCEPT_NOT_FOUND`

### Scenario: Invalid relate depth is rejected

- **When** I call `weave_relate` with `depth: 6`
- **Then** the call fails with `INVALID_REQUEST`

### Scenario: Tool calls are read-only and bundle-bound

- **Given** an in-process server bound to one bundle
- **When** I call any tool
- **Then** no tool accepts a bundle path argument
- **And** the source bundle remains unchanged
