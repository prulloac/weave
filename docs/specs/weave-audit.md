---
type: specification
title: "Weave Audit Specification"
description: "CLI command for auditing bundle conformance, graph quality, keyword coverage, index consistency and dangling links."
tags:
  - "specification"
  - "cli"
  - "audit"
  - "quality"
  - "graph"
generated:
  by: opencode/deepseek-v4.1-flash
  at: "2026-10-26T00:00:00Z"
status: stable
---

# Specification: Weave Audit

Status: **Implemented** for v1.0. `--permissive` is reserved for the permissive-ingestion feature.

## 1. Overview

`weave audit` evaluates a bundle as a knowledge base, not just as a parser input. It reports
conformance failures and graph-quality issues so the bundle can be fixed in CI or locally.

```sh
weave audit <path> [--fail-on error|warning|none] [--verify-keys] [--json] [--no-cache]
```

## 2. Goals and Non-Goals

### Goals

- Surface invalid documents, missing keywords and broken references.
- Report orphan concepts and index inconsistencies.
- Produce a machine-readable report with stable finding codes.
- Work as a CI quality gate.

### Non-Goals

- Automatic fixes or writes into the bundle.
- Style/grammar linting.
- Semantic duplicate detection.
- Ranking quality evaluation (that is calibration).

## 3. Command Contract

```text
weave audit <path> [--fail-on error|warning|none] [--verify-keys] [--json] [--no-cache]
```

- `--fail-on` defaults to `error`.
- `--verify-keys` is a v1.0 privacy check: report unavailable keys for documents that use
  encryption/tokenization/hashing. It is a no-op without privacy directives.
- `--permissive` (v1.0 ingestion mode) downgrades missing frontmatter/keywords from errors to
  warnings so dirty Markdown trees can be audited without failing. It is delivered with
  [Permissive Ingestion](./permissive-ingestion.md), not by the initial audit implementation.
- `--json` emits one `weave.cli/v1` envelope.
- Exit codes:
  - `0` — audit completed and no finding met the failure threshold.
  - `1` — findings met the failure threshold.
  - `2` — usage/invalid root.
  - `3` — permission denied.

The audit itself succeeding is different from the bundle passing. In JSON mode a completed audit
uses a success envelope containing the report, even when the process exits `1`.

## 4. Report Model

```json
{
  "path": "/absolute/bundle",
  "ok": false,
  "fail_on": "error",
  "summary": {
    "concepts": 5,
    "valid": false,
    "edges": 5,
    "orphans": 1,
    "broken_links": 1,
    "missing_keywords": 1,
    "missing_index_targets": 0,
    "unindexed_concepts": 1
  },
  "findings": [
    {
      "code": "MISSING_KEYWORDS",
      "severity": "error",
      "concept": "concepts/example",
      "message": "concept has no keywords"
    },
    {
      "code": "BROKEN_LINK",
      "severity": "error",
      "source": "concepts/example",
      "target": "concepts/missing.md",
      "message": "link target does not resolve in the bundle"
    },
    {
      "code": "ORPHAN_CONCEPT",
      "severity": "warning",
      "concept": "concepts/isolated",
      "message": "concept has no incoming or outgoing edges"
    }
  ]
}
```

### Finding codes

| Code | Severity | Meaning |
| --- | --- | --- |
| `MISSING_FRONTMATTER` | error | File excluded from concepts |
| `MISSING_TYPE` | error | Concept has no non-empty type |
| `MISSING_KEYWORDS` | error | Concept has no non-empty keyword list |
| `BROKEN_LINK` | error | Markdown link does not resolve in-bundle |
| `BROKEN_RELATED` | error | `related` entry does not resolve |
| `MISSING_INDEX_TARGET` | error | Root index entry points to an unknown concept |
| `UNINDEXED_CONCEPT` | warning | Concept is absent from the root index |
| `ORPHAN_CONCEPT` | warning | Concept has no edges |
| `UNSUPPORTED_OKF_VERSION` | warning | Root index declares an unsupported version |
| `EMPTY_BUNDLE` | warning | No Markdown files |
| `DANGLING_EXTERNAL` | info | External/dangling link; informational only |
| `KEY_UNAVAILABLE` | warning | Privacy directive references a missing key (v1.0, `--verify-keys`) |

Severity ordering: `error` > `warning` > `info`.

### Summary

- `concepts`: parsed concept count
- `valid`: bundle validation result
- `edges`: resolved graph edge count
- `orphans`: concepts with degree zero
- `broken_links`: unresolved in-bundle links
- `missing_keywords`: invalid-keyword concepts
- `missing_index_targets`: index entries with unknown targets
- `unindexed_concepts`: concepts not listed in the root index

## 5. Data Sources

The audit consumes the engine's existing structures:

- `Bundle.validation`
- `Bundle.index`
- `Graph.Nodes`, `Graph.Edges`, `Graph.Dangling`
- concept links and frontmatter

No new parser behavior is required.

## 6. Human Output

```text
weave audit: /notes
  5 concepts · 5 edges · 1 orphan
  1 missing keywords · 1 broken link

errors
  concepts/example  MISSING_KEYWORDS  concept has no keywords
  concepts/example  BROKEN_LINK       concepts/missing.md

warnings
  concepts/isolated ORPHAN_CONCEPT    no incoming or outgoing edges
```

Exit lines:

- `audit: ok` when no threshold failure.
- `audit: failed (N errors, M warnings)` when the threshold is met.

## 7. Determinism

- Findings sort by severity (error, warning, info), then code, then concept/source, then target.
- Summary counts are deterministic.
- JSON fields use `snake_case`.

## 8. Testing

- Fixture with missing keywords, broken links, orphans and index gaps.
- Exit-code matrix for `--fail-on error|warning|none`.
- Empty bundle and invalid root.
- Deterministic ordering.
- CLI envelope and human output.

## 9. Acceptance Criteria

- `weave audit` reports every finding code above on the relevant fixture.
- `--fail-on` changes only the exit code, not the report.
- JSON reports are stable and schema-validated.
- No writes occur inside the bundle.
- CI can use the command as a docs quality gate.

## Related Documents

- [Keyword + Fuzzy Search spec](./keyword-fuzzy-search.md)
- [Node Graph Engine spec](./node-graph-engine.md)
- [Privacy & Extensions](./privacy-and-extensions.md)
- [Permissive Ingestion](./permissive-ingestion.md)
- [CLI layer](../architecture/layers/cli-go.md)
- [Integration contracts](../architecture/integration/contracts.md)
- [ROADMAP](../../ROADMAP.md)
