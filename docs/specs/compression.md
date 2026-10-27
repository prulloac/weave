---
type: specification
title: "Compression Specification"
description: "V2 body and posting compression contract: LZ4/Zstandard codecs, thresholds, storage schema and failure behavior."
tags:
  - "specification"
  - "compression"
  - "storage"
  - "v2"
generated:
  by: opencode/deepseek-v4.1-flash
  at: "2026-10-27T00:00:00Z"
status: draft
---

# Specification: Compression

## 1. Overview

V2 stores document bodies and inverted postings compactly. Compression is an internal storage
contract, not a new CLI surface.

## 2. Goals and Non-Goals

### Goals

- Reduce SQLite and transient RAM footprint.
- Keep decompression fast enough for snippets and explain.
- Make codecs explicit and versioned.
- Never corrupt or silently truncate content.

### Non-Goals

- Compressing ciphertext twice.
- User-configurable compression at query time.
- Lossy compression.
- Custom codecs in v1.

## 3. Body Compression

Schema:

```sql
ALTER TABLE documents ADD COLUMN body_codec TEXT NOT NULL DEFAULT 'none';
```

- Default codec: LZ4 frame format.
- Optional codec: Zstandard.
- Bodies below 1 KiB: `body_codec = 'none'`.
- Ciphertext bodies: stored as-is, `body_codec = 'none'`; never double-compress.
- `documents.body_compressed` holds the encoded bytes.
- The codec name is persisted per row for forward compatibility.

Rules:

- Compression happens inside the sync transaction.
- Decompression happens lazily; `weave search` only decompresses when a snippet is requested.
- A decompression failure yields an empty body plus a warning; it never crashes a query.
- A re-sync may change the codec without changing document content.

## 4. Posting Compression

Transient in-memory postings (`--no-cache`):

- Document ids are delta-encoded.
- Integers use standard variable-byte encoding.
- Posting lists remain sorted by document id.
- No compression is applied to SQLite FTS5 tables; FTS5 owns its own storage format.

## 5. Determinism and Performance

- Output is deterministic for a given codec and version.
- Record the codec version in configuration/manifest for migrations.
- Targets are measured on the v2 benchmark corpus; regressions require a threshold review.

## 6. Failure and Recovery

- Unknown codec: treat as corrupted body, warn, keep metadata.
- Truncated blob: warning, empty body.
- Codec upgrade: `weave rebuild` re-encodes all bodies.
- Codec downgrade: not automatic; requires rebuild.

## 7. Testing

- Round-trip every codec.
- Threshold boundary below/above 1 KiB.
- Encrypted bodies remain uncompressed.
- Corrupted/truncated blob behavior.
- Snippet decompression.
- Rebuild re-encodes.
- Varint/delta posting round-trip and ordering.

## 8. Acceptance Criteria

- All bodies round-trip without loss.
- Encrypted content is never double-compressed.
- Corrupt bodies do not crash queries.
- Compression is deterministic.
- Rebuild migrates codecs safely.
- Memory and storage targets are measured.

## Related Documents

- [V2 Storage & Sync](./v2-storage-and-sync.md)
- [Privacy & Extensions](./privacy-and-extensions.md)
- [ROADMAP](../../ROADMAP.md)
