---
type: specification
title: "V2 Storage, Persistence & Incremental Sync"
description: "Persistent SQLite storage, sha256 incremental sync, cascading graph updates and compressed in-memory indexes."
tags:
  - "specification"
  - "storage"
  - "sqlite"
  - "sync"
  - "v2"
generated:
  by: opencode/deepseek-v4.1-flash
  at: "2026-10-27T00:00:00Z"
status: draft
---

# Weave V2: Storage, Persistence & Incremental Sync Specification

Status: **Proposed**  
Target: **v2.0**

---

## 1. Overview

Weave v2 transitions the storage engine from a transient, in-memory parser to an incremental, persistent SQLite database. The goals are:

1. **Sub-10ms CLI cold starts:** Querying a 10,000-document repository without re-parsing files.
2. **Zero-CGO static compilation:** Utilizing pure-Go SQLite (`modernc.org/sqlite`).
3. **Deterministic upsert synchronization:** Efficient diffing via content hashes (`sha256`) instead of unreliable filesystem timestamps.
4. **Cascading graph integrity:** Recomputing backlinks, degree counts, and paths when files are added, modified, or deleted.

---

## 2. SQLite Schema Design

The SQLite database lives at `.weave/index.db`. It is configured with `PRAGMA journal_mode=WAL;` and `PRAGMA foreign_keys=ON;`.

```sql
-- 1. Document Registry
CREATE TABLE IF NOT EXISTS documents (
    id TEXT PRIMARY KEY,               -- Relative path or explicit bundle ID
    file_path TEXT NOT NULL UNIQUE,     -- Path relative to repository root
    content_hash TEXT NOT NULL,         -- SHA-256 of raw file content
    title TEXT NOT NULL,
    description TEXT,
    doc_type TEXT,                      -- OKF type (e.g. runbook, spec, adr)
    body_compressed BLOB NOT NULL,      -- LZ4 or Zstandard compressed body text
    privacy_policy TEXT DEFAULT 'none', -- none | encrypt | redact
    updated_at INTEGER NOT NULL         -- Unix timestamp of sync
);

-- 2. Graph Edges
CREATE TABLE IF NOT EXISTS edges (
    source_id TEXT NOT NULL,
    target_id TEXT NOT NULL,
    edge_type TEXT NOT NULL,            -- link | related
    dangling INTEGER DEFAULT 0,         -- 1 if target does not resolve
    PRIMARY KEY (source_id, target_id, edge_type),
    FOREIGN KEY (source_id) REFERENCES documents(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_edges_target ON edges(target_id);

-- 3. Full-Text Search (FTS5)
CREATE VIRTUAL TABLE IF NOT EXISTS fts_documents USING fts5 (
    id UNINDEXED,
    title,
    description,
    keywords,
    body,
    tokenize = 'porter unicode61'
);

-- 4. Trigram Index for Substring / Typo Matching
CREATE VIRTUAL TABLE IF NOT EXISTS fts_trigrams USING fts5 (
    id UNINDEXED,
    field UNINDEXED,                    -- title | description | body
    token,
    tokenize = 'trigram'
);

-- 5. Extension Enrichments (Key-Value Keyed by Provider)
CREATE TABLE IF NOT EXISTS enrichments (
    doc_id TEXT NOT NULL,
    enricher_id TEXT NOT NULL,          -- e.g. weave-ai/embeddings-v1
    data_json TEXT NOT NULL,            -- Structured payload returned by plugin
    PRIMARY KEY (doc_id, enricher_id),
    FOREIGN KEY (doc_id) REFERENCES documents(id) ON DELETE CASCADE
);
```

---

## 3. Incremental Sync Workflow (`weave sync`)

Syncing avoids parsing unchanged files by maintaining a hash registry:

```text
Filesystem Scan (*.md)
         │
         ▼
Compute SHA-256 of file content
         │
         ├── Hash matches DB? ─────────► Skip (No-op)
         │
         ├── Hash differs from DB? ────► Re-parse AST & Update
         │
         └── File removed from disk? ──► Delete & Cascade
```

### Cascade Rules on Modification / Deletion

1. **Delete / Prune:**
   - Removing `Doc A` triggers `ON DELETE CASCADE`, removing its outgoing edges from `edges` and its entries in `fts_documents` and `fts_trigrams`.
   - Incoming edges targeting `Doc A` are marked as `dangling = 1`.
2. **Update:**
   - If `Doc A` changes, its outgoing links are re-extracted. Old edges are deleted; new edges inserted.
   - Any edges where `Doc A` was previously marked as dangling are reconciled (`dangling = 0`).

---

## 4. Memory & Storage Compression Strategy

Compression is a storage implementation detail with its own contract. Summary:

- **Index Memory (Postings):** document IDs in transient postings are delta-encoded with
  variable-byte (`varint`) encoding.
- **Body Blobs:** Markdown bodies in `documents.body_compressed` use LZ4 by default, with an optional
  Zstandard codec recorded in `body_codec`.

Authoritative rules — codec selection, thresholds, ciphertext handling, corruption recovery and
rebuild behavior — are in [Compression](./compression.md).

## Related Documents

- [Compression](./compression.md)
- [Permissive Ingestion](./permissive-ingestion.md)
- [Watch Mode](./watch-mode.md)
- [BM25 Ranking](./bm25-ranking.md)
- [Privacy & Extensions](./privacy-and-extensions.md)
- [Keyword + Fuzzy Search](./keyword-fuzzy-search.md)
- [MCP Server](./mcp-server.md)
- [ROADMAP](../../ROADMAP.md)
