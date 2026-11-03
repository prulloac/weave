# BDD: Codec Extensions

Human-readable behavior cases for `weave-codec-<name>` discovery, manifests and raw-byte compression.

**Spec reference:** [codec-extensions.md](https://github.com/prulloac/weave-docs/blob/main/specs/codec-extensions.md)
**Go tests:** planned: `cmd/weave/codec_extensions_bdd_test.go`, `engine/codecs/codec_test.go`, `storage/sqlite/compression_test.go`

Core ships `none` and `zstd`. Any other codec is an extension. Codec extensions use raw stdin/stdout,
not `weave.cli/v1` JSON.

---

## Feature: Discovery and Manifest

### Scenario: Built-ins and extensions are listed

- **Given** a bundle-local codec and a PATH codec
- **When** I run `weave codecs list --json`
- **Then** `none` and `zstd` appear as built-ins
- **And** both extensions appear with their source and path
- **And** entries are sorted deterministically

### Scenario: Machine-readable manifest

- **When** I run `weave-codec-<name> --describe --json`
- **Then** it returns `weave.extension/v1`
- **And** it names the codec and its directions
- **And** the name matches the binary suffix

### Scenario: Invalid manifest makes the codec unavailable

- **Given** an extension with malformed `--describe --json`
- **When** I run `--codec <name>`
- **Then** the command exits `2`
- **And** the error code is `CODEC_PROTOCOL`

---

## Feature: Raw Transport

### Scenario: Compress and decompress round-trip

- **Given** a codec extension implementing both directions
- **When** core compresses a binary blob with `--codec <name>`
- **Then** the extension receives raw bytes on stdin
- **And** returns only codec bytes on stdout
- **And** the stored row records the codec name
- **When** I run `weave show --out`
- **Then** the extension decompresses the row
- **And** the output matches the source bytes

### Scenario: Diagnostics do not corrupt output

- **Given** a codec that writes diagnostics to stderr
- **When** compression runs
- **Then** stderr is captured as diagnostics
- **And** stdout contains only codec bytes

### Scenario: Text-readable content does not invoke a codec

- **Given** a Markdown document or a source-code blob
- **When** I ingest with `--codec <name>`
- **Then** the stored codec is `none`
- **And** the extension is not invoked
- **And** the report counts one warning

---

## Feature: Selection and Availability

### Scenario: Binary default is zstd

- **Given** a binary blob and no `--codec`
- **When** I ingest with `--assets`
- **Then** the stored codec is `zstd`
- **And** no codec extension is invoked

### Scenario: Explicit missing codec fails before write

- **Given** no `weave-codec-<name>` on disk
- **When** I ingest a binary blob with `--codec <name>`
- **Then** the command exits `2`
- **And** the error code is `CODEC_NOT_FOUND`
- **And** no document row is written

### Scenario: Missing stored codec fails reads

- **Given** a row whose codec extension is not installed
- **When** I run `weave show --out`
- **Then** the command fails with `CODEC_UNAVAILABLE`
- **And** no empty or partial bytes are returned

### Scenario: LZ4 is an extension

- **Given** `weave-codec-lz4` is not installed
- **When** I run `--codec lz4`
- **Then** the command fails with `CODEC_NOT_FOUND`
- **Given** the extension is installed
- **Then** `--codec lz4` round-trips and records `lz4`

---

## Feature: Failures and Limits

### Scenario: Non-zero exit aborts the write

- **Given** a codec that exits non-zero
- **When** I ingest a binary blob with that codec
- **Then** the transaction aborts
- **And** the error is `CODEC_FAILED`
- **And** no document row is written

### Scenario: Timeout aborts the write

- **Given** a slow codec
- **When** I ingest with `--timeout 50ms`
- **Then** the error is `CODEC_TIMEOUT`
- **And** no document row is written

### Scenario: Oversized output is rejected

- **Given** a codec that expands output beyond the configured bound
- **When** compression or decompression runs
- **Then** the operation fails with `CODEC_FAILED`
- **And** no partial data is returned

### Scenario: Codec never receives the database path

- **When** a codec runs
- **Then** its environment contains no index path or key material
- **And** it cannot modify the bundle or database
