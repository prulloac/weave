# BDD: Generic Blob Assets

Human-readable behavior cases for core-owned opaque blob documents.

**Spec reference:** [blob-assets.md](https://github.com/prulloac/weave-docs/blob/main/specs/blob-assets.md)
**Go tests:** planned: `cmd/weave/blob_assets_bdd_test.go`, `storage/sqlite/blob_test.go`, `mcp/blob_test.go`

Blob assets are opt-in. Core stores the bytes and generic metadata; an extension supplies extracted
text and kind-specific attributes. No core behavior is specific to images, video, audio or PDF.

---

## Feature: Opt-In Discovery

### Scenario: Markdown-only bundles are unchanged

- **Given** a bundle with `index.md`, `note.md` and `photo.png`
- **When** I run `weave ingest <path>` without `--assets`
- **Then** only the two Markdown documents are ingested
- **And** no blob document is stored

### Scenario: Explicit asset types select files

- **Given** a bundle with a PNG, a PDF and a text fixture
- **When** I run `weave ingest <path> --assets --asset-types image/png,application/pdf`
- **Then** the PNG and PDF become blob documents
- **And** the text fixture is ignored

### Scenario: Extension asset types enable discovery

- **Given** an installed extension whose `--describe --json` advertises `image/*`
- **When** I ingest with `--assets --with-enricher <name>`
- **Then** only files matching the advertised media types become blob documents

### Scenario: Missing asset types is a usage error

- **Given** no `--asset-types` and no extension capability
- **When** I run `weave ingest <path> --assets`
- **Then** the command exits `2`
- **And** the error code is `ASSET_TYPES_REQUIRED`
- **And** no blob document is stored

### Scenario: Blob ingestion requires persistence

- **When** I run `weave ingest <path> --assets --asset-types image/png --no-db`
- **Then** the command exits `2`
- **And** the error code is `BLOBS_REQUIRE_DB`

### Scenario: Unrelated files and symlink escapes are ignored

- **Given** `.git/`, `node_modules/` and a symlink pointing outside the bundle
- **When** I ingest with `--assets --asset-types image/png`
- **Then** ignored directories are not scanned
- **And** the escaping symlink is rejected with a warning

---

## Feature: Blob Documents

### Scenario: Blob metadata is stored generically

- **Given** a PNG matching the asset filter
- **When** I ingest it with `--assets --asset-types image/png --json`
- **Then** the document has `content_kind: blob`
- **And** `media_type: image/png`
- **And** the file name is used as the display title until metadata exists
- **And** `body_compressed` holds zstd bytes with `body_codec: zstd`
- **And** `body_raw` is NULL
- **And** `show --out` returns the original PNG bytes

### Scenario: Text-readable blobs are indexed from body_raw

- **Given** a `.txt`, JSON or source-code blob
- **When** I run `weave search <path> "<term in the file>"`
- **Then** the blob document is returned
- **And** the snippet comes from `body_raw`

### Scenario: Binary extracted text is searchable

- **Given** a binary blob enriched with `content_metadata.text`
- **When** I run `weave search <path> "<text>"`
- **Then** the blob document is returned
- **And** the snippet comes from the extracted text

### Scenario: Blob bytes are never indexed as text

- **Given** a binary blob whose bytes contain a keyword but whose extracted text is empty
- **When** I search for that keyword
- **Then** the blob document is not returned by body matching

### Scenario: Metadata-only blobs remain searchable

- **Given** a blob with a title, description and keywords but no extracted text
- **When** I search for a title or keyword term
- **Then** the blob document is returned

### Scenario: Invalid content metadata is a warning

- **Given** an enricher returning malformed `inferred_content_metadata`
- **When** I ingest a blob
- **Then** the document is stored
- **And** empty content metadata is kept
- **And** the report counts one warning

### Scenario: Extensions cannot change core-owned fields

- **Given** an enricher returning `content_kind`, `media_type` or `content_hash` in its response
- **When** I ingest a blob
- **Then** the core-owned values are unchanged
- **And** the extension fields are ignored

---

## Feature: Metadata Precedence

### Scenario: Declared metadata wins

- **Given** a blob document with a declared title and description
- **When** an enricher returns different inferred values
- **Then** the declared title and description are preserved

### Scenario: Inferred metadata fills empty fields

- **Given** a blob with an empty title and description
- **When** an enricher returns inferred title and description
- **Then** both fields are stored

### Scenario: Inferred keywords append

- **Given** a blob with declared keywords
- **When** an enricher returns more keywords
- **Then** inferred keywords are appended after declared keywords
- **And** duplicates are removed case-insensitively

### Scenario: OCR text wins over transcription

- **Given** a blob with existing `content_metadata.text` and `text_source: ocr`
- **When** an enricher returns different `inferred_text`
- **Then** the OCR text is preserved

---

## Feature: Retrieval

### Scenario: show --out returns exact bytes

- **Given** an indexed blob document
- **When** I run `weave show <path> <id> --out output.bin`
- **Then** `output.bin` is byte-for-byte identical to the source blob

### Scenario: show --json returns metadata, not bytes

- **When** I run `weave show <path> <id> --json`
- **Then** the result includes content kind, media type, body form, codec, hash, size and content
  metadata
- **And** no body bytes are embedded

### Scenario: Existing output requires --force

- **Given** `output.bin` already exists
- **When** I run `weave show ... --out output.bin` without `--force`
- **Then** the command exits `2`
- **And** the existing file is unchanged
- **When** I add `--force`
- **Then** the file is replaced

### Scenario: MCP returns the exact blob

- **Given** a running MCP server over the bundle
- **When** I read `weave://blob/<id>`
- **Then** the resource content is a blob
- **And** the bytes match the source
- **And** the MIME type matches `media_type`

### Scenario: Search and list expose generic fields

- **When** I run `weave search` or `weave list --json`
- **Then** blob results include `content_kind` and `media_type`
- **And** no result includes blob bytes

---

## Feature: Limits and Lifecycle

### Scenario: Oversized blobs are skipped

- **Given** a blob above the configured size limit
- **When** I ingest with `--assets`
- **Then** the blob is skipped with a warning
- **And** no document is written for it
- **When** I use `--strict`
- **Then** the command fails

### Scenario: Sync updates changed bytes

- **Given** an indexed blob
- **When** the source file changes and I run `weave sync`
- **Then** the document hash and stored bytes are updated
- **And** the previous blob bytes are no longer returned

### Scenario: Delete removes the blob document

- **Given** an indexed blob
- **When** the source file is deleted and I run `weave sync`
- **Then** the document is removed
- **And** `weave show` returns not found

### Scenario: Codecs are recorded per row

- **Given** a Markdown document, a source-code blob and a PDF in one bundle
- **When** I ingest with `--assets` and no `--codec`
- **Then** the Markdown and source-code rows use `body_raw`
- **And** `body_compressed` and `body_codec` are NULL for them
- **And** the PDF row uses `body_compressed` with `zstd`
- **And** `weave show --out` returns each original
- **And** `weave show --json` reports the body form and codec for each

### Scenario: Raw export includes blobs

- **When** I run `weave export <path> --out <dir> --format raw`
- **Then** each blob is written at its bundle-relative path
- **And** the export manifest records media type, hash and size

### Scenario: Encrypted export skips blobs with a warning

- **When** I run `weave export <path> --out <dir> --format encrypted`
- **Then** blob documents are skipped
- **And** one warning names each skipped blob
- **And** Markdown documents are still exported

### Scenario: One core path serves every blob kind

- **Given** an image, a PDF and an arbitrary binary fixture
- **When** I ingest all three with matching asset types and the same fake enricher
- **Then** all three use the same blob document lifecycle
- **And** core behavior differs only by media type and extension metadata
