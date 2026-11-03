# BDD: Body Compression

Human-readable behavior cases for text-raw storage, binary zstd defaults and codec extensions.

**Spec reference:** [compression.md](https://github.com/prulloac/weave-docs/blob/main/specs/compression.md)
**Go tests:** planned: `cmd/weave/compression_bdd_test.go`, `storage/sqlite/compression_test.go`

Text-readable content is stored without compression. Binary blobs use zstd by default. LZ4 and other
algorithms come from `weave-codec-<name>` extensions.

---

## Feature: Text-Readable Content

### Scenario: Markdown bodies are stored raw

- **Given** a Markdown document
- **When** I run `weave ingest` without `--codec`
- **Then** `body_raw` contains the Markdown body
- **And** `body_compressed` and `body_codec` are NULL
- **And** `weave show` returns the original body

### Scenario: Text-readable blobs are stored raw

- **Given** `.txt`, JSON, XML and CSV blobs
- **When** I ingest them with `--assets --asset-types text/*`
- **Then** every row uses `body_raw`
- **And** `body_compressed` and `body_codec` are NULL
- **And** every `show --out` returns the original bytes

### Scenario: Source code is stored raw

- **Given** Go, Python, Rust and shell source files ingested as blobs
- **When** I ingest them with `--assets --asset-types text/*`
- **Then** each row is classified text-readable
- **And** each row uses `body_raw` with NULL `body_compressed`/`body_codec`

### Scenario: Text ignores --codec with a warning

- **Given** a Markdown document and a source-code blob
- **When** I run `weave ingest --codec zstd`
- **Then** both rows still use `body_raw`
- **And** `body_codec` stays NULL
- **And** the report counts one warning per text document

### Scenario: Encrypted text is never double-compressed

- **Given** an encrypted Markdown document
- **When** I ingest it
- **Then** `body_raw` contains the `enc:v1:` envelope
- **And** `body_compressed` and `body_codec` are NULL
- **When** I run `weave show --decrypt`
- **Then** the body matches the source plaintext

---

## Feature: Binary Blobs

### Scenario: Binary blobs use zstd by default

- **Given** a PDF or arbitrary binary blob
- **When** I ingest it with `--assets --asset-types application/pdf,application/octet-stream`
- **Then** `body_compressed` contains the zstd bytes
- **And** `body_raw` is NULL
- **And** `body_codec` is `zstd`
- **And** `weave show --out` returns the original bytes

### Scenario: Zstd is used even when output is larger

- **Given** a small incompressible binary blob
- **When** I ingest it with `--assets --asset-types application/octet-stream`
- **Then** `body_compressed` uses zstd
- **And** `body_codec` is `zstd`

### Scenario: --codec none stores binary bytes raw

- **Given** a binary blob
- **When** I ingest it with `--assets --asset-types application/pdf --codec none`
- **Then** `body_compressed` holds the source bytes
- **And** `body_codec` is `none`
- **And** `body_raw` is NULL

### Scenario: Text and binary rows keep different codecs

- **Given** a Markdown document and a PDF in one bundle
- **When** I ingest both with `--assets --asset-types application/pdf`
- **Then** the Markdown row uses `body_raw`
- **And** the PDF row uses `body_compressed` with `zstd`
- **And** both round-trip through `show`

---

## Feature: Codec Extensions

### Scenario: Missing codec fails before writing

- **Given** no `weave-codec-lz4` on disk
- **When** I ingest a binary blob with `--codec lz4`
- **Then** the command exits `2`
- **And** the error code is `CODEC_NOT_FOUND`
- **And** no document is written

### Scenario: LZ4 extension stores and reads LZ4

- **Given** `weave-codec-lz4` is installed
- **When** I ingest a binary blob with `--codec lz4`
- **Then** `body_compressed` holds LZ4 bytes
- **And** `body_raw` is NULL
- **And** `body_codec` is `lz4`
- **And** `weave show --out` returns the original bytes

### Scenario: Custom codec extension round-trips

- **Given** a custom `weave-codec-<name>` with compress and decompress
- **When** I ingest a binary blob with `--codec <name>`
- **Then** `body_compressed` stores the encoded bytes
- **And** `body_codec` stores the codec name
- **And** `show --out` returns the original bytes

### Scenario: Missing stored codec fails reads

- **Given** a row with `body_compressed` and `body_codec = lz4` but no LZ4 extension
- **When** I run `weave show --out`
- **Then** the command fails with `CODEC_UNAVAILABLE`
- **And** it does not return empty or partial bytes

### Scenario: Codec failure aborts the write

- **Given** a codec extension that exits non-zero or times out
- **When** I ingest a binary blob with that codec
- **Then** the transaction aborts
- **And** no document row is written

### Scenario: --timeout is enforced

- **Given** a slow codec extension
- **When** I ingest with `--timeout 50ms`
- **Then** the command fails with `CODEC_TIMEOUT`
- **And** no document row is written

---

## Feature: Migration and Rebuild

### Scenario: Text rows migrate to raw

- **Given** a database with zstd-compressed Markdown rows
- **When** I upgrade to schema v7
- **Then** those rows are decompressed into `body_raw`
- **And** `body_compressed` and `body_codec` are NULL
- **And** their bodies round-trip unchanged

### Scenario: Migration requires the LZ4 codec for text rows

- **Given** a v6 database with LZ4-compressed Markdown rows
- **When** I open it without `weave-codec-lz4`
- **Then** the migration fails with `CODEC_UNAVAILABLE`
- **And** `user_version` stays 6
- **When** I install `weave-codec-lz4` and reopen
- **Then** those rows migrate into `body_raw`
- **And** their bodies round-trip unchanged

### Scenario: Rebuild re-encodes binary rows only

- **Given** text and binary rows with mixed codecs
- **When** I run `weave rebuild --codec zstd`
- **Then** text rows use `body_raw`
- **And** binary rows use `body_compressed` with `zstd`
- **And** all bodies round-trip

### Scenario: Sync preserves per-row codecs

- **Given** a binary row using LZ4 and a text row using `body_raw`
- **When** I modify both and run `weave sync` without `--codec`
- **Then** the binary row stays LZ4
- **And** the text row stays `body_raw`
- **When** I run `weave sync --codec zstd`
- **Then** the binary row uses zstd
- **And** the text row stays `body_raw`

---

## Feature: Failure Recovery and Parity

### Scenario: Corrupt binary body fails safely

- **Given** a truncated or corrupt `body_compressed` row
- **When** I read it
- **Then** the failure is `BODY_DECODE_FAILED`
- **And** no partial plaintext is returned
- **And** queries do not crash

### Scenario: Search and explain are unchanged

- **Given** a mixed text/binary bundle
- **When** I search and explain a text term
- **Then** results match the uncompressed bundle
- **And** calibration stays green
