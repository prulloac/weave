---
type: Concept
title: "Weave Vision"
description: "Core vision of Weave: a portable, CLI-driven visualization engine that unlocks latent structure within OKF bundles by weaving flat files into a living universe of knowledge."
tags:
  - "vision"
  - "architecture"
  - "cli"
  - "okf"
  - "knowledge-management"
  - "local-first"
generated:
  by: opencode/big-pickle
  at: "2026-08-06T00:00:00Z"
verified:
  - by: "human:developer"
    at: "2026-08-06T00:00:00Z"
status: stable
---

# Weave Vision

> "To weave flat files into a living universe of knowledge."

## Summary

Weave is a portable, CLI-driven visualization engine designed to unlock the latent structure within Open Knowledge Format (OKF) bundles. It bridges the gap between static, portable file formats and interactive data exploration, letting individuals parse, query, and map deeply interconnected Markdown and YAML structures directly from their terminal.

## Details & Contracts

### The Purpose

Weave bridges static file formats and interactive exploration. Core contract:

- Input: OKF bundles composed of Markdown and YAML flat files.
- Operation: parse, query, and map deeply interconnected structures.
- Surface: CLI-driven, portable, no GUI required.
- Output: node graph and search index exposing latent structure.

### The Architecture

Weave acts as a dynamic visual lens that mounts over any local Git repository without polluting underlying data.

| Contract | Guarantee |
| --- | --- |
| Mount strategy | Single command wraps target directory with high-performance static engine |
| Pollution-free | No mutation of underlying repository data |
| Graph construction | Temporary symlinks + build-time metadata validation |
| Performance | Fully traversable node graph + search index generated in milliseconds |
| Operational mode | Strictly offline, local file system only |

### The User Experience

Engineered for deep focus, reclaiming knowledge exploration from sluggish web interfaces.

| Contract | Guarantee |
| --- | --- |
| Environment | Local-first, low-latency |
| Input flow | Write Markdown locally, run Weave command |
| Interaction | Frictionless, keyboard-only, never leaves terminal |
| Output | Macro-level, physics-based map of personal knowledge ecosystem |

## Related Concepts

- Depends on [OKF Specification](./okf-bundle.md)
- Related to [Node Graph Engine](./node-graph-engine.md)
- Related to [Search Index](./search-index.md)
- Depends on [Git Repository Mount](./git-repository-mount.md)
