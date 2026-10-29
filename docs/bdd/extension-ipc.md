# BDD: Extension Subprocess IPC

Human-readable behavior cases for `weave-<name>` discovery, enrichment and reranking.

**Spec reference:** [extension-ipc.md](../specs/extension-ipc.md)
**Go tests:** `cmd/weave/extensions_bdd_test.go`

Extensions are one-shot subprocesses speaking `weave.cli/v1` JSON over stdin/stdout. Failures
degrade to core behavior with warnings; they never abort writes or searches.

---

## Feature: Discovery

### Scenario: Extensions are listed deterministically

- **Given** a bundle-local extension and a PATH extension
- **When** I run `weave extensions list <path> --json`
- **Then** the bundle-local extension wins on name conflicts
- **And** entries are sorted by name and source

### Scenario: Invalid names are rejected

- **When** I run `weave ingest --with-enricher ../escape`
- **Then** the command exits `2`
- **And** the error code is `EXTENSION_NOT_FOUND`
- **And** no database is written

### Scenario: Missing extension fails before writing

- **Given** no `weave-missing` binary on disk
- **When** I ingest with `--with-enricher missing`
- **Then** the command exits `2`
- **And** no database file exists

---

## Feature: Enrichment

### Scenario: Inferred keywords become searchable

- **Given** an extension returning `inferred_keywords`
- **When** I ingest with `--with-enricher echo`
- **Then** the stored keywords append inferred terms after declared ones
- **And** duplicate keywords are removed case-insensitively
- **And** `weave search` finds the inferred keyword

### Scenario: Enrichment metadata is persisted

- **Given** an extension returning `custom_metadata`
- **When** I ingest with the enricher
- **Then** the `enrichments` table holds the enricher id and response JSON

### Scenario: Failed enrichment falls back

- **Given** an extension that exits non-zero
- **When** I ingest with the enricher
- **Then** the write still succeeds
- **And** the report counts one warning
- **And** stderr names the extension
- **And** the stored keywords stay unchanged

### Scenario: Malformed response falls back

- **Given** an extension that prints invalid JSON
- **When** I ingest with the enricher
- **Then** the write still succeeds with a warning
- **And** no inferred keywords are stored

### Scenario: Timeouts fall back

- **Given** a slow extension
- **When** I ingest with `--timeout 50ms`
- **Then** the write finishes without waiting for the extension
- **And** the report counts a warning

### Scenario: Read-only commands never invoke extensions

- **Given** an extension that records invocations
- **When** I run `parse`, `show` or `search` without extension flags
- **Then** the extension is not invoked

### Scenario: Timeout flag requires an extension

- **When** I run `weave ingest --timeout 5s` without `--with-enricher`
- **Then** the command exits `2`

---

## Feature: Reranking

### Scenario: Rerank scores are blended and reordered

- **Given** three keyword hits and a reranker returning reverse scores
- **When** I search with `--rerank rerank`
- **Then** results are ordered by `0.5 * core + 0.5 * rerank`
- **And** every hit carries `rerank_score`

### Scenario: Rerank output is deterministic

- **Given** the same query and reranker
- **When** I run the search twice
- **Then** both JSON responses are identical

### Scenario: Failed reranker keeps core order

- **Given** a reranker that exits non-zero
- **When** I search with `--rerank`
- **Then** the core ordering is unchanged
- **And** stderr names the extension

### Scenario: Incomplete rerank response is rejected

- **Given** a reranker that omits candidates
- **When** I search with `--rerank`
- **Then** the core ordering is unchanged

### Scenario: Rerank timeout keeps core order

- **Given** a slow reranker
- **When** I search with `--timeout 50ms --rerank slow`
- **Then** the core ordering is returned promptly

### Scenario: Missing reranker fails fast

- **When** I search with `--rerank ghost`
- **Then** the command exits `2`
- **And** the error code is `EXTENSION_NOT_FOUND`
