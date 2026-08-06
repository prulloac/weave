---
type: Concept
title: "Node Graph Engine"
description: "Static engine that builds a fully traversable node graph from OKF bundles using temporary symlinks and build-time metadata validation."
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

High-performance static engine that turns OKF bundles into a fully traversable node graph. Builds in milliseconds using temporary symlinks and build-time metadata validation, fully offline.

## Details & Contracts

| Contract | Guarantee |
| --- | --- |
| Input | Parsed [OKF Bundle](./okf-bundle.md) |
| Output | Fully traversable node graph |
| Edge discovery | Markdown links + frontmatter metadata |
| Build technique | Temporary symlinks + build-time metadata validation |
| Performance | Graph generated in milliseconds |
| Operational mode | Strictly offline, local file system only |
| Persistence | Stateless artifacts; repository data never mutated |

## Related Concepts

- Consumes [OKF Bundle](./okf-bundle.md)
- Feeds [Search Index](./search-index.md)
- Runs inside [Git Repository Mount](./git-repository-mount.md)
- Implements [Weave Vision](./weave-vision.md)
