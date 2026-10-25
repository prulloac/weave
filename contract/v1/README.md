# Weave Contract v1

This directory is the language-neutral source of truth for Weave's domain, operation and CLI
payloads. The Go engine, the CLI, the renderer and the TypeScript lens must all conform to it.

## Layout

```text
contract/v1/
├── schema/
│   ├── defs.schema.json          # authoritative definitions
│   ├── bundle.schema.json
│   ├── concept.schema.json
│   ├── graph.schema.json
│   ├── serialized-graph.schema.json
│   ├── search.schema.json
│   ├── find-results.schema.json
│   ├── list-results.schema.json
│   ├── navigation.schema.json
│   ├── error.schema.json
│   ├── envelope.schema.json
│   └── cli.schema.json
├── types.ts                      # generated TypeScript types
└── golden/
    ├── manifest.json             # cases and operations to export
    ├── bundles/                  # input OKF fixtures
    └── expected/                 # normalized expected JSON
```

`defs.schema.json` is the authoritative file. The other schema files are entrypoint references
for validators and reviewers.

## Source Order

When changing the contract:

1. Update `schema/defs.schema.json`.
2. Update Go structs in Phase 2+.
3. Regenerate `types.ts`.
4. Update or add golden fixtures.
5. Run `bun run contract:export`.
6. Review the golden diff.
7. Run `bun run contract:check`.

Never hand-edit `types.ts` or files under `golden/expected`. They are generated.

## Commands

```sh
bun run contract:types    # regenerate TypeScript types from the schema
bun run contract:export   # regenerate golden expected JSON from fixtures
bun run contract:check    # validate all goldens against the schemas
bun run contract:verify   # export + check
```

## Adding a Golden Case

1. Add a fixture bundle under `golden/bundles/<case>`.
2. Add the case to `golden/manifest.json`, including search and navigation operations.
3. Run `bun run contract:export`.
4. Review `golden/expected/<case>.*.json`.
5. Run `bun run contract:check`.

Golden updates are reviewed like code. CI must never silently accept a changed golden.

## Normalization

- `Bundle.root` is written as `<root>` so goldens are machine-independent.
- Concepts, graph nodes, edges and navigation results are sorted deterministically.
- Search and find scores are rounded to four decimal places.
- Path separators are POSIX.
- `weave parse` output includes concept bodies; graph, find, list, backlinks and path outputs do
  not, except `show`.

## Fixture Notes

- `symlinked-dirs` uses a committed symlink (`linked -> real`) and a self-referential
  (`loop -> .`) to exercise one-level symlink traversal and cycle protection. Windows checkouts
  may need Developer Mode or a generated fixture.
- `invalid-utf8` contains a deliberately invalid byte sequence.
- `empty` contains only `.gitkeep`.
- `nested-index` verifies that only the root `index.md` sets bundle metadata.
- `verified-frontmatter` verifies list-of-mappings YAML.
- `reference-links` and `code-links` pin Markdown link semantics.

## Contract Version

- Domain: `weave.contract/v1`
- CLI envelope: `weave.cli/v1`
- Breaking changes require `contract/v2` and a new CLI major.
