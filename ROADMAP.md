# Weave Roadmap

> Weave loads OKF bundles into memory, relates documents as a graph, and answers calibrated keyword and fuzzy top-K searches through a Go CLI.

Status: **search v1 implemented and on `main`**. Roadmap items below are proposals until their specs are approved.

## 1. What Weave Is

Weave is an **in-memory related-document database for OKF bundles**:

- load a bundle into memory,
- build concept nodes and link/`related` edges,
- search by mandatory frontmatter keywords and weighted fuzzy similarity,
- return keyword, fuzzy and hybrid top-K results,
- navigate the graph with `show`, `backlinks` and `path`.

It is **CLI-first**, local, offline and has **no UI**.

## 2. Current Capabilities

### 2.1 Engine

| Capability | Status | Notes |
| --- | --- | --- |
| OKF bundle parsing | Shipped | Go, `yaml.v3`, UTF-8 strict, recursive directories |
| Mandatory `keywords` | Shipped | Documents without keywords are invalid but remain in graph/fuzzy |
| Link extraction | Shipped | Inline, titled, angle, reference, collapsed and shortcut links |
| Link resolution | Shipped | Fragments/queries stripped; bundle-relative and external targets |
| Symlink handling | Shipped | One directory level with realpath cycle guard |
| Node/edge graph | Shipped | `link` and `related` edges, dangling targets, degrees |
| BFS pathfinding | Shipped | `weave path` |
| Keyword index | Shipped | Exact + prefix, AND across query tokens |
| Fuzzy index | Shipped | Field token postings + trigram index |
| Levenshtein + trigram Dice | Shipped | Max of both, threshold `0.55` |
| Weighted fields | Shipped | Title `1.00`, description `0.60`, body `0.30` |
| Hybrid ranking | Shipped | `0.50 * keyword + 0.50 * fuzzy` |
| Top-K | Shipped | Default 10, max 100 |
| Explainable scores | Partial | Aggregate component scores shipped; per-token explain pending |
| Parse cache | Shipped | Opt-in engine, enabled by CLI, concurrency-safe |
| Calibration | Shipped | Labeled corpus, NDCG@K and MRR@K baselines |
| Conformance | Shipped | Bundle, graph and search goldens |

### 2.2 CLI

```sh
weave contract --json
weave parse <path> --json
weave graph <path> --json
weave search <path> <query> [--mode hybrid|keyword|fuzzy] [--top-k <n>] --json
weave list <path> [--type <t>] [--tag <t>] [--status <s>] [--limit <n>] --json
weave show <path> <id|file> --json
weave backlinks <path> <id|file> --json
weave path <path> <id|file> <id|file> --json
weave version --json
```

- `--json` emits one `weave.cli/v1` envelope.
- stdout is payload only; stderr is diagnostics.
- Exit codes: `0` success, `1` runtime/no match, `2` usage, `3` permission.
- Cache enabled by default; `--no-cache` disables it.

### 2.3 Search Semantics

- **Keyword:** exact token `1.0`, prefix `0.75`, every query token must match.
- **Fuzzy:** max of normalized Levenshtein and trigram Dice.
- **Field weights:** title `1.00`, description `0.60`, body `0.30`.
- **Hybrid:** 50% keyword, 50% fuzzy.
- **Top-K:** default 10, max 100.
- Results contain keyword, fuzzy and hybrid lists with component scores and match metadata.

### 2.4 Quality Gates

- `go test -race ./...`
- `go vet ./...`
- `gofmt`
- contract goldens regenerate with no drift
- `TestCalibration` baseline: NDCG@10 ≥ 0.75, MRR@10 ≥ 0.70
- `go run ./cmd/docs-lint`
- Markdown lint in CI

### 2.5 Runtime and Packaging

- Go 1.27 module at the repository root.
- Single CLI binary: `go build -o weave ./cmd/weave`.
- No Bun, Node, Astro, Preact, Playwright or UI dependencies.
- Contract v1 is authoritative.

## 3. Architecture at a Glance

```text
cmd/weave + engine/operations
  search · parse · graph · list · show · backlinks · path · version
        |
engine/search
  documents · keyword postings · fuzzy/trigram indexes · ranking
        |
engine/graph + engine/okf
  nodes · edges · traversal · parser · validation · cache
        |
contract/v1
  schemas · Go types · goldens
```

## 4. Limitations and Non-Goals

- In-memory; not an enterprise-scale search cluster.
- No semantic/vector search. Fuzzy handles typos and partial words, not conceptual similarity.
- No synonyms/stemming yet.
- No PDF/HTML/web ingestion; input is OKF Markdown/YAML.
- No writes, transactions or multiuser persistence.
- No HTTP REST API.
- No browser UI, mount server, renderer or lens.
- No remote multi-tenant MCP hosting in the near term.

## 5. Roadmap

### Now — search v1

Shipped:

- mandatory keywords,
- keyword/fuzzy/hybrid top-K,
- `weave search`,
- calibration,
- contract/goldens,
- Go-only cleanup.

### Next — introspection commands

These are specified but not implemented yet:

| Command | Purpose | Spec | Status |
| --- | --- | --- | --- |
| `weave audit` | Bundle conformance, graph quality, keyword coverage, index consistency | [spec](docs/specs/weave-audit.md) | proposed |
| `weave relate` | Bounded graph neighborhood by depth and direction | [spec](docs/specs/weave-relate.md) | proposed |
| `weave explain` | Per-token/per-field ranking explanation | [spec](docs/specs/weave-explain.md) | proposed |

These three commands turn Weave from “search” into “inspect and trust the knowledge base.”

### Later — MCP server

Spec: [MCP Server](docs/specs/mcp-server.md)

The MCP server wraps the same operations layer:

| Phase | Scope |
| --- | --- |
| M0 | Stabilize operations for audit/relate/explain |
| M1 | MCP tool schemas and in-process harness |
| M2 | Optional stdio transport for local agents |
| M3 | TCP MCP server with token auth, lifecycle and concurrency |
| M4 | Resources, limits, metrics and packaging |

Planned MCP tools:

- `weave_search`
- `weave_explain`
- `weave_relate`
- `weave_audit`
- `weave_show`
- `weave_backlinks`
- `weave_path`
- `weave_contract`

Design constraints:

- one bundle per server process,
- read-only,
- localhost by default,
- token required for non-loopback binding,
- no arbitrary filesystem access,
- same scoring and payloads as the CLI.

### Future candidates

Not committed; requires specs and calibration:

- Synonyms and stemming.
- Keyword aliases / controlled vocabularies.
- Query analytics and zero-result reporting.
- Multi-bundle namespaces.
- Incremental reload/watch mode.
- Semantic/vector hybrid as an optional dimension.
- Concept validation profiles beyond `type`/`keywords`.

## 6. Success Metrics

- Search quality: NDCG@K and MRR@K above the committed baseline.
- Determinism: identical inputs produce identical top-K order and goldens.
- Explainability: every ranked result can be decomposed into keyword/fuzzy contributions.
- Auditability: `weave audit` can gate documentation quality in CI.
- Integration: CLI and MCP return the same payloads for the same operations.
- Performance: 10k documents within the documented parse/search budgets.

## 7. Useful Applications

- Documentation reliability gates.
- Service catalogs, ownership and runbooks.
- Incident response knowledge under degraded conditions.
- Compliance/policy/control mapping.
- Data catalogs and lineage.
- ADR/architecture knowledge.
- Offline and air-gapped field knowledge.
- Curriculum/prerequisite graphs.
- Local retrieval for agents and MCP clients.

## 8. Documentation

- [Architecture](docs/architecture/index.md)
- [Keyword + Fuzzy Search spec](docs/specs/keyword-fuzzy-search.md)
- [Weave Audit spec](docs/specs/weave-audit.md)
- [Weave Relate spec](docs/specs/weave-relate.md)
- [Weave Explain spec](docs/specs/weave-explain.md)
- [MCP Server spec](docs/specs/mcp-server.md)
- [Backlog](docs/backlog.md)
