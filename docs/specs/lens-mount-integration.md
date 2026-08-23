---
type: specification
title: "Lens Mount Integration Specification"
description: "Contract for upgrading the weave mount server from a single static HTML string into a styled, multi-route local site: styled zero-JS explorer, force-directed lens route, and navigable concept pages, with client JS bundled at mount time via Bun.build."
tags:
  - "cli"
  - "bun"
  - "visualization"
  - "astro"
  - "component-spec"
generated:
  by: opencode/big-pickle
  at: "2026-08-23T00:00:00Z"
status: draft
---

# Specification: Lens Mount Integration

## 1. Overview

Today `weave mount` serves one unstyled HTML string (the Tier-1 BDD test harness) for every
request, and clicking any node or link does nothing. The Tier-3 Visualization Lens exists only
on the Astro dev site (`/lens`) and reads a hardcoded test fixture — it never renders a real
mounted bundle.

This feature integrates the lens into the mount server and turns every mount into an
Obsidian-like local knowledge site:

- **`/`** — styled bundle overview (index, validation, concept list). Zero JS.
- **`/lens`** — full-viewport, physics-based force graph of the mounted bundle. Hydrated island.
- **`/concept/<id>`** — rendered concept page with body, links, and backlinks. Zero JS.
- Clicking a node in the lens navigates to its concept page; links inside concepts resolve to
  other concept pages.

All pollution-free and offline guarantees of [Weave CLI Mount](./weave-cli.md) are preserved:
client assets are generated inside the temporary artifact layer and removed on unmount.

## 2. Routes & Component Structure

### Route Table

| Route | Renderer | JS | Purpose |
| --- | --- | --- | --- |
| `/` | `renderExplorer(bundle)` (styled shell wrapper) | none | Bundle overview; keeps all existing `data-testid` hooks |
| `/lens` | `renderLensShell(graph)` | `lens.js` | Force-directed map, full viewport |
| `/concept/<id>` | `renderConceptPage(bundle, graph, id)` | none | Concept body, metadata, links, backlinks |
| `/assets/lens.js` | static artifact | — | Bundled Preact entry (generated at mount time) |
| `/assets/weave.css` | static artifact | — | Shared stylesheet for all routes |

### Visual Baseline

"Zero JS" in this specification is a **script budget, not a style budget**: every route —
including the zero-JS ones — links `/assets/weave.css` and renders as a coherent, dark,
Obsidian-inspired interface. The minimum visual contract:

- CSS custom properties for theme tokens (background, surface, text, accent), applied globally.
- System font stack; readable typography and spacing for rendered Markdown bodies.
- Styled navigation header shared by all routes (bundle title, link to `/lens`, link to `/`).
- No default browser styling left on list/table/heading elements.

A dedicated design pass (design system extraction, light/dark toggle, typography scale) is
tracked separately in the [backlog](../backlog.md) ("Visual Design Pass") and is out of scope
here; this feature must already look intentional, not raw.

### Module Structure

```text
cli/
├── server.ts            # route-dispatching fetch handler + static asset serving
└── client-bundle.ts     # Bun.build() packaging of the lens entrypoint into the artifact dir
src/
├── entrypoints/
│   └── lens-client.tsx  # standalone Preact entry: mounts ForceGraph + Legend onto the shell
└── lib/
    ├── explorer-render.ts   # unchanged output contract; wrapped in styled shell
    ├── concept-render.ts    # NEW: SSR HTML for /concept/<id>
    └── lens-shell.ts        # NEW: HTML shell for /lens with inline JSON payload
```

### Client Entry

```text
lens-client.tsx (bundled by Bun.build at mount time)
  └── render(<ForceGraph /> + <Legend />, #weave-lens-root)
        ├── <canvas> (existing ForceGraph renderer, unchanged)
        └── node click → location.href = /concept/<id>
```

## 3. Component Contracts

### 3.1 Server Handler

`startServer` changes from a single-string renderer to a request handler:

```typescript
export async function startServer(
  handle: (request: Request) => Response | Promise<Response>,
  port?: number,
): Promise<MountServer>
```

The shutdown endpoint (`/__weave/shutdown`) keeps its existing behavior and precedence.

### 3.2 Concept Page View-Model

```typescript
export interface ConceptPage {
  id: string;
  title: string;
  type: string;
  status?: string;
  tags?: string[];
  description?: string;
  bodyHtml: string;          // markdown body → sanitized HTML
  links: Array<{ raw: string; target: string; href: string | null }>; // href null = dangling
  backlinks: Array<{ id: string; title: string }>;
}
```

### 3.3 Lens Data Payload

The `/lens` shell embeds the [SerializedGraph](./node-graph-engine.md) inline so the client
needs no fetches:

```html
<script type="application/json" id="weave-graph">{ ...serialized graph... }</script>
<div id="weave-lens-root"></div>
<script type="module" src="/assets/lens.js"></script>
```

The graph is produced by the Node Graph Engine's `buildGraph()` + `serializeGraph()` — the
hand-rolled construction in `src/pages/lens.astro` is considered superseded by this contract.

### 3.4 Client Bootstrap Contract

`lens-client.tsx` reads `#weave-graph`, parses it as `SerializedGraph`, computes viewport
dimensions from `#weave-lens-root`, and renders. No global variables beyond that element.

## 4. Data Flow & Dependencies

1. At mount time, `parseBundle(artifactDir)` runs once (as today).
2. `buildGraph(bundle)` + `serializeGraph(...)` produce the payload shared by `/lens`.
3. `Bun.build()` bundles `src/entrypoints/lens-client.tsx` (Preact + ForceGraph + Legend)
   into `<artifactDir>/assets/lens.js`. Target: single-digit-milliseconds build; on failure,
   mount falls back to serving the legacy static explorer only.
4. The server dispatches by pathname; unknown routes return a styled 404.
5. No network calls anywhere; artifacts are cleaned up by the existing unmount lifecycle.

Hydration strategy per invariant #4 (zero-JS by default): `/` and `/concept/<id>` ship no JS;
only `/lens` loads `lens.js`, which is justified interactivity.

## 5. Edge Cases & Errors

- **Unknown concept id** → styled 404 page listing suggestions (index entries), exit-neutral.
- **Empty bundle** → `/lens` shows the existing "No concepts found" empty state.
- **Bun.build failure** → mount still succeeds; `/lens` degrades to the zero-JS explorer view
  with a warning banner.
- **XSS:** serialized-graph JSON must escape `</script>` sequences (`<` → `\u003c`); all
  interpolated strings pass through the existing HTML escapers; markdown bodies are rendered
  with escaping (no raw inline HTML pass-through).
- **URL encoding:** concept ids are kebab-case but paths may contain `/`; ids are
  percent-decoded before lookup and re-encoded when generating hrefs.
- **Large graphs (> 1000 nodes):** existing degraded rendering mode applies unchanged.
- **Port reuse across mounts:** each artifact dir is isolated; assets are namespaced per mount.

## 6. Non-Goals

- No search UI (search remains CLI-only until specced separately).
- No live reload or file watching; remount reflects changes.
- No editing or writes into the target repository.
- No design-system extraction or theming options — deferred to the backlog's "Visual Design
  Pass" feature; this feature delivers the single dark baseline theme only.
- The Astro dev-site pages are out of scope beyond adopting `buildGraph()` if trivial.

## Related Concepts

- Extends [Weave CLI Mount](./weave-cli.md) with a multi-route server
- Integrates [Visualization Lens](./visualization-lens.md) into mounted bundles
- Consumes [Node Graph Engine](./node-graph-engine.md) serialized output
- Implements the "living universe" goal of the Weave Vision
