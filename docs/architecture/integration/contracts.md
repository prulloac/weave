---
type: Integration Contract
title: "Integration Contracts"
description: "Go package, HTTP, DOM, asset and filesystem contracts between the Weave layers and host projects."
tags:
  - "architecture"
  - "integration"
  - "contract"
  - "http"
  - "api"
generated:
  by: opencode/deepseek-v4.1-flash
  at: "2026-10-25T00:00:00Z"
status: draft
---

# Integration Contracts

This document defines every boundary a host project or another layer may depend on. Anything not
listed here is internal and may change.

There are five contracts:

1. Domain JSON — [Contract layer](../layers/contract.md).
2. Go package API.
3. HTTP API.
4. DOM and asset contract.
5. Filesystem and input contract.

## Versioning

- Domain contract: `weave.contract/v1`.
- Go module: semantic versioning; v1 API is stable within a major.
- HTTP: `/api/v1/*`; HTML routes are unversioned but satisfy the DOM contract.
- Responses carry `X-Weave-Contract: 1`.
- Additive changes are allowed in v1. Removals, renames and semantic changes require a new major.
- Consumers ignore unknown JSON fields and unknown HTML data attributes.

## 1. Go Package API

### Engine

```go
eng, err := engine.Open(ctx, root, engine.Options{})
defer eng.Close()

bundle := eng.Bundle()
graph := eng.Graph()
hits := eng.Find("okf", search.Options{Limit: 10})
```

Guarantees:

- `Open` is the only expensive call; queries are read-only and concurrency-safe.
- Returned structures are immutable after `Open`.
- Context cancellation aborts `Open`; query methods do not take a context in v1.
- Errors are typed and do not contain CLI exit codes.
- The engine does not read environment variables or global configuration.

### Mount

```go
site, err := mount.NewSite(ctx, target, mount.Options{
    BasePath: "/weave/",
})
defer site.Close()

http.Handle("/weave/", http.StripPrefix("/weave", site.Handler()))
```

Guarantees:

- `NewSite` does not bind ports, install signals, or write the registry.
- `Handler` is safe for concurrent use.
- `Close` is idempotent and removes temporary artifacts.
- `BasePath` controls generated links and asset URLs.
- The host owns lifecycle, auth, TLS and reverse-proxy concerns.

### Render

```go
r, err := render.New(render.Options{Theme: theme, Assets: assets, BasePath: "/"})
html, err := r.Explorer(bundle)
```

Guarantees:

- Renderers are pure functions of the bundle, graph and options.
- No filesystem access at request time.
- Template and Markdown errors are returned, not panics.

## 2. HTTP API

### HTML Routes

| Method | Path | Success | Failure | Content type |
| --- | --- | --- | --- | --- |
| GET | `/` | 200 | 500 | `text/html; charset=utf-8` |
| GET | `/lens` | 200 | 500 | `text/html; charset=utf-8` |
| GET | `/concept/{id}` | 200 | 404 | `text/html; charset=utf-8` |
| GET | `/assets/weave.css` | 200 | 404 | `text/css; charset=utf-8` |
| GET | `/assets/lens.js` | 200 | 404 | `text/javascript; charset=utf-8` |
| POST | `/__weave/shutdown` | 200 | 404/405 | `text/plain; charset=utf-8` |
| GET | any other | 404 | — | `text/html; charset=utf-8` |

Rules:

- `{id}` is decoded exactly once and must match a concept id.
- Unknown concepts return the styled 404 page, not JSON.
- Every HTML response includes `data-testid="mount-nav"`.
- Asset routes are allowlisted; no directory listing or traversal.
- `POST /__weave/shutdown` is available only when `EnableShutdown` is true.

### Optional JSON API

If enabled, the JSON API is served under `/api/v1`. It is optional for v1 but recommended for
non-Go hosts and debugging.

| Method | Path | Response |
| --- | --- | --- |
| GET | `/api/v1/bundle` | `Bundle` |
| GET | `/api/v1/graph` | `Graph` |
| GET | `/api/v1/graph/serialized` | `SerializedGraph` |
| GET | `/api/v1/concepts/{id}` | `ShowResult` |
| GET | `/api/v1/search?q=&limit=&fields=` | `SearchResult[]` |
| GET | `/api/v1/backlinks/{id}` | `BacklinksResult` |
| GET | `/api/v1/path?from=&to=` | `PathResult` |

Examples:

```http
GET /api/v1/search?q=graph&limit=5
X-Weave-Contract: 1
```

```json
[
  {
    "id": "concepts/node-graph-engine",
    "title": "Node Graph Engine",
    "path": "concepts/node-graph-engine.md",
    "type": "Concept",
    "score": 11.82,
    "best_field": "title"
  }
]
```

Errors:

```json
{
  "error": {
    "code": "CONCEPT_NOT_FOUND",
    "message": "concept not found: does-not-exist",
    "details": {"id": "does-not-exist"}
  }
}
```

Error codes:

| Code | HTTP | Meaning |
| --- | --- | --- |
| `INVALID_REQUEST` | 400 | Missing or malformed query parameter |
| `CONCEPT_NOT_FOUND` | 404 | Unknown concept id |
| `PATH_NOT_FOUND` | 404 | No route between concepts |
| `PARSE_ERROR` | 422 | Bundle cannot be parsed at all |
| `INTERNAL` | 500 | Unexpected failure |

Response headers:

- `X-Weave-Contract: 1`
- `X-Weave-Version: <semver>`
- `Cache-Control: no-store` for JSON unless a host adds caching.

## 3. DOM and Asset Contract

### Shared shell

| Hook | Required | Meaning |
| --- | --- | --- |
| `data-testid="mount-nav"` | yes | Site header |
| `link[rel="stylesheet"][href$="/assets/weave.css"]` | yes | Theme stylesheet |
| `main.weave-main` | yes | Main content region |
| script elements | only `/lens` | No scripts on `/` or `/concept/*` |

### Explorer

| Hook | Meaning |
| --- | --- |
| `data-testid="bundle-meta"` | Bundle summary block |
| `data-testid="bundle-version"` | Declared OKF version |
| `data-testid="bundle-valid"` | `true`/`false` |
| `data-testid="bundle-count"` | Concept count |
| `data-testid="index-entry"` | Root index entry; `data-target` |
| `data-testid="index-description"` | Optional description |
| `data-testid="validation-entry"` | Per-concept validation |
| `data-testid="concept"` | Concept card; `data-id` |
| `data-testid="status"` | Concept status line |
| `data-testid="link"` | Link row; `data-target`, `data-resolved`, `data-resolves` |

### Concept page

| Hook | Meaning |
| --- | --- |
| `data-testid="concept-page"` | Article; `data-id` |
| `data-testid="concept-type"` | Type text only |
| `data-testid="concept-status"` | Status text only |
| `data-testid="concept-body"` | Rendered Markdown |
| `data-testid="concept-link"` | Outbound row; `data-resolved` |
| `data-testid="backlinks"` | Backlinks section |
| `data-testid="backlink"` | Backlink anchor; `data-id` |
| `data-testid="not-found"` | 404 view |

### Lens

| Hook | Meaning |
| --- | --- |
| `#weave-lens-root` | Preact mount point |
| `#weave-graph` | JSON payload script |
| `data-testid="lens-index"` | Server-rendered index container |
| `data-testid="lens-index-entry"` | Anchor with `data-id`, `href` |
| `data-testid="force-graph-canvas"` | Canvas after hydration |
| `data-testid="legend"` | Legend after hydration |
| `data-testid="empty-message"` | Empty state |
| `data-testid="lens-warning"` | Optional asset-build warning |

### Asset manifest

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

Rules:

- The Go binary embeds the manifest and asset together.
- URL cache busting uses the hash (for example `/assets/lens.<hash>.js`).
- The renderer receives URLs, not filesystem paths.
- A missing asset disables `/lens` JS but not the server-rendered index.

## 4. Filesystem and Input Contract

Input is an OKF v0.2 bundle directory.

| Rule | Guarantee |
| --- | --- |
| Input format | Markdown `.md` with YAML frontmatter |
| Required field | Non-empty `type` |
| Reserved names | `index.md`, `log.md` (never concepts) |
| Root version | Only root `index.md` may declare `okf_version: "0.2"` |
| Encoding | UTF-8; invalid files are reported and skipped |
| Paths | POSIX concept ids regardless of host OS |
| Symlinks | Files followed; directories one level with cycle guard |
| Writes | Never write to the target bundle |
| Cache | Optional, outside the target tree |

Host embedding rules:

- Pass an absolute or resolvable target path.
- Do not mutate the target during a site's lifetime; reopen the site to observe changes.
- The host is responsible for file permissions and secret material in the bundle.
- The artifact layer is temporary and must be removed with `Close`.

## Compatibility Matrix

| Component | Engine contract | Lens contract | HTTP API | Notes |
| --- | --- | --- | --- | --- |
| Go engine v1 | v1 | n/a | n/a | Reference for ports |
| Go render v1 | v1 | v1 | HTML | DOM contract |
| Go mount v1 | v1 | v1 | v1 optional | In-process handler |
| Lens v1 | n/a | v1 | n/a | Ignores unknown fields |
| TS reference (frozen) | v1 | v1 | HTML | Used for conformance until cutover |

## Related Documents

- [Contract layer](../layers/contract.md)
- [Engine layer](../layers/engine-go.md)
- [Render layer](../layers/render-go.md)
- [Lens layer](../layers/lens-ts.md)
- [Mount layer](../layers/mount-go.md)
- [Integration steps](steps.md)
- [Integration testing](testing.md)
