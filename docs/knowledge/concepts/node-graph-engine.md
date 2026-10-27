---
type: Concept
title: "Node Graph Engine"
description: "Static engine that builds a fully traversable node graph from parsed OKF bundles in memory."
tags:
  - "architecture"
  - "graph"
  - "engine"
  - "static"
generated:
  by: opencode/big-pickle
  at: "2026-08-06T00:00:00Z"
verified:
  - by: "human:developer"
    at: "2026-08-06T00:00:00Z"
status: stable
---

# Node Graph Engine

## Summary

High-performance static engine that turns parsed OKF bundles into a fully traversable node graph. Nodes carry keywords; edges come from resolved Markdown links and `related` frontmatter. Fully offline.

## Details & Contracts

| Contract | Guarantee |
| --- | --- |
| Input | Parsed [OKF Bundle](./okf-bundle.md) |
| Output | Fully traversable node graph |
| Edge discovery | Resolved Markdown links + `related` frontmatter |
| Build technique | In-memory assembly from parsed concepts |
| Performance | Graph generated in milliseconds |
| Operational mode | Strictly offline, local file system only |
| Persistence | In-memory; no artifacts, no target writes |

## Related Concepts

- Consumes [OKF Bundle](./okf-bundle.md)
- Feeds [Search Index](./search-index.md)
- Consumed by the Go CLI
- Implements [Weave Vision](./weave-vision.md)
