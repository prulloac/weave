---
type: behavior
title: "Extension Data Behavior Scenarios"
description: "Behavior cases for opaque extension-owned artifacts, workspace import, bindings, reads, export and lifecycle."
tags:
  - "behavior"
  - "extensions"
  - "assets"
  - "storage"
  - "post-ga"
generated:
  by: opencode/deepseek-v4.1-flash
  at: "2026-10-31T00:00:00Z"
status: stable
---

# Behavior Scenarios: Extension Data

Specification: [extension-data.md](https://github.com/prulloac/weave-docs/blob/main/specs/extension-data.md)

Go tests: `cmd/weave/extension_data_bdd_test.go`, `cmd/weave/export_bdd_test.go`,
`engine/extensions/data_test.go`, `engine/operations/extension_data_test.go`,
`storage/sqlite/extension_data_test.go`, `mcp/extension_data_test.go`.

These scenarios describe observable behavior. They do not prescribe implementation details.

## Feature: Publishing opaque data

### Scenario: An extension publishes an image during ingest

- **Given** an indexed Markdown document that references an image
- **When** an enricher writes the image bytes and a manifest into its workspace
- **Then** ingest succeeds
- **And** `weave show <bundle> <doc> --json` lists one extension data reference
- **And** the reference includes the extension, key, media type, size and SHA-256
- **And** `show` does not embed the image bytes

### Scenario: Weave does not parse the artifact

- **Given** a published PNG
- **When** I run `weave list`, `weave graph` or `weave search`
- **Then** the image is not a document, graph node or search hit
- **And** the host Markdown document remains the only concept
- **And** its extension-added keywords are searchable

### Scenario: The artifact round-trips exactly

- **Given** a published artifact with a known SHA-256
- **When** I run `weave extensions data get <bundle> <extension> <key> --out <file> --json`
- **Then** the output file SHA-256 equals the reference
- **And** the JSON envelope contains the same metadata

### Scenario: Identical bytes are stored once

- **Given** two documents publish identical artifact bytes under different keys
- **When** both imports succeed
- **Then** both metadata rows exist
- **And** the sidecar contains one blob for that SHA-256

## Feature: Manifest and workspace validation

### Scenario: A valid manifest imports all items

- **Given** a manifest with several files and metadata
- **When** the enricher exits successfully
- **Then** every item is stored
- **And** each item is bound to the current document

### Scenario: An empty manifest removes previous data

- **Given** a document with existing extension data
- **When** the enricher publishes a valid empty manifest
- **Then** the previous data set for that extension and document is removed
- **And** unreferenced blobs are garbage-collected

### Scenario: No manifest keeps previous data

- **Given** a document with existing extension data
- **When** the enricher exits successfully without writing a manifest
- **Then** the previous data set is unchanged

### Scenario: An invalid manifest imports nothing

- **Given** a manifest with a duplicate key, missing file or bad hash
- **When** the enricher exits successfully
- **Then** no item from that manifest is stored
- **And** the previous data set is unchanged
- **And** one warning is written to stderr

### Scenario: Path traversal is rejected

- **Given** a manifest path containing `../` or an absolute path
- **When** the enricher runs
- **Then** the import fails safely
- **And** the error names `EXTENSION_DATA_INVALID`
- **And** no file outside the workspace is read

### Scenario: Symlinks and special files are rejected

- **Given** a workspace file that is a symlink, hardlink, device or FIFO
- **When** the enricher runs
- **Then** the import fails safely
- **And** no bytes are stored

### Scenario: Limits are enforced

- **Given** a file larger than 32 MiB, more than 256 items, or a metadata object over 8 KiB
- **When** the enricher runs
- **Then** the import is skipped
- **And** the warning names the limit
- **And** the document itself is still indexed

### Scenario: Quotas are enforced

- **Given** a document or bundle over its data quota
- **When** the enricher publishes more data
- **Then** the excess import is rejected
- **And** existing valid data is unchanged
- **And** the warning names the quota

## Feature: Reading data

### Scenario: List filters by extension and document

- **Given** several extensions publish data
- **When** I run `weave extensions data list --extension images --doc <id> --json`
- **Then** only matching metadata rows are returned
- **And** no bytes are returned

### Scenario: Get requires an output file

- **Given** a published artifact
- **When** I run `weave extensions data get <bundle> <extension> <key> --json` without `--out`
- **Then** the command exits `2`
- **And** the error code is `OUTPUT_REQUIRED`
- **And** no binary data is written to stdout

### Scenario: Existing output requires force

- **Given** an existing target file
- **When** I run `data get --out <file>` without `--force`
- **Then** the command exits `2`
- **And** the error code is `OUTPUT_EXISTS`
- **And** the existing file is unchanged
- **When** I add `--force`
- **Then** the file is replaced

### Scenario: Missing data fails cleanly

- **Given** no matching extension data
- **When** I run `data get`
- **Then** the command exits `1`
- **And** the error code is `EXTENSION_DATA_NOT_FOUND`
- **And** no partial file is created

### Scenario: MCP returns the same bytes

- **Given** a published artifact
- **When** an MCP client reads `weave://extension/<name>/<key>`
- **Then** the resource content is a blob
- **And** its decoded SHA-256 equals the metadata hash

## Feature: Lifecycle

### Scenario: Sync replaces a document's data set

- **Given** a document with extension data
- **When** the document changes and sync runs with the enricher
- **Then** the new manifest replaces the old data set for that document
- **And** old unreferenced blobs are garbage-collected

### Scenario: Deleting a document cascades

- **Given** a document with extension data
- **When** I delete the Markdown file and run sync
- **Then** the document metadata is removed
- **And** its extension data rows are removed
- **And** unreferenced blobs are garbage-collected

### Scenario: Rebuild without an enricher removes data

- **Given** a bundle with extension data
- **When** I run `weave rebuild` without `--with-enricher`
- **Then** all extension data for the bundle is removed
- **And** unreferenced blobs are garbage-collected

### Scenario: Garbage collection is safe

- **Given** referenced and orphaned blobs
- **When** I run `weave extensions data gc --dry-run`
- **Then** only orphaned blobs are reported
- **When** I run it without `--dry-run`
- **Then** only orphaned blobs are deleted

## Feature: Export and compatibility

### Scenario: Raw export copies extension data

- **Given** a bundle with published artifacts
- **When** I run `weave export --format raw`
- **Then** `_extensions/<extension>/<key>` exists
- **And** `_extensions/manifest.json` contains the same hashes and metadata
- **And** each exported file hashes to the recorded SHA-256

### Scenario: Encrypted export excludes opaque data

- **Given** a bundle with published artifacts
- **When** I run `weave export --format encrypted`
- **Then** the export succeeds with one warning
- **And** no extension data files are written

### Scenario: Default behavior is unchanged

- **Given** a bundle without extension data
- **When** I run ingest, search, show, audit or export
- **Then** the output is byte-identical to the current release
- **And** no `extension_data` fields are emitted

### Scenario: No native asset surface exists

- **Given** the CLI and MCP surfaces
- **When** I list commands, tools and resource templates
- **Then** there is no native `asset`, `image` or `attachment` command or tool
- **And** the only surface is the generic extension data API
