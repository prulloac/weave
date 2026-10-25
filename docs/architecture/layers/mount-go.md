---
type: Layer Specification
title: "Mount Layer (Go)"
description: "Go http.Handler, artifact layer, route dispatch, asset serving, registry and embedding recipes for the Weave mount."
tags:
  - "architecture"
  - "mount"
  - "go"
  - "http"
  - "embedding"
generated:
  by: opencode/deepseek-v4.1-flash
  at: "2026-10-25T00:00:00Z"
status: draft
---

# Mount Layer (Go)

## Purpose

The mount layer turns an engine instance and a renderer into a local knowledge site and, when
embedded, into an `http.Handler` that a host Go application can mount anywhere.

It owns:

- the artifact layer that exposes a target repository without writing into it,
- one-time parse and site construction,
- route dispatch and asset serving,
- lifecycle hooks (`Close`, shutdown endpoint),
- the optional mount registry used by the standalone CLI.

It does not own argument parsing, process exit codes, or signal handling. Those belong to the
[CLI layer](cli-go.md).

## Package Layout

```text
mount/
├── site.go              # NewSite, Handler, Close
├── routes.go            # route table and path decoding
├── assets.go            # embedded/on-disk asset serving
├── artifacts/
│   ├── artifact.go      # symlink/hardlink materialization
│   └── cleanup.go
├── registry/
│   ├── registry.go      # read/write/add/remove/prune
│   └── status.go        # human-readable status output
├── lensassets/
│   ├── embed.go         # //go:embed of built lens assets
│   └── dist/            # populated by the lens build
└── shutdown.go          # optional standalone shutdown endpoint
```

## Public API

```go
package mount

type Options struct {
    // BasePath defaults to "/" and must start and end with "/".
    BasePath string
    // Theme overrides the default renderer theme.
    Theme render.Theme
    // Assets overrides the lens asset manifest.
    Assets render.AssetManifest
    // EnableShutdown exposes POST /__weave/shutdown. Default false for embedded sites.
    EnableShutdown bool
    // EngineOptions are forwarded to engine.Open.
    EngineOptions engine.Options
}

type Site struct {
    // unexported
}

func NewSite(ctx context.Context, target string, opts Options) (*Site, error)
func (s *Site) Handler() http.Handler
func (s *Site) Close() error

func (s *Site) Bundle() *contract.Bundle
func (s *Site) Graph() *graph.Graph
func (s *Site) Root() string
```

Rules:

- `NewSite` creates the artifact layer, opens the engine once, builds the renderer and wires the
  handler. It does not bind a port, install signal handlers, or touch the registry.
- `Handler` is safe for concurrent requests.
- `Close` removes the artifact directory and releases resources. It is idempotent.
- Embedded hosts may ignore the registry and shutdown endpoint entirely.
- `BasePath` lets a host mount the site under `/weave/` without rewriting templates.

## Route Table

| Method | Path | Response | Notes |
| --- | --- | --- | --- |
| GET | `/` | HTML 200 | Styled explorer, zero JS |
| GET | `/lens` | HTML 200 | Lens shell, index, payload, optional script |
| GET | `/concept/{id}` | HTML 200 | Concept page |
| GET | `/concept/{id}` | HTML 404 | Unknown concept, with suggestions |
| GET | `/assets/weave.css` | CSS 200 | Theme stylesheet |
| GET | `/assets/lens.js` | JS 200 | Lens bundle, if built |
| POST | `/__weave/shutdown` | text 200 | Only when `EnableShutdown` |
| GET | `/api/v1/*` | JSON | Optional; see [Integration contracts](../integration/contracts.md) |
| * | anything else | HTML 404 | Styled not-found page |

Path handling:

- Decode `{id}` exactly once, after splitting the route prefix.
- Reject ids containing `..` after cleaning.
- Never map request paths directly to filesystem paths.
- Serve only allowlisted assets; no directory listing.
- Set `X-Weave-Contract: 1` on HTML and JSON responses.

## Artifact Layer

The artifact layer gives the engine a stable directory tree without copying file contents or
mutating the target.

1. Walk the target tree, skipping `.git` and `node_modules`.
2. Mirror directories.
3. Materialize files as symlinks.
4. On platforms where symlinks fail (for example Windows without Developer Mode), fall back to
   hard links.
5. Remove the artifact root on `Close`.

Guarantees:

- The target tree is never written to.
- The engine sees regular file contents and stable relative paths.
- The artifact root is unique per site instance and safe to delete recursively.

Tests must verify the no-write guarantee and the hardlink fallback.

## Site Construction Flow

```text
NewSite(target)
  → artifacts.Materialize(target)       temp root
  → engine.Open(artifactRoot)           parse, graph, search
  → render.New(theme, assets, basePath)
  → Site{handler}
```

- Parsing happens once. Requests only read immutable structures.
- If site construction fails after the artifact layer is created, the artifact root is removed.
- The engine cache is disabled for mounts by default because the artifact root is fresh each
  time.

## Asset Serving

- The lens asset is embedded at compile time and served from memory, not from the artifact tree.
- The theme stylesheet comes from the renderer and is embedded as well.
- Use `Content-Length`, `Content-Type`, `ETag` and `Cache-Control: public, max-age=31536000,
  immutable` when the URL contains a content hash; otherwise use `no-cache`.
- Missing lens assets are not a fatal error: `/lens` degrades to the server-rendered index.

## Registry (Standalone Only)

The registry exists so `weave status` and `weave unmount` can find live mounts.

```json
[
  {
    "port": 4318,
    "target": "/home/user/notes",
    "started_at": "2026-10-25T00:00:00Z",
    "url": "http://localhost:4318",
    "artifact_dir": "/tmp/weave-mount-123"
  }
]
```

Rules:

- Default location: `os.TempDir()/weave/registry.json`.
- Writes are atomic: temp file plus rename.
- `add` replaces an entry with the same port.
- `remove` ignores unknown ports.
- `prune` probes each URL with a short timeout and removes entries whose artifact directory is
  missing or whose server is dead.
- Registry functions accept a path or interface so tests and embedded hosts can avoid global state.
- Embedded sites never write the registry.

## Shutdown

- The standalone CLI may enable `POST /__weave/shutdown`.
- The endpoint returns `200 ok`, then stops the server after a short delay.
- It must not be enabled by default for embedded sites.
- The CLI owns SIGINT/SIGTERM handling: it calls `Site.Close()` and exits cleanly.

## Embedding Recipes

### In-process Go handler

```go
site, err := mount.NewSite(ctx, "/srv/notes", mount.Options{})
if err != nil { return err }
defer site.Close()

mux := http.NewServeMux()
mux.Handle("/weave/", http.StripPrefix("/weave", site.Handler()))
log.Fatal(http.ListenAndServe(":8080", mux))
```

Set `BasePath: "/weave/"` so generated links stay under the prefix.

### Standalone binary

`cmd/weave mount <path>` creates a site, binds a port, writes the registry and serves until
shutdown or signal.

### Static export

A future option may render all routes to disk. It is not part of v1; the current site is
request-driven.

## Security

- No writes into the target repository.
- No traversal outside the artifact root through concept ids.
- Escape all rendered data; see [Render layer](render-go.md).
- Bound request body size for the shutdown endpoint; it accepts no payload.
- Do not expose the site publicly without an authenticating proxy; it is a localhost tool by
  default.
- Registry files may reveal local paths; keep them under the user's temp directory.

## Testing

1. `httptest` route tests for every route, including 404 and content types.
2. Artifact tests: symlink path, hardlink fallback, skipped directories, cleanup, no source writes.
3. Handler tests with `BasePath` and embedded mode.
4. Registry tests: atomic write, duplicate port, prune, unknown port.
5. Integration test against the existing fixture bundle asserting the DOM contract.
6. `go test -race ./mount/...`.
7. Playwright E2E against a real `cmd/weave mount` process.

## Parity Map from TypeScript

| TypeScript | Go target |
| --- | --- |
| `cli/mount.ts` | `mount/site.go`, `mount/routes.go` |
| `cli/server.ts` | `mount/site.go` (handler), `cmd/weave` (listener) |
| `cli/artifacts.ts` | `mount/artifacts/*` |
| `cli/status.ts` | `mount/registry/*` |
| `src/lib/*-render.ts` | `render/*` |

## Related Documents

- [Engine layer](engine-go.md)
- [Render layer](render-go.md)
- [CLI layer](cli-go.md)
- [Integration contracts](../integration/contracts.md)
- [Integration testing](../integration/testing.md)
