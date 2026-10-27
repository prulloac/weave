---
type: specification
title: "Weave AI Extension Specification"
description: "Optional subprocess extension for inferred keywords, enrichment and optional embedding-based reranking."
tags:
  - "specification"
  - "extension"
  - "ai"
  - "enrichment"
  - "v2"
generated:
  by: opencode/deepseek-v4.1-flash
  at: "2026-10-27T00:00:00Z"
status: draft
---

# Specification: Weave AI Extension

## 1. Overview

`weave-ai` is an optional extension that runs outside the Weave core. It can infer keywords,
produce a summary or custom metadata during sync, and optionally rerank search candidates.

It is never required by the core engine.

```sh
weave sync <path> --with-enricher ai --json
weave search <path> <query> --rerank ai --json
```

## 2. Goals and Non-Goals

### Goals

- Add enrichment without growing the core dependency tree.
- Support a local provider (Ollama) and opt-in cloud providers.
- Keep sensitive content inside explicit privacy boundaries.
- Preserve deterministic fallback when the extension is unavailable.
- Reuse the subprocess IPC contract from Privacy & Extensions.

### Non-Goals

- Bundling model weights or a runtime in core.
- Sending encrypted content to providers without explicit permission.
- Replacing BM25/fuzzy ranking.
- Automatic keyword mutation of source files.

## 3. Extension Contract

The extension is an executable named `weave-ai` in `$PATH` or `.weave/bin/`.

- Enrichment: `weave sync --with-enricher ai`.
- Reranking: `weave search --rerank ai`.
- Protocol: `weave.cli/v1` JSON over stdin/stdout.
- Timeout: 5 seconds default; configurable `--timeout`.
- Failure: core continues with deterministic scores and logs a warning.

### Enrichment request

Same shape as the [Privacy & Extensions](./privacy-and-extensions.md) enrichment request:
document id, title, frontmatter, body. Sensitive spans are redacted unless allowed.

### Enrichment response

```json
{
  "v": "weave.cli/v1",
  "inferred_keywords": ["sqlite", "zero-cgo", "wal"],
  "custom_metadata": {
    "summary": "Selection of pure-Go SQLite for static builds."
  }
}
```

`inferred_keywords` are stored as enrichment metadata, never as authoritative frontmatter keywords.
The operator may review and promote them.

### Rerank request/response

Uses the generic rerank contract from Privacy & Extensions. `rerank_score` is normalized to `[0,1]`
and blended by the configured hybrid weight `c`.

## 4. Configuration

`.weave/ai.yaml`:

```yaml
provider: ollama          # ollama | gemini | openai
model: qwen2.5:7b
base_url: http://127.0.0.1:11434
allow_sensitive: false
timeout: 5s
```

- Local `ollama` is the default.
- Cloud providers require explicit API key environment variables.
- `allow_sensitive: false` prevents encrypted/tokenized spans from being sent.
- Configuration is read only by the extension, not by the core.

## 5. Privacy Rules

- Encrypted blocks are redacted before IPC.
- Tokenized/hashed fields remain as stored; no reverse mapping is available.
- Cloud providers are opt-in per document via `privacy.ai.send: true` or a global explicit flag.
- Audit warns if AI enrichment is configured while sensitive documents are present without a
  policy.
- The core never logs request bodies.

## 6. Failure and Fallback

- Missing binary: enrichment/rerank is skipped with a warning.
- Non-zero exit: fallback to core scores.
- Timeout: fallback, warning.
- Invalid JSON: fallback, warning.
- Partial enrichment: store only valid fields.
- Reranker output missing candidates: retain core order for missing ids.

## 7. Testing

- Mock extension returning deterministic keywords.
- Mock reranker reordering candidates.
- Timeout and invalid JSON fallback.
- Sensitive redaction.
- Provider config validation.
- No extension installed.
- Conformance of enrichment/rerank payloads.
- Race/concurrency with multiple documents.

## 8. Acceptance Criteria

- Core works identically when `weave-ai` is absent.
- Inferred keywords are enrichment metadata, not contract keywords.
- Reranking modifies order only when valid scores are returned.
- Sensitive content is never sent when disallowed.
- Failures degrade gracefully and are logged.
- Protocol payloads validate against the extension contract.

## Related Documents

- [Privacy & Extensions](./privacy-and-extensions.md)
- [BM25 Ranking](./bm25-ranking.md)
- [V2 Storage & Sync](./v2-storage-and-sync.md)
- [ROADMAP](../../ROADMAP.md)
