---
name: okf-generator
description: Generates valid Open Knowledge Format (OKF v0.2) markdown documents with YAML frontmatter, following Google Cloud's OKF specification.
---

# Open Knowledge Format (OKF) Generator Protocol

You are in **Knowledge Engineer Mode**. Your objective is to create valid OKF concepts formatted as **Markdown documents with YAML frontmatter** according to the official OKF specification.

## OKF Specification Rules

1. **File Format:** Plain Markdown (`.md`) with YAML frontmatter delimited by `---`.
2. **Concept Identifier:** Derived from the file path without `.md` (e.g., `concepts/architecture/event-driven`).
3. **Required Frontmatter:** `type` is the ONLY strictly required field.
4. **Graph Relationships:** Expressed using **standard Markdown links** in the document body, using bundle-relative paths (e.g., `[User Table](/tables/users.md)`).

---

## Document Structure Template

```markdown
---
type: concept # REQUIRED. Options: concept, architecture, dataset, metric, process, policy
title: "Human Readable Title"
description: "A concise 1-2 sentence summary of this concept."
tags:
  - "domain"
  - "architecture"
generated:
  by: "/opencode/okf-generator" # Actor format: / for tools, human: for people, process: for automation
  at: "YYYY-MM-DDTHH:MM:SSZ"
verified:
  - by: "human:developer"
    at: "YYYY-MM-DDTHH:MM:SSZ"
status: active
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
2. **Assign Type & Path**: Set a valid type and output file path under docs/knowledge/<type>s/<concept-id>.md.
3. **Build Frontmatter**: Include type (required) along with title, description, tags, generated, and verified metadata.
4. **Draft Markdown Body**: Write structured markdown headings, tables, or code fences.
5. **Cross-Link Graph**: Link to other OKF concepts in the body using absolute bundle paths (/path/to/concept.md).

## Validation Hand-off

Before finishing, verify that:

- `okf_version` is explicitly defined as "1.0".
- `metadata.id` uses valid kebab-case.
- ISO 8601 timestamps are populated for `created_at` and `updated_at`.
