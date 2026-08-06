---
name: okf-generator
description: Generates valid Open Knowledge Format (OKF v0.2) markdown documents with YAML frontmatter, following the Google Cloud knowledge-catalog OKF SPEC.
---

# Open Knowledge Format (OKF) Generator Protocol

You are in **Knowledge Engineer Mode**. Your objective is to create valid OKF v0.2 concepts formatted as **Markdown documents with YAML frontmatter** according to the official OKF specification (https://github.com/GoogleCloudPlatform/knowledge-catalog/blob/main/okf/SPEC.md).

## OKF v0.2 Specification Rules

1. **File Format:** Plain Markdown (`.md`) with YAML frontmatter delimited by `---`.
2. **Concept Identifier:** The path of the concept's file within the bundle, with the `.md` suffix removed (e.g., `concepts/architecture/event-driven`). There is NO `metadata.id` field.
3. **Required Frontmatter:** `type` is the ONLY required field. It is a short descriptive string, not a fixed enum (e.g., `BigQuery Table`, `Metric`, `Playbook`, `Concept`). Types are not registered centrally; consumers MUST tolerate unknown types.
4. **Recommended Frontmatter:** `title` (display name), `description` (one-line summary), `resource` (canonical URI for the underlying asset; omit for abstract concepts), `tags` (YAML list).
5. **Reserved Filenames:** `index.md` (directory listing) and `log.md` (update history) at any level MUST NOT be used for concept documents.
6. **Graph Relationships:** Expressed using standard Markdown links. Prefer **absolute bundle-relative** paths (e.g., `[User Table](/tables/users.md)`); relative paths are also allowed. Links are untyped directed edges.
7. **`okf_version`:** Only permitted in a bundle-root `index.md` frontmatter block (`okf_version: "0.2"`). Concept documents MUST NOT carry `okf_version`.
8. **Version:** This project targets OKF v0.2, NOT 1.0.

## Frontmatter Families (all optional)

### Trust: `generated` and `verified`

```yaml
generated:
  by: reference_agent/gemini-2.5-pro  # actor: <producer>/<version>, human:<id>, or process:<id>
  at: "2026-06-20T22:53:05Z"          # ISO 8601 datetime, last meaningful content change
verified:
  - by: human:developer               # human:<id> actors imply human-reviewed trust tier
    at: "2026-06-25T09:00:00Z"
```

`generated.by` is REQUIRED inside `generated`. A single `verified` entry MAY be a bare `{ by, at }` mapping (consumers MUST treat it as a one-element list).

**Actor convention:** `human:<id>` for people, `<producer>/<version>` for agents/tools, `process:<id>` for automated processes. Tools MUST NOT use `/`-prefixed forms.

### Lifecycle: `status` and `stale_after`

```yaml
status: stable        # draft | stable | deprecated (absent ⇒ stable)
stale_after: "2026-09-23"   # optional absolute YYYY-MM-DD
```

### Provenance: `sources` and `usage_window`

```yaml
sources:
  - id: rev-policy
    resource: https://wiki.acme/finance/revenue-recognition
    title: Revenue recognition policy
    author: team:finance-fpa      # optional credibility signal
    usage_count: 5000             # optional liveness signal
    last_modified: 2026-06-18
usage_window: { from: 2026-06-01, to: 2026-06-30 }
```

`resource` is REQUIRED inside each `sources` entry (URL, bundle-relative path, or scope descriptor). Per-claim attribution uses markdown footnotes keyed to `sources[].id`.

## Document Structure Template

```markdown
---
type: Concept # REQUIRED. Descriptive string; only required field.
title: "Human Readable Title"
description: "A concise one-line summary of this concept."
tags:
  - "domain"
  - "architecture"
generated:
  by: opencode/big-pickle # Actor: <producer>/<version>, human:<id>, process:<id>
  at: "2026-06-20T22:53:05Z"
verified:
  - by: "human:developer"
    at: "2026-06-25T09:00:00Z"
status: stable
---

# Title

## Summary
Core explanation of the concept...

## Details & Contracts
Inject domain-specific sections here (e.g., code blocks, tables, schemas).

## Related Concepts
- Linked to [Parent Domain](/domains/backend.md)
- Depends on [Data Source](/concepts/data-pipeline.md)
```

## Instructions

1. **Parse Input**: Identify the domain, title, and key concept details.
2. **Assign Type & Path**: Set a descriptive `type` and output file path under docs/knowledge/<type>s/<concept-id>.md. Concept ID = file path minus `.md`.
3. **Build Frontmatter**: Include `type` (required) plus `title`, `description`, `tags`, `generated`, `verified`, `status` as appropriate. No `metadata` block, no `okf_version`, no `created_at`/`updated_at`.
4. **Draft Markdown Body**: Write structured markdown headings, tables, or code fences.
5. **Cross-Link Graph**: Link to other OKF concepts in the body using absolute bundle-relative paths (/path/to/concept.md). Link every concept referenced; consumers tolerate broken links.

## Bundle-Root index.md (version declaration)

When generating a new knowledge corpus, create a bundle-root `docs/knowledge/index.md` declaring the target version and listing concepts for progressive disclosure:

```markdown
---
okf_version: "0.2"
---

# Knowledge Index

## Concepts
- [Weave Vision](concepts/weave-vision.md) - One-line description
```

This is the ONLY place `okf_version` is permitted.

## Validation Hand-off

Before finishing, verify that:

- `type` is present and non-empty.
- `okf_version` appears ONLY in a bundle-root `index.md` and equals `"0.2"` — never "1.0", never in concept documents.
- No `metadata` block; concept ID is derived from the file path.
- `generated.at` and any `verified[].at` are ISO 8601 datetimes.
- `generated.by` and `verified[].by` use the actor convention (`human:<id>`, `<producer>/<version>`, `process:<id>`) — no `/`-prefixed tool names.
- `status`, if present, is `draft`, `stable`, or `deprecated`.
- All referenced concept links resolve within the bundle, or are acknowledged as forward references.
- No concept file uses a reserved filename (`index.md`, `log.md`).
