---
type: behavior
title: "Weave Code Extension Behavior Scenarios"
description: "Behavior cases for source discovery, project analysis, code relations, symbol documents and extension failure handling."
tags:
  - "behavior"
  - "code"
  - "extensions"
  - "graph"
status: draft
generated:
  by: opencode/deepseek-v4.1-flash
---

# Behavior Scenarios: Weave Code Extension

Specification: [weave-code.md](https://github.com/prulloac/weave-docs/blob/main/specs/weave-code.md)

Architecture decision: [extension-provided graph relations and project analysis](https://github.com/prulloac/weave-docs/blob/main/architecture/adr/0010-extension-provided-graph-relations-and-project-analysis.md)

Go tests (planned): `cmd/weave/weave_code_bdd_test.go`, `engine/assets/code_test.go`,
`engine/graph/relations_test.go`, and the standalone `weave-code` repository tests.

These scenarios describe observable behavior. They do not prescribe implementation details.

## Feature: Source discovery

### Scenario: Code scanning requires the code enricher

- **Given** a repository containing Go and Python source files
- **When** I run `weave ingest <repository>` without `--with-enricher code`
- **Then** no source file becomes a document
- **And** search output is identical to a Markdown-only ingest

### Scenario: The code enricher discovers known source extensions

- **Given** a repository containing `.go`, `.py`, `.rs` and `.java` files
- **When** I run `weave ingest <repository> --with-enricher code`
- **Then** each known source file becomes a `Code` document
- **And** I do not need `--assets` or `--asset-types`

### Scenario: Unknown text files stay out

- **Given** a repository containing `notes.txt`, `data.csv` and `main.go`
- **When** I run the code enricher
- **Then** `main.go` becomes a `Code` document
- **And** `notes.txt` and `data.csv` do not become code documents

### Scenario: Language media types survive content sniffing

- **Given** a Go source file with valid UTF-8 text
- **When** it is ingested
- **Then** its `media_type` is `text/x-go`
- **And** no media mismatch warning is emitted

### Scenario: Oversized source files are skipped

- **Given** a source file larger than 4 MiB
- **When** the project is ingested
- **Then** the file is skipped with a warning
- **And** the rest of the project is indexed

### Scenario: Generated files are skipped by default

- **Given** a repository with `api.gen.go` and `api.go`
- **When** I run the code enricher with default configuration
- **Then** `api.go` is indexed
- **And** `api.gen.go` is not indexed

### Scenario: Generated files can be opted in

- **Given** `.weave/code.yaml` sets `parse_generated: true`
- **When** I run the code enricher
- **Then** `api.gen.go` is indexed

## Feature: File documents

### Scenario: A source file is a text document

- **Given** a Go source file
- **When** it is ingested
- **Then** its `content_kind` is `blob`
- **And** its `doc_type` is `Code`
- **And** its original source is searchable and retrievable with `weave show --out`

### Scenario: File metadata carries language and symbols

- **Given** a Python module with functions and imports
- **When** it is ingested
- **Then** its title names the module
- **And** its description comes from the module documentation
- **And** its keywords include the language, module and exported symbols
- **And** `content_metadata` names the language, module, parser and symbol count

### Scenario: Parse failure keeps the file

- **Given** a source file with invalid syntax
- **When** it is ingested
- **Then** it remains a `Code` document
- **And** its metadata marks the parser result as fallback
- **And** one warning is reported

## Feature: Project analysis

### Scenario: Cross-file imports resolve to documents

- **Given** `src/main.go` imports `src/server.go`
- **When** the project is analyzed
- **Then** an `imports` edge points from `src/main.go` to `src/server.go`

### Scenario: Cross-file calls resolve to symbols

- **Given** `src/main.go#main` calls `src/server.go#Run`
- **When** symbol documents are enabled
- **Then** a `calls` edge points from `src/main.go#main` to `src/server.go#Run`

### Scenario: Inheritance and implementation resolve

- **Given** a Java class `Server` that implements `Handler`
- **When** symbol documents are enabled
- **Then** an `implements` edge points from `Server` to `Handler`

### Scenario: One external reference stays in metadata

- **Given** a Go file imports `fmt` once
- **When** the project is analyzed
- **Then** no `_code/external/go/fmt` document exists
- **And** the import remains in file metadata

### Scenario: Repeated external reference becomes a document

- **Given** five Go files import `fmt`
- **When** the project is analyzed
- **Then** `_code/external/go/fmt` exists as a `CodeExternal` document
- **And** `imports` edges point to it from the importing files

### Scenario: A failed project analysis keeps the previous index

- **Given** an indexed project with code relations
- **When** the next analysis returns a malformed response in permissive mode
- **Then** the previous code relations remain
- **And** one warning is reported

## Feature: Symbol documents

### Scenario: Symbol documents are off by default

- **Given** a source file with several functions
- **When** I run the code enricher without configuration
- **Then** only the file document exists
- **And** no `#` document exists

### Scenario: Environment or configuration enables symbol documents

- **Given** `WEAVE_CODE_SYMBOL_DOCUMENTS=1`
- **When** I ingest the project
- **Then** a `CodeSymbol` document exists for each supported symbol
- **And** its ID uses `<file>#<symbol>`

### Scenario: Symbol documents are searchable

- **Given** symbol documents are enabled
- **When** I search for a function name
- **Then** the symbol document appears in the results
- **And** its body contains the signature and doc comment, not the whole file

### Scenario: Symbol documents are not exported as source files

- **Given** symbol documents are enabled
- **When** I export the bundle in raw format
- **Then** `_code/symbols.json` and `_code/relations.json` are written
- **And** no file is written at a `#` path
- **And** one warning is emitted per excluded synthetic document

## Feature: Graph relations

### Scenario: The graph reports code relation kinds

- **Given** a fully analyzed project
- **When** I run `weave graph --json`
- **Then** edges carry `imports`, `declares`, `references`, `calls`, `inherits` and `implements`
  where applicable

### Scenario: Default traversal excludes calls and references

- **Given** a symbol with a call edge to another symbol
- **When** I run `weave relate <symbol> --depth 1`
- **Then** the call target is not included
- **And** `imports`, `declares`, `inherits` and `implements` edges are included

### Scenario: Relation filter includes calls

- **Given** the same symbol
- **When** I run `weave relate <symbol> --depth 1 --relation calls`
- **Then** the call target is included

### Scenario: Path respects the relation filter

- **Given** two symbols connected only by calls
- **When** I run `weave path <from> <to>` without `--relation`
- **Then** no path is found
- **When** I run `weave path <from> <to> --relation calls`
- **Then** the path is found

### Scenario: Audit counts code edges

- **Given** a code file with imports and declared symbols
- **When** I run `weave audit`
- **Then** edge and degree counts include code relations
- **And** a symbol with no edges is reported as an orphan

## Feature: Limits and failures

### Scenario: Missing code extension fails fast

- **Given** no `weave-code` executable
- **When** I run `weave ingest <repository> --with-enricher code`
- **Then** the command exits `2`
- **And** the error code is `EXTENSION_NOT_FOUND`

### Scenario: Project timeout fails strict mode

- **Given** a code extension that exceeds the timeout
- **When** I ingest with `--strict`
- **Then** the ingest fails
- **And** the previous index remains intact

### Scenario: Oversized project response fails safely

- **Given** a project analysis response larger than 64 MiB
- **When** I ingest in permissive mode
- **Then** the file documents are kept
- **And** one `EXTENSION_PROTOCOL` warning is reported

## Feature: Compatibility

### Scenario: Existing bundles are unchanged

- **Given** a Markdown-only bundle
- **When** I run the normal release golden suite
- **Then** output is byte-identical
- **And** no code relation or synthetic document appears

### Scenario: Markdown-only pathfinding is unchanged

- **Given** a Markdown bundle with links
- **When** I run `weave path` without `--relation`
- **Then** link traversal and output match the previous release
