# BDD: MCP Resources

Human-readable behavior cases for MCP bundle resources.

**Spec reference:** [mcp-server.md](../specs/mcp-server.md)
**Go tests:** `mcp/resource_test.go`

---

## Feature: Resource Discovery

### Scenario: Initialize advertises resources

- **Given** an initialized stdio session
- **Then** initialize capabilities contain `resources`

### Scenario: Static resources are listed

- **When** I send `resources/list`
- **Then** the result contains `weave://bundle/index`
- **And** the result contains `weave://bundle/stats`

### Scenario: Resource templates are listed

- **When** I send `resources/templates/list`
- **Then** the result contains the concept and neighborhood templates

---

## Feature: Resource Reads

### Scenario: Read the root index

- **When** I read `weave://bundle/index`
- **Then** the content is an `IndexEntry[]`

### Scenario: Read bundle stats

- **When** I read `weave://bundle/stats`
- **Then** the content includes concept, edge, orphan and index counts
- **And** the validation flag

### Scenario: Read a concept

- **When** I read `weave://concept/concepts/okf-bundle`
- **Then** the content is a `ShowResult`

### Scenario: Read a graph neighborhood

- **When** I read `weave://graph/neighborhood/concepts/okf-bundle`
- **Then** the content is a `RelateResult`

### Scenario: Resource payloads are JSON text

- **When** I read any resource
- **Then** `mimeType` is `application/json`
- **And** the text parses as the expected payload

---

## Feature: Resource Errors

### Scenario: Unknown resource is rejected

- **When** I read `weave://bundle/missing`
- **Then** the response is JSON-RPC `-32602`
- **And** the error data code is `RESOURCE_NOT_FOUND`

### Scenario: Reads before initialize are rejected

- **When** I read a resource before initialize
- **Then** the response is `-32002 NOT_INITIALIZED`

### Scenario: Resources are read-only

- **Given** an initialized session
- **When** I read resources
- **Then** no method takes a bundle path
- **And** the source bundle is unchanged
