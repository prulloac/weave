# BDD: MCP Multiple Sources

Human-readable behavior cases for serving several bundle roots through one MCP server.

**Spec reference:** [multi-bundle.md](../specs/multi-bundle.md)
**Go tests:** `mcp/multi_test.go`, `cmd/weave/mcp_test.go`

MCP sources mirror the CLI: repeatable `--source`, deterministic merged reads, qualified targets and
`source` tool arguments. Resources add a `weave://source/<alias>/...` namespace.

---

## Feature: Server Sources

### Scenario: Multiple sources are configured

- **Given** `weave mcp --bundle a --source b=/notes/b`
- **When** the server starts
- **Then** tools can read both sources

### Scenario: Source flags are repeatable

- **When** I pass two `--source` flags
- **Then** the parser keeps both entries in order

---

## Feature: Tools

### Scenario: Search merges sources

- **When** I call `weave_search` without a source
- **Then** hits include both sources
- **And** each hit carries `source`

### Scenario: List and audit merge

- **When** I call `weave_list` or `weave_audit`
- **Then** entries and findings carry `source`
- **And** summaries aggregate across sources

### Scenario: Ambiguous show fails

- **Given** the same id exists in two sources
- **When** I call `weave_show` without `source`
- **Then** the call fails with `INVALID_REQUEST`

### Scenario: Source scopes a tool call

- **When** I call `weave_show` with `source: "b"`
- **Then** the result comes from `b`
- **And** the result carries `source`

### Scenario: Explain accepts source

- **When** I call `weave_explain` with `source`
- **Then** the explanation is scoped to that source

---

## Feature: Resources

### Scenario: Source resources resolve

- **When** I read `weave://source/b/concept/common`
- **Then** the payload is the concept from `b`
- **And** it carries `source: "b"`

### Scenario: Unknown source resources fail

- **When** I read `weave://source/missing/concept/common`
- **Then** the read fails with a resource error

### Scenario: Existing resources target the primary source

- **When** I read `weave://concept/common`
- **Then** the concept comes from the primary bundle

---

## Feature: Tool Schema

### Scenario: Every tool accepts a source

- **Then** each advertised tool schema has an optional `source` property
