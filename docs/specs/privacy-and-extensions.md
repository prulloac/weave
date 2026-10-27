---
type: specification
title: "Privacy Directives & Extension Architecture"
description: "Explicit encryption, tokenization and hashing directives, plus Git-style subprocess extensions over weave.cli/v1."
tags:
  - "specification"
  - "privacy"
  - "crypto"
  - "extensions"
  - "v2"
generated:
  by: opencode/deepseek-v4.1-flash
  at: "2026-10-27T00:00:00Z"
status: draft
---

# Weave: Privacy Directives & Extension Architecture Specification

Status: **Proposed**  
Target: **v2.0**

---

## 1. Privacy Directives

Weave enforces sensitive data boundaries **deliberately declared by the user**. Heuristic regex detectors are prohibited to avoid accidental data leakage and false positives.

### 1.1 Directives Syntax

#### Frontmatter (File-Level)

Encrypts or tokenizes the entire document body:

```yaml
---
title: "Core Database Incident Protocol"
type: runbook
keywords: [infra, database, credentials]
privacy:
  policy: encrypt
  provider: aes-256-gcm
  key_id: ops-prod-key
---
```

#### Inline Directives (Block-Level)

Encloses sensitive spans within standard CommonMark HTML comments:

```markdown
The staging bastion host is accessible via `bastion.internal`.

<!-- weave:encrypt provider="aes-256-gcm" key_id="staging-secrets" -->
Root deployment token: sec_live_994821a8f9
<!-- /weave:encrypt -->

Audit logs reference customer accounts:
<!-- weave:tokenize provider="hmac-sha256" key_id="tenant-salt" -->
Account ID: internal-client-alpha
<!-- /weave:tokenize -->
```

### 1.2 Directive Actions & Search Behavior

| Directive | Storage in SQLite (`show`) | FTS5 Indexing (`search`) | Search Retrieval Mode |
| --- | --- | --- | --- |
| `weave:encrypt` | AES-256-GCM ciphertext BLOB | Excluded completely | Omitted from search. Restored on `weave show --decrypt` with key. |
| `weave:tokenize` | Pseudonym alias | Indexes pseudonym token | Searchable via matching token query. |
| `weave:hash` | HMAC-SHA256 hex digest | Indexes digest | **Exact match only** (`HMAC(query) == digest`). Trigram/fuzzy bypassed. |

### 1.3 Key Resolution Order

1. **Environment Variables:** `WEAVE_KEY_<KEY_ID_UPPERCASE>` (e.g., `WEAVE_KEY_OPS_PROD_KEY`).
2. **Local Security Config:** `.weave/security.yaml` (never committed to version control).
3. **Missing Keys:**
   - During `weave audit --verify-keys`: Emits lint warning `KEY_UNAVAILABLE`.
   - During `weave show --decrypt`: Fails with Exit Code `3`.

---

## 2. Extension Architecture (Subprocess IPC)

Weave uses a Git-style subprocess model for extensibility. Any executable named `weave-<plugin>` located in `$PATH` or `.weave/bin/` is recognized as an extension.

```text
Host: weave search --rerank my-ai
                 │
                 │ JSON over stdin (weave.cli/v1)
                 ▼
Subprocess: weave-my-ai (Ollama / Python / Rust / Go)
                 │
                 │ JSON over stdout (weave.cli/v1)
                 ▼
Host: Blends scores and prints final Top-K
```

### 2.1 Standard Extension Protocol (`weave.cli/v1`)

#### Ingestion Enrichment Request (`stdin`)

Fired during `weave sync --with-enricher <name>`:

```json
{
  "v": "weave.cli/v1",
  "type": "enrichment_request",
  "doc_id": "docs/adrs/adr-004.md",
  "title": "Database Storage Selection",
  "frontmatter": { "type": "adr", "keywords": ["database", "storage"] },
  "body": "We decided to adopt pure-Go SQLite..."
}
```

#### Ingestion Enrichment Response (`stdout`)

```json
{
  "v": "weave.cli/v1",
  "inferred_keywords": ["sqlite", "cgo-free", "wal"],
  "custom_metadata": {
    "summary": "Selection of pure-Go SQLite for zero-CGO static builds."
  }
}
```

#### Reranker Request (`stdin`)

Fired during `weave search <query> --rerank <name>`:

```json
{
  "v": "weave.cli/v1",
  "type": "rerank_request",
  "query": "zero cgo database",
  "candidates": [
    { "id": "docs/adrs/adr-004.md", "lexical_score": 0.82, "fuzzy_score": 0.45 },
    { "id": "docs/adrs/adr-001.md", "lexical_score": 0.31, "fuzzy_score": 0.10 }
  ]
}
```

#### Reranker Response (`stdout`)

```json
{
  "v": "weave.cli/v1",
  "ranked_candidates": [
    { "id": "docs/adrs/adr-004.md", "rerank_score": 0.95 },
    { "id": "docs/adrs/adr-001.md", "rerank_score": 0.15 }
  ]
}
```

### 2.2 Extension Guarantees & Fallbacks

- Extensions must time out after 5 seconds by default unless `--timeout` is set.
- If an extension fails or exits non-zero, Weave falls back to the deterministic core search score and logs a diagnostic warning to `stderr`.
- Extensions do not have write access to `.weave/index.db`; all persistence is mediated strictly by Weave core transactions.

### 2.3 External Crypto Providers

Crypto providers may be supplied as extensions (`weave-crypto-age`, `weave-crypto-vault`,
`weave-crypto-kms`) using the same subprocess IPC.

- Provider input: operation (`encrypt`, `decrypt`, `hash`, `tokenize`), key id, plaintext/ciphertext.
- Provider output: ciphertext/digest/pseudonym plus provider metadata.
- Keys never enter Weave core logs or the SQLite database.
- Missing/failed providers fall back to `KEY_UNAVAILABLE`; encrypted reads require an explicit
  provider and key.
- Provider implementations are opt-in and never required for core operation.

The optional AI extension is specified separately: [Weave AI Extension](./weave-ai-plugin.md).

## Related Documents

- [V2 Storage & Sync](./v2-storage-and-sync.md)
- [Weave AI Extension](./weave-ai-plugin.md)
- [Keyword + Fuzzy Search](./keyword-fuzzy-search.md)
- [Weave Audit](./weave-audit.md)
- [MCP Server](./mcp-server.md)
- [ROADMAP](../../ROADMAP.md)
