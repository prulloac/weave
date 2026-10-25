---
type: Test Strategy
title: "Integration Testing"
description: "Conformance corpus, test pyramid, CI matrix, commands and acceptance gates for the Go engine/render/mount and TypeScript lens."
tags:
  - "architecture"
  - "testing"
  - "conformance"
  - "ci"
  - "go"
generated:
  by: opencode/deepseek-v4.1-flash
  at: "2026-10-25T00:00:00Z"
status: draft
---

# Integration Testing

## Strategy

The migration is verified at six levels. Lower levels are fast and exhaustive; higher levels prove
that the layers actually compose.

| Level | Scope | Tooling | Gate |
| --- | --- | --- | --- |
| 0 | Schema validity | JSON Schema validators | Every golden file validates |
| 1 | Semantic conformance | TS exporter vs Go output | Zero unexplained diffs |
| 2 | Go unit and race | `go test`, `-race`, fuzz | Package coverage and no races |
| 3 | TS lens unit | `bun test` | Layout, payload, bootstrap |
| 4 | HTTP integration | `httptest`, `go test` | Routes, statuses, DOM hooks |
| 5 | Browser E2E | Playwright against Go binary | User journeys pass |
| 6 | Performance | Benchmarks, size budgets | Budgets met |

## Conformance Corpus

Location: `contract/v1/golden`.

```text
contract/v1/golden/
├── manifest.json
├── bundles/
│   ├── okf-bundle/            # cross-links, index, validation
│   ├── anchors/               # #fragment and ?query links
│   ├── nested-index/          # sub/index.md must not replace root metadata
│   ├── verified-frontmatter/  # YAML list-of-mappings
│   ├── reference-links/       # [x][ref], [x][], titled, angle destinations
│   ├── code-links/            # links inside fences and inline code are ignored
│   ├── symlinked-dirs/        # one-level symlink follow and cycle guard
│   ├── invalid-utf8/          # per-file parse error, skipped
│   ├── empty/                 # valid empty bundle plus warning
│   ├── version-mismatch/      # warning, best-effort parse
│   ├── deep-tree/             # nested directories and parent-relative links
│   ├── escaping/              # HTML/script payloads in text and frontmatter
│   └── large/                 # generated or checked-in medium bundle
└── expected/
    ├── <case>.bundle.json
    ├── <case>.graph.json
    ├── <case>.search-<query>.json
    └── <case>.nav-<command>.json
```

Each case is listed in `manifest.json`:

```json
{
  "contract": "weave.contract/v1",
  "cases": [
    {
      "name": "anchors",
      "bundle": "bundles/anchors",
      "expect": {
        "bundle": "expected/anchors.bundle.json",
        "graph": "expected/anchors.graph.json"
      }
    }
  ]
}
```

Normalization rules:

- Sort concept, node, edge and result arrays by id (or preserve documented order where the
  contract requires it, such as index entries).
- Remove absolute roots or replace them with `"<root>"`.
- Pretty-print with two spaces.
- Normalize path separators to `/`.
- Do not normalize scores, snippets, warnings or validation flags.

## Level 0 — Schema Validation

- Validate every expected JSON file against its schema.
- Validate HTTP JSON responses against the same schemas.
- Validate the lens payload as `SerializedGraph`.
- Reject additional fields only where the contract says they are invalid; otherwise allow them.

## Level 1 — Semantic Conformance

The TypeScript implementation is the reference until cutover.

Harness:

1. A TypeScript exporter reads a fixture bundle and writes normalized JSON for bundle, graph,
   search and navigation.
2. The Go conformance test reads the same fixture and writes the same normalized JSON.
3. A differ compares them and prints the first N differences with JSON pointers.
4. The test fails on any difference not listed in `manifest.json` as an accepted, documented
   divergence.

Example commands:

```sh
bun run contract:export -- contract/v1/golden/bundles/okf-bundle /tmp/ts
go test ./contract/v1 -run TestConformance -args --root contract/v1/golden --actual /tmp/go
bun run contract:diff /tmp/ts /tmp/go
```

Accepted divergences must be explicit and versioned, for example a documented YAML superset
behavior that the fixtures do not exercise.

## Level 2 — Go Unit, Race and Fuzz

```sh
go test ./...
go test -race ./...
go vet ./...
golangci-lint run ./...
```

Coverage targets:

- `engine/okf`: walk, frontmatter, links, parsing, validation, cache.
- `engine/graph`: edge dedupe, degrees, BFS, components.
- `engine/search`: tokenization, BM25 constants, prefix matching, snippets, tie-breaking.
- `engine/nav`: target normalization, filters, backlinks, path.
- `render`: templates, escaping, Markdown, test IDs, zero-JS.
- `mount`: routes, assets, artifacts, registry, handler lifecycle.

Fuzz targets:

- `FuzzFrontmatter` — YAML bombs, deeply nested maps, invalid UTF-8, comments.
- `FuzzLinkTarget` — schemes, fragments, traversal, encoded slashes.
- `FuzzConceptID` — traversal, empty segments, Windows separators.

Fuzz failures become regression fixtures.

## Level 3 — TypeScript Lens Unit

```sh
cd lens
bun test
```

Coverage:

- Layout math and viewport bounds.
- Empty graph, single node, many nodes.
- Degraded threshold above 1000 nodes.
- Payload parsing with unknown fields.
- Node hit-testing and URL segment encoding.
- Bootstrap idempotence and error path.
- No forbidden imports (engine/parser/search/Node built-ins).

## Level 4 — HTTP Integration

Use `httptest` against `mount.NewSite`:

```go
site, _ := mount.NewSite(ctx, fixture, mount.Options{})
defer site.Close()
srv := httptest.NewServer(site.Handler())
defer srv.Close()
```

Assertions:

- Every route status and content type.
- `/concept/unknown` returns 404 and `data-testid="not-found"`.
- `/` and `/concept/*` contain zero `<script>`.
- `/lens` contains the JSON payload and the module script when the asset is built.
- Missing lens asset yields the server index, empty/no script and `lens-warning`.
- `BasePath` rewriting works for every generated link and asset.
- Concurrent requests do not race (`-race`).
- `Close` removes the artifact root and is idempotent.

## Level 5 — Browser E2E

Run Playwright against the Go binary:

```sh
go build -o /tmp/weave ./cmd/weave
/tmp/weave mount contract/v1/golden/bundles/okf-bundle --port 4322
bunx playwright test
```

Required scenarios:

- Styled overview preserves the explorer contract.
- Shared stylesheet is linked on every route; body background is themed.
- Navigation header reaches `/lens`.
- Zero-JS routes contain no scripts.
- Lens renders canvas and legend.
- Lens index lists every concept and links to concept pages.
- Clicking an index entry reaches the concept page.
- Concept page shows title, type, status, description and body.
- Resolved and dangling links are distinguishable.
- Backlinks list incoming concepts.
- Body links navigate.
- Unknown concept renders 404 with nav.
- Concept pages contain no scripts.
- Empty bundle shows the empty state.
- Embedded subpath deployment (`BasePath`) works.

The current `tests/e2e/lens-mount-integration.spec.ts` is the starting point.

## Level 6 — Performance and Size

```sh
go test ./engine/... -bench . -benchmem -run '^$'
```

Budgets:

| Metric | Budget |
| --- | --- |
| 10k small files, no cache | Under 500 ms local SSD |
| Search, 10k docs, common query | Under 20 ms |
| Lens asset | Under 150 KB uncompressed |
| Site memory | Documented baseline, no regression per release |
| Binary | Documented baseline including embedded assets |

CI runs a smoke benchmark with a generous ceiling. Release branches record the measured numbers
in the release notes.

## Golden Update Procedure

1. Run the conformance suite and inspect the diff.
2. Decide whether the difference is a bug or an intended contract change.
3. For intended changes, update the schema and docs first.
4. Regenerate goldens explicitly with `UPDATE_GOLDEN=1`.
5. Review the golden diff in the pull request.
6. Never auto-accept goldens in CI; a red conformance gate blocks merge.

## CI Matrix

| Job | OS | Commands |
| --- | --- | --- |
| Go test | ubuntu, macos, windows | `go test -race ./...` |
| Go lint | ubuntu | `gofmt -l`, `go vet`, `golangci-lint` |
| Conformance | ubuntu | TS exporter, Go exporter, diff |
| TS unit | ubuntu | `bun test`, `biome check` |
| Lens build | ubuntu | `bun build`, size budget, manifest check |
| E2E | ubuntu | Playwright against Go binary |
| Docs | ubuntu | markdownlint |

Windows is required for symlink and hardlink artifact tests.

## Acceptance Gates per Layer

| Layer | Must pass before next phase |
| --- | --- |
| Contract | Schemas validate; TS passes goldens |
| Engine | All bundle/graph/search/nav goldens; race; fuzz corpus clean |
| Render | DOM contract; zero-JS; XSS; golden HTML |
| Lens | Unit, payload, E2E, size, no forbidden imports |
| Mount | Route parity; artifacts; registry; race; E2E |
| CLI | Exit codes; JSON stability; command E2E |
| Cutover | Dual-run diff clean; performance; rollback tested |

## Failure Triage

- **Golden diff:** identify the JSON pointer, classify as bug or intended change, fix or update
  docs/schema first.
- **Renderer mismatch:** compare the DOM contract table, then the normalized HTML; check
  `html/template` escaping and `BasePath`.
- **E2E route failure:** reproduce with `httptest`, then with a real mount, then in Playwright.
- **Lens failure:** verify the payload first, then the DOM hooks, then the client bundle.
- **Flaky tests:** no retries in CI without a filed issue; fix determinism.

## Related Documents

- [Architecture index](../index.md)
- [Contract layer](../layers/contract.md)
- [Engine layer](../layers/engine-go.md)
- [Render layer](../layers/render-go.md)
- [Lens layer](../layers/lens-ts.md)
- [Mount layer](../layers/mount-go.md)
- [Integration contracts](contracts.md)
- [Integration steps](steps.md)
