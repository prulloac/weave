---
type: Concept
title: "Weave Vision"
description: "Core vision of Weave: a local-first related-document database that weaves flat OKF bundles into a searchable, navigable knowledge graph."
tags:
  - "vision"
  - "architecture"
  - "cli"
  - "okf"
  - "search"
  - "knowledge-management"
  - "local-first"
generated:
  by: opencode/big-pickle
  at: "2026-08-06T00:00:00Z"
verified:
  - by: "human:developer"
    at: "2026-08-06T00:00:00Z"
status: draft
---

# Weave Vision

> "To weave flat files into a living universe of knowledge."

## Summary

Weave is a local-first related-document database for Open Knowledge Format (OKF) bundles. The core
loads flat Markdown and YAML files into memory, builds concept nodes and link edges, and answers
keyword and weighted fuzzy searches with top-K results. v1.0 adds optional SQLite persistence, BM25
ranking, privacy directives, extensions and MCP. It is CLI-first, local, and has no browser UI.

## Details & Contracts

### The Purpose

Weave turns a static bundle into a queryable document store:

- Input: OKF bundles of Markdown and YAML flat files.
- Relationships: Markdown links and `related` entries become graph edges.
- Retrieval: mandatory frontmatter keywords plus weighted fuzzy search.
- Surface: Go CLI, local-first, machine-readable envelopes.
- Output: ranked top-K results and a traversable graph.

### Search Contract

| Dimension | Guarantee |
| --- | --- |
| Keywords | Mandatory frontmatter field; exact token plus prefix matching |
| Fuzzy fields | Title highest, description second, body last |
| Results | Keyword, fuzzy and hybrid top-K lists |
| Default mode | Hybrid |
| Default top-K | 10 |
| Calibration | Labeled corpus with NDCG@K and MRR@K baselines |
| Operational mode | Offline, deterministic; in-memory core with optional v1.0 SQLite persistence |

### What Weave Is Not

- Not a browser visualization or lens product.
- Not a mount server or static site generator.
- Not an embedding or natural-language question-answering system in the core engine.

## Related Concepts

- Depends on [OKF Specification](./okf-bundle.md)
- Related to [Node Graph Engine](./node-graph-engine.md)
- Related to [Search Index](./search-index.md)
- Implemented by [ADR 0003](../../architecture/adr/0003-keyword-fuzzy-search-and-ui-retirement.md)
