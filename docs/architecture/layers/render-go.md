---
type: Layer Specification
title: "Render Layer (Go)"
description: "Go HTML templating, Markdown rendering, theme injection, zero-JS page shells and test-ID contract for the mounted site."
tags:
  - "architecture"
  - "render"
  - "go"
  - "html"
  - "markdown"
generated:
  by: opencode/deepseek-v4.1-flash
  at: "2026-10-25T00:00:00Z"
status: draft
---

# Render Layer (Go)

## Purpose

The renderer turns engine results into the mounted site:

- `/` — styled zero-JS bundle overview.
- `/concept/<id>` — zero-JS concept page with body, links and backlinks.
- `/lens` — HTML shell plus serialized graph payload; the TypeScript lens hydrates it.
- `/404` — styled not-found page with suggestions.

It owns Markdown-to-HTML conversion, escaping, page chrome and theme. It does not walk the
filesystem, build graphs, or serve HTTP.

## Package Layout

```text
render/
├── render.go            # Renderer facade and options
├── explorer.go          # bundle overview view
├── concept.go           # concept page view
├── lens.go              # lens shell and payload
├── not_found.go
├── markdown/
│   ├── markdown.go      # goldmark configuration
│   ├── links.go         # link rewriting to /concept routes
│   └── sanitize.go
├── templates/
│   ├── layout.html.tmpl
│   ├── explorer.html.tmpl
│   ├── concept.html.tmpl
│   ├── lens.html.tmpl
│   └── not_found.html.tmpl
├── assets/
│   └── weave.css
└── theme.go
```

Templates and the default stylesheet are compiled into the binary with `//go:embed`.

## Public API

```go
package render

type AssetManifest struct {
    CSSPath      string // URL path, default /assets/weave.css
    LensJSPath   string // URL path, default /assets/lens.js
    LensJSBuilt  bool
    ContractHash string // optional asset cache-busting token
}

type Options struct {
    BasePath string
    Theme    Theme
    Assets   AssetManifest
}

type Renderer struct { /* unexported */ }

func New(opts Options) (*Renderer, error)

func (r *Renderer) Explorer(b *contract.Bundle) ([]byte, error)
func (r *Renderer) Concept(b *contract.Bundle, g *graph.Graph, id string) ([]byte, bool, error)
func (r *Renderer) LensShell(g *contract.SerializedGraph, opts LensOptions) ([]byte, error)
func (r *Renderer) NotFound(b *contract.Bundle, id string) []byte
func (r *Renderer) Stylesheet() []byte

type LensOptions struct {
    Script  bool
    Warning string
}
```

Error handling: template execution errors are returned; a concept that does not exist returns
`(nil, false, nil)` so the mount can render a 404.

## Page Shell

Every route shares:

```html
<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>{{.Title}}</title>
    <link rel="stylesheet" href="{{.CSSPath}}" />
  </head>
  <body>
    <header data-testid="mount-nav" class="weave-nav">
      <a class="weave-brand" href="{{.HomeHref}}">Weave</a>
      <nav>
        <a href="{{.HomeHref}}">Explorer</a>
        <a href="{{.LensHref}}">Lens</a>
      </nav>
    </header>
    <main class="weave-main">{{.Body}}</main>
    {{.Scripts}}
  </body>
</html>
```

Rules:

- `/` and `/concept/<id>` must contain **no `<script>` elements**.
- `/lens` contains two scripts: the graph JSON payload and the module entry.
- All internal links pass through `BasePath` helpers so the site can be hosted under a subpath.
- Every URL in an `href`, `src` or `data-*` attribute is escaped by `html/template`.

## Markdown Rendering

Use goldmark with GFM and no raw HTML:

```go
md := goldmark.New(
    goldmark.WithExtensions(extension.GFM),
    goldmark.WithParserOptions(parser.WithAutoHeadingID()),
    goldmark.WithRendererOptions(
        html.WithUnsafe(), // disabled: leave raw HTML escaped
    ),
)
```

Requirements:

- Escape raw HTML from authors.
- Render headings, paragraphs, lists, blockquotes, code spans, fenced code, emphasis and links.
- Rewrite in-bundle links to `/concept/<encoded-id>`.
- Leave external links unchanged, adding `rel="noreferrer"`.
- Anchors (`#section`) stay as anchors.
- Broken in-bundle body links render as anchors to the would-be concept route so the user can see
  where they point; the outbound links section marks them as non-resolving without an anchor.
- Strip the leading H1 when it duplicates the page title.
- Do not render tables of contents, footnotes, or custom syntax in v1.

Test IDs in the concept body are not required; the body is user content.

## Concept Page Contract

```html
<article data-testid="concept-page" data-id="concepts/node-graph-engine">
  <h1>Node Graph Engine</h1>
  <p class="concept-meta">
    Type: <span data-testid="concept-type">Concept</span> ·
    Status: <span data-testid="concept-status">draft</span>
  </p>
  <p class="concept-description">Builds a traversable graph from bundle edges.</p>
  <section data-testid="concept-body">…rendered Markdown…</section>
  <section data-testid="concept-links">
    <h2>Links</h2>
    <ul>
      <li>
        <a data-testid="concept-link" data-resolved="true" href="/concept/concepts/search-index">
          bundle edges
        </a>
      </li>
      <li><span data-testid="concept-link" data-resolved="false">broken links</span></li>
    </ul>
  </section>
  <section data-testid="backlinks">
    <h2>Backlinks</h2>
    <ul>
      <li>
        <a data-testid="backlink" data-id="concepts/node-graph-engine" href="/concept/…">
          Node Graph Engine
        </a>
      </li>
    </ul>
  </section>
</article>
```

Rules:

- `data-testid="concept-type"` and `concept-status` contain only the value, not a prefix.
- Resolved outbound links put `data-testid`, `data-resolved="true"` and `href` on the anchor.
- Dangling outbound links put `data-testid` and `data-resolved="false"` on a `<span>` with no
  anchor.
- Backlinks deduplicate by source id and sort by title, then id.
- Missing description, status, tags or links render an empty or "None" state without breaking
  the contract.

## Explorer Page Contract

Preserve the existing test hooks and structure:

- `data-testid="bundle-meta"`, `bundle-version`, `bundle-valid`, `bundle-count`.
- `data-testid="bundle-index"` with `index-entry` and optional `index-description`.
- `data-testid="bundle-validation"` with `validation-entry`.
- `data-testid="concept"` with `data-id`, plus `status`, links and `data-resolved` hooks.

The explorer remains zero-JS and server-rendered.

## Lens Shell Contract

```html
<div id="weave-lens-root" data-testid="lens-canvas">
  <!-- server-rendered warning and empty state only -->
</div>
<aside data-testid="lens-index">
  <a data-testid="lens-index-entry" data-id="concepts/okf-bundle"
     href="/concept/concepts/okf-bundle">OKF Bundle</a>
</aside>
<script type="application/json" id="weave-graph">{…}</script>
<script type="module" src="/assets/lens.js"></script>
```

Rules:

- The JSON payload is the `SerializedGraph` contract, sorted by id.
- Escape `<` as `\u003c` (and `\u2028`/`\u2029`) inside the JSON script.
- Render the lens index server-side for keyboard access and deterministic tests.
- If the graph is empty, render `data-testid="empty-message"` server-side and do not load the
  lens script.
- If the lens bundle was not built, omit the script tag and render
  `data-testid="lens-warning"` with the build error.
- `data-testid="lens-index-entry"` lives on the anchor, not the list item.

## Theme

```go
type Theme struct {
    CSS  []byte            // full stylesheet override
    Vars map[string]string // optional CSS custom-property overrides
}
```

- The default theme is dark, using CSS custom properties (`--weave-bg`, `--weave-surface`,
  `--weave-text`, `--weave-accent`, `--weave-border`).
- Hosts may replace the stylesheet entirely or override variables.
- The lens canvas reads the same variables where possible; color constants that cannot be
  expressed in CSS are defined in the lens asset.

## Base Path and Assets

- `BasePath` defaults to `/` and must start and end with `/`.
- Helpers: `r.URL("/assets/weave.css")`, `r.ConceptURL(id)`.
- The mount passes its own asset manifest; the renderer never reads files at request time.
- Asset URLs may include a content hash for cache busting.

## Security

- `html/template` autoescaping is mandatory for all data.
- The only trusted HTML is the Markdown renderer output, which must not pass raw author HTML.
- Never interpolate user data with `template.HTML`.
- Escape JSON payloads for script context.
- `NotFound` must escape the requested id and suggestion titles.
- `BasePath` is validated, never concatenated from untrusted input.

## Testing

1. Golden HTML tests per fixture, normalized for whitespace.
2. Test-ID contract tests for explorer, concept, lens and not-found.
3. Zero-JS assertions: no `<script` on `/` and concept pages.
4. XSS tests: `<script>`, `<img onerror>`, quotes in attributes, `</script>` in graph JSON.
5. Markdown cases: headings, lists, code fences, emphasis, reference links, broken links.
6. Playwright E2E against the Go mount using the existing scenarios.
7. Accessibility smoke checks: heading order, link names, navigation landmark.

## Parity Map from TypeScript

| TypeScript | Go target |
| --- | --- |
| `src/lib/markdown.ts` | `render/markdown/*` |
| `src/lib/concept-render.ts` | `render/concept.go` |
| `src/lib/explorer-render.ts` | `render/explorer.go` |
| `src/lib/lens-shell.ts` | `render/lens.go` |
| `src/lib/site-render.ts` | `render/render.go`, `theme.go`, `templates/*` |
| `src/styles/global.css` (visual tokens) | `render/assets/weave.css` |

## Related Documents

- [Contract layer](contract.md)
- [Engine layer](engine-go.md)
- [Lens layer](lens-ts.md)
- [Mount layer](mount-go.md)
- [Integration contracts](../integration/contracts.md)
