# BDD: `weave sync --watch`

Human-readable behavior cases for the sync watch CLI wrapper.

**Spec reference:** [watch-mode.md](../specs/watch-mode.md)
**Go tests:** `cmd/weave/sync_watch_bdd_test.go`

The shared polling watcher already powers `weave mcp --watch`. This slice wires the same watcher into
`weave sync` with JSON events and clean shutdown.

---

## Feature: Configuration

### Scenario: Watch requires a database

- **When** I run `weave sync <path> --watch` without `--db`
- **Then** the command exits `2`
- **When** I add `--no-db`
- **Then** the command exits `2`

### Scenario: Watch requires an indexed bundle

- **Given** a database that does not contain the bundle
- **When** I run `weave sync <path> --watch --db <file> --json`
- **Then** the command exits `1`
- **And** the error code is `BUNDLE_NOT_INDEXED`

### Scenario: Unsupported flags are rejected

- **When** I combine `--watch` with `--with-enricher`
- **Then** the command exits `2`
- **When** I pass `--events` without `--watch`
- **Then** the command exits `2`

---

## Feature: Runtime

### Scenario: Initial sync runs before polling

- **Given** an indexed bundle
- **When** the watcher starts
- **Then** it prints one initial sync report
- **And** the report includes document counts

### Scenario: Changes are synced

- **Given** a running `weave sync --watch`
- **When** I change a document
- **Then** a later line reports `updated=1`
- **And** the database reflects the new content

### Scenario: JSON mode emits an envelope and JSONL events

- **Given** `--json --events`
- **When** the watcher starts
- **Then** stdout begins with one `weave.cli/v1` success envelope
- **When** a document changes
- **Then** stdout receives a `{"event":"sync", ...}` line
- **And** update counts are included

### Scenario: Shutdown is clean

- **Given** a running watcher
- **When** I send SIGINT or cancel its context
- **Then** `Run` returns without error
- **And** the exit code is `0`
