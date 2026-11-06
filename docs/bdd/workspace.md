# BDD: Workspace and Source Registry

Human-readable behavior cases for a durable multi-source workspace: a manifest, a persisted source
registry, CLI management commands, and an MCP server that re-reconciles sources at runtime.

**Spec reference:** [workspace.md](https://github.com/prulloac/weave-docs/blob/main/specs/workspace.md)
**Go tests:** `cmd/weave/workspace_bdd_test.go`, `storage/sqlite/schema_v8_test.go`, `mcp/workspace_test.go`

A workspace groups named bundle sources under one index database. Sources are added and removed
through the CLI or manifest edits; MCP serves the workspace read-only and never accepts a
client-provided path.

---

## Feature: Manifest and Discovery

### Scenario: Workspace is discovered from the manifest

- **Given** a directory containing `weave.workspace.yaml`
- **When** I run a workspace command from inside it without `--workspace`
- **Then** the manifest is found and used

### Scenario: Missing manifest fails

- **Given** no `weave.workspace.yaml` in the current directory or its ancestors
- **When** I run `weave workspace show`
- **Then** the command exits `2` with `WORKSPACE_NOT_FOUND`

### Scenario: Invalid manifest fails

- **Given** `weave.workspace.yaml` with an unknown `version`
- **When** I run a workspace command
- **Then** the command exits `2` with `WORKSPACE_INVALID`

### Scenario: Relative and home paths resolve

- **Given** a manifest with a source path `../other` and `~/notes`
- **When** the manifest is loaded
- **Then** each source root resolves to an absolute path against the manifest directory

---

## Feature: CLI Workspace and Sources

### Scenario: Workspace init writes a minimal manifest

- **When** I run `weave workspace init <dir>`
- **Then** `weave.workspace.yaml` is created with `version: 1` and an empty `sources`
- **And** an existing manifest is not overwritten

### Scenario: Source add registers a source

- **When** I run `weave source add docs ../weave-docs`
- **Then** the manifest gains a `docs` source
- **And** `weave source list` reports `docs` with status `active` after indexing

### Scenario: Source add rejects duplicates

- **When** I add two sources with the same alias
- **Then** the command exits `2` with `WORKSPACE_INVALID`
- **When** I add two aliases for the same absolute path
- **Then** the command exits `2` with `WORKSPACE_INVALID`

### Scenario: Reserved and malformed aliases fail

- **When** I add an alias `contract`, `weave` or `workspace`
- **Then** the command exits `2` with `INVALID_REQUEST`
- **When** I add an alias with uppercase or symbols
- **Then** the command exits `2` with `INVALID_REQUEST`

### Scenario: Source remove unregisters a source

- **Given** a workspace with an active source `notes`
- **When** I run `weave source remove notes`
- **Then** `notes` leaves the registry and is no longer served
- **And** its index rows are retained
- **And** with `--prune` its index rows are deleted

### Scenario: Source remove of an unknown alias fails

- **When** I run `weave source remove missing`
- **Then** the command exits `1` with `SOURCE_NOT_FOUND`

### Scenario: Workspace and bundle are exclusive

- **When** I pass both `--workspace` and `--bundle`
- **Then** the command exits `2` with `INVALID_REQUEST`

---

## Feature: Registry and Storage

### Scenario: Registry persists across restarts

- **Given** a workspace with sources `docs` and `notes`
- **When** the index is reopened in a new process
- **Then** `weave source list` still reports both aliases and their status

### Scenario: Migration backfills aliases

- **Given** a v7 database with two bundle roots
- **When** it is opened as a workspace
- **Then** a `sources` table is created
- **And** each root gains an alias derived from its base name
- **And** name collisions get `-2`, `-3` suffixes

---

## Feature: MCP Workspace Mode

### Scenario: MCP serves the workspace

- **Given** `weave mcp --workspace .`
- **When** the server starts
- **Then** tools read the active sources
- **And** the advertised tools are unchanged

### Scenario: Registry is exposed with counts

- **When** I call `weave_sources`
- **Then** the result lists each source with `alias`, `path`, `status`, `concepts`, `edges`,
  `index_entries` and `valid`

### Scenario: Workspace resources resolve

- **When** I read `weave://workspace/sources`
- **Then** the payload is the registry JSON with counts
- **When** I read `weave://workspace/concepts`
- **Then** the payload lists concepts across active sources with `source`
- **When** I read `weave://workspace/stats`
- **Then** the payload contains aggregate and per-source counts
- **When** I read `weave://source/docs/concept/common`
- **Then** the concept comes from the `docs` source

### Scenario: Workspace bundle resources are explicit

- **When** I read `weave://bundle/index` or `weave://bundle/stats` in workspace mode
- **Then** the read fails with `INVALID_REQUEST`
- **And** the message points to the workspace or source form

### Scenario: Empty bundle index is an array

- **Given** a non-workspace bundle with no root `index.md`
- **When** I read `weave://bundle/index`
- **Then** the payload is `[]`, not `null`

### Scenario: MCP never accepts a client path

- **When** I call `weave_search` with a `path` or `bundle` argument
- **Then** the call fails with `INVALID_REQUEST`

---

## Feature: Re-reconciliation

### Scenario: A new source is served without restart

- **Given** a running `weave mcp --workspace . --watch`
- **When** `weave source add notes ~/notes` is run
- **Then** `notes` becomes `active` and appears in `weave_sources`
- **And** `weave_search` returns hits from `notes`

### Scenario: A removed source stops being served

- **Given** a running server and a source `notes`
- **When** `weave source remove notes` is run
- **Then** `notes` leaves `weave_sources`
- **And** `weave://source/notes/...` resources return not found
- **And** other sources keep serving

### Scenario: A failed reconcile keeps the healthy set

- **Given** a running server with sources `docs` and `notes`
- **When** the manifest is changed to reference an unreadable path
- **Then** `docs` and `notes` keep serving
- **And** an error is reported, not a crash

---

## Feature: Lifecycle and Writer Lock

### Scenario: A source is indexing before it is active

- **When** a source is added but its first sync is not complete
- **Then** its status is `indexing`
- **And** it is not served by MCP tools

### Scenario: A degraded source serves its last snapshot

- **Given** an active source whose watch sync later fails
- **When** MCP reads it
- **Then** its status is `degraded`
- **And** the last snapshot is served with a warning

### Scenario: Only one writer holds the index

- **Given** a process holds the workspace index lock
- **When** a second process tries to write
- **Then** it exits `1` with `INDEX_LOCKED`
- **And** readers are unaffected

---

## Feature: Ephemeral Mode Parity

### Scenario: Bundle and source flags keep working

- **When** I run `weave mcp --bundle a --source b=/notes/b`
- **Then** behavior matches the multi-bundle contract
- **And** no manifest or registry is created

### Scenario: Single-source output is unchanged

- **When** I run commands without a workspace
- **Then** existing JSON output is byte-identical
