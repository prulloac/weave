---
type: Concept
title: "OKF Bundle"
description: "Portable collection of OKF-compliant Markdown and YAML flat files that serves as the input format for Weave."
tags:
  - "format"
  - "okf"
  - "markdown"
  - "yaml"
generated:
  by: opencode/big-pickle
  at: "2026-08-06T00:00:00Z"
verified:
  - by: "human:developer"
    at: "2026-08-06T00:00:00Z"
status: stable
---

# OKF Bundle

## Summary

A portable bundle of OKF-compliant Markdown and YAML flat files. Weave's primary input: the latent structure inside these bundles is what Weave exposes as a traversable graph.

## Details & Contracts

| Contract | Guarantee |
| --- | --- |
| Composition | Markdown (`.md`) and YAML flat files |
| Compliance | Documents follow Open Knowledge Format v0.2 |
| Graph edges | Expressed via bundle-relative Markdown links (e.g. `[User Table](/tables/users.md)`) |
| Portability | Static, versionable, transferable without a server |
| Requirement | OKF: non-empty `type`. Weave profile: additionally a non-empty `keywords` list. Concept id is the file path minus `.md` |

## Related Concepts

- Feeds [Node Graph Engine](./node-graph-engine.md)
- Consumed by [Search Index](./search-index.md)
- Loaded by the Go CLI (the mount concept is retired)
- Enables [Weave Vision](./weave-vision.md)
