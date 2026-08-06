---
type: Concept
title: "Search Index"
description: "Offline full-text index built alongside the node graph, enabling keyboard-driven querying of OKF bundle content."
tags:
  - "architecture"
  - "search"
  - "index"
  - "offline"
generated:
  by: opencode/big-pickle
  at: "2026-08-06T00:00:00Z"
verified:
  - by: "human:developer"
    at: "2026-08-06T00:00:00Z"
status: stable
---

# Search Index

## Summary

Offline full-text index generated alongside the node graph. Powers instant, keyboard-only querying of Markdown and YAML content, keeping the exploration flow inside the terminal.

## Details & Contracts

| Contract | Guarantee |
| --- | --- |
| Input | Parsed [OKF Bundle](./okf-bundle.md) + node graph |
| Output | Queryable full-text index |
| Latency | Low-latency, local-first |
| Operational mode | Strictly offline |
| UX | Keyboard-driven queries, no browser needed |

## Related Concepts

- Indexes [OKF Bundle](./okf-bundle.md)
- Built alongside [Node Graph Engine](./node-graph-engine.md)
- Runs inside [Git Repository Mount](./git-repository-mount.md)
- Implements [Weave Vision](./weave-vision.md)
