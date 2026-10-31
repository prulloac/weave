---
type: behavior
title: "Release Readiness Behavior Scenarios"
description: "Behavior cases for the documentation integrity gate, built-binary smoke suite and release gate."
tags:
  - "behavior"
  - "release"
  - "testing"
  - "documentation"
generated:
  by: opencode/deepseek-v4.1-flash
  at: "2026-10-31T00:00:00Z"
status: stable
---

# Behavior Scenarios: Release Readiness

Specification: [release-readiness.md](../specs/release-readiness.md)

Go tests: `cmd/docs-lint/main_test.go`, `cmd/weave/smoke_test.go`,
`release/release_test.go`.

These scenarios describe observable behavior. They do not prescribe implementation details.

## Feature: Documentation integrity

### Scenario: A dead link fails the documentation gate

- **Given** a Markdown document that links to `missing.md`
- **When** `cmd/docs-lint` runs
- **Then** it reports `DL001` with the document and line
- **And** it exits non-zero

### Scenario: A missing anchor fails the documentation gate

- **Given** a Markdown document that links to `architecture/index.md#no-such-heading`
- **When** `cmd/docs-lint` runs
- **Then** it reports `DL002` with the document and line
- **And** it exits non-zero

### Scenario: Missing required frontmatter fails the documentation gate

- **Given** a specification without a `title` or `status`
- **When** `cmd/docs-lint` runs
- **Then** it reports `DL003` for that file
- **And** it exits non-zero

### Scenario: An invalid status value fails the documentation gate

- **Given** a specification whose frontmatter status is `shipped`
- **When** `cmd/docs-lint` runs
- **Then** it reports `DL004` for that file
- **And** it exits non-zero

### Scenario: A duplicate body status line fails the documentation gate

- **Given** a specification with `Status: **Proposed**` in the body
- **When** `cmd/docs-lint` runs
- **Then** it reports `DL005` for that file
- **And** it exits non-zero

### Scenario: An ADR numbering gap fails the documentation gate

- **Given** ADR files `0001-one.md` and `0003-three.md`
- **When** `cmd/docs-lint` runs
- **Then** it reports `DL006` for the missing `0002`
- **And** it exits non-zero

### Scenario: An out-of-order feature register fails the documentation gate

- **Given** a Feature Register with IDs `1`, `3`, `2`
- **When** `cmd/docs-lint` runs
- **Then** it reports `DL007` for the register
- **And** it exits non-zero

### Scenario: A register and backlog mismatch fails the documentation gate

- **Given** a register feature with no matching backlog row
- **When** `cmd/docs-lint` runs
- **Then** it reports `DL008` naming the missing row
- **And** it exits non-zero

### Scenario: A spec without a BDD set fails the documentation gate

- **Given** a specification with no behavior scenario set and no exemption
- **When** `cmd/docs-lint` runs
- **Then** it reports `DL009` for that spec
- **And** it exits non-zero

### Scenario: An orphan document fails the documentation gate

- **Given** a document under `docs/` that no index links
- **When** `cmd/docs-lint` runs
- **Then** it reports `DL010` for that document
- **And** it exits non-zero

### Scenario: A dangling ADR reference fails the documentation gate

- **Given** a document that references `ADR 9999`
- **When** `cmd/docs-lint` runs
- **Then** it reports `DL011` with the reference and line
- **And** it exits non-zero

### Scenario: The release repository passes the documentation gate

- **Given** the release branch
- **When** `go run ./cmd/docs-lint` runs
- **Then** every rule passes
- **And** the command exits zero
- **And** it prints the number of checked documents and links

## Feature: Built-binary smoke suite

### Scenario: A clean environment can ingest, search and navigate

- **Given** a temporary HOME, cache and database
- **And** a valid bundle
- **When** the built binary runs `ingest`
- **Then** it reports the expected document count
- **And** `search` returns results in keyword, fuzzy, hybrid and BM25 modes
- **And** `explain`, `audit`, `relate`, `show`, `backlinks` and `path` succeed
- **And** every JSON response validates against its committed schema

### Scenario: Sync handles no-op, change, delete and rename

- **Given** an indexed bundle
- **When** `sync` runs with no changes
- **Then** the report shows every document skipped
- **When** one document changes, one is deleted and one is renamed
- **Then** the report shows the updated, removed and added counts
- **And** a subsequent search sees the new state

### Scenario: Privacy and export round-trip

- **Given** a bundle with encrypted, tokenized and hashed content
- **When** `show --decrypt` runs with the key
- **Then** the plaintext is returned
- **When** `export --format encrypted` runs
- **Then** the export can be ingested again
- **And** `audit --verify-keys` reports the expected key findings

### Scenario: MCP stdio serves the same operations

- **Given** the built binary started with `mcp --stdio`
- **When** the client sends `initialize`, `tools/list` and `weave_search`
- **Then** the responses use the MCP protocol
- **And** encrypted fields are redacted by default
- **When** `resources/list` is called
- **Then** the bundle resources are returned

### Scenario: A smoke step failure is diagnostic

- **Given** a deliberately broken binary or fixture
- **When** the smoke suite runs
- **Then** it exits non-zero
- **And** it prints the failing step, the exact command, stdout and stderr

### Scenario: A release artifact passes the smoke suite

- **Given** an extracted release archive
- **When** the smoke suite runs with `WEAVE_SMOKE_BINARY` pointing at its binary
- **Then** `version --json` reports the tagged version
- **And** `contract --json` succeeds
- **And** the full smoke scenario passes

## Feature: Release gate

### Scenario: Local and CI gates are identical

- **Given** a clean checkout
- **When** `./scripts/verify.sh` runs locally
- **Then** it runs the same functional checks as CI
- **And** it includes the smoke suite, documentation gate, staticcheck and govulncheck
- **And** it exits zero only when every check passes

### Scenario: A flaky or order-dependent test blocks the release

- **Given** the release candidate
- **When** the gate runs three times with `-count=1 -shuffle=on`
- **Then** all three runs pass
- **And** any failure blocks the tag

### Scenario: Documentation drift blocks the release

- **Given** a release candidate with a dead link, stale status or register mismatch
- **When** the release workflow runs the gate
- **Then** the release job stops before building artifacts

### Scenario: A dry run builds artifacts without creating a release

- **Given** a `workflow_dispatch` run with a version
- **When** the workflow completes
- **Then** the full build matrix and archives are produced
- **And** no GitHub release is created

### Scenario: Publishing stays manual

- **Given** a tagged release candidate
- **When** the workflow completes
- **Then** a draft release exists with archives, checksums and build info
- **And** publishing requires an explicit maintainer action
