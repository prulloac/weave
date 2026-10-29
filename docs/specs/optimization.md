---
type: specification
title: "Performance and Maintainability Optimization Specification"
description: "Continuous optimization program for Weave time, space, startup, maintainability and user experience with benchmarks, budgets and regression gates."
tags:
  - "specification"
  - "performance"
  - "benchmarks"
  - "maintainability"
  - "v1"
generated:
  by: opencode/deepseek-v4.1-flash
  at: "2026-10-29T00:00:00Z"
status: draft
---

# Specification: Performance and Maintainability Optimization

Status: **Proposed**
Target: **v1.0**

## 1. Overview

This is a cross-cutting program, not a user-facing feature. It establishes measurements, budgets and
a repeatable workflow so time, space, startup, maintainability and user experience improve without
changing search ranking, the contract or storage semantics.

The program ships in measured slices; every slice records a before/after number and keeps the full
verification gate green.

Non-goals:

- Changing ranking formulas, weights or calibration expectations.
- Rewriting the engine in another architecture.
- Premature concurrency or worker pools without measurements.
- GC tuning, `unsafe` or assembly hot paths.
- Changing the CLI contract for performance reasons.
- A UI or interactive editor; UX work stays CLI-first.

## 2. Contracts

### 2.1 Benchmark corpus

- `benchmarks/corpus/` generates deterministic bundles of 100, 1,000 and 10,000 documents.
- Each document has frontmatter (`type`, `keywords`), a short description, a 1–4 KiB body, and 0–5
  links to other generated documents.
- Generation is seeded and checked in as a script; the corpus itself is generated on demand and not
  committed.
- A calibration-like manifest pins document counts, average body size and link density.

### 2.2 Benchmark entry points

```sh
go test ./engine/okf -run '^$' -bench . -benchmem
go test ./engine/search -run '^$' -bench . -benchmem
go test ./storage/sqlite -run '^$' -bench . -benchmem
go test ./engine/compression -run '^$' -bench . -benchmem
go run ./cmd/bench -corpus benchmarks/corpus/10k -json
```

- `cmd/bench` drives end-to-end CLI-equivalent flows: ingest, sync, search per mode, explain, audit
  and export.
- Benchmarks must not depend on network, wall-clock time of day or random seeds beyond the pinned
  generator seed.

### 2.3 Budgets

Measured on the standard CI runner with the 10,000-document corpus unless noted:

| Dimension | Budget |
| --- | --- |
| Cold `weave version` | < 50 ms |
| Parse 1,000 documents | < 2 s |
| Ingest 10,000 documents | < 30 s |
| Incremental sync, 1% changed | < 2 s |
| Keyword search, p95 | < 80 ms |
| Fuzzy search, p95 | < 120 ms |
| BM25 search, p95 | < 150 ms |
| Explain, p95 | < 150 ms |
| Search index memory | < 3× total body bytes |
| Stored zstd bodies | < 60% of raw body bytes on the corpus |
| Release binary | < 25 MB |

- Budgets are goals for the program; each optimization slice must move at least one budget or remove
  a listed hotspot.
- A tracked benchmark regressing by more than **10%** fails the performance job once the baseline is
  committed. Before that, the job is advisory and reports deltas.

### 2.4 Maintainability targets

- No Go source file above 1,200 lines; any file that grows past it must be split in the same PR.
- Current hotspots to split: `storage/sqlite/store.go` (schema/read/write/search),
  `engine/operations/operations.go` (dispatch, search, write, extensions) and `cmd/weave/main.go`
  (flags, dispatch, output).
- One shared CLI test harness for envelope decoding, temp bundles and DB inspection replaces
  duplicated per-feature helpers.
- Error mapping lives in one table per boundary (operations, CLI, MCP) instead of scattered switches.
- Dependency direction stays `contract -> okf/graph -> search/nav -> operations -> cmd/weave`;
  `storage/sqlite` may import engine packages but never `cmd` or `mcp`.
- New packages require a package comment and at least one table-driven test.

### 2.5 User experience targets

- Recurring flags must be configurable once: global config file plus `WEAVE_*` environment variables
  (see [Configuration](./configuration.md)).
- Setting up a bundle for daily use takes at most two commands after configuration: `ingest` then
  `search`.
- Every non-zero exit prints an actionable next step, not only the failure.
- `--help` lists global config keys and points to `weave config show`.
- Errors name the offending flag, environment variable or file path.
- No UX change may make JSON output or exit codes less deterministic.

## 3. Behavior

### 3.1 O1 — Measure

- Add benchmarks and `cmd/bench`, generate the corpus, and publish
  `docs/architecture/performance.md` with the baseline table.
- Record CPU, allocations, DB size, index memory and binary size.
- No production behavior changes in this phase.

### 3.2 O2 — Time

Candidates, validated by profiles before implementation:

- Reuse zstd encoders with `sync.Pool` instead of allocating per document.
- Cache key resolution per bundle root and per command instead of reading `security.yaml` per concept.
- Merge `documentHashes` and `documentCodecs` into one query.
- Update FTS rows only for changed documents during sync instead of rebuilding the whole bundle.
- Decode bodies lazily for `list`, `audit`, `graph` and `relate`, which never need bodies.
- Avoid re-parsing the bundle for enrichment by passing the parsed concepts through.
- Reduce CLI startup work: defer DB and cache setup until the command needs it.

Each candidate PR includes `benchstat`-style before/after numbers and is reverted if it regresses a
tracked budget beyond the 10% gate.

### 3.3 O3 — Space

- Tune zstd level and window for the measured document size distribution; keep determinism.
- Remove duplicate index rows where FTS5 already covers a field.
- Evaluate compact postings (delta + varint) only after the search index refactor; posting compression
  stays out of scope until then.
- Report database size split by documents, FTS and edges in `cmd/bench`.
- Never trade correctness or privacy for size: ciphertext stays uncompressed and hashes stay salted.

### 3.4 O4 — Maintainability

- Split hotspot files by responsibility without behavior changes; goldens and BDD suites must stay
  unchanged.
- Consolidate CLI test helpers and error-code assertions.
- Add a dependency-direction check to CI (forbidden imports between packages).
- Keep `go vet`, gofmt, docs-lint and markdownlint in the required gate.
- Track TODO/FIXME density and package count in the performance report; no hard gate in v1.0.

### 3.5 O5 — User experience

The first slice is **global configuration**: a YAML file plus `WEAVE_*` environment variables with
deterministic precedence and `weave config show|path|init`, specified in
[Configuration](./configuration.md).

Subsequent UX candidates, ordered by measured friction:

- Replace repeated flags in docs and examples with configuration-driven flows.
- Add remediation hints to common errors (`BUNDLE_NOT_INDEXED`, `PRIVACY_KEY_UNAVAILABLE`,
  `EXTENSION_NOT_FOUND`, `CONFIG_INVALID`).
- Teach configuration in `--help` and shell completions.
- Improve first-run behavior: empty database, missing `.weave` directory and missing extension
  messages.
- Coherent human output spacing and summary lines without breaking JSON.
- Defer shell completion generation to a follow-up if it needs a new dependency.

UX changes are validated by CLI tests and docs updates; they never alter the contract or default
behavior for existing flags.

### 3.6 O6 — Continuous gate

- Add a non-blocking GitHub Actions performance job on `main` that runs `cmd/bench` and uploads
  results.
- Commit the baseline JSON; the job fails on >10% regressions for tracked benchmarks.
- PRs touching `engine/`, `storage/` or `cmd/` may request the perf label to run the comparison.
- Release workflow (see [Release Automation](./release-automation.md)) runs the functional gate, not
  the full benchmark, to keep release time bounded.

## 4. Data Flow

```text
benchmarks/corpus generator
  -> cmd/bench (ingest, sync, search, explain, audit, export)
  -> baseline JSON + performance.md
  -> CI perf job compares against baseline
  -> optimization PRs move budgets and split hotspots
```

## 5. Validation and Errors

| Condition | Behavior |
| --- | --- |
| Benchmark corpus missing | `cmd/bench` fails with usage exit `2` |
| Trace/profile unavailable | Report partial metrics, no failure |
| Regression >10% on a tracked benchmark | Perf job fails |
| Behavior change detected by goldens/BDD | PR blocked by the full gate |
| New file above 1,200 lines | Review blocker; split in the PR |

## 6. Testing

- Benchmark results are deterministic within a tolerance for the pinned corpus and Go version.
- `cmd/bench` smoke test on the 100-document corpus in `go test`.
- Baseline JSON schema validated.
- Goldens, conformance, calibration and BDD suites unchanged by refactors.
- Dependency-direction check has a failing fixture and a passing repository run.
- Configuration precedence, `config show/path/init` and remediation messages covered by CLI tests.
- UX changes keep goldens and JSON output stable.
- Race suite green for every optimization slice.

## 7. Acceptance Criteria

- Benchmarks, baseline and budgets are published and reproducible.
- Every shipped optimization slice reports before/after measurements and moves at least one budget
  or removes a listed hotspot.
- `storage/sqlite/store.go`, `engine/operations/operations.go` and `cmd/weave/main.go` are split
  below the line limit with no behavior change.
- The CLI test harness is shared across feature suites.
- Global configuration and environment variables replace recurring flags, with visible precedence and
  actionable errors.
- Ranking calibration, contract goldens and the full verification gate stay green.
- The performance CI job fails on regressions above 10% once the baseline is committed.

## Related Documents

- [Storage & Sync](./storage-and-sync.md)
- [BM25 Ranking](./bm25-ranking.md)
- [Compression](./compression.md)
- [Extension Subprocess IPC](./extension-ipc.md)
- [Release Automation](./release-automation.md)
- [Configuration](./configuration.md)
- [Architecture](../architecture/index.md)
- [ROADMAP](../../ROADMAP.md)
