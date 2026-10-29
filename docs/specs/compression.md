---
type: specification
title: "Compression Specification"
description: "v1.0 body compression contract: LZ4/Zstandard codecs, thresholds, storage schema, codec migration and failure behavior."
tags:
  - "specification"
  - "compression"
  - "storage"
  - "v1"
generated:
  by: opencode/deepseek-v4.1-flash
  at: "2026-10-28T00:00:00Z"
status: draft
---

# Specification: Compression

Status: **Implemented** (zstd default, LZ4 opt-in, per-row codec, decode recovery).

## 1. Overview

Weave stores document bodies compactly in SQLite. Compression is an internal storage contract with a
per-row codec; it adds no read-time flag and does not change search results, snippets or privacy
semantics.

This slice delivers **body compression**. In-memory posting compression is explicitly deferred (see
§4) because the v1 search index is a map-based posting set and SQLite FTS5 already owns its own
compressed storage format.

## 2. Goals and Non-Goals

### Goals

- Reduce the on-disk size of `documents.body_compressed`.
- Keep decompression fast enough for parse-independent reads, snippets and explain.
- Make the codec explicit, persisted per row and deterministic.
- Never corrupt, truncate or silently drop content.

### Non-Goals

- Compressing ciphertext twice.
- Query-time codec selection.
- Lossy compression.
- Custom or user-supplied codecs.
- Delta/varint posting compression in this slice.
- Compressing FTS5 tables; FTS5 owns its storage format.

## 3. Body Compression

### 3.1 Schema

The `documents` table already has `body_codec`; this slice adds no columns:

```sql
body_compressed BLOB NOT NULL,
body_codec TEXT NOT NULL DEFAULT 'none'
```

- `body_compressed` holds the encoded bytes.
- `body_codec` is `none`, `lz4` or `zstd`.
- The codec is persisted per row so reads never need the write-time configuration.

### 3.2 Codecs

| Codec | Library | Default | Notes |
| --- | --- | --- | --- |
| `zstd` | `github.com/klauspost/compress/zstd` | yes | best ratio for the usual sub-1 MB document |
| `lz4` | `github.com/pierrec/lz4/v4` | no | faster encode/decode, lower ratio |
| `none` | standard library | no | short, empty or encrypted bodies |

- `zstd` is the default for new writes: document bodies are usually well under 1 MB, where the extra
  ratio outweighs LZ4's speed advantage.
- `lz4` stays available for write-heavy workflows that prefer faster encoding.
- Both encoders are pure Go and deterministic for the same input, options and library version.
- Codec version is pinned by `go.mod`; no separate manifest is required.

### 3.3 Selection Rules

- Bodies smaller than **1024 bytes** after privacy transformation use `body_codec = 'none'`.
- Encrypted whole-body documents (`privacy_policy = 'encrypt'`) always use `none`; ciphertext is
  stored as produced and never double-compressed.
- Tokenized and hashed bodies compress normally.
- Mixed documents compress normally; inline `enc:v1:` envelopes remain inside the compressed body.
- Empty bodies use `none`.

### 3.4 Write Path

CLI:

```sh
weave ingest <path> [--codec lz4|zstd|none] [--strict|--permissive] --json
weave rebuild <path> [--codec lz4|zstd|none] [--strict|--permissive] --json
weave sync <path> [--codec lz4|zstd|none] [--strict|--permissive] --json
```

- `ingest` and `rebuild` default to `zstd`; `--codec` applies to every stored document.
- `sync` writes new documents with `zstd`. Updated documents keep their stored codec unless
  `--codec` is provided.
- `--codec none` stores bodies verbatim and is the migration path to disable compression.
- Compression runs inside the existing write transaction; a failure aborts the transaction.
- No other CLI or contract field changes: `body_codec` is internal to storage.

### 3.5 Read Path

- `LoadBundle` decompresses each body from its stored codec.
- A decompression failure yields an empty body for that document and appends the bundle-level
  warning `BODY_DECODE_FAILED`; metadata, links and edges stay intact.
- An unknown `body_codec` is treated as a decode failure with the same warning.
- Queries never crash on corrupt bodies and never return partial plaintext.
- The engine materializes decompressed bodies for search; compression reduces disk footprint, not
  steady-state RAM in this slice.

## 4. Posting Compression (deferred)

The v1 in-memory index stores postings as token maps, and SQLite FTS5 owns lexical storage. Delta and
variable-byte posting compression is deferred until the index gains a compact posting representation.
The following remain future requirements:

- Document ids delta-encoded and sorted.
- Integers variable-byte encoded.
- No additional compression layered on FTS5.

## 5. Data Flow

```text
source or stored body
  -> privacy transform        (may produce ciphertext or aliases)
  -> compression selection    (encrypt/empty/short -> none; else lz4 or zstd)
  -> documents.body_compressed + body_codec
  -> LoadBundle               (decode by body_codec; warn on failure)
  -> engine/search/nav        (unchanged)
```

## 6. Validation and Errors

| Condition | Behavior |
| --- | --- |
| `--codec` unknown | Usage error, exit `2` |
| Compression failure | Transaction aborts, `INTERNAL`, exit `1` |
| Unknown stored codec | Warning `BODY_DECODE_FAILED`, empty body, query continues |
| Truncated or corrupt blob | Warning `BODY_DECODE_FAILED`, empty body, query continues |
| Body below threshold | `body_codec = 'none'`, bytes verbatim |
| Encrypted whole-body document | `body_codec = 'none'` |

Exit codes are unchanged: `0` success, `1` runtime failure, `2` usage, `3` privacy failure.

## 7. Testing and Calibration

- LZ4 and zstd round-trip every fixture body byte-for-byte.
- Determinism: encoding the same body twice yields identical bytes.
- Threshold boundary: 1023 bytes → `none`, 1024 bytes and above → configured codec.
- Encrypted bodies store ciphertext with `body_codec = 'none'`.
- Tokenized, hashed and mixed bodies round-trip through the codec.
- Corrupt and unknown codecs produce `BODY_DECODE_FAILED` and an empty body without failing the query.
- Read paths (`show`, `search`, `explain`, `list`, `backlinks`) work on compressed bundles.
- `rebuild --codec` migrates all rows; `sync` preserves per-row codecs unless `--codec` is set.
- Compression reduces size for a repetitive fixture above the threshold.
- Full gate: `go test ./...`, `go test -race ./...`, `go vet ./...`, `./scripts/verify.sh`,
  docs-lint and markdownlint.
- Ranking behavior is unchanged; `TestCalibration` must remain green.

## 8. Acceptance Criteria

- All stored bodies round-trip without loss through `zstd`, `lz4` and `none`.
- `zstd` is the default codec for new writes.
- Encrypted content is never double-compressed.
- Corrupt bodies never crash queries and never emit partial plaintext.
- Compression is deterministic for a given codec and library version.
- `weave rebuild --codec` migrates codecs safely; sync preserves per-row codecs.
- On-disk body size drops for compressible fixtures above the threshold.
- The full verification gate stays green and calibration is unchanged.

## Related Documents

- [Storage & Sync](./storage-and-sync.md)
- [Privacy Directives](./privacy-directives.md)
- [Privacy & Extensions](./privacy-and-extensions.md)
- [BM25 Ranking](./bm25-ranking.md)
- [ROADMAP](../../ROADMAP.md)
