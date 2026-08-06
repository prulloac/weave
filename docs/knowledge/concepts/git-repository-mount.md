---
type: Concept
title: "Git Repository Mount"
description: "Strategy where Weave mounts over any local Git repository via a single command, wrapping it with the static engine without polluting underlying data."
tags:
  - "architecture"
  - "git"
  - "mount"
  - "local-first"
generated:
  by: opencode/big-pickle
  at: "2026-08-06T00:00:00Z"
verified:
  - by: "human:developer"
    at: "2026-08-06T00:00:00Z"
status: stable
---

# Git Repository Mount

## Summary

Weave mounts gracefully over any local Git repository by executing a single command. It wraps the target directory with the static engine while leaving repository data untouched.

## Details & Contracts

| Contract | Guarantee |
| --- | --- |
| Trigger | Single CLI command |
| Scope | Any local Git repository |
| Mechanism | Temporary symlinks wrap target directory |
| Pollution | None; repository data never mutated |
| Build artifacts | Ephemeral; removable without residue |
| Operational mode | Strictly offline, local file system only |

## Related Concepts

- Hosts [OKF Bundle](/concepts/okf-bundle.md)
- Hosts [Node Graph Engine](/concepts/node-graph-engine.md)
- Hosts [Search Index](/concepts/search-index.md)
- Enables [Weave Vision](/concepts/weave-vision.md)
