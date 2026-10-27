---
type: specification
title: "Permissive Ingestion Specification"
description: "v1.0 ingestion mode for indexing arbitrary Markdown trees with warnings instead of hard validation failures."
tags:
  - "specification"
  - "ingestion"
  - "markdown"
  - "v1"
generated:
  by: opencode/deepseek-v4.1-flash
  at: "2026-10-27T00:00:00Z"
status: draft
---

# Specification: Permissive Ingestion

## 1. Overview

Strict parsing accepts only OKF documents with `type` and `keywords`. Permissive ingestion lets
v1.0 index arbitrary Markdown trees while preserving warnings. It is opt-in and never changes the
meaning of strict mode.

```sh
weave ingest <path> [--permissive|--strict] --json
weave sync <path> [--permissive|--strict] --json
weave audit <path> --permissive --json
```

## 2. Goals and Non-Goals

### Goals

- Ingest dirty or non-OKF Markdown without failing the whole bundle.
- Preserve warning visibility and auditability.
- Keep keyword search deterministic: synthetic documents do not invent keywords.
- Make migration from existing Markdown trees practical.

### Non-Goals

- Guessing keywords heuristically.
- Semantic classification of arbitrary documents.
- Silently making invalid documents valid.
- Changing strict mode behavior.

## 3. Mode Contract

| Command | Default | Override |
| --- | --- | --- |
| `weave ingest` | permissive | `--strict` |
| `weave sync` | mode recorded at ingest | `--permissive` / `--strict` |
| `weave parse`, `graph`, `search` | strict/v1 behavior | no change |

- `--permissive` and `--strict` are mutually exclusive.
- The mode is stored with the v1.0 database so later syncs are consistent.
- `--strict` on a permissive database reports documents that would be invalid.

## 4. Synthetic Documents

When frontmatter is missing or incomplete in permissive mode:

| Missing | Fallback |
| --- | --- |
| Frontmatter block | `frontmatter = {}` |
| `type` | `type = "Document"` |
| `keywords` | `keywords = []` |
| `title` | first H1, otherwise filename without `.md` |
| `description` | first non-heading paragraph, truncated to 160 chars |

- Original body is preserved.
- Unknown/absent frontmatter keys remain absent; the parser does not fabricate metadata beyond the
  fields above.
- `index.md` and `log.md` remain reserved and are not concepts.
- Link extraction and resolution are identical to strict mode.

## 5. Validation and Search

Validation findings remain visible:

- `MISSING_FRONTMATTER` — warning
- `MISSING_TYPE` — warning with synthetic `type: Document`
- `MISSING_KEYWORDS` — warning; concept has empty keywords
- broken links remain errors

Search behavior:

| Dimension | Synthetic document |
| --- | --- |
| Exact keyword mode | Excluded (no keywords) |
| BM25 lexical mode | Included using title/description/body |
| Fuzzy mode | Included |
| Hybrid | Included via BM25/fuzzy components |
| Graph | Included |

`Bundle.validation.valid` is `false` when findings exist, but `weave ingest --permissive` still
succeeds. `weave audit --permissive` treats these findings as warnings; `--fail-on error` therefore
does not fail solely because of them.

## 6. Output

`weave ingest`/`sync` report:

```json
{
  "mode": "permissive",
  "documents": 42,
  "synthetic": 7,
  "warnings": 9,
  "errors": 0
}
```

The audit report remains the source of detailed per-document findings.

## 7. Determinism

- Synthetic titles/descriptions derive deterministically from source order.
- Document order and IDs remain path-based.
- No inference depends on filesystem order.

## 8. Testing

- Empty Markdown file.
- Markdown with no frontmatter.
- Frontmatter missing `type`, `keywords`, or both.
- Frontmatter with invalid types.
- Strict mode on the same fixtures.
- Search inclusion/exclusion matrix.
- Audit `--permissive` severity behavior.
- Deterministic synthesis.

## 9. Acceptance Criteria

- `weave ingest --permissive` indexes a plain Markdown tree without failing.
- Synthetic documents never appear in exact keyword results.
- They do appear in BM25 and fuzzy results.
- Warnings remain auditable.
- Strict mode behavior is unchanged.
- Mode persists across syncs.

## Related Documents

- [Storage & Sync](./storage-and-sync.md)
- [BM25 Ranking](./bm25-ranking.md)
- [Keyword + Fuzzy Search](./keyword-fuzzy-search.md)
- [Weave Audit](./weave-audit.md)
- [ROADMAP](../../ROADMAP.md)
