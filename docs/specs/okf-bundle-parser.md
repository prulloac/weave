---
type: specification
title: "OKF Bundle Parser Specification"
description: "Contract for parsing OKF v0.2 bundles into validated, traversable concept documents with extracted graph edges."
tags:
  - "okf"
  - "parser"
  - "specification"
generated:
  by: opencode/big-pickle
  at: "2026-08-06T00:00:00Z"
status: draft
---

# Specification: OKF Bundle Parser

## 1. Overview

Pure, offline TypeScript module that turns an OKF v0.2 bundle (a directory tree of Markdown files with YAML frontmatter) into validated `ParsedBundle` structures. It is the foundation for both the [Node Graph Engine](./node-graph-engine.md) and [Search Index](./search-index.md); every downstream engine consumes its output.

Conformance follows the permissive OKF v0.2 rules: a bundle is valid if every non-reserved `.md` file has parseable frontmatter with a non-empty `type`; unknown keys, unknown types, broken links, and missing optional fields are tolerated, never rejected.

## 2. Module Structure & Execution Context

- **Library Root:** `src/lib/okf/`
- **No hydration / no client JS.** Runs in the Astro server runtime and CLI only. Zero browser bundle.
- **Runtime:** Bun-first; uses native `Bun.file`, `Bun.glob`, and Node-compatible `fs/promises` fallbacks. No third-party YAML parser — hand-rolled minimal frontmatter + YAML-subset parser keeps the dependency tree empty and the build static.

**Files:**

- `src/lib/okf/types.ts` — shared interfaces and enums.
- `src/lib/okf/frontmatter.ts` — YAML frontmatter block extraction and parsing.
- `src/lib/okf/links.ts` — Markdown link extraction (both `(/abs)` and `(rel.md)` forms).
- `src/lib/okf/parser.ts` — bundle walker + concept assembly.
- `src/lib/okf/validate.ts` — conformance validation.

## 3. Component Contracts

### 3.1 Concept

```typescript
export interface OkfConcept {
  /** Concept ID = file path within bundle, minus .md */
  id: string;
  /** Bundle-relative path, e.g. "concepts/okf-bundle.md" */
  path: string;
  /** Raw frontmatter record; unknown keys preserved */
  frontmatter: Record<string, unknown>;
  /** Required by OKF v0.2; non-empty string */
  type: string;
  /** Optional display name */
  title?: string;
  /** Optional one-line summary */
  description?: string;
  /** Optional cross-cutting tags */
  tags?: string[];
  /** Optional lifecycle status: draft | stable | deprecated */
  status?: 'draft' | 'stable' | 'deprecated';
  /** Body text after frontmatter */
  body: string;
  /** Outgoing edges: resolved link targets */
  links: OkfLink[];
}

export interface OkfLink {
  /** Raw markdown link text */
  raw: string;
  /** Bundle-relative target with .md, or absolute URL */
  target: string;
  /** Resolved path relative to bundle root; null for external URLs */
  resolved?: string;
  /** True if target file exists within bundle */
  resolvesInBundle: boolean;
}
```

### 3.2 Bundle

```typescript
export interface ParsedBundle {
  /** Absolute root path of the bundle */
  root: string;
  /** Declared version from root index.md frontmatter, if present */
  okfVersion?: '0.2';
  /** All concepts keyed by concept ID */
  concepts: Map<string, OkfConcept>;
  /** Root index.md listing, parsed into entries */
  index?: IndexEntry[];
  /** Validation result */
  validation: ValidationResult;
}

export interface IndexEntry {
  title: string;
  /** Bundle-relative target */
  target: string;
  description?: string;
}

export interface ValidationResult {
  valid: boolean;
  concepts: Map<string, ConceptValidation>;
}

export interface ConceptValidation {
  /** Parseable YAML frontmatter present */
  hasFrontmatter: boolean;
  /** Non-empty type present */
  hasType: boolean;
  /** Warnings only — never reject: missing optional fields, broken links */
  warnings: string[];
}
```

### 3.3 Parser API

```typescript
export function parseBundle(root: string): Promise<ParsedBundle>;
export function parseConcept(filePath: string, bundleRoot: string): Promise<OkfConcept>;
export function extractFrontmatter(raw: string): { data: Record<string, unknown>; body: string } | null;
export function extractLinks(body: string): OkfLink[];
```

## 4. Data Flow & Dependencies

- Walk bundle tree with `Bun.glob('**/*.md')`.
- Skip reserved filenames `index.md` and `log.md` from concept mapping (index.md is parsed into `index`; log.md is ignored by the parser).
- Per file: split frontmatter (`---` delimited), parse YAML subset, validate `type`, extract links from body.
- Resolve links to bundle-relative paths; mark `resolvesInBundle` against the concept map.
- Targets: build graph ([Node Graph Engine](./node-graph-engine.md)) and corpus ([Search Index](./search-index.md)) directly from `ParsedBundle`.
- Implements contract from [OKF Bundle](../knowledge/concepts/okf-bundle.md).

## 5. Performance, Edge Cases & Errors

- **Performance:** parse + validate a 10k-file bundle in under 100 ms on a local SSD; streaming reads, no full-tree string buffering. Meets vision's "milliseconds" graph generation budget.
- **Missing/broken frontmatter:** file excluded from `concepts`, recorded with `hasFrontmatter: false`; never throws.
- **Missing `type`:** concept included, flagged `hasType: false`; consumers decide routing.
- **Broken links:** kept as edges with `resolvesInBundle: false`; graph consumers MUST tolerate.
- **Unknown keys/types:** preserved and ignored; MUST NOT reject.
- **Non-UTF-8 or binary `.md`:** treated as parse error, recorded, skipped.
- **Symlinked directories:** followed at most one level deep; cycles guarded with a visited-set.
- **Empty bundle / no `.md` files:** valid bundle, empty `concepts`, warning emitted.
- **`okf_version` mismatch (e.g. `"1.0"`):** warning, best-effort parse per spec §12.

## Related Concepts

- Depends on [OKF Bundle](../knowledge/concepts/okf-bundle.md)
- Feeds [Node Graph Engine](../knowledge/concepts/node-graph-engine.md)
- Feeds [Search Index](../knowledge/concepts/search-index.md)
- Runs inside [Git Repository Mount](../knowledge/concepts/git-repository-mount.md)
