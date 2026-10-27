---
type: specification
title: "Weave Relate Specification"
description: "CLI command for exploring the graph neighborhood of a concept by depth and direction."
tags:
  - "specification"
  - "cli"
  - "graph"
  - "relate"
generated:
  by: opencode/deepseek-v4.1-flash
  at: "2026-10-26T00:00:00Z"
status: stable
---

# Specification: Weave Relate

Status: **Implemented** for v1.0.

## 1. Overview

`weave relate` answers “what is connected to this concept?” It walks the graph from a root concept
and returns the neighborhood as a bounded subgraph.

```sh
weave relate <path> <id|file> [--depth <n>] [--direction out|in|both] [--json] [--no-cache]
```

## 2. Goals and Non-Goals

### Goals

- Show direct neighbors and multi-hop context.
- Distinguish inbound, outbound and related edges.
- Make the graph usable without a UI.
- Return a deterministic, machine-readable subgraph.

### Non-Goals

- Ranking by relevance.
- Graph visualization.
- Whole-bundle mutation.
- Pathfinding between arbitrary nodes (use `weave path`).

## 3. Command Contract

```text
weave relate <path> <id|file> [--depth <n>] [--direction out|in|both] [--json] [--no-cache]
```

- `--depth` defaults to `1`; valid range `0..5`.
- `--direction` defaults to `both`.
- `--json` emits one `weave.cli/v1` envelope.
- Exit codes:
  - `0` — root found and neighborhood returned.
  - `1` — root concept not found.
  - `2` — usage/invalid root/invalid depth/direction.
  - `3` — permission denied.

## 4. Result Model

```json
{
  "root": "concepts/okf-bundle",
  "direction": "both",
  "depth": 1,
  "nodes": [
    {
      "id": "concepts/node-graph-engine",
      "title": "Node Graph Engine",
      "type": "Concept",
      "keywords": ["graph", "engine"],
      "in_degree": 1,
      "out_degree": 1
    }
  ],
  "edges": [
    {
      "from": "concepts/okf-bundle",
      "to": "concepts/node-graph-engine",
      "source": "link",
      "raw": "[Node Graph Engine](node-graph-engine.md)",
      "depth": 1
    }
  ]
}
```

### Fields

- `nodes` includes the root and every reached node; sorted by id.
- `edges` includes traversed edges; sorted by `from`, `to`, `source`, then depth.
- `depth` on an edge is the hop distance from the root.
- `source` is `link` or `related`.
- `raw` is present for link edges.
- `direction` records the requested traversal direction.

## 5. Traversal Semantics

- `out`: follow `outgoing` edges.
- `in`: follow `incoming` edges.
- `both`: follow both, deduplicating nodes and edges.
- Depth `0` returns only the root node and no edges.
- BFS with a visited-node set prevents cycles and duplicate expansion.
- The root itself is always included when found.
- Dangling/external links are not graph edges and are not included; use `weave show` or `weave audit`
  for those.

## 6. Human Output

```text
concepts/okf-bundle  (1 hop, both)
  outbound
    link → concepts/node-graph-engine
    link → tables/users
  inbound
    concepts/search-index
    tables/users
```

At depth greater than 1, group by hop:

```text
hop 1
  ...
hop 2
  ...
```

## 7. Determinism and Limits

- Nodes sort by id; edges by source id, target id, source, then depth.
- Depth is capped at 5 to bound output size.
- A neighborhood with no edges returns the root node and an empty edge list.

## 8. Testing

- Depth 0/1/2.
- `out`, `in` and `both`.
- Cycles and self-links.
- Missing root.
- Invalid depth/direction.
- Deterministic ordering.
- CLI envelope and human output.

## 9. Acceptance Criteria

- `relate` never exceeds the requested depth.
- No node or edge is duplicated.
- Direction semantics are correct and tested.
- Output is stable and schema-validated.
- `--json` returns a success envelope only when the root exists.

## Related Documents

- [Node Graph Engine spec](./node-graph-engine.md)
- [Weave Explain spec](./weave-explain.md)
- [CLI layer](../architecture/layers/cli-go.md)
- [Integration contracts](../architecture/integration/contracts.md)
- [ROADMAP](../../ROADMAP.md)
