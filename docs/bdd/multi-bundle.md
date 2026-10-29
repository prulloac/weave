# BDD: Multiple Bundle Sources

Human-readable behavior cases for reading and writing several bundle roots in one invocation.

**Spec reference:** [multi-bundle.md](../specs/multi-bundle.md)
**Go tests:** `cmd/weave/multi_bundle_bdd_test.go`

Sources use aliases derived from the directory name or supplied as `alias=path`. Graph and links
stay bundle-scoped; search, list and audit merge deterministically.

---

## Feature: Source Resolution

### Scenario: Aliases derive from directory names

- **Given** a positional bundle `team-a` and `--source b=/path/team-b`
- **When** I run a multi-source command
- **Then** results use `team-a` and `b` as sources

### Scenario: Duplicate or invalid aliases are rejected

- **When** I reuse the primary alias for another source
- **Then** the command exits `2`
- **When** I pass an alias with uppercase or symbols
- **Then** the command exits `2`
- **When** I pass the same root twice
- **Then** the command exits `2`

### Scenario: Single-source output is unchanged

- **When** I run a command without `--source`
- **Then** results contain no `source` fields
- **And** existing JSON output is byte-identical

---

## Feature: Merged Reads

### Scenario: Search merges and orders deterministically

- **Given** two bundles with matching keywords
- **When** I search with one `--source`
- **Then** hits include both sources
- **And** ordering is score descending, then source, then id
- **And** the merged list is capped at top-K

### Scenario: List merges after filtering

- **When** I list with one `--source`
- **Then** entries are ordered by source then id
- **And** `--limit` applies after merging

### Scenario: Audit merges findings and summaries

- **When** I audit with one `--source`
- **Then** findings carry `source`
- **And** summary counts are summed across sources

### Scenario: Parse and graph return per-source payloads

- **When** I parse or graph with one `--source`
- **Then** the result contains a `sources` array
- **And** each entry carries the source alias and its bundle or graph

---

## Feature: Targets and Paths

### Scenario: Ambiguous targets fail

- **Given** the same id exists in two sources
- **When** I run `show` without an alias
- **Then** the command exits `2`
- **And** the error is `INVALID_REQUEST`

### Scenario: Qualified targets resolve

- **When** I run `show b:shared`
- **Then** the source field is `b`

### Scenario: Paths never cross sources

- **Given** a document in `team-a` linking to an id that exists only in `b`
- **When** I run `path` across both sources
- **Then** the command exits `2` with `INVALID_REQUEST`

---

## Feature: Multi-Source Writes

### Scenario: Ingest writes every source

- **When** I ingest with one `--source`
- **Then** the report aggregates document counts
- **And** `sources` lists per-source counts

### Scenario: Sync updates every source

- **When** I modify one source and sync both
- **Then** the aggregate report counts the update
- **And** per-source reports are present

### Scenario: Cross-source search works after ingest

- **When** I search with `--mode bm25`
- **Then** lexical hits from both sources are merged
