# Weave Contract v1

This directory is the language-neutral source of truth for Weave's documents, keywords, graph,
search responses and CLI envelopes.

## Layout

```text
contract/v1/
├── schema/
│   ├── defs.schema.json          # authoritative definitions
│   ├── bundle.schema.json
│   ├── concept.schema.json
│   ├── graph.schema.json
│   ├── search.schema.json
│   ├── list-results.schema.json
│   ├── navigation.schema.json
│   ├── show.schema.json
│   ├── backlinks.schema.json
│   ├── path.schema.json
│   ├── audit.schema.json
│   ├── error.schema.json
│   ├── envelope.schema.json
│   └── cli.schema.json
├── types.go                      # hand-written Go structs, schema-tested
└── golden/
    ├── manifest.json             # cases, search queries and nav operations
    ├── bundles/                  # input OKF fixtures with keywords
    └── expected/                 # generated expected JSON
```

`defs.schema.json` is authoritative. The other schema files are entrypoint references for
validators and reviewers.

## Source Order

1. Update `schema/defs.schema.json`.
2. Update `types.go`.
3. Update or add golden fixtures with `keywords`.
4. Run `go run ./cmd/contract-export`.
5. Review the golden diff.
6. Run `go test ./contract/v1`.

Never hand-edit files under `golden/expected`; they are generated.

## Commands

```sh
go run ./cmd/contract-export   # regenerate expected JSON from fixtures
go test ./contract/v1          # schema validation and struct round-trips
go test ./engine/...           # conformance, search and calibration
```

## Adding a Golden Case

1. Add a fixture under `golden/bundles/<case>` with `type` and `keywords`.
2. Add the case to `golden/manifest.json`, including search and navigation operations.
3. Run `go run ./cmd/contract-export`.
4. Review `golden/expected/<case>.*.json`.
5. Run `go test ./contract/v1`.

Golden updates are reviewed like code. CI fails on drift.

## Normalization

- `Bundle.root` is written as `<root>`.
- Concepts, graph nodes, edges and search results are sorted deterministically.
- Scores are rounded to four decimals.
- Path separators are POSIX.
- `weave parse` includes bodies; graph/search/list/backlinks/path do not; `show` does.

## Fixture Notes

- `symlinked-dirs` uses committed symlinks (`linked -> real`, `loop -> .`); Windows checkouts may
  need Developer Mode.
- `invalid-utf8` contains a deliberately invalid byte sequence.
- `empty` contains only `.gitkeep`.
- `nested-index` verifies root-only index metadata.
- `verified-frontmatter` verifies list-of-mappings YAML.
- `reference-links` and `code-links` pin link extraction.

## Contract Version

- Domain: `weave.contract/v1` (new in-place v1 with keywords and search).
- CLI envelope: `weave.cli/v1`.
- Until v1.0.0 GA, `contract/v1` may evolve in place. After GA, breaking changes require
  `contract/v2`.
