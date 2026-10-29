# BDD: SQLite Storage Foundation

Human-readable behavior cases for the persistent storage foundation.

**Spec reference:** [storage-and-sync.md](../specs/storage-and-sync.md)
**Go tests:** `cmd/weave/storage_bdd_test.go`

Incremental `sync` and watch mode are out of scope for this slice.

All scenarios use `--strict`; permissive mode is covered by its own BDD suite.

---

## Feature: Ingest Into the Default Database

As a knowledge worker, I want my bundle indexed once so that reads do not reparse Markdown.

### Scenario: Ingest creates the default database

- **Given** an OKF bundle with three concepts and three edges
- **When** I run `weave ingest <bundle> --json`
- **Then** the process exits `0`
- **And** the report names `$HOME/.weave/index.db`
- **And** the report names the absolute bundle root
- **And** the report contains `documents: 3`, `edges: 3`
- **And** the default database file exists

### Scenario: Repeated ingest is idempotent

- **Given** a bundle already ingested into the default database
- **When** I run `weave ingest <bundle> --json` again
- **Then** the document and edge counts are unchanged
- **And** a database-backed `parse` still returns exactly three concepts

### Scenario: Ingest never writes into the source bundle

- **Given** a copied bundle with known file contents
- **When** I run `weave ingest <bundle> --json`
- **Then** every source file remains byte-identical
- **And** no `.weave` directory is created inside the bundle

---

## Feature: Persistence Is On by Default

As a CLI user, I want reads to use the database unless I explicitly opt out.

### Scenario: A read uses the database after the sources are removed

- **Given** a bundle ingested into the default database
- **When** I delete the source directory
- **And** I run `weave parse <bundle> --json`
- **Then** the process exits `0`
- **And** the result still contains three concepts
- **And** the command does not reparse Markdown sources

### Scenario: `--no-db` uses the transient in-memory parser

- **Given** a bundle that has not been ingested
- **When** I run `weave parse <bundle> --no-db --json`
- **Then** the process exits `0`
- **And** the default database is not created

### Scenario: `--db` overrides the default path

- **Given** a custom database path
- **When** I run `weave ingest <bundle> --db <custom> --json`
- **Then** the report names the custom path
- **And** the custom database exists
- **And** the default database does not exist
- **When** I run `weave parse <bundle> --db <custom> --json`
- **Then** the result contains three concepts

### Scenario: Database-backed reads match in-memory reads

- **Given** a bundle ingested into the default database
- **When** I run `weave search <bundle> alpha --json`
- **And** I run `weave search <bundle> alpha --no-db --json`
- **Then** both results are identical

---

## Feature: The Database Can Hold Multiple Bundles

As a user with several projects, I want one index database without projects overwriting each other.

### Scenario: Two bundles coexist

- **Given** bundle A with three concepts and bundle B with two concepts
- **When** I ingest both into the default database
- **Then** a database-backed `parse` of A returns three concepts
- **And** a database-backed `parse` of B returns two concepts

### Scenario: Re-ingesting one bundle leaves the other intact

- **Given** bundles A and B already ingested
- **When** I ingest A again
- **Then** a database-backed `parse` of B still returns two concepts

### Scenario: Rebuild only affects its bundle

- **Given** bundles A and B already ingested
- **When** I run `weave rebuild <bundle A> --json`
- **Then** the rebuild report contains three documents and three edges
- **And** a database-backed `parse` of B still returns two concepts

---

## Feature: Explicit Persistence Errors

### Scenario: Missing database is a usage error

- **Given** a database path that does not exist
- **When** I run `weave parse <bundle> --db <missing> --json`
- **Then** the process exits `2`
- **And** the failure envelope reports `INVALID_REQUEST`

### Scenario: Unindexed bundle is reported explicitly

- **Given** bundle A ingested into the default database
- **And** bundle B not ingested
- **When** I run `weave parse <bundle B> --json`
- **Then** the process exits `1`
- **And** the failure envelope reports `BUNDLE_NOT_INDEXED`

### Scenario: `--db` and `--no-db` are mutually exclusive

- **When** I run `weave parse <bundle> --db <file> --no-db --json`
- **Then** the process exits `2`
- **And** the failure envelope reports `INVALID_REQUEST`

### Scenario: Ingest rejects `--no-db`

- **When** I run `weave ingest <bundle> --no-db --json`
- **Then** the process exits `2`
- **And** the failure envelope reports `INVALID_REQUEST`

### Scenario: Invalid root is rejected

- **When** I run `weave ingest <missing-root> --json`
- **Then** the process exits `2`
- **And** the failure envelope reports `INVALID_ROOT`
