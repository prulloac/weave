# BDD: Body Compression

Human-readable behavior cases for zstd/LZ4 body storage.

**Spec reference:** [compression.md](../specs/compression.md)
**Go tests:** `cmd/weave/compression_bdd_test.go`

Documents are compressed after the privacy transform. Zstandard is the default; LZ4 is opt-in for
faster writes, and bodies below 1 KiB stay verbatim.

---

## Feature: Codec Selection

### Scenario: Default ingest uses zstd for large bodies

- **Given** a bundle with a body above 1 KiB
- **When** I run `weave ingest` without `--codec`
- **Then** the stored body codec is `zstd`
- **And** `weave show` returns the original body

### Scenario: Small bodies stay uncompressed

- **Given** a body of 1023 bytes
- **When** I ingest the bundle
- **Then** the stored body codec is `none`

### Scenario: Threshold boundary compresses at 1024 bytes

- **Given** a body of exactly 1024 bytes
- **When** I ingest the bundle
- **Then** the stored body codec is `zstd`

### Scenario: LZ4 is opt-in

- **Given** a bundle with a large body
- **When** I run `weave ingest --codec lz4`
- **Then** the stored body codec is `lz4`
- **And** `weave show` returns the original body

### Scenario: Compression can be disabled

- **When** I run `weave ingest --codec none`
- **Then** large bodies are stored verbatim
- **And** the stored bytes equal the parsed body

### Scenario: Unknown codec is a usage error

- **When** I run `weave ingest --codec brotli`
- **Then** the command exits `2`

---

## Feature: Privacy Interaction

### Scenario: Encrypted bodies are never double-compressed

- **Given** a large encrypted document
- **When** I ingest the bundle
- **Then** the stored body codec is `none`
- **And** the stored body is an `enc:v1:` envelope

### Scenario: Encrypted bodies still decrypt after storage

- **Given** an indexed encrypted document
- **When** I run `weave show --decrypt`
- **Then** the body matches the source plaintext

---

## Feature: Codec Migration

### Scenario: Rebuild migrates codecs

- **Given** a bundle ingested with `--codec none`
- **When** I run `weave rebuild --codec zstd`
- **Then** every large body uses `zstd`
- **And** bodies round-trip unchanged
- **When** I run `weave rebuild --codec none`
- **Then** bodies are stored verbatim again

### Scenario: Sync preserves per-row codecs

- **Given** a bundle ingested with `--codec lz4`
- **When** I modify an existing document and add a new one
- **And** I run `weave sync` without `--codec`
- **Then** the updated document stays `lz4`
- **And** the new document uses `zstd`
- **When** I modify the document again and run `weave sync --codec zstd`
- **Then** the updated document uses `zstd`

---

## Feature: Failure Recovery

### Scenario: Unknown stored codec does not crash queries

- **Given** a stored body with an unknown codec
- **When** I run `weave parse --db <file>`
- **Then** the bundle carries a `BODY_DECODE_FAILED` warning
- **And** the concept body is empty
- **And** `weave show` still exits successfully

### Scenario: Truncated blob does not crash queries

- **Given** a stored body truncated to one byte
- **When** I run `weave parse --db <file>`
- **Then** the bundle carries a `BODY_DECODE_FAILED` warning
- **And** search still returns an envelope instead of failing

---

## Feature: Storage and Ranking Parity

### Scenario: Compression reduces on-disk size

- **Given** a repetitive body above 1 KiB
- **When** I ingest with the default codec
- **Then** the stored blob is smaller than the parsed body

### Scenario: Search and explain are unchanged

- **Given** a compressed bundle
- **When** I search and explain a body term
- **Then** results match the uncompressed bundle
- **And** calibration stays green
