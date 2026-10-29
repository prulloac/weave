# BDD: Permissive Ingestion

Human-readable behavior cases for permissive Markdown ingestion.

**Spec reference:** [permissive-ingestion.md](../specs/permissive-ingestion.md)
**Go tests:** `cmd/weave/permissive_bdd_test.go`

`sync` integration is deferred until incremental sync lands; the scenarios below cover
`ingest`/`rebuild`, stored reads, search and audit.

---

## Feature: Ingest Arbitrary Markdown

As a user with an existing Markdown tree, I want to index it without converting every file to OKF.

### Scenario: Ingest defaults to permissive

- **Given** a bundle with valid and invalid Markdown documents
- **When** I run `weave ingest <bundle> --json`
- **Then** the process exits `0`
- **And** the report mode is `permissive`
- **And** the report contains 6 documents and 5 synthetic documents
- **And** the report contains 5 warnings and 0 errors

### Scenario: Explicit `--permissive` produces the same result

- **Given** the same bundle
- **When** I run `weave ingest <bundle> --permissive --json`
- **Then** the report mode is `permissive`
- **And** documents, synthetic, warnings and errors match the default run

### Scenario: Strict ingest rejects invalid documents

- **Given** the same bundle
- **When** I run `weave ingest <bundle> --strict --json`
- **Then** the process exits `1`
- **And** the report mode is `strict`
- **And** the report contains errors for the invalid documents
- **And** the report contains 0 synthetic documents

### Scenario: `--permissive` and `--strict` are mutually exclusive

- **When** I run `weave ingest <bundle> --permissive --strict --json`
- **Then** the process exits `2`
- **And** the failure envelope reports `INVALID_REQUEST`

---

## Feature: Deterministic Synthetic Documents

### Scenario: Missing metadata receives deterministic fallbacks

- **Given** the bundle is ingested permissively
- **When** I read `plain` from the database
- **Then** its type is `Document`
- **And** its keywords are empty
- **And** its title is `Plain Notes`
- **And** its description is the first non-heading paragraph
- **And** its original body is preserved

### Scenario: Missing type and missing keywords are handled separately

- **Given** the bundle is ingested permissively
- **When** I read `no-type` and `no-keywords`
- **Then** `no-type` has type `Document` and retains its keyword
- **And** `no-keywords` retains its type and has no keywords

### Scenario: An empty Markdown file becomes a synthetic document

- **Given** an empty Markdown file
- **When** the bundle is ingested permissively
- **Then** it appears with type `Document` and title `empty`
- **And** its body is empty

### Scenario: Reserved files are not concepts

- **Given** the bundle contains `index.md` and `log.md`
- **When** the bundle is ingested permissively
- **Then** neither `index` nor `log` appears as a concept

### Scenario: Re-ingest is deterministic

- **Given** the bundle is ingested permissively
- **When** I ingest it again
- **Then** document, synthetic, warning and error counts are unchanged

---

## Feature: Synthetic Documents in Search and Graph

### Scenario: Exact keyword search excludes synthetic documents

- **Given** the bundle is ingested permissively
- **When** I run `weave search <bundle> plain --mode keyword --json`
- **Then** the process exits `1`
- **And** no synthetic document is returned

### Scenario: Fuzzy and hybrid search include synthetic documents

- **Given** the bundle is ingested permissively
- **When** I run `weave search <bundle> plain --mode fuzzy --json`
- **And** I run `weave search <bundle> plain --json`
- **Then** `plain` appears in both result sets

### Scenario: Synthetic documents participate in the graph

- **Given** the bundle is ingested permissively
- **When** I run `weave graph <bundle> --json`
- **Then** `links` and `valid` are nodes
- **And** an edge `links → valid` exists

---

## Feature: Permissive Audit

### Scenario: Permissive audit downgrades synthetic findings to warnings

- **Given** the source bundle is not indexed
- **When** I run `weave audit <bundle> --permissive --no-db --json`
- **Then** the process exits `0`
- **And** missing frontmatter, type and keywords are warnings
- **And** there are no error-severity findings
- **And** `summary.valid` is `false`

### Scenario: Strict audit reports the same findings as errors

- **Given** the source bundle is not indexed
- **When** I run `weave audit <bundle> --no-db --json`
- **Then** the process exits `1`
- **And** missing frontmatter, type and keywords are errors

### Scenario: Permissive audit can still fail on warnings

- **Given** the source bundle is not indexed
- **When** I run `weave audit <bundle> --permissive --fail-on warning --no-db --json`
- **Then** the process exits `1`
- **And** the report still contains only warning-severity synthetic findings
