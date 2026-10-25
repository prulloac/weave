---
type: Layer Specification
title: "Contract Layer"
description: "Language-neutral domain model, JSON schemas, semantic rules and versioning shared by the Go engine and the TypeScript lens."
tags:
  - "architecture"
  - "contract"
  - "json-schema"
  - "okf"
generated:
  by: opencode/deepseek-v4.1-flash
  at: "2026-10-25T00:00:00Z"
status: draft
---

# Contract Layer

## Purpose

The contract is the single source of truth shared by every implementation. It defines:

- the domain model (`Bundle`, `Concept`, `Graph`, search and navigation results),
- the observable parsing and resolution semantics,
- the versioning policy,
- the golden fixtures every implementation must pass.

The contract must be implementable in Go, TypeScript, and any future language without reading
another implementation's source.

## Package Layout

```text
contract/
├── README.md
├── v1/
│   ├── schema/
│   │   ├── bundle.schema.json
│   │   ├── concept.schema.json
│   │   ├── graph.schema.json
│   │   ├── search.schema.json
│   │   ├── navigation.schema.json
│   │   └── error.schema.json
│   ├── types.go              # hand-maintained Go structs with json tags
│   ├── types.ts              # generated TypeScript types for the lens/tooling
│   └── golden/
│       ├── manifest.json
│       ├── bundles/          # input OKF fixtures
│       └── expected/         # normalized expected JSON
└── v2/                       # only when a breaking change is unavoidable
```

Go consumers import `weave/contract/v1`. TypeScript consumers import generated types from
`@weave/contract` or consume the JSON schemas directly.

## Naming and Encoding

- JSON wire format uses `snake_case`.
- Go structs use `PascalCase` fields with explicit `json:"..."` tags.
- TypeScript types use `snake_case` property names to avoid a mapping layer.
- Bytes are UTF-8. Invalid UTF-8 input is a per-file parse error.
- Paths in the contract are **bundle-relative POSIX paths** with forward slashes, regardless of
  the host OS. Absolute roots appear only in `Bundle.root`.
- Concept IDs never include the `.md` suffix.
- Arrays are explicitly ordered; consumers must not rely on object key order.
- Unknown JSON fields are ignored by consumers and preserved in `frontmatter`.

## Domain Model

### Bundle

```json
{
  "root": "/absolute/path/to/bundle",
  "okf_version": "0.2",
  "concepts": [],
  "index": [],
  "validation": {
    "valid": true,
    "concepts": [],
    "warnings": []
  }
}
```

| Field | Type | Required | Notes |
| --- | --- | --- | --- |
| `root` | string | yes | Absolute bundle root as resolved by the host |
| `okf_version` | `"0.2"` | no | Present only when the root `index.md` declares it |
| `concepts` | Concept[] | yes | Sorted by `id` ascending |
| `index` | IndexEntry[] | no | Root index listing order preserved |
| `validation` | Validation | yes | Bundle-level and per-concept validation |
| `warnings` | string[] | yes | Bundle-level warnings |

### Concept

```json
{
  "id": "concepts/okf-bundle",
  "path": "concepts/okf-bundle.md",
  "type": "Concept",
  "title": "OKF Bundle",
  "description": "Portable collection of OKF-compliant Markdown files.",
  "tags": ["format", "okf"],
  "status": "stable",
  "frontmatter": {
    "type": "Concept",
    "title": "OKF Bundle",
    "tags": ["format", "okf"],
    "status": "stable"
  },
  "body": "\n# OKF Bundle\n\n...",
  "links": [
    {
      "raw": "[User Table](/tables/users.md)",
      "target": "/tables/users.md",
      "resolved": "tables/users.md",
      "resolves_in_bundle": true
    }
  ]
}
```

| Field | Type | Required | Notes |
| --- | --- | --- | --- |
| `id` | string | yes | Bundle-relative path minus `.md` |
| `path` | string | yes | Bundle-relative path including `.md` |
| `type` | string | yes | Empty string when frontmatter lacks a non-empty `type` |
| `title` | string | no | Stringified frontmatter value |
| `description` | string | no | Stringified frontmatter value |
| `tags` | string[] | no | String or list normalized to a list |
| `status` | `"draft" \| "stable" \| "deprecated"` | no | Other values are ignored |
| `frontmatter` | object | yes | Raw parsed frontmatter, unknown keys preserved |
| `body` | string | yes | Text after the closing delimiter |
| `links` | Link[] | yes | Document order |

### Link

| Field | Type | Required | Notes |
| --- | --- | --- | --- |
| `raw` | string | yes | Original Markdown source |
| `target` | string | yes | Destination as written |
| `resolved` | string | no | Bundle-relative path without `.md` normalization; omitted for external targets |
| `resolves_in_bundle` | boolean | yes | True only when a concept with that id exists |

Rules:

- `#fragment` and `?query` are removed before resolution.
- A leading `/` means bundle-root-relative; otherwise resolve relative to the concept directory.
- External schemes (`https:`, `mailto:`, etc.) are never resolved.
- Image embeds are not links.
- Links inside fenced code blocks and inline code spans are ignored.
- Reference-style, collapsed-reference, shortcut-reference, titled, and angle-bracket
  destinations are supported.
- Broken link targets remain in the list with `resolves_in_bundle: false`.
- Autolinks (`<https://...>`) are excluded in contract v1.

### IndexEntry

| Field | Type | Required | Notes |
| --- | --- | --- | --- |
| `title` | string | yes | Link label from the root index |
| `target` | string | yes | Bundle-relative target without leading `/` |
| `description` | string | no | Text after a `-`, `–`, or `—` separator |

### Validation

```json
{
  "valid": false,
  "concepts": [
    {
      "id": "concepts/missing-type",
      "has_frontmatter": true,
      "has_type": false,
      "warnings": ["missing type"]
    }
  ],
  "warnings": ["broken link: concepts/broken-file.md"]
}
```

- `valid` is true only when every concept has frontmatter and a non-empty `type`.
- Files without frontmatter are excluded from `concepts` but recorded in `validation.concepts`.
- Missing optional fields produce warnings, never rejection.
- Broken links produce per-concept warnings.

### Graph

```json
{
  "nodes": [
    {
      "id": "concepts/okf-bundle",
      "path": "concepts/okf-bundle.md",
      "type": "Concept",
      "title": "OKF Bundle",
      "status": "stable",
      "tags": ["format", "okf"],
      "in_degree": 1,
      "out_degree": 2
    }
  ],
  "edges": [
    {
      "from": "concepts/okf-bundle",
      "to": "tables/users",
      "source": "link",
      "raw": "[User Table](/tables/users.md)"
    }
  ],
  "dangling": {
    "concepts/node-graph-engine": [
      {
        "raw": "[broken links](broken-file.md)",
        "target": "broken-file.md",
        "resolved": "concepts/broken-file",
        "resolves_in_bundle": false
      }
    ]
  }
}
```

- `source` is `link` for Markdown links and `related` for the frontmatter `related` array.
- Nodes and edges are emitted in deterministic id order.
- `dangling` keys are source concept ids.
- `SerializedGraph` is the subset `{ nodes, edges }` sent to the lens; it omits paths, dangling
  entries and raw link text.

### Search

```json
{
  "id": "concepts/okf-bundle",
  "title": "OKF Bundle",
  "path": "concepts/okf-bundle.md",
  "type": "Concept",
  "score": 12.34,
  "best_field": "title",
  "snippet": "…around the match…"
}
```

The search contract fixes the observable algorithm:

- Tokenization: lowercase, split on any non-letter/non-number Unicode run.
- Fields and weights: `title` 10, `tags` 6, `description` 4, `body` 3, `path` 2, `type` 2,
  `frontmatter` 1.
- BM25 constants: `k1 = 1.2`, `b = 0.75`.
- Query terms match a field token when it is equal or the field token starts with the query token.
- A document must match every query token across the selected fields.
- Scores are summed per field, then a graph degree boost (`in_degree + out_degree`) is added.
- Results sort by score descending, then id ascending; default limit 20.
- Snippets are only produced for body matches: a ±40/120 character window around the earliest
  body term.
- Empty or whitespace-only queries return no results.

### Navigation

| Result | Fields |
| --- | --- |
| `FindResult` | `id`, `title`, `path`, `type`, `score`, `snippet?` |
| `ListResult` | `id`, `title`, `path`, `type`, `status?`, `tags?` |
| `ShowResult` | `id`, `path`, `type`, `title?`, `description?`, `tags?`, `status?`, `body`, `links`, `backlinks` |
| `BacklinksResult` | `id`, `backlinks[]{from,title}` |
| `PathResult` | `from`, `to`, `path []string \| null` |

- Targets may be concept ids, bundle-relative paths, or absolute paths inside the bundle.
- `find` ranks with the search contract.
- `list` filters on type, tag (all requested tags), status and limit; results sort by id.
- `backlinks` deduplicates by source id.
- `path` performs BFS over directed edges and returns the shortest hop sequence, or `null`.

### Error Envelope

```json
{
  "error": {
    "code": "CONCEPT_NOT_FOUND",
    "message": "concept not found: does-not-exist",
    "details": {"id": "does-not-exist"}
  }
}
```

Error codes are stable and shared by the operations layer, the CLI and any future transport
adapter. CLI exit codes are a separate concern documented in [CLI layer](cli-go.md).

## Parsing Semantics

| Rule | Contract behavior |
| --- | --- |
| Concept identity | Bundle-relative path minus `.md`; `metadata.id` is opaque frontmatter |
| Required frontmatter | Non-empty `type`; missing frontmatter excludes the file from concepts |
| Unknown keys | Preserved in `frontmatter`, never rejected |
| Reserved filenames | `index.md` and `log.md` anywhere are not concepts |
| Root index | Only `<root>/index.md` sets `index` and `okf_version` |
| Nested index | Ignored as a concept and never replaces bundle metadata |
| Version mismatch | Warning plus best-effort parse, never rejection |
| YAML | Go accepts full YAML 1.2; v1 fixtures define the required observable subset |
| Comments | `#` starts a comment only at line start or after whitespace |
| Anchors and queries | Stripped before link resolution |
| Symlinks | Files followed; directories followed one level with a realpath cycle guard |
| Binary or invalid UTF-8 | Per-file parse error, recorded and skipped |
| Empty bundle | Valid, empty concepts, warning emitted |
| Determinism | Files, concepts, graphs and results use stable lexicographic ordering |

## Versioning Policy

- The contract version is `weave.contract/v1`; HTTP and serialized payloads advertise
  `X-Weave-Contract: 1`.
- Within v1: optional fields may be added; fields may not be removed, renamed, or change meaning.
- Consumers must ignore unknown fields.
- Producers must not emit fields with a different type under the same name.
- A breaking change creates `contract/v2`; both versions may be served during migration.
- The lens declares the minimum contract version it supports and must degrade gracefully when
  optional fields are absent.

## Golden Fixtures

Every implementation must pass the same corpus:

```text
contract/v1/golden/
├── manifest.json
├── bundles/
│   ├── okf-bundle/
│   ├── anchors/
│   ├── nested-index/
│   ├── verified-frontmatter/
│   ├── reference-links/
│   ├── symlinked-dirs/
│   ├── invalid-utf8/
│   ├── empty/
│   └── version-mismatch/
└── expected/
    ├── okf-bundle.bundle.json
    ├── okf-bundle.graph.json
    ├── okf-bundle.search.json
    └── ...
```

`manifest.json` lists each case, the command/output it applies to, and any normalization rules.
Golden JSON is sorted and pretty-printed so diffs are reviewable. Updating goldens requires an
explicit review, never an automatic accept in CI.

The current TypeScript fixtures under `tests/fixtures/okf-bundle` are the seed for this corpus.

## Related Documents

- [Architecture index](../index.md)
- [Engine layer](engine-go.md)
- [Operations layer](operations.md)
- [CLI layer](cli-go.md)
- [Render layer](render-go.md)
- [Lens layer](lens-ts.md)
- [Integration contracts](../integration/contracts.md)
- [Integration testing](../integration/testing.md)
