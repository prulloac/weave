---
type: specification
title: "Visualization Lens Specification"
description: "Contract for a physics-based, interactive node graph visualization that renders an OKF bundle as an explorable force-directed map in the browser."
tags:
  - "visualization"
  - "physics"
  - "graph"
  - "astro"
  - "component-spec"
generated:
  by: opencode/big-pickle
  at: "2026-08-17T00:00:00Z"
status: draft
---

# Specification: Visualization Lens

## 1. Overview

An Astro page that renders a [SerializedGraph](./node-graph-engine.md) as an interactive, physics-based force-directed graph in the browser. Nodes represent concepts; edges represent Markdown links and `related` metadata. The lens is the "macro-level map" of the [Weave Vision](../knowledge/concepts/weave-vision.md) — a living, explorable universe of knowledge rendered as an interactive topology.

The visualization hydrates only when scrolled into view (`client:visible`) and is strictly optional — all other Weave features remain zero-JS.

## 2. Route & Component Structure

- **Route:** `src/pages/lens.astro`
- **Island:** `src/components/lens/ForceGraph.tsx` → `client:visible`
- **Server helper:** `src/lib/lens/layout.ts` — physics layout seed computation (runs at build time, passed as prop)

### Component Tree

```text
lens.astro (SSR)
  └── ForceGraph.tsx (client:visible)
        ├── <canvas> (WebGL/SVG renderer)
        └── <Legend /> (static, no hydration)
```

## 3. Component Contracts

### 3.1 ForceGraphProps

```typescript
export interface ForceGraphProps {
  /** Serialized node/edge payload from buildGraph().serializeGraph() */
  graph: SerializedGraph;
  /** Viewport dimensions for initial layout */
  width: number;
  height: number;
}
```

### 3.2 SerializedGraph (input)

Consumes the `SerializedGraph` contract from [Node Graph Engine](./node-graph-engine.md):

```typescript
interface SerializedGraph {
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

### 3.3 Internal State

```typescript
interface NodePosition {
  id: string;
  x: number;
  y: number;
  vx: number;
  vy: number;
}
```

## 4. Physics Simulation

- **Force model:** velocity Verlet integration with three forces:
  - **Coulomb repulsion** between all node pairs (prevents overlap).
  - **Hookean spring attraction** along edges (pulls connected nodes together).
  - **Centering gravity** toward viewport center (prevents drift).
- **Seed layout:** circular layout seeded at build time by `layout.ts` to avoid a blank initial frame.
- **Tick budget:** simulation runs for a fixed 300 ticks, then freezes. Users can trigger reheat via a button.
- **Interaction:** nodes are draggable; dragging applies a strong external force to the dragged node and reheats the simulation for 50 ticks.

## 5. Rendering

- **Renderer:** HTML Canvas 2D (no WebGL dependency) for broad compatibility.
- **Node rendering:** circles, sized by `inDegree + outDegree` (logarithmic scale), colored by `type` using a fixed palette.
- **Edge rendering:** directed lines with arrowheads; `related` edges dashed, `link` edges solid.
- **Labels:** displayed for nodes above a degree threshold or on hover.
- **Legend:** static legend component mapping type colors and edge styles.

## 6. Data Flow

1. `lens.astro` fetches the `SerializedGraph` from the [Node Graph Engine](./node-graph-engine.md) at build time via Astro frontmatter.
2. Passes the serialized payload + viewport dimensions as props to `ForceGraph.tsx`.
3. The island hydrates on visibility, runs the force simulation on a Web Worker (if available) or main thread, and renders to canvas.
4. No network calls; all data is static and bundled at build time.

## 7. Edge Cases & Errors

- **Empty graph:** renders a centered "No concepts found" message, no canvas.
- **Single node:** renders the node with no edges, centered.
- **Large graphs (> 1000 nodes):** degrade to simplified rendering (no labels, thicker edges) to maintain 60 fps.
- **Web Worker unavailable:** simulation runs on main thread with a warning; still functional.
- **Viewport resize:** canvas rescales, simulation re-centers without reheat.
- **Browser without Canvas support:** fallback to an SVG renderer (progressive enhancement).

## Related Concepts

- Consumes [Node Graph Engine](./node-graph-engine.md) serialized output
- Implements [Weave Vision](../knowledge/concepts/weave-vision.md) "physics-based map"
- Runs inside [Git Repository Mount](../knowledge/concepts/git-repository-mount.md) via [Weave CLI Mount](./weave-cli.md)
