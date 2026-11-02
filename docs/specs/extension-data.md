---
type: specification
title: "Extension Data and Opaque Artifact API Specification"
description: "Post-GA extension API for publishing, binding and consuming opaque artifacts without native asset semantics."
tags:
  - "specification"
  - "extensions"
  - "assets"
  - "storage"
  - "post-ga"
generated:
  by: opencode/deepseek-v4.1-flash
  at: "2026-10-31T00:00:00Z"
status: stable
---

# Specification: Extension Data and Opaque Artifact API

## 1. Overview

This is a **post-GA** feature. Weave does not gain native image, PDF, audio, video, code or binary
asset support.

Instead, Weave exposes a generic **extension data API**:

- an extension receives a temporary workspace during enrichment;
- it may publish opaque files and metadata into that workspace;
- Weave validates, stores and binds those files to the current document;
- consumers read them through a generic list/get API and an MCP resource;
- Weave never parses, classifies, renders, indexes or transforms the bytes.

An image extension is therefore just an extension that reads image files, writes them into the
workspace and optionally adds keywords to the host document. A PDF extension, a code-snippet
extension or an audio extension uses the same API. Weave does not need to know which one it is.

The API is intentionally generic. There are no native `Asset`, `Image` or `Attachment` concepts, no
MIME sniffing, no thumbnails and no OCR.

Non-goals:

- Native image, PDF, audio, video or binary ingestion.
- Native parsing, MIME detection, OCR, captioning, transcription or rendering.
- Native search indexing of extension data.
- A native asset command, asset type or asset table.
- Extension access to write the SQLite database directly.
- Extension-managed schema changes or migrations.
- Remote artifact hosting, CDN integration or multi-tenant storage.
- Sandboxing, signature verification or malware scanning.
- Encrypted export of opaque extension data in the first slice.

## 2. Impact

| Area | Impact | Detail |
| --- | --- | --- |
| Parser and graph | Low | Markdown remains the only native input. Extension data is not a graph node or edge. |
| Search and ranking | Low | No native indexing. Extensions may add keywords to the host document. |
| Contract | Medium | Additive `ExtensionData` references and CLI/MCP result shapes. |
| SQLite/storage | High | New metadata table, sidecar blob store, quotas, cascade cleanup and GC. |
| Extension IPC | High | Workspace, manifest, `bundle_root`, `doc_path` and atomic import. |
| CLI/MCP | Medium-high | `extensions data list/get/gc` and a blob resource. |
| Export | Medium-high | Raw export must copy bytes and a manifest; encrypted export policy is explicit. |
| Security | High | Opaque bytes, path validation, quotas, hash verification and trusted extensions. |
| Testing | High | Binary fixtures, large files, corruption, sync lifecycle, export and MCP blobs. |
| v1.0 | None | Deferred until after the v1.0 release. |

Alternatives considered:

| Alternative | Decision |
| --- | --- |
| Native asset parser | Rejected: violates the extension-only principle |
| External URI references only | Rejected: no offline consumption, export or backup |
| Extension-created virtual documents | Rejected: high parser, sync, graph, search and export impact |
| In-process Go plugin | Rejected: platform and compatibility burden |
| SQLite BLOB for all data | Rejected for the default path: large artifacts bloat the database |
| Sidecar content-addressed blobs | Selected: bounded database growth and simple exports |

## 3. Principles

1. **Weave is transport and durability; the extension is semantics.** Weave stores opaque bytes and
   metadata. It does not decide whether a file is an image, a PDF or a code snippet.
2. **Document-bound by default.** Every extension data item is bound to one document. Bundle-level
   shared assets are deferred.
3. **Content-addressed.** Weave hashes every file with SHA-256 and de-duplicates identical bytes.
4. **No native discovery.** A search or graph query never returns extension data. Consumers use the
   extension data API or MCP resources.
5. **Additive.** Existing extensions, search, ranking, storage and export behavior remain unchanged
   unless they opt into the workspace.
6. **Trusted code.** Extensions are trusted executables. Weave validates paths, sizes and hashes; it
   does not sandbox or scan content.

## 4. Contracts

### 4.1 Enrichment request additions

The existing `enrichment_request` gains three optional fields:

```json
{
  "v": "weave.cli/v1",
  "type": "enrichment_request",
  "doc_id": "docs/adr-004.md",
  "doc_path": "docs/adr-004.md",
  "bundle_root": "/home/u/notes",
  "workspace": "/tmp/weave-ext-123",
  "title": "Database Storage Selection",
  "frontmatter": {"type": "adr", "keywords": ["database", "storage"]},
  "body": "We decided to adopt pure-Go SQLite..."
}
```

| Field | Meaning |
| --- | --- |
| `doc_path` | Bundle-relative Markdown path |
| `bundle_root` | Absolute bundle root; lets the extension read source assets |
| `workspace` | Absolute temporary directory for this invocation |

- The workspace contains an empty `files/` directory and an optional `manifest.json`.
- The workspace is removed after import; extensions must not retain the path.
- `bundle_root` is omitted for in-memory runs. Extension data requires persistent storage; in-memory
  operations do not run enrichers or import data.
- Existing extensions ignore the new fields.

### 4.2 Extension data manifest

The extension writes `manifest.json` at the workspace root:

```json
{
  "v": "weave.extension-data/v1",
  "items": [
    {
      "key": "images/architecture.png",
      "path": "files/architecture.png",
      "media_type": "image/png",
      "metadata": {"caption": "Index and cache layout"}
    }
  ]
}
```

| Field | Required | Rules |
| --- | --- | --- |
| `v` | yes | `weave.extension-data/v1` |
| `items` | yes | 0–256 entries; empty removes previous data for the document |
| `items[].key` | yes | 1–256 bytes; `/`-separated; no leading/trailing slash, no `.` or `..` segment; `[A-Za-z0-9._/-]` |
| `items[].path` | yes | Relative path under `workspace/files/`; regular file only |
| `items[].media_type` | no | Opaque label, 1–128 bytes; Weave does not validate or sniff it |
| `items[].metadata` | no | JSON object, at most 8 KiB; opaque to Weave |

- Weave computes SHA-256 and size; an optional `sha256` field must match.
- Duplicate keys are invalid.
- A manifest with an invalid entry fails the whole import. Previous data is kept.
- A valid empty manifest removes the previous data set for that extension and document.
- No manifest means no data; existing data is left unchanged.

### 4.3 Import flow

1. Core creates the workspace.
2. Core invokes the enricher with `bundle_root`, `doc_path` and `workspace`.
3. The extension writes `files/` and optionally `manifest.json`.
4. Core validates the manifest and every file.
5. Core copies validated files into the content-addressed blob store.
6. Core replaces the extension's previous data set for the document in the same transaction as the
   document write.
7. Failed validation or process failure keeps the previous data and adds one warning.

Import is all-or-nothing per invocation.

### 4.4 Storage model

Metadata lives in SQLite, schema version 6:

```sql
CREATE TABLE IF NOT EXISTS extension_data (
    bundle_root TEXT NOT NULL,
    extension TEXT NOT NULL,
    doc_id TEXT NOT NULL,
    key TEXT NOT NULL,
    sha256 TEXT NOT NULL,
    size INTEGER NOT NULL,
    media_type TEXT,
    metadata_json TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    PRIMARY KEY (bundle_root, extension, doc_id, key)
);
```

Blobs live in a Weave-managed sidecar store:

```text
<database-directory>/weave-extension-data/<bundle-hash>/<sha256>
```

- `<bundle-hash>` is the first 16 hex characters of SHA-256 over the normalized absolute bundle
  root.
- Files are `0600`; directories are `0700`.
- Writes use a temporary file, `fsync` and atomic rename.
- Identical bytes are stored once; rows reference the same blob.
- Absolute blob paths are never stored in the database.
- The database remains the source of truth for bindings and ownership.

### 4.5 Read API

```sh
weave extensions data list <path> [--extension <name>] [--doc <id>] [--json]
weave extensions data get <path> <extension> <key> --out <file> [--force] [--json]
weave extensions data gc <path> [--dry-run] [--json]
```

- `list` returns metadata only: extension, doc id, key, media type, size, SHA-256 and metadata.
- `get` writes bytes to `--out` and prints one JSON envelope with the metadata. Without `--out`, it
  exits `2` with `OUTPUT_REQUIRED`. An existing output file is rejected unless `--force` is set.
- `gc` removes blobs with no metadata row. `--dry-run` reports without deleting.
- Multi-source read commands are deferred; the first slice accepts one bundle path.
- `show` includes generic `extension_data` references for the document:

```json
{
  "extension_data": [
    {
      "extension": "images",
      "key": "images/architecture.png",
      "media_type": "image/png",
      "size": 48213,
      "sha256": "..."
    }
  ]
}
```

No bytes are embedded in `show` or search output.

### 4.6 MCP resources

A new resource template exposes opaque bytes:

```text
weave://extension/<name>/<key>
```

- `resources/list` includes extension data references for the configured bundle.
- `resources/read` returns the blob as MCP `blob` content with base64 data.
- The URI is resolved against the bundle root selected by the server.
- Missing data returns the existing MCP resource-not-found error.
- Multi-source qualified URIs are deferred.

### 4.7 Export

- Raw export copies extension data under `_extensions/<extension>/<key>` and writes
  `_extensions/manifest.json` with key, doc id, media type, size and SHA-256.
- A missing blob is an export error; Weave never silently drops extension data.
- Encrypted export excludes extension data in the first slice and emits one warning. Weave cannot
  safely transform opaque bytes without knowing the extension's format.
- Exporting multiple sources remains out of scope.

### 4.8 Lifecycle

| Event | Behavior |
| --- | --- |
| Ingest with enricher | Import the manifest and bind to the document |
| Sync with enricher | Replace the data set for changed documents |
| Document deletion | Cascade-delete metadata rows; GC unreferenced blobs |
| Rebuild with enricher | Replace data for rebuilt documents |
| Rebuild without enricher | Remove all extension data for the bundle and GC |
| Extension failure | Keep the previous data set and add one warning |
| Empty manifest | Remove the previous data set for that extension and document |
| `gc` | Remove blobs with no metadata row |
| Database vacuum | Never touches the sidecar store |
| Database deletion | Sidecar store is not deleted automatically; operators remove it explicitly |

### 4.9 Contract additions

| Type | Field | Meaning |
| --- | --- | --- |
| `ShowResult` | `extension_data` | Optional metadata references |
| `ExtensionData` | new type | Extension, key, media type, size, SHA-256, metadata |
| `EnrichmentRequest` | `bundle_root`, `doc_path`, `workspace` | Optional extension workspace context |
| CLI schemas | `extension-data-list`, `extension-data-get` | Additive command results |
| MCP | `weave://extension/<name>/<key>` | Blob resource |

### 4.10 Errors

| Code | Meaning |
| --- | --- |
| `EXTENSION_DATA_INVALID` | Invalid manifest, key, path, media type or metadata |
| `EXTENSION_DATA_TOO_LARGE` | A file, manifest or total workspace exceeds a limit |
| `EXTENSION_DATA_QUOTA` | The document or bundle quota is exceeded |
| `EXTENSION_DATA_NOT_FOUND` | Requested extension data does not exist |
| `OUTPUT_REQUIRED` | `data get` was called without `--out` |
| `OUTPUT_EXISTS` | `data get` target exists and `--force` is absent |
| `EXTENSION_PROTOCOL` | Existing extension transport failure |

Import errors are warnings on ingest/sync, never a document failure, unless the extension itself
fails before producing a manifest.

## 5. Security and Privacy

- Paths are relative, cleaned and rejected if they escape `workspace/files/`.
- Symlinks, hardlinks, devices, sockets and FIFOs are rejected.
- Files are regular, finite and readable; Weave hashes and copies them.
- `media_type` is an opaque label. Weave never executes, renders or interprets data.
- Extensions are trusted code and inherit the process environment. Weave does not sandbox them.
- Operators must treat extension data as untrusted content when rendering or opening it.
- Extension data is not encrypted or transformed by Weave in the first slice. An extension that
  needs encryption must encrypt before publishing.
- If the source document is encrypted, the existing enrichment privacy rules apply; Weave does not
  send plaintext to the extension.
- Quotas are enforced before import and before export.

## 6. Limits

| Limit | Value |
| --- | --- |
| Manifest size | 1 MiB |
| Items per document | 256 |
| Single file | 32 MiB |
| Workspace total | 256 MiB |
| Data per document | 128 MiB |
| Data per bundle | 1 GiB |
| Metadata per item | 8 KiB |
| Key length | 256 bytes |
| Media type length | 128 bytes |

Limits are fixed in the first slice. Configurable quotas are a later enhancement.

## 7. Data Flow

```text
CLI ingest/sync --with-enricher
  -> engine/operations
  -> engine/extensions runner
       -> create workspace
       -> invoke weave-<name> with bundle_root, doc_path, workspace
       -> read manifest.json and files/
       -> validate paths, sizes, hashes and quotas
  -> storage/sqlite
       -> insert extension_data rows
       -> copy blobs to content-addressed sidecar
       -> commit document + data in one transaction
  -> CLI/MCP
       -> show extension_data refs
       -> data list/get
       -> weave://extension/<name>/<key>
```

Search and graph flows do not change. An extension that wants extension data to affect text search
must add keywords to the host document through the existing enrichment response.

## 8. Testing

Behavior scenarios: [extension-data.md](../bdd/extension-data.md).

- Unit tests for manifest parsing, key/path validation, symlink rejection, hash mismatch, size and
  quota limits.
- Binary fixtures: a small PNG, a text file and an empty file.
- Tests for duplicate keys, invalid media types and metadata size.
- Atomic import: a failed manifest imports nothing and keeps previous data.
- Empty manifest removes the previous data set.
- Sync update, document deletion and rebuild without enricher cascade correctly.
- GC removes only unreferenced blobs.
- Raw export round-trips bytes and metadata; encrypted export warns and excludes.
- MCP resource read returns the exact SHA-256 bytes.
- `show` includes references but no bytes.
- Path traversal, absolute paths and symlinks fail with `EXTENSION_DATA_INVALID`.
- Race coverage for concurrent CLI invocations.
- Space test: duplicate blobs add no new file.

## 9. Acceptance Criteria

1. An extension can publish an image as opaque bytes during ingest and bind it to a Markdown
   document.
2. Weave stores the bytes without parsing, rendering or indexing them.
3. `show` lists extension data metadata; `extensions data get --out` returns the exact bytes.
4. MCP `resources/read` returns the same bytes as a blob.
5. The extension can add keywords to the host document so text search still finds it.
6. Update, delete, rebuild and GC keep metadata and blobs consistent.
7. Raw export copies extension data and its manifest; encrypted export warns and excludes it.
8. Invalid paths, symlinks, oversized files, duplicate keys and bad hashes fail safely.
9. Default parser, graph, search, ranking, privacy and output are unchanged without extension data.
10. The feature is not part of the v1.0 release gate.

## 10. Delivery Slices

| Slice | Scope |
| --- | --- |
| E1 | Workspace, manifest, validation, schema v6 and import |
| E2 | Show references, list/get CLI and GC |
| E3 | MCP resource and blob transfer |
| E4 | Raw export, encrypted-export policy and lifecycle tests |
| E5 | BDD scenarios, binary fixtures, docs and benchmarks |

Each slice is a separate stacked PR. E1 must not change default ingest or search output.

## 11. Open Questions

- Should bundle-scoped shared artifacts be allowed after document-scoped data ships?
- Should quotas be configurable per extension or per repository?
- Should encrypted export encrypt opaque bytes generically, or require extensions to provide an
  encrypted form?
- Should a consumer extension receive existing data references in its enrichment request, or always
  use the CLI/MCP read API?

## 12. Related Documents

- [Extension Subprocess IPC](./extension-ipc.md) — workspace, enrichment and runner contract
- [Storage & Sync](./storage-and-sync.md) — schema, transactions and lifecycle
- [Bundle Export](./bundle-export.md) — raw and encrypted export
- [MCP Server](./mcp-server.md) — resources and transport
- [Pluggable Scoring](./extension-scoring.md) — post-GA scorer contract
- [Privacy Directives](./privacy-directives.md) — sensitive content rules
- [Roadmap](../../ROADMAP.md) — post-GA feature register
