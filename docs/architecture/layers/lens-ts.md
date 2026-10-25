---
type: Layer Specification
title: "Lens Layer (TypeScript Assets)"
description: "Standalone Preact lens package, DOM bootstrap contract, graph payload, build pipeline and Go embedding rules."
tags:
  - "architecture"
  - "lens"
  - "typescript"
  - "preact"
  - "assets"
generated:
  by: opencode/deepseek-v4.1-flash
  at: "2026-10-25T00:00:00Z"
status: draft
---

# Lens Layer (TypeScript Assets)

## Purpose

The lens is the only browser-executed part of Weave. It renders the force-directed graph and
legend from a serialized graph payload produced by the Go renderer. It contains no engine,
parser, search or filesystem code.

It is built independently into a versioned static asset and embedded into the Go binary.

## Package Layout

```text
lens/
├── package.json
├── tsconfig.json
├── biome.json
├── src/
│   ├── entrypoint.tsx
│   ├── components/
│   │   ├── ForceGraph.tsx
│   │   └── Legend.tsx
│   └── lib/
│       ├── types.ts
│       ├── layout.ts
│       └── dom.ts
├── tests/
│   ├── layout.test.ts
│   └── payload.test.ts
├── playwright/
│   └── lens.spec.ts
└── dist/
    ├── lens.js
    └── manifest.json
```

The package has its own dependency set (`preact` only, plus dev tooling). It must not depend on
the Go module, Astro, Tailwind, or the old `src/lib` tree.

## Responsibility Boundary

| Owns | Does not own |
| --- | --- |
| Canvas force simulation and rendering | OKF parsing |
| Hover, reheat, node click navigation | Graph construction |
| Legend rendering | Search |
| DOM bootstrap from the shell | HTML shell and routes |
| Asset-local colors and layout | Theme stylesheet |
| Empty-state rendering after hydration | Empty-state server rendering |

## DOM Contract

The Go renderer must provide exactly these hooks:

| Hook | Requirement |
| --- | --- |
| `#weave-lens-root` | Container element; lens mounts here |
| `#weave-graph` | `<script type="application/json">` with a `SerializedGraph` payload |
| `data-testid="lens-index"` | Server-rendered list of concept links |
| `data-testid="lens-index-entry"` | Anchor with `data-id` and `href="/concept/<id>"` |
| `data-testid="lens-canvas"` | Canvas container (may be the root) |
| `data-testid="empty-message"` | Server-rendered when the graph has no nodes |

The lens must:

- Render `data-testid="force-graph-canvas"` for a non-empty graph.
- Render `data-testid="legend"` alongside the graph.
- Render `data-testid="empty-message"` if invoked with an empty graph.
- Navigate to `/concept/<id>` on node click, using segment-wise URL encoding.
- Read colors from CSS custom properties when available, with constants as fallback.

The lens must not assume the presence of the index or navigation chrome; those belong to the
renderer.

## Graph Payload

The payload is `contract/v1` `SerializedGraph`:

```json
{
  "nodes": [
    {
      "id": "concepts/okf-bundle",
      "title": "OKF Bundle",
      "type": "Concept",
      "status": "stable",
      "tags": ["format", "okf"],
      "in_degree": 1,
      "out_degree": 2
    }
  ],
  "edges": [
    {"from": "concepts/okf-bundle", "to": "tables/users", "source": "link"}
  ]
}
```

Rules:

- Nodes and edges are sorted deterministically by the Go renderer.
- The lens ignores unknown fields.
- Missing optional fields (`status`, `tags`) are tolerated.
- The lens does not fetch, paginate or mutate the payload.
- Large graphs: above 1000 nodes, switch to the degraded rendering mode (edges thicker, node
  labels suppressed) as today.

## Bootstrap

```tsx
const root = document.getElementById('weave-lens-root')
const payload = document.getElementById('weave-graph')

if (root && payload) {
  const graph = JSON.parse(payload.textContent ?? '{}') as SerializedGraph
  const types = [...new Set(graph.nodes.map((node) => node.type))].sort()
  render(
    <>
      <ForceGraph graph={graph} width={root.clientWidth || 960} height={root.clientHeight || 600} />
      <Legend types={types} />
    </>,
    root,
  )
}
```

Rules:

- No global variables other than the two DOM elements.
- No network requests.
- Errors during bootstrap render a plain text message inside the root; they must not throw.
- The entrypoint is idempotent: loading it twice into the same root must not duplicate the canvas.

## Build Pipeline

```sh
cd lens
bun install --frozen-lockfile
bun build src/entrypoint.tsx \
  --outfile dist/lens.js \
  --target browser \
  --format esm \
  --minify
```

Requirements:

- Output is a single ESM file with no external imports.
- No Node built-ins, no dynamic `import()` of server modules, no source maps in production.
- Deterministic output for a given lockfile and source revision.
- A content hash is recorded in `dist/manifest.json`:

```json
{
  "name": "lens",
  "version": "1.0.0",
  "contract": "weave.contract/v1",
  "file": "lens.js",
  "sha256": "…",
  "bytes": 18234
}
```

- The bundle size budget is 150 KB uncompressed, enforced by CI.
- The lens version and contract version are checked at build time; a lens that requires a newer
  contract must fail the build rather than silently misrender.

## Go Embedding

The Go binary embeds the built assets:

```go
//go:embed dist/lens.js dist/manifest.json
var lensAssets embed.FS
```

Rules:

- The mount serves `/assets/lens.js` with `text/javascript; charset=utf-8`.
- Use the content hash in the URL or an `ETag` for cache busting; never serve stale assets after
  a binary upgrade.
- The lens asset is optional: if it is absent, `/lens` still renders the server-side index and a
  warning, and no script tag is emitted.
- The renderer receives only the asset URL and whether it was built; it never reads the lens
  source tree.

## Compatibility

| Lens version | Contract | Notes |
| --- | --- | --- |
| 1.x | `weave.contract/v1` | Current target |
| 2.x | new major | Only if graph payload changes incompatibly |

- Additive contract fields are safe; the lens ignores them.
- Removing or renaming a field requires a lens major and a new contract major.
- The Go renderer may support multiple lens versions during migration by serving versioned assets.

## Testing

1. **Unit**: layout math, node hit-testing, empty graph, degraded threshold.
2. **Payload**: parse a golden `SerializedGraph` fixture; ignore unknown fields.
3. **DOM**: jsdom or happy-dom for bootstrap idempotence and error handling.
4. **E2E**: Playwright against the Go mount:
   - canvas visible,
   - legend visible,
   - lens index lists every concept,
   - clicking an index entry reaches the concept page,
   - empty bundle shows the empty state,
   - node click navigates to `/concept/<id>`.
5. **Boundary**: a lint rule or test ensures no imports from `engine/`, `src/lib/okf`, `src/lib/graph`,
   `src/lib/search`, or Node built-ins.
6. **Size**: bundle size budget.
7. **Cross-version**: contract fixture with extra unknown fields renders successfully.

## Migration from the Current Tree

| Current | Lens package |
| --- | --- |
| `src/components/lens/ForceGraph.tsx` | `lens/src/components/ForceGraph.tsx` |
| `src/components/lens/Legend.tsx` | `lens/src/components/Legend.tsx` |
| `src/entrypoints/lens-client.tsx` | `lens/src/entrypoint.tsx` |
| `src/lib/lens/layout.ts` | `lens/src/lib/layout.ts` |
| `src/lib/lens/types.ts` | `lens/src/lib/types.ts` (contract-generated) |

The old files are removed only after the Go mount serves the new asset and the E2E suite passes.

## Related Documents

- [Contract layer](contract.md)
- [Render layer](render-go.md)
- [Mount layer](mount-go.md)
- [Integration contracts](../integration/contracts.md)
- [Integration testing](../integration/testing.md)
