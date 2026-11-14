---
type: behavior
title: "Generic Extension Output Boundary Behavior"
description: "Behavior cases for generic extension documents, relations, artifacts, namespaces, opaque options, legacy compatibility and behavioral preservation."
tags:
  - "behavior"
  - "extensions"
  - "contract"
  - "graph"
  - "source"
status: draft
generated:
  by: opencode/deepseek-v4.1-flash
---

# Behavior Scenarios: Generic Extension Output Boundary

Specification:
[extension-output-boundary.md](https://github.com/prulloac/weave-docs/blob/main/specs/extension-output-boundary.md)

Architecture decision:
[extension-owned documents and opaque options](https://github.com/prulloac/weave-docs/blob/main/architecture/adr/0012-extension-owned-documents-and-opaque-options.md)

Planned Go tests: `cmd/weave/extension_boundary_bdd_test.go`,
`engine/operations/extension_documents_test.go`, `engine/extensions/boundary_test.go`,
`mcp/workspace_test.go`, and the extension repositories' conformance tests.

These scenarios describe observable behavior. They do not prescribe internal implementation.

---

## Feature: Generic descriptors

### Scenario: A provider adds documents without a core change

- **Given** a provider that is not git and declares a document namespace
- **When** I ingest its snapshot
- **Then** its synthetic documents appear in the graph with the declared types and metadata
- **And** no core source-provider branch is required

### Scenario: An analyzer adds documents without a core change

- **Given** a project analyzer that emits a new document type and content kind
- **When** I run it during ingest
- **Then** the documents appear with the declared type and content kind
- **And** core does not map them to `Code`, `CodeSymbol` or `CodeExternal`

### Scenario: Scope creates containment

- **Given** a synthetic document with `scope: "web"`
- **When** the bundle is indexed
- **Then** that document contains the files under `web/`
- **And** a root-scope document contains every file
- **And** the deepest project owns a file when projects nest

### Scenario: Path relations resolve after parsing

- **Given** a relation with `to_path` pointing at an indexed file
- **When** the bundle is indexed
- **Then** the relation connects to the file document
- **And** a missing `to_path` target produces a warning and no edge

## Feature: Namespace and relation safety

### Scenario: Undeclared namespaces are rejected

- **Given** a provider that emits `_custom/thing` without declaring `_custom/`
- **When** I ingest the source
- **Then** ingestion fails with `EXTENSION_PROTOCOL`
- **And** no partial graph is written

### Scenario: Namespace collisions are rejected

- **Given** two active extensions declaring the same document namespace
- **When** either role is invoked
- **Then** core rejects the collision

### Scenario: Relation kinds are registered

- **Given** an extension declaring a `custom_edge` relation kind with `default: true`
- **When** its documents are indexed
- **Then** the relation is accepted and traversable
- **And** it joins default traversal for that invocation
- **And** an undeclared relation kind is rejected

### Scenario: Core relation vocabulary is reserved

- **Given** an extension tries to redefine `link`, `related` or `contains`
- **When** its manifest is read
- **Then** the manifest is rejected

## Feature: Opaque provider options

### Scenario: Provider options pass through unchanged

- **Given** a source request with `provider_options` for a provider-specific feature
- **When** the provider runs
- **Then** it receives the same keys and values
- **And** core does not interpret or reject unknown provider options

### Scenario: Invalid provider options are provider errors

- **Given** an invalid provider option value
- **When** the provider validates the request
- **Then** the provider returns a protocol error
- **And** core reports the provider failure without inventing a domain-specific message

### Scenario: CLI flags remain sugar

- **Given** a git source with `--git --worktree`
- **When** core invokes the provider
- **Then** the request contains the equivalent `provider_options`
- **And** user-visible git behavior is unchanged

## Feature: Source metadata and MCP

### Scenario: Source metadata is generic

- **Given** a source provider returning `source_metadata`
- **When** I run `weave source list --json`
- **Then** the registry exposes the provider and metadata map
- **And** git host, owner and URL values are ordinary metadata keys

### Scenario: MCP exposes generic provenance

- **Given** a workspace source with generic source metadata
- **When** an MCP client reads `weave://workspace/sources`
- **Then** it receives the provider and metadata
- **And** MCP does not require git-specific fields

## Feature: Artifacts

### Scenario: Extensions write artifacts

- **Given** an extension returning a JSON artifact under a declared artifact namespace
- **When** I export the bundle
- **Then** the artifact is written at the declared path
- **And** core does not special-case `_code` or `_sources`

### Scenario: Unsafe artifact paths are rejected

- **Given** an artifact path that is absolute or escapes the export root
- **When** I export the bundle
- **Then** export fails
- **And** no file is written outside the target

### Scenario: Duplicate artifact paths are rejected

- **Given** two extensions writing the same artifact path
- **When** I export the bundle
- **Then** export fails before writing either artifact

## Feature: Compatibility and behavioral preservation

### Scenario: Legacy source responses are rejected

- **Given** a provider returning `repository`, `projects` or `commits` without synthetic documents
- **When** I ingest the source
- **Then** ingestion fails with `EXTENSION_PROTOCOL`
- **And** no partial graph is written
- **And** the error tells the operator to upgrade the extension

### Scenario: Legacy project responses are rejected

- **Given** a code analyzer returning documents without `type` or `content_kind`
- **When** I ingest the project
- **Then** ingestion fails with `EXTENSION_PROTOCOL`
- **And** no `_code` fallback export is generated

### Scenario: Enrichment and reranking are unchanged

- **Given** the existing enrichment and rerank extension suites
- **When** the generic boundary is enabled
- **Then** inferred keywords, metadata, warnings, scores and fallbacks are unchanged

### Scenario: Codecs and crypto are unchanged

- **Given** the existing codec and external-crypto suites
- **When** the generic boundary is enabled
- **Then** codec selection, compression, recovery, key resolution and audit behavior are unchanged

### Scenario: AI, image and extension data are unchanged

- **Given** the existing AI, image and extension-data suites
- **When** the generic boundary is enabled
- **Then** redaction, requests, responses, artifacts and list/get/gc behavior are unchanged

### Scenario: The supported extension floor is explicit

- **Given** a core release with the generic boundary
- **When** the operator uses a descriptor-capable `weave-git` or `weave-code`
- **Then** ingestion and export succeed
- **And** older extension releases must be upgraded before use
- **And** the release notes state the minimum extension versions
