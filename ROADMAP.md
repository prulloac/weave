# Weave Roadmap

> Weave loads OKF bundles into memory, relates documents as a graph, and answers calibrated keyword and fuzzy top-K searches through a Go CLI.

Status: **v1 shipped on `main`**. The v2 platform below is proposed and specified but not implemented.

## 1. What Weave Is

Weave is a **local-first, related-document knowledge database**:

- **v1 (shipped):** ingest OKF/Markdown bundles into memory, build a graph, search by mandatory
  keywords and weighted fuzzy similarity, and navigate with `show`, `backlinks` and `path`.
- **v2 (proposed):** persist and incrementally sync bundles into SQLite, add lexical BM25, support
  permissive Markdown ingestion, protect sensitive content with explicit privacy directives, and
  run external enrichment/reranking extensions.
- **Later:** expose the same operations through an MCP server.

It is **CLI-first**, local, offline-capable, and has **no UI**.

## 2. Current Capabilities (v1)

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

## 3. Proposed v2 Platform

| Capability | Target | Notes |
| --- | --- | --- |
| Persistent SQLite storage | v2 | Pure-Go `modernc.org/sqlite`, WAL, fast cold reads |
| Incremental sync & watch | v2 | `sha256` content-hash upserts, cascading edge updates, `fsnotify` |
| Permissive ingestion | v2 | Index dirty/non-OKF Markdown trees with warnings |
| BM25 lexical ranking | v2 | SQLite FTS5 BM25 replaces strict AND for multi-token queries |
| Memory/body compression | v2 | Varint/delta postings, LZ4/Zstandard body blobs |
| Privacy directives | v2 | `encrypt`, `tokenize`, `hash` in frontmatter and inline comments |
| Extension runner | v2 | Subprocess IPC (`weave-<name>`) over `weave.cli/v1` stdio |
| Explainable search | v1.1 | Per-token/per-field `weave explain` |
| Audit and relate | v1.1 | Bundle quality gate and graph neighborhoods |
| MCP server | post-v2 | TCP MCP over the operations layer, optional sensitive access |

## 4. CLI Surface

### v1 shipped

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

### v1.1 introspection (specified)

```sh
weave audit <path> [--fail-on error|warning|none] --json
weave relate <path> <id|file> [--depth <n>] [--direction in|out|both] --json
weave explain <path> <query> [--id <id>] [--mode hybrid|keyword|fuzzy] --json
```

### v2 persistence and extensions (proposed)

```sh
weave ingest <path> [--db <file>] [--permissive] --json
weave sync <path> [--watch] [--with-enricher <ext>] --json
weave rebuild <path> --json
weave search <path> <query> [--rerank <ext>] --json
weave show <path> <id|file> [--decrypt] --json
weave audit <path> [--verify-keys] --json
```

## 5. Search Semantics

### v1 (shipped)

- **Keyword:** exact token `1.0`, prefix `0.75`, AND across query tokens.
- **Fuzzy:** max of normalized Levenshtein and trigram Dice, threshold `0.55`.
- **Field weights:** title `1.00`, description `0.60`, body `0.30`.
- **Hybrid:** `0.50 * keyword + 0.50 * fuzzy`.
- **Top-K:** default 10, max 100.

### v2 target

- **Lexical:** SQLite FTS5 BM25 with term coordination; replaces strict AND for multi-token
  queries.
- **Fuzzy:** unchanged.
- **Hybrid:** `a * lexical + b * fuzzy + c * rerank`; initial a/b remain 0.50/0.50.
- **Extensions:** optional rerankers may adjust ranking after core scoring.

## 6. Privacy Semantics (v2 proposed)

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

Full contract: [Privacy & Extensions](docs/specs/privacy-and-extensions.md).

## 7. Architecture

### v1 current

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

### v2 target

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

### Feature Dependency Map

The phases below are not arbitrary; they follow this dependency graph.

| Feature | Hard dependencies | Enables |
| --- | --- | --- |
| Contract types for introspection | `contract/v1`, operations | audit, relate, explain, MCP tool schemas |
| `weave relate` | graph, contract | graph workflows, MCP relate |
| `weave audit` | parser validation, graph, root index, contract | CI docs gate, MCP audit, v2 key checks |
| `weave explain` | search scoring internals, contract | ranking debugging, calibration, MCP explain |
| Permissive ingestion | parser/validation | v2 ingest of dirty Markdown trees |
| SQLite storage | parser, graph, contract | sync, BM25, compression, privacy storage, MCP v2 |
| Incremental sync | storage, parser, graph | watch, enrichment, MCP v2 |
| Watch mode | incremental sync | live MCP reload |
| BM25 lexical ranking | storage/FTS5, search contract, calibration | hybrid v2 ranking |
| Compression | storage | memory/body budgets |
| Privacy directives | parser, storage, search, CLI, contract, key resolver | decrypt, audit key checks, sensitive MCP, crypto plugins |
| Extension runner | operations/contract | enrichment, rerank, AI and crypto plugins |
| Enrichment extension | sync + extension runner | v2 metadata |
| Rerank extension | search + extension runner | optional ranking |
| AI plugin | extension runner | inferred keywords, embeddings |
| External crypto plugins | privacy + extension runner | Age, Vault, KMS |
| MCP v1 | operations + relate/audit/explain | in-memory agent retrieval |
| MCP v2 | MCP v1 + storage/sync/watch/privacy | persistent, sensitive-aware MCP |

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
                   ├── MCP v1
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
- BM25 lexical ranking,
- compression,
- MCP v1 (independent of v2; can be pulled earlier if desired).

**Wave 4 — lifecycle and privacy:**

- watch mode (after sync),
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
- MCP v2 with storage/watch/privacy.

**Critical path:**

```text
contract/v1 -> parser/graph/search -> storage/sqlite -> sync -> BM25 -> privacy -> extension runner -> MCP v2
```

`audit`, `relate`, `explain`, permissive ingestion, compression and MCP v1 are parallel tracks,
not prerequisites for the critical path.

### Phase 1 — Search v1 (shipped)

- Mandatory keywords validation.
- In-memory keyword/fuzzy/hybrid top-K search.
- Link resolution and BFS pathfinding.
- Calibration suite (NDCG@10 ≥ 0.75, MRR@10 ≥ 0.70).
- Pure Go CLI with contract goldens.

### Phase 2 — Introspection and adoption (next)

- `weave audit`: bundle conformance, graph validation, keyword/index consistency.
- `weave relate`: bounded graph neighborhoods by depth and direction.
- `weave explain`: per-token, per-field score decomposition.
- Permissive ingestion mode: warn instead of fail on dirty Markdown trees.

Specs: [audit](docs/specs/weave-audit.md), [relate](docs/specs/weave-relate.md),
[explain](docs/specs/weave-explain.md).

### Phase 3 — Persistent storage and sync (v2)

- Pure-Go SQLite (`modernc.org/sqlite`) in WAL mode.
- `weave ingest` and `weave sync` with `sha256` content-hash upserts.
- Cascading graph updates for edits/deletes.
- `fsnotify` watch mode.
- Varint/delta postings and compressed body storage.

Spec: [V2 Storage & Sync](docs/specs/v2-storage-and-sync.md).

### Phase 4 — Privacy primitives (v2)

- Standard-library `aes-256-gcm`, `sha256`, `hmac-sha256`.
- Frontmatter and inline privacy directives.
- Key resolution via environment and `.weave/security.yaml`.
- Audit key-availability checks.

Spec: [Privacy & Extensions](docs/specs/privacy-and-extensions.md).

### Phase 5 — Extension architecture (v2)

- `weave-<name>` subprocess plugins over `weave.cli/v1` stdio.
- Enrichment during sync; reranking during search.
- Optional `weave-ai` plugin for local/cloud keyword extraction and embeddings.
- External crypto providers (Age, Vault, KMS).

Spec: [Privacy & Extensions](docs/specs/privacy-and-extensions.md).

### Phase 6 — MCP server

- TCP MCP server over operations.
- Bind to persistent storage and support watch-based reload in v2.
- Tools: `weave_search`, `weave_explain`, `weave_relate`, `weave_audit`, `weave_show`,
  `weave_path`.
- Strict sensitive-data boundary; encrypted fields require explicit opt-in.

Spec: [MCP Server](docs/specs/mcp-server.md).

The operations layer makes MCP transport-independent; it can be pulled earlier than Phase 6 if
needed.

## 9. Quality Gates and Runtime

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

- v1 is in-memory; not an enterprise search cluster.
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
- [V2 Storage & Sync](docs/specs/v2-storage-and-sync.md)
- [Privacy & Extensions](docs/specs/privacy-and-extensions.md)
- [MCP Server](docs/specs/mcp-server.md)
- [Architecture](docs/architecture/index.md)
- [Backlog](docs/backlog.md)
