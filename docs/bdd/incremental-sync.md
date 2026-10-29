# BDD: Incremental Sync

Human-readable behavior cases for hash-based incremental sync.

**Spec reference:** [storage-and-sync.md](../specs/storage-and-sync.md)
**Go tests:** `cmd/weave/sync_bdd_test.go`

Watch mode and extension enrichment are out of scope for this slice.

---

## Feature: Hash-Based Sync

### Scenario: No changes reports every document as skipped

- **Given** a three-document bundle ingested into the default database
- **When** I run `weave sync <bundle> --strict --json`
- **Then** the process exits `0`
- **And** the report contains `added: 0`, `updated: 0`, `removed: 0`, `skipped: 3`
- **And** `documents` is `3`

### Scenario: Touching a file without changing content still skips it

- **Given** an ingested bundle
- **When** I change one file's modification time without changing its bytes
- **And** I run `weave sync <bundle> --strict --json`
- **Then** `updated` is `0`
- **And** `skipped` is `3`

### Scenario: Changed content updates exactly one document

- **Given** an ingested bundle
- **When** I change one document's title, keyword and body
- **And** I run `weave sync <bundle> --strict --json`
- **Then** `updated` is `1` and `skipped` is `2`
- **And** a database-backed `parse` shows the new body
- **And** exact keyword search finds the new keyword

### Scenario: A new file is added

- **Given** an ingested three-document bundle
- **When** I create a fourth valid document
- **And** I run `weave sync <bundle> --strict --json`
- **Then** `added` is `1` and `skipped` is `3`
- **And** a database-backed `parse` returns four concepts

### Scenario: A deleted file is removed

- **Given** an ingested three-document bundle
- **When** I delete one document
- **And** I run `weave sync <bundle> --strict --json`
- **Then** `removed` is `1` and `skipped` is `2`
- **And** a database-backed `parse` returns two concepts

### Scenario: A rename is an addition plus a removal

- **Given** an ingested three-document bundle
- **When** I rename one Markdown file
- **And** I run `weave sync <bundle> --strict --json`
- **Then** `added` is `1` and `removed` is `1`
- **And** `skipped` is `2`
- **And** the old concept ID is gone and the new one is present

### Scenario: No-op sync leaves source files untouched

- **Given** an ingested bundle
- **When** I run `weave sync <bundle> --strict --json`
- **Then** every source file remains byte-identical

---

## Feature: Cascade and Dangling Links

### Scenario: Deleting a link target leaves the source link dangling

- **Given** a bundle where `b` links to `c`, and both are ingested
- **When** I delete `c` and run `weave sync <bundle> --strict --json`
- **Then** the database-backed bundle no longer contains `c`
- **And** `b` still exists
- **And** `weave audit <bundle> --json` reports `BROKEN_LINK` and exits `1`

---

## Feature: Sync Errors and Mode

### Scenario: Sync requires an indexed bundle

- **Given** bundle A is indexed and bundle B is not
- **When** I run `weave sync <bundle B> --json`
- **Then** the process exits `1`
- **And** the failure envelope reports `BUNDLE_NOT_INDEXED`

### Scenario: Sync requires an existing database

- **When** I run `weave sync <bundle> --db <missing> --json`
- **Then** the process exits `2`
- **And** the failure envelope reports `INVALID_REQUEST`

### Scenario: Sync rejects `--no-db`

- **When** I run `weave sync <bundle> --no-db --json`
- **Then** the process exits `2`
- **And** the failure envelope reports `INVALID_REQUEST`

### Scenario: Sync preserves the stored permissive mode

- **Given** a bundle ingested permissively
- **When** I run `weave sync <bundle> --json` without a mode flag
- **Then** the report mode is `permissive`
- **And** all synthetic documents are counted as skipped
