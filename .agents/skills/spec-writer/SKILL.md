---
name: spec-writer
description: Generates strict OKF Markdown specifications for Go engine, search and CLI contracts.
---

# Spec-Driven Development (SDD) via OKF Protocol

You are in **Architect Mode**. Define contracts, algorithms, CLI behavior, validation and ranking
expectations as valid Open Knowledge Format Markdown.

Do NOT write implementation code or tests during this phase.

## Target Output

`../weave-docs/specs/<feature-id>.md`

## OKF Format

````markdown
---
type: specification
title: "Feature Name Specification"
description: "Feature boundary and contract."
tags:
  - "specification"
  - "go"
generated:
  by: opencode/deepseek-v4.1-flash
  at: "2026-10-26T00:00:00Z"
status: draft
---

# Specification: Feature Name

## 1. Overview
What the feature does, its package boundaries and its non-goals.

## 2. Contracts
Go types/interfaces, JSON schemas, CLI commands, flags, envelopes and exit codes.

## 3. Behavior
Algorithms, ordering, determinism and edge cases.

## 4. Data Flow
How data moves through contract -> okf/graph -> search/nav -> operations -> cmd/weave.

## 5. Validation and Errors
Required fields, invalid inputs, warnings, error codes and exit codes.

## 6. Testing and Calibration
Unit, conformance, CLI process tests, golden updates and ranking baselines.

## 7. Acceptance Criteria
Observable, testable outcomes.
````

## Instructions

1. **Read the docs rules:** Read `../weave-docs/STYLE.md` and `../weave-docs/specs/README.md`.
2. **Analyze Requirements:** Identify the package(s), CLI surface and contract changes.
3. **Build YAML Frontmatter:** `type: specification`, title, description, tags, generated, status.
4. **Draft Contracts:** Use Go types and JSON examples, not Astro/TypeScript props.
5. **Define Search Behavior:** If ranking is involved, specify weights, thresholds, top-K and
   calibration metrics.
6. **Add OKF Links:** Cross-link related specs and architecture docs.
7. **Register the spec:** Add a row to `../weave-docs/specs/README.md`. For a roadmap feature, add
   the Feature Register row to `../weave-docs/ROADMAP.md` and the backlog row to
   `../weave-docs/backlog.md`.
8. **Save:** `../weave-docs/specs/<feature-id>.md`.

## Hand-off

Pause and request approval of the spec before BDD or implementation.
