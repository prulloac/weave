# BDD: Watch-Based Reload

Human-readable behavior cases for the polling watcher and MCP reload.

**Spec reference:** [watch-mode.md](../specs/watch-mode.md)
**Go tests:** `watch/watcher_test.go`, `cmd/weave/mcp_test.go`

`fsnotify` optimization is deferred; this slice uses SHA-256 polling snapshots.

---

## Feature: Polling Watch Loop

### Scenario: Initial sync runs before watching

- **Given** an indexed bundle and a watcher
- **When** the watcher starts
- **Then** it runs one incremental sync
- **And** reports the sync result

### Scenario: Modified files update the database

- **Given** a running watcher
- **When** I change a document body
- **Then** the stored bundle eventually reflects the new body
- **And** the search index sees the new keyword

### Scenario: Deleted files are removed

- **Given** a running watcher
- **When** I delete a Markdown file
- **Then** the stored bundle no longer contains that concept

### Scenario: Burst edits are coalesced

- **Given** a running watcher
- **When** I write several files in quick succession
- **Then** the watcher reaches a consistent final snapshot
- **And** does not require one sync per write

### Scenario: Ignored directories do not trigger sync

- **Given** a running watcher
- **When** I create a Markdown file under `.git/`
- **Then** no sync event is emitted
- **And** the file never becomes a concept

### Scenario: Shutdown is clean

- **Given** a running watcher
- **When** I cancel its context
- **Then** `Run` returns without error
- **And** in-flight sync completes before exit

---

## Feature: MCP Reload

### Scenario: MCP search sees watch updates

- **Given** an indexed bundle, a running watcher and an MCP server
- **When** I change a document to include a new keyword
- **And** the watcher syncs it
- **Then** an MCP `weave_search` call for the new keyword returns the document

### Scenario: Queries remain available during reindex

- **Given** a watcher is syncing changes
- **When** MCP clients query the server concurrently
- **Then** queries use the last committed snapshot
- **And** no race is reported

---

## Feature: Watch Configuration

### Scenario: Watch requires persistent storage

- **When** I run `weave mcp --bundle <path> --watch` without `--db`
- **Then** the process exits `2`

### Scenario: Watch requires an indexed bundle

- **Given** a database without the bundle
- **When** the watcher starts
- **Then** it returns `BUNDLE_NOT_INDEXED`

### Scenario: Watch interval is configurable

- **Given** a watcher with a short interval
- **When** it runs
- **Then** it polls at that interval
