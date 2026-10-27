---
type: Concept
title: "Keyword + Fuzzy Search"
description: "In-memory keyword and weighted fuzzy search over related OKF documents, returning keyword, fuzzy and hybrid top-K results."
tags:
  - "architecture"
  - "search"
  - "fuzzy"
  - "keywords"
  - "index"
  - "offline"
generated:
  by: opencode/big-pickle
  at: "2026-08-06T00:00:00Z"
verified:
  - by: "human:developer"
    at: "2026-08-06T00:00:00Z"
status: draft
---

# Keyword + Fuzzy Search

## Summary

Weave keeps an in-memory search index beside the graph. Search has two dimensions: mandatory
frontmatter keywords and weighted fuzzy matching over title, description and body. Every query
returns top-K results.

## Details & Contracts

| Contract | Guarantee |
| --- | --- |
| Keyword input | Required `keywords` frontmatter list |
| Keyword matching | Exact token (1.0) or prefix (0.75), AND across query tokens |
| Fuzzy fields | Title 1.00, description 0.60, body 0.30 |
| Fuzzy similarity | Normalized Levenshtein and trigram Dice, maximum |
| Modes | `keyword`, `fuzzy`, `hybrid` |
| Default mode | `hybrid` |
| Result shape | Keyword, fuzzy and hybrid top-K lists with component scores |
| Default K | 10; maximum 100 |
| Operational mode | In-memory, offline, deterministic |
| Calibration | Labeled corpus with NDCG@K and MRR@K baselines |

Documents missing keywords are invalid for the Weave profile: they remain available for graph
navigation and fuzzy search but never appear in keyword results.

## Related Concepts

- Indexes [OKF Bundle](./okf-bundle.md)
- Built alongside [Node Graph Engine](./node-graph-engine.md)
- Specified by [Keyword + Fuzzy Search](../../specs/keyword-fuzzy-search.md)
- Implements [Weave Vision](./weave-vision.md)
