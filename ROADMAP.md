# Weave Roadmap

> Weave loads OKF bundles into memory or a persistent SQLite index, relates documents as a graph, and answers calibrated keyword, BM25 and fuzzy top-K searches through a Go CLI.

Status: **v1 in progress.** The core engine, introspection commands, SQLite storage foundation,
incremental sync, watch reload, BM25, permissive ingestion, privacy directives, body compression,
extension IPC with enrichment/rerank, and the MCP server through watch reload are shipped on `main`;
external crypto providers, the AI plugin, MCP sensitive access, `weave sync --watch`, release
automation, bundle export, multiple bundle sources, global configuration and the optimization
program remain.

## 1. What Weave Is

Weave is a **local-first, related-document knowledge database**:

- **v1 core (shipped):** ingest OKF/Markdown bundles into memory or a persistent SQLite index,
  build a graph, search by mandatory keywords, weighted fuzzy similarity and BM25 lexical ranking,
  navigate with `show`, `backlinks` and `path`, audit bundle quality with `weave audit`, explore
  graph neighborhoods with `weave relate`, explain ranking with `weave explain`, ingest non-OKF
  Markdown permissively, and sync indexed bundles incrementally.
- **v1.0 platform (in progress):** persist and incrementally sync bundles into SQLite, add lexical
  BM25, support permissive Markdown ingestion, protect sensitive content with explicit privacy
  directives, and run external enrichment/reranking and crypto extensions.
- **v1.0 MCP (in progress):** expose the same operations through an in-memory and then a
  persistent, sensitive-aware MCP server.

It is **CLI-first**, local, offline-capable, and has **no UI**.

## 2. Current Capabilities (shipped v1 core)

| Capability | Status | Notes |
| --- | --- | --- |
| OKF bundle parsing | Shipped | Go, `yaml.v3`, UTF-8 strict, recursive directories |
| Mandatory `keywords` | Shipped | Missing keywords invalidate the document; kept for graph/fuzzy |
| Link extraction | Shipped | Inline, titled, angle, reference, collapsed and shortcut links |
| Link resolution | Shipped | Fragments/queries stripped; bundle-relative and external targets |
| Symlink handling | Shipped | One directory level with realpath cycle guard |
| Node/edge graph | Shipped | `link` and `related` edges, dangling targets, degrees |
| BFS pathfinding | Shipped | `weave path` |
| Keyword index | Shipped | Exact + prefix, AND semantics |
| Fuzzy index | Shipped | Field token postings + trigram index |
| Levenshtein + trigram Dice | Shipped | Max of both, threshold `0.55` |
| Weighted fields | Shipped | Title `1.00`, description `0.60`, body `0.30` |
| Hybrid ranking | Shipped | `0.50 * keyword + 0.50 * fuzzy` |
| Top-K | Shipped | Default 10, max 100 |
| Parse cache | Shipped | Opt-in engine, enabled by CLI, concurrency-safe |
| Calibration | Shipped | NDCG@K and MRR@K baselines |
| Conformance | Shipped | Bundle, graph and search goldens |
| Bundle audit | Shipped | Stable finding codes, `--fail-on` gate, JSON and human reports |
| Graph relate | Shipped | Bounded depth/direction graph neighborhoods |
| Search explain | Shipped | Per-token keyword and per-field fuzzy score breakdowns |
| SQLite storage | Shipped | Default `~/.weave/index.db`, bundle-scoped, `--no-db` opt-out |
| Permissive ingestion | Shipped | Synthetic Markdown documents with warnings; exact keyword excluded |
| Incremental sync | Shipped | SHA-256 add/update/remove/skip with cascade and dangling links |
| BM25 lexical ranking | Shipped | SQLite FTS5 `--mode bm25` with field weights and phrases |

## 3. v1.0 Scope (in progress)

Everything below is part of v1.0. There is no separate v2 platform; phases are internal delivery
waves, not product versions.

| Capability | Target | Notes |
| --- | --- | --- |
| Persistent SQLite storage | v1.0 | Shipped: pure-Go `modernc.org/sqlite`, bundle-scoped, WAL |
| Incremental sync | v1.0 | Shipped: SHA-256 add/update/remove/skip with cascade |
| Watch mode | v1.0 | Shipped: SHA-256 polling watcher, MCP live reload; `fsnotify` deferred |
| Permissive ingestion | v1.0 | Shipped: synthetic Markdown documents with warnings |
| BM25 lexical ranking | v1.0 | Shipped: SQLite FTS5 `--mode bm25` |
| BM25 calibration | v1.0 | Shipped: BM25 + fuzzy hybrid corpus and baseline |
| BM25 explain | v1.0 | Shipped: lexical raw/normalized scores, fields and term evidence |
| Memory/body compression | v1.0 | Shipped: zstd default, LZ4 opt-in; postings deferred |
| Privacy directives | v1.0 | Shipped: `encrypt`, `tokenize`, `hash`, key resolution, `show --decrypt` |
| Extension runner | v1.0 | Shipped: subprocess IPC (`weave-<name>`) over `weave.cli/v1` stdio |
| Enrichment extension | v1.0 | Shipped: `--with-enricher`, inferred keywords and metadata |
| Rerank extension | v1.0 | Shipped: `--rerank`, 0.5/0.5 blend and fallback |
| External crypto providers | v1.0 | Age, HashiCorp Vault and cloud KMS over provider IPC |
| Explainable search | v1.0 | Shipped: per-token/per-field `weave explain` |
| Bundle audit | v1.0 | Shipped: stable finding codes, `--fail-on` gate, JSON and human reports |
| Graph relate | v1.0 | Shipped: bounded depth/direction subgraphs |
| MCP server | v1.0 | Tools, stdio/TCP transport, auth, resources and watch reload shipped; sensitive access pending |
| Release automation | v1.0 | Tag-driven verified builds and draft GitHub Releases |
| Bundle export | v1.0 | Raw plaintext export with confirmation; encrypted OKF export |
| Multiple bundle sources | v1.0 | Repeatable `--source` aliases with merged search |
| Optimization program | v1.0 | Benchmarks, budgets, maintainability and UX targets |
| Global configuration | v1.0 | YAML config plus `WEAVE_*` variables and `weave config` |

## 4. CLI Surface

### Shipped

```sh
weave contract --json
weave parse <path> --json
weave graph <path> --json
weave search <path> <query> [--mode hybrid|keyword|fuzzy|bm25] [--top-k <n>] [--rerank <name>] [--timeout <d>] --json
weave list <path> [--type <t>] [--tag <t>] [--status <s>] [--limit <n>] --json
weave show <path> <id|file> [--decrypt] --json
weave backlinks <path> <id|file> --json
weave path <path> <id|file> <id|file> --json
weave audit <path> [--fail-on error|warning|none] [--verify-keys] --json
weave relate <path> <id|file> [--depth <n>] [--direction out|in|both] --json
weave explain <path> <query> [--id <id>] [--mode hybrid|keyword|fuzzy] [--top-k <n>] --json
weave ingest <path> [--db <file>] [--strict|--permissive] [--codec zstd|lz4|none] [--with-enricher <name>] [--timeout <d>] --json
weave rebuild <path> [--db <file>] [--strict|--permissive] [--codec zstd|lz4|none] [--with-enricher <name>] [--timeout <d>] --json
weave sync <path> [--db <file>] [--strict|--permissive] [--codec zstd|lz4|none] [--with-enricher <name>] [--timeout <d>] --json
weave extensions list [<path>] --json
weave mcp --bundle <path> [--stdio|--listen <addr>] [--token <t>] [--db <file>] [--watch]
weave version --json
```

Bundle-reading commands default to `~/.weave/index.db`; `--no-db` forces in-memory parsing and
`--db <file>` selects another database.

### Planned v1.0 — remaining surface

```sh
weave sync <path> --watch [--with-enricher <ext>] --json
weave export <path> --out <dir> --format raw|encrypted [--key-id <id>] [--yes] [--force] --json
weave search <path> [--source <alias=path> ...] <query> --json
weave ingest <path> [--source <alias=path> ...] --json
weave config show|path|init [--json]
```

## 5. Search Semantics

### Shipped

- **Keyword:** exact token `1.0`, prefix `0.75`, AND across query tokens.
- **Fuzzy:** max of normalized Levenshtein and trigram Dice, threshold `0.55`.
- **Field weights:** title `1.00`, description `0.60`, body `0.30`.
- **Hybrid:** persistent searches use `0.50 * BM25 + 0.50 * fuzzy`; `--no-db` keeps legacy
  keyword + fuzzy.
- **BM25:** SQLite FTS5 `--mode bm25`, OR terms, phrases, negative terms.
- **Top-K:** default 10, max 100.

### v1.0 target

- **Lexical:** BM25 explain evidence shipped.
- **Fuzzy:** unchanged.
- **Hybrid:** `a * lexical + b * fuzzy + c * rerank`; persistent hybrid uses lexical BM25.
- **Extensions:** optional rerankers may adjust ranking after core scoring.

## 6. Privacy Semantics (shipped)

Privacy is **explicitly declared**, never heuristic.

| Directive | Storage | Search behavior |
| --- | --- | --- |
| `encrypt` | AES-256-GCM ciphertext | Excluded from index; masked snippets; restored only with key |
| `tokenize` | Pseudonym | Indexed and searchable as the pseudonym |
| `hash` | HMAC-SHA256 digest | Exact digest match only; fuzzy/trigram bypassed |

- Frontmatter file-level policy: `privacy.policy: encrypt`.
- Inline block directives: `<!-- weave:encrypt -->`, `<!-- weave:tokenize -->`,
  `<!-- weave:hash -->`.
- Keys resolve from `WEAVE_KEY_<ID>` and `.weave/security.yaml`.
- Missing keys are audit warnings; `show --decrypt` fails with exit `3`.

Full contract: [Privacy Directives](docs/specs/privacy-directives.md). Extension IPC:
[Privacy & Extensions](docs/specs/privacy-and-extensions.md). External key stores:
[External Crypto Providers](docs/specs/external-crypto-providers.md).

## 7. Architecture

### Current

```text
cmd/weave + engine/operations
  ingest · rebuild · sync · search · parse · graph · list · show · backlinks · path · audit · relate · explain · version
        |
engine/search
  documents · keyword postings · fuzzy/trigram indexes · ranking · explain evidence
        |
engine/audit
  conformance · links · index coverage · orphans
        |
engine/relate
  bounded BFS · depth · direction · subgraphs
        |
storage/sqlite
  bundle registry · documents · edges · schema version · incremental sync
        |
engine/graph + engine/okf
  nodes · edges · traversal · parser · validation · cache
        |
contract/v1
  schemas · Go types · goldens
```

### v1.0 target

```text
cmd/weave (CLI) / cmd/weave-mcp (daemon)
                  |
          engine/operations
 ingest · sync · search · audit · relate · explain
                  |
   ┌──────────────┼──────────────┐
engine/okf     engine/search   engine/crypto
Goldmark AST   SQLite FTS5     AES / HMAC
Links          BM25 + trigram  Key resolver
Frontmatter    Varint postings Extension runner
   └──────────────┼──────────────┘
                  |
           storage/sqlite
    WAL · modernc.org/sqlite
    docs · edges · fts · enrichments
```

## 8. Phased Roadmap

### Feature Register

Every roadmap feature has exactly one authoritative spec and exactly one backlog row. This register
is the completeness check: no feature is considered planned until it appears here and in
[`docs/backlog.md`](docs/backlog.md). Delivery status lives in the backlog.

| # | Feature | Authoritative spec | Backlog | Depends on | Wave / gate | Status |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | Introspection contract types | [audit](docs/specs/weave-audit.md), [relate](docs/specs/weave-relate.md), [explain](docs/specs/weave-explain.md) | Tier 3 | `contract/v1`, operations | 1 / Phase 2 | shipped |
| 2 | `weave relate` | [weave-relate.md](docs/specs/weave-relate.md) | Tier 3 | graph, contract | 1 / Phase 2 | shipped |
| 3 | `weave audit` | [weave-audit.md](docs/specs/weave-audit.md) | Tier 3 | parser, graph, root index, contract | 1 / Phase 2 | shipped |
| 4 | `weave explain` | [weave-explain.md](docs/specs/weave-explain.md) | Tier 3 | search scoring, contract | 1 / Phase 2 | shipped |
| 5 | Permissive ingestion | [permissive-ingestion.md](docs/specs/permissive-ingestion.md) | Tier 5 | parser validation | 1 / Phase 2 | shipped |
| 6 | SQLite storage | [storage-and-sync.md](docs/specs/storage-and-sync.md) | Tier 5 | parser, graph, contract | 2 / Phase 3 | shipped |
| 7 | Incremental sync | [storage-and-sync.md](docs/specs/storage-and-sync.md) | Tier 5 | storage, parser, graph | 3 / Phase 3 | shipped |
| 8 | Watch mode | [watch-mode.md](docs/specs/watch-mode.md) | Tier 5 | incremental sync | 4 / Phase 3 | shipped |
| 9 | BM25 lexical ranking | [bm25-ranking.md](docs/specs/bm25-ranking.md) | Tier 5 | storage/FTS5, search contract | 3 / Phase 3 | shipped |
| 10 | BM25 calibration | [bm25-ranking.md](docs/specs/bm25-ranking.md) | Tier 5 | BM25, calibration corpus | 3 / Phase 3 | shipped |
| 11 | Compression | [compression.md](docs/specs/compression.md) | Tier 5 | storage schema | 3 / Phase 3 | shipped |
| 12 | Privacy directives | [privacy-directives.md](docs/specs/privacy-directives.md) | Tier 5 | parser, storage, search, contract, key resolver | 4 / Phase 4 | shipped |
| 13 | Extension subprocess IPC | [extension-ipc.md](docs/specs/extension-ipc.md) | Tier 5 | operations, contract | 5 / Phase 5 | shipped |
| 14 | Enrichment extension | [extension-ipc.md](docs/specs/extension-ipc.md) | Tier 5 | sync, extension runner | 5 / Phase 5 | shipped |
| 15 | Rerank extension | [extension-ipc.md](docs/specs/extension-ipc.md) | Tier 5 | search, extension runner | 5 / Phase 5 | shipped |
| 16 | AI plugin | [weave-ai-plugin.md](docs/specs/weave-ai-plugin.md) | Tier 5 | extension runner | 6 / Phase 5 | planned |
| 17 | External crypto providers | [external-crypto-providers.md](docs/specs/external-crypto-providers.md) | Tier 5 | privacy directives, extension runner | 6 / Phase 5 | planned |
| 18 | Sensitive MCP policies | [mcp-server.md](docs/specs/mcp-server.md), [privacy-and-extensions.md](docs/specs/privacy-and-extensions.md) | Tier 6 | privacy directives, MCP in-memory | 5 / Phase 6 | planned |
| 19 | MCP in-memory | [mcp-server.md](docs/specs/mcp-server.md) | Tier 6 | operations, relate/audit/explain | 3 / Phase 6 (can be pulled earlier) | shipped |
| 20 | MCP persistent | [mcp-server.md](docs/specs/mcp-server.md) | Tier 6 | MCP in-memory, storage, sync, watch, privacy | 6 / Phase 6 | shipped |
| 21 | BM25 explain evidence | [weave-explain.md](docs/specs/weave-explain.md), [bm25-ranking.md](docs/specs/bm25-ranking.md) | Tier 5 | BM25, storage, explain | 3 / Phase 3 | shipped |
| 22 | MCP stdio/TCP transport | [mcp-server.md](docs/specs/mcp-server.md) | Tier 6 | MCP in-memory | 6 / Phase 6 | shipped |
| 23 | MCP resources | [mcp-server.md](docs/specs/mcp-server.md) | Tier 6 | MCP transport | 6 / Phase 6 | shipped |
| 24 | MCP watch reload | [watch-mode.md](docs/specs/watch-mode.md), [mcp-server.md](docs/specs/mcp-server.md) | Tier 6 | incremental sync, MCP persistent | 4 / Phase 3 | shipped |
| 25 | Release automation | [release-automation.md](docs/specs/release-automation.md) | Tier 7 | contract, CI | 7 / Phase 7 (P1) | planned |
| 26 | Bundle export | [bundle-export.md](docs/specs/bundle-export.md) | Tier 7 | storage, privacy, contract | 7 / Phase 7 (P1) | planned |
| 27 | Multiple bundle sources | [multi-bundle.md](docs/specs/multi-bundle.md) | Tier 7 | storage, operations, contract | 7 / Phase 7 (P2) | planned |
| 28 | Performance and maintainability | [optimization.md](docs/specs/optimization.md) | Tier 7 | all packages | 8 / Phase 8 (P3) | planned |
| 29 | Global configuration | [configuration.md](docs/specs/configuration.md) | Tier 7 | CLI, contract, optimization | 8 / Phase 8 (P2) | planned |

Rules:

- A feature may have several specs only when the behavior is genuinely shared (introspection types,
  sensitive MCP); the first listed spec owns the contract.
- A spec may own several features only when they share one delivery boundary (storage/sync,
  extension runner, MCP in-memory/persistent).
- If a feature is added to the roadmap without a spec and backlog row, the register is incomplete.
- Delivery gate means the phase in which the feature's acceptance criteria must pass before the
  phase is considered done.

### Feature Dependency Map

The phases below are not arbitrary; they follow this dependency graph.

| Feature | Hard dependencies | Enables |
| --- | --- | --- |
| Introspection contract types | `contract/v1`, operations | audit, relate, explain, MCP tool schemas |
| `weave relate` (shipped) | graph, contract | graph workflows, MCP relate |
| `weave audit` (shipped) | parser validation, graph, root index, contract | CI docs gate, MCP audit, privacy key checks |
| `weave explain` (shipped) | search scoring internals, contract | ranking debugging, calibration, MCP explain |
| Permissive ingestion (shipped) | parser/validation | ingestion of dirty Markdown trees |
| SQLite storage | parser, graph, contract | sync, BM25, compression, privacy storage, MCP persistent |
| Incremental sync (shipped) | storage, parser, graph | watch, enrichment, MCP persistent |
| Watch mode | incremental sync | live MCP reload |
| BM25 lexical ranking (shipped) | storage/FTS5, search contract, calibration | BM25 hybrid ranking |
| Compression | storage | memory/body budgets |
| Privacy directives | parser, storage, search, CLI, contract, key resolver | decrypt, audit key checks, sensitive MCP, crypto plugins |
| Extension runner | operations/contract | enrichment, rerank, AI and crypto plugins |
| Enrichment extension | sync + extension runner | persisted enrichment metadata |
| Rerank extension | search + extension runner | optional ranking |
| AI plugin | extension runner | inferred keywords, embeddings |
| External crypto plugins | privacy + extension runner | Age, Vault, KMS |
| MCP in-memory | operations + relate/audit/explain | in-memory agent retrieval |
| MCP persistent | MCP in-memory + storage/sync/watch/privacy | persistent, sensitive-aware MCP |

```text
contract/v1
  ├── okf parser ──┬── graph ──┬── relate
  │                │           ├── audit
  │                │           └── path/show/backlinks
  │                ├── permissive ingestion
  │                └── storage/sqlite ──┬── sync ──┬── watch
  │                                      │          └── enrichment
  │                                      ├── BM25
  │                                      └── compression
  ├── search (keyword/fuzzy/hybrid) ──┬── explain
  │                                   ├── privacy search rules
  │                                   └── rerank
  └── operations ──┬── CLI
                   ├── MCP in-memory
                   └── extension runner ──┬── AI plugin
                                          └── external crypto
privacy directives ──┬── show --decrypt
                     ├── audit --verify-keys
                     ├── sensitive MCP
                     └── crypto plugins
```

### Topological Execution Order

**Wave 0 — shipped:** parser, graph, keyword/fuzzy search, CLI, calibration.

**Wave 1 — introspection and adoption (parallel):**

- contract types for audit/relate/explain,
- `weave relate`,
- `weave audit`,
- `weave explain`,
- permissive ingestion.

**Wave 2 — storage foundation:**

- SQLite storage.

**Wave 3 — storage consumers (parallel):**

- incremental sync,
- BM25 lexical ranking, calibration and explain evidence,
- compression,
- MCP in-memory (independent of persistence; can be pulled earlier if desired).

**Wave 4 — lifecycle and privacy:**

- watch mode (shipped),
- privacy directives (after storage + search),
- `show --decrypt` and `audit --verify-keys`.

**Wave 5 — extensions:**

- extension runner,
- enrichment (after sync),
- rerank (after search),
- sensitive MCP policies.

**Wave 6 — ecosystem and persistent MCP:**

- AI plugin,
- external crypto plugins,
- MCP persistent with storage/watch/privacy.

**Wave 7 — delivery and data portability (priority order):**

1. release automation (tag-driven verified artifacts),
2. bundle export (raw with confirmation, encrypted),
3. multiple bundle sources (aliases and merged search).

**Wave 8 — optimization program (continuous):**

- benchmarks and budgets,
- time/space hotspots,
- maintainability refactors and the performance gate,
- UX optimization with global configuration and environment variables first.

**Critical path:**

```text
contract/v1 -> parser/graph/search -> storage/sqlite -> sync -> BM25 -> privacy -> extension runner -> MCP persistent
```

`audit`, `relate`, `explain`, permissive ingestion, compression and MCP in-memory are parallel
tracks, not prerequisites for the critical path.

### Phase 1 — Core search (shipped)

- Mandatory keywords validation.
- In-memory keyword/fuzzy/hybrid top-K search.
- Link resolution and BFS pathfinding.
- Calibration suite (NDCG@10 ≥ 0.75, MRR@10 ≥ 0.70).
- Pure Go CLI with contract goldens.

### Phase 2 — Introspection and adoption (complete)

- `weave audit` (shipped): bundle conformance, graph validation, keyword/index consistency.
- `weave relate` (shipped): bounded graph neighborhoods by depth and direction.
- `weave explain` (shipped): per-token, per-field score decomposition.
- Permissive ingestion mode (shipped): warn instead of fail on dirty Markdown trees.

Specs: [audit](docs/specs/weave-audit.md), [relate](docs/specs/weave-relate.md),
[explain](docs/specs/weave-explain.md), [permissive ingestion](docs/specs/permissive-ingestion.md).

### Phase 3 — Persistent storage and sync (v1.0)

- Pure-Go SQLite (`modernc.org/sqlite`) in WAL mode.
- Default bundle-scoped database at `~/.weave/index.db`.
- `weave ingest`, `weave rebuild`, `weave sync` and `--mode bm25` shipped.
- `sha256` content hashes, cascading graph updates and a polling watch loop shipped; `fsnotify`
  event delivery remains an optimization.
- Body compression shipped with zstd by default and LZ4 opt-in; `weave sync --watch` CLI wiring and
  posting compression remain.

Specs: [Storage & Sync](docs/specs/storage-and-sync.md),
[Watch Mode](docs/specs/watch-mode.md), [BM25 Ranking](docs/specs/bm25-ranking.md),
[Compression](docs/specs/compression.md).

### Phase 4 — Privacy primitives (complete)

- Standard-library `aes-256-gcm`, `sha256`, `hmac-sha256` (shipped).
- Frontmatter and inline privacy directives (shipped).
- Key resolution via environment and `.weave/security.yaml` (shipped).
- Audit key-availability checks (shipped).

Specs: [Privacy Directives](docs/specs/privacy-directives.md),
[Privacy & Extensions](docs/specs/privacy-and-extensions.md).

### Phase 5 — Extension architecture (runner, enrichment and rerank complete)

- `weave-<name>` subprocess plugins over `weave.cli/v1` stdio (shipped).
- Enrichment during ingest, rebuild and sync (shipped).
- Reranking during search (shipped).
- Optional `weave-ai` plugin for local/cloud keyword extraction and embeddings.
- External crypto providers (Age, Vault, KMS).

Specs: [Extension Subprocess IPC](docs/specs/extension-ipc.md),
[Privacy & Extensions](docs/specs/privacy-and-extensions.md),
[Weave AI Extension](docs/specs/weave-ai-plugin.md),
[External Crypto Providers](docs/specs/external-crypto-providers.md).

### Phase 6 — MCP server (v1.0)

- TCP MCP server over operations.
- Bind to persistent storage and support watch-based reload (shipped).
- Tools: `weave_search`, `weave_explain`, `weave_relate`, `weave_audit`, `weave_show`,
  `weave_path`.
- Strict sensitive-data boundary; encrypted fields require explicit opt-in (pending).

Spec: [MCP Server](docs/specs/mcp-server.md).

The operations layer makes MCP transport-independent; it can be pulled earlier than Phase 6 if
needed.

## 9. Quality Gates and Runtime

### Versioning Policy

- **Product:** v1.0 is the single full-scope release. Every feature in the register gates v1.0;
  phases are internal delivery waves, not separate releases.
- **Contract:** `weave.contract/v1` is authoritative. Until v1.0.0 GA it may evolve in place; after
  GA, breaking changes require `contract/v2`.
- **CLI and extensions:** `weave.cli/v1` is authoritative. Additive messages and fields are allowed
  before and after GA; breaking changes after GA require `weave.cli/v2`.
- **Storage:** the SQLite schema uses `PRAGMA user_version` with explicit migrations, independent of
  the product and contract versions.
- **Feature status:** roadmap and backlog use `shipped`, `planned` and pipeline states; there are no
  v1.1 or v2 feature labels.

### Quality gates

- `go test -race ./...`
- `go vet ./...`
- `gofmt`
- contract goldens regenerate with zero drift
- `TestCalibration` baseline
- `go run ./cmd/docs-lint`
- Markdown lint in CI

### Runtime and packaging

- Go 1.27 module.
- Pure Go binary: `go build -o weave ./cmd/weave`.
- Zero CGO in core and planned SQLite (`modernc.org/sqlite`).
- No Node, Bun, Python or UI runtime dependencies.
- `weave.cli/v1` is authoritative across CLI, MCP and extensions.

## 10. Limitations and Non-Goals

- The shipped core is in-memory; v1.0 adds optional SQLite persistence. It is not an enterprise
  search cluster.
- No semantic/vector search in core. Fuzzy handles typos, not meaning.
- No synonyms/stemming yet.
- No PDF/HTML/web ingestion.
- No writes to source bundles.
- No HTTP REST API.
- No browser UI, mount server, renderer or lens.
- No multi-tenant remote MCP hosting.

## 11. Use Cases

- Documentation reliability gates.
- Service catalogs, ownership and runbooks.
- Incident response under degraded conditions.
- Compliance/policy/control mapping.
- Data catalogs and lineage.
- ADR/architecture knowledge.
- Offline and air-gapped field knowledge.
- Curriculum/prerequisite graphs.
- Local retrieval for agents and MCP clients.

## 12. Specifications

- [Keyword + Fuzzy Search](docs/specs/keyword-fuzzy-search.md)
- [Weave Audit](docs/specs/weave-audit.md)
- [Weave Relate](docs/specs/weave-relate.md)
- [Weave Explain](docs/specs/weave-explain.md)
- [Permissive Ingestion](docs/specs/permissive-ingestion.md)
- [Storage & Sync](docs/specs/storage-and-sync.md)
- [Watch Mode](docs/specs/watch-mode.md)
- [BM25 Ranking](docs/specs/bm25-ranking.md)
- [Compression](docs/specs/compression.md)
- [Privacy Directives](docs/specs/privacy-directives.md)
- [Extension Subprocess IPC](docs/specs/extension-ipc.md)
- [Release Automation](docs/specs/release-automation.md)
- [Bundle Export](docs/specs/bundle-export.md)
- [Multiple Bundle Sources](docs/specs/multi-bundle.md)
- [Performance and Maintainability](docs/specs/optimization.md)
- [Configuration](docs/specs/configuration.md)
- [Privacy & Extensions](docs/specs/privacy-and-extensions.md)
- [External Crypto Providers](docs/specs/external-crypto-providers.md)
- [Weave AI Extension](docs/specs/weave-ai-plugin.md)
- [MCP Server](docs/specs/mcp-server.md)
- [Architecture](docs/architecture/index.md)
- [Backlog](docs/backlog.md)
