---
type: proposal
title: "Time and Space Optimization Proposal"
description: "Measurement-first workstreams to remove duplicate parsing, eager body decoding, per-document codec allocation and full-bundle sync rewrites in Weave."
tags:
  - proposal
  - performance
  - memory
  - sqlite
  - optimization
generated:
  by: subagent
  at: "2026-10-31T02:36:57Z"
status: draft
---

# Proposal: Time and Space Optimization

## Context and goals

The [optimization spec](../../specs/optimization.md) sets 10k-document budgets: ingest 30 s, 1% sync
2 s, BM25 p95 150 ms, index memory 3x body bytes, stored zstd bodies 60% of raw, cold start 50 ms.
This proposal covers O4 (time) and O5 (space). Baselines come from planned
`proposal-benchmarks.md`; IDs below are `WEAVE-B-OKF-PARSE-10K`,
`WEAVE-B-STORE-INGEST-10K`, `WEAVE-B-STORE-SYNC-10K-NOOP`, `WEAVE-B-STORE-SYNC-10K-1PCT`, `WEAVE-B-CLI-LIST-10K`, `WEAVE-B-CLI-SHOW-10K`,
`WEAVE-B-MCP-WARM`, `WEAVE-B-WATCH-TICK`, `WEAVE-B-SPACE-DB-10K`, `WEAVE-B-MEM-INDEX-10K`, `WEAVE-B-MEM-ALLOCS-INGEST`
and `WEAVE-B-SPACE-WAL-10K`. The MCP and watch IDs are follow-ups outside the B1 registry; add them
to the workstream that needs them before optimizing.

Goals: make sync and reads proportional to changed or requested data; avoid materializing unused
bodies and search structures; cut per-document allocations; publish the size split. Gains are
estimates.

## Evidence from the code

- **Sync is O(bundle).** `Store.Sync` (`storage/sqlite/store.go:225`) reparses every file (`:246`),
  deletes all edges (`:359`), and `populateFTS` (`:734`) deletes and reinserts every FTS row with
  decode and stripping. A no-op sync still updates `bundles` (`:287`).
- **Files are read two or three times.** `contentHash` (`:1455`) re-reads files during sync
  (`:296`) and again in `insertConcept` (`:1220`); `documentHashes` (`:409`) and `documentCodecs`
  (`:428`) query the same rows twice.
- **`LoadBundle` decodes every body.** `LoadBundle` (`:541`) selects all `body_compressed` rows;
  `scanConcept` (`:1398`) decompresses each and unmarshals four JSON columns. It feeds `openEngine`
  (`engine/operations/operations.go:352`) for read commands such as `list`, `graph` and `relate`, which
  rarely read `Body`.
- **Search and graph are always built.** `engine.FromBundle` (`engine/engine.go:50`) calls
  `graph.Build` and `search.Build`. `search.Build` (`engine/search/build.go:41`) tokenizes every body
  into postings and trigrams, fills a dead `FieldTokens` map (`:28`) and keeps an unused
  `Document.Body` (`:78`).
- **Codecs allocate per document.** `encodeZstd` (`engine/compression/compression.go:94`) creates a
  writer per body; `Decode` (`:68`) creates a reader per row.
- **Keys repeat YAML I/O.** `privacy.ResolveKey` (`engine/privacy/keys.go:21`) loads `security.yaml`
  per call (`:63`), from `applyLocal` per directive or span (`privacy.go:287`, `:310`) and from
  `ExpandQuery` per alias (`store.go:806`).
- **MCP reloads per request.** `mcp.Server.execute` (`mcp/server.go:354`) runs
  `operations.Execute`, loading SQLite and the bundle each call; stats runs `OpParse` and `OpGraph`
  separately (`:488`, `:498`).
- **The watcher hashes everything.** `snapshotBundle` (`watch/watcher.go:119`) reads and hashes all
  Markdown each 500 ms tick (`:85`, `:142`).

## Candidate workstreams

| ID | Change | Expected impact (estimate) | Effort | Risk |
| --- | --- | --- | --- | --- |
| C1 | Incremental FTS/edge writes | Sync O(changed); CPU/I/O win | L | M |
| C2 | Hash once; parse changed only | I/O/CPU; no reparse | M/L | M-H |
| C3 | Lazy graph/search; slim index | CPU/heap read win | M | M |
| C4 | Metadata-only load; single body | CPU/heap peak win | M | M |
| C5 | Pool zstd codecs | Allocation/CPU win | S | M |
| C6 | Warm MCP bundle/store | CPU/I/O per call | M | M |
| C7 | Stat-first watcher | Idle CPU/I/O win | S | M |
| C8 | Memoize key resolution | CPU/I/O privacy win | S | L |

### C1 - Incremental sync writes

**Hypothesis.** Rebuilding FTS/edges dominates sync. **Change.** Diff first; write FTS rows and
outgoing edges only for changed ids. **Gain (est.).** No-op sync skips decode/strip/reinsert; WAL
scales. **Effort/risk.** L/M. **Measure.** `WEAVE-B-STORE-SYNC-10K-NOOP`, `WEAVE-B-STORE-SYNC-10K-1PCT`,
`WEAVE-B-SPACE-WAL-10K`. **Rollback.** Revert; keep rebuild behind an option.

### C2 - Hash once, parse only changed files

**Hypothesis.** Reparse and repeated hashing dominate sync I/O. **Change.** (a) Pass the parser's
SHA-256 to `insertConcept`; merge hash/codec queries. (b) Hash first, parse changed files
only. **Gain (est.).** Removes duplicate reads and no-op parsing.
**Effort/risk.** M-L/M-H. **Measure.** `WEAVE-B-STORE-SYNC-10K-NOOP`, `WEAVE-B-STORE-SYNC-10K-1PCT`, `WEAVE-B-OKF-PARSE-10K`.
**Rollback.** Split (a)/(b); revert (b).

### C3 - Lazy graph/search views and slim index

**Hypothesis.** Most commands pay for an unused index. **Change.** Build graph/search lazily; delete
unused `Document.Body`/`FieldTokens`; trigrams only for fuzzy/hybrid. **Gain (est.).** Latency/heap
drop; helps the 3x budget. **Effort/risk.** M/M. **Measure.** `WEAVE-B-CLI-LIST-10K`,
`WEAVE-B-MEM-INDEX-10K`, alloc profile. **Rollback.** Revert; eager wrapper stays.

### C4 - Metadata-only load and single-body reads

**Hypothesis.** Eager body decode inflates latency and heap. **Change.** Add `LoadBundleOptions{Body}`
and `LoadBody`; metadata only for `list`/`graph`/`relate`/`path`, one body for `show`, all for
audit/export/hybrid. **Gain (est.).** Medium latency, large heap drop. **Effort/risk.** M/M.
**Measure.** `WEAVE-B-CLI-LIST-10K`, `WEAVE-B-CLI-SHOW-10K`, `WEAVE-B-MEM-INDEX-10K`. **Rollback.** Revert; `BodyAll` default.

### C5 - Pool zstd encoders and decoders

**Hypothesis.** Per-document codec construction dominates allocations. **Change.** Pool writers/readers
with `Reset`/`EncodeAll`/`DecodeAll`; fix options for determinism. **Gain (est.).** Large allocation
drop and ingest-CPU win. **Effort/risk.** S/M. **Measure.** `WEAVE-B-STORE-INGEST-10K`, `WEAVE-B-MEM-ALLOCS-INGEST`.
**Rollback.** Revert to per-call constructors.

### C6 - Warm MCP bundle and store reuse

**Hypothesis.** MCP reloads on every call. **Change.** Cache one store per DB and bundle per root;
invalidate on `updated_at`/`data_version` and watch syncs. **Gain (est.).** Large agent-loop
latency win. **Effort/risk.** M/M (staleness). **Measure.** `WEAVE-B-MCP-WARM`, `WEAVE-B-MEM-INDEX-10K`, stale tests.
**Rollback.** Revert to per-request load.

### C7 - Stat-first watcher snapshots

**Hypothesis.** Hashing all files each tick burns idle CPU. **Change.** Snapshot `(size,mtime)`; hash
stat-changed entries; full hash after sync. **Gain (est.).** Large idle CPU/I/O drop.
**Effort/risk.** S/M. **Measure.** `WEAVE-B-WATCH-TICK`, CPU profile, missed-change tests. **Rollback.**
Revert.

### C8 - Memoize key resolution

**Hypothesis.** Per-directive YAML reads repeat I/O. **Change.** Cache key material per command or
parsed file by `(root,mtime,size)`; check env first. **Gain (est.).** Privacy-path CPU win.
**Effort/risk.** S/L. **Measure.** privacy `WEAVE-B-STORE-INGEST-10K`, syscall counts. **Rollback.** Revert.

## Prioritization matrix

| Candidate | Time | Space | Risk | Priority |
| --- | --- | --- | --- | --- |
| C1 Sync | H | M | M | P0 |
| C2 Parse scope | H | L | M-H | P0 |
| C3 Lazy views | H | H | M | P0 |
| C4 Lazy bodies | M | H | M | P1 |
| C5 Codec pool | H | L | M | P1 |
| C6 MCP warm | H | M | M | P1 |
| C7 Watcher | M | L | M | P2 |
| C8 Key cache | M | L | L | P2 |

## Sequencing

**T1 - Stop duplicate work (1-2 weeks).** C3, C4, C5, C2a. Exit: baseline committed; `WEAVE-B-CLI-LIST-10K`
and `WEAVE-B-MEM-INDEX-10K` measured against it; metadata commands decode no bodies or build no search index
(allocation-profile proof); goldens, contract and calibration unchanged.

**T2 - Make sync incremental (2-3 weeks).** C1, C2b, then C6. Exit: `WEAVE-B-STORE-SYNC-10K-NOOP` under 500 ms and
`WEAVE-B-STORE-SYNC-10K-1PCT` under 2 s on 10k; WAL and FTS work scale with changes; FTS/edge/dangling parity and
MCP invalidation tests pass.

**T3 - Space and long-lived processes (2 weeks).** Compression sweep, `fts_trigrams` decision, C7,
C8. Exit: `WEAVE-B-SPACE-DB-10K` splits documents/FTS/edges; stored bodies under 60%, index memory under 3x
body bytes; watcher cost lower; schema changes migrate.

## Space-specific work

**DB layout.** `documents` and `fts_documents` both hold body text; keep FTS's stripped copy for BM25.
`fts_trigrams` (`store.go:1623`) is never populated or queried: drop by migration. Leave
`frontmatter_json`; the contract needs it.

**Index duplication.** Remove dead `FieldTokens` and `Document.Body`; keep one body text per document;
build trigrams only for fuzzy/hybrid. Measure with `WEAVE-B-MEM-INDEX-10K`.

**Compression tuning.** Sweep `SpeedFastest`, `SpeedDefault` and `SpeedBetterCompression`, plus the
1 KiB threshold, on the body distribution; keep output deterministic; never compress ciphertext.
Measure `WEAVE-B-SPACE-DB-10K` and `WEAVE-B-STORE-INGEST-10K`.

**Postings deferred.** No delta/varint posting compression here. FTS5 owns lexical storage; in-memory
maps stay until a compact representation exists.

## Risks and non-goals

- No ranking, weight, hybrid-formula or calibration changes; `TestCalibration` stays untouched.
- No `unsafe`, assembly, `GOGC` tuning or GC ballast; no dependency without a spec change.
- No stale cache without tested invalidation; no schema change without a
  `user_version` migration, rebuild path and rollback plan.
- No estimate is a result; every slice needs before/after measurement and reverts on >10% regression.

## Acceptance criteria

- `proposal-benchmarks.md`, `cmd/bench` and the baseline JSON exist before implementation.
- Each slice ships benchstat, profiles and a budget movement.
- No-op and 1% sync scale with changes and meet budgets.
- Metadata commands decode no bodies or build no search index; codec allocation is gone in profiles.
- MCP warm state invalidates correctly; watcher ticks avoid hashing unchanged files.
- Database size is reported by table class; body/index budgets are green.
- Goldens, conformance, calibration, race suite and `./scripts/verify.sh` stay green.

## References

- [Optimization specification](../../specs/optimization.md)
- [Storage and sync](../../specs/storage-and-sync.md)
- [BM25 ranking](../../specs/bm25-ranking.md)
- [Compression](../../specs/compression.md)
- `proposal-benchmarks.md` (planned)
- `docs/architecture/performance.md` (planned)
