---
type: specification
title: "Node Graph Engine Specification"
description: "Contract for building a fully traversable, bidirectional node graph from a parsed OKF bundle, powering backlink discovery, pathfinding, and the visualization lens."
tags:
  - "graph"
  - "engine"
  - "traversal"
  - "specification"
generated:
  by: opencode/big-pickle
  at: "2026-08-06T00:00:00Z"
status: draft
---

# Specification: Node Graph Engine

## 1. Overview

Pure, offline, synchronous TypeScript module that turns a validated `ParsedBundle` (from [OKF Bundle Parser](./okf-bundle-parser.md)) into a **bidirectional `NodeGraph`**. Each concept becomes a node; edges are derived from resolved bundle-relative Markdown links plus a conventional frontmatter `related` array. The engine computes backlinks (reverse edges) at build time, making the graph fully traversable in both directions — the "living" structure Weave promises.

The graph is the connective tissue of the vision: it feeds backlink-aware rendering in the explorer, degree-aware ranking in the [Search Index](./search-index.md), and the physics-based [Visualization Lens](../knowledge/concepts/node-graph-engine.md) (Tier 3) via a lossless JSON serialization.

## 2. Module Structure & Execution Context

- **Library Root:** `src/lib/graph/`
- **No hydration / no client JS.** Runs in the Astro server runtime and CLI only. Zero browser bundle.
- **Synchronous and dependency-free.** The engine consumes an already-parsed in-memory bundle, so it builds without I/O and without third-party libraries. Uses native `Map`-based adjacency, aligned with the parser's zero-dependency ethos.
- **Runtime:** Bun-first; plain TypeScript, no Node polyfills required.

**Files:**

- `src/lib/graph/types.ts` — graph interfaces and enums.
- `src/lib/graph/build.ts` — `buildGraph()`: node/edge assembly + backlink inversion.
- `src/lib/graph/traverse.ts` — traversal and topology queries (backlinks, paths, components).

## 3. Component Contracts

### 3.1 GraphNode

```typescript
export interface GraphNode {
  /** Concept ID = bundle-relative path minus .md */
  id: string;
  /** Bundle-relative path, e.g. "concepts/okf-bundle.md" */
  path: string;
  /** Concept type from frontmatter */
  type: string;
  /** Display title; falls back to the concept id */
  title: string;
  /** Optional lifecycle status: draft | stable | deprecated */
  status?: 'draft' | 'stable' | 'deprecated';
  /** Optional cross-cutting tags */
  tags?: string[];
  /** Number of incoming edges (backlinks) */
  inDegree: number;
  /** Number of outgoing edges */
  outDegree: number;
}
```

### 3.2 GraphEdge

```typescript
export interface GraphEdge {
  /** Source node id */
  from: string;
  /** Target node id — always an in-bundle concept */
  to: string;
  /** How the edge was discovered */
  source: 'link' | 'related';
  /** Raw markdown link text when source is 'link' */
  raw?: string;
}
```

Edge sources:

- `link` — a Markdown link in the source concept body whose target resolves to an in-bundle concept (`OkfLink.resolvesInBundle === true`).
- `related` — an entry in the source concept's frontmatter `related` array (bundle-relative path with or without leading `/` and `.md` suffix, or a concept id). Weave convention; documented as an extension point for future OKF relation keys.

Broken or external targets (`resolvesInBundle === false`) are **not** graph edges; they are preserved as `dangling` metadata so the explorer can still surface them.

### 3.3 NodeGraph

```typescript
export interface NodeGraph {
  /** All nodes keyed by concept id */
  nodes: Map<string, GraphNode>;
  /** Outgoing adjacency: node id -> edges */
  outgoing: Map<string, GraphEdge[]>;
  /** Incoming adjacency (backlinks): node id -> edges */
  incoming: Map<string, GraphEdge[]>;
  /** Non-resolving targets per node: broken links + external URLs */
  dangling: Map<string, OkfLink[]>;
}

export interface SerializedGraph {
  nodes: Array<{
    id: string;
    title: string;
    type: string;
    status?: string;
    tags?: string[];
    inDegree: number;
    outDegree: number;
  }>;
  edges: Array<{ from: string; to: string; source: 'link' | 'related' }>;
}
```

### 3.4 Build API

```typescript
export function buildGraph(bundle: ParsedBundle): NodeGraph;
export function serializeGraph(graph: NodeGraph): SerializedGraph;
export function nodeCount(graph: NodeGraph): number;
export function edgeCount(graph: NodeGraph): number;
```

### 3.5 Traversal API

```typescript
export function getNode(graph: NodeGraph, id: string): GraphNode | undefined;
export function outgoing(graph: NodeGraph, id: string): GraphEdge[];
export function incoming(graph: NodeGraph, id: string): GraphEdge[];
export function neighbors(graph: NodeGraph, id: string): string[];
export function findPath(graph: NodeGraph, from: string, to: string): string[] | null;
export function connectedComponents(graph: NodeGraph): string[][];
export function orphanNodes(graph: NodeGraph): string[];
```

## 4. Data Flow & Dependencies

- Input: `ParsedBundle` from [OKF Bundle Parser](./okf-bundle-parser.md).
- Per concept, extract edges:
  1. From `concept.links` where `resolvesInBundle === true` → `source: 'link'`.
  2. From the conventional frontmatter `related` array (if present) → `source: 'related'`.
- Non-resolving links and external URLs → appended to `dangling[id]`, never to `outgoing`/`incoming`.
- Deduplicate edges: a single `from → to` pair appears at most once per `source`; `raw` keeps the first occurrence.
- Build `incoming` by inverting `outgoing` (backlinks), so `incoming[id]` answers "what references this concept?".
- Compute `inDegree`/`outDegree` per node from the adjacency maps.
- `serializeGraph` emits a compact JSON graph (id/title/type only, no bodies) for the Tier 3 lens and the explorer's embed path.
- Explorer consumption: render backlinks per concept and graph stats (node/edge counts, orphan list) using `data-testid` hooks for behavior tests.
- Implements contract from [Node Graph Engine](../knowledge/concepts/node-graph-engine.md); feeds [Search Index](./search-index.md) and the Visualization Lens (Tier 3).

## 5. Performance, Edge Cases & Errors

- **Performance:** graph build adds < 20 ms to a 10k-node bundle on a local SSD; `findPath` and `neighbors` are linear in edge count; `connectedComponents` is O(V + E). Stays inside the vision's "milliseconds" budget combined with the parser and index build.
- **Empty bundle:** empty `NodeGraph`, zero nodes/edges, no error.
- **Broken/external links:** never crash; surfaced via `dangling` per node.
- **Orphan nodes:** concepts with no edges are present as nodes; `orphanNodes()` lists them. "Living universe" rule: every parsed concept becomes a node, even disconnected ones.
- **Duplicate edges:** deduplicated per `from → to` + `source`.
- **Self-links and cycles:** self-edges are allowed and counted; traversal (`findPath`, components) MUST guard against cycles with a visited-set to avoid infinite loops.
- **Missing frontmatter / invalid concepts:** excluded by the parser; never present in the graph.
- **`related` entries that do not resolve:** treated as dangling, warned in the bundle validation, never an edge.
- **Determinism:** node iteration order follows the bundle's sorted concept order; graph output is reproducible across builds.

## Related Concepts

- Depends on [OKF Bundle Parser](./okf-bundle-parser.md)
- Depends on [OKF Bundle](../knowledge/concepts/okf-bundle.md)
- Feeds [Search Index](./search-index.md)
- Feeds [Visualization Lens](../knowledge/concepts/node-graph-engine.md) (Tier 3)
- Runs inside [Git Repository Mount](../knowledge/concepts/git-repository-mount.md)
- Implements [Node Graph Engine](../knowledge/concepts/node-graph-engine.md)
