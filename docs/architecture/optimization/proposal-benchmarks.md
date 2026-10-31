---
type: proposal
title: "Benchmark Program for Weave"
description: "Deterministic 100/1K/10K corpus, package benchmarks, end-to-end cmd/bench scenarios, budgets and a 10% CI regression gate."
tags:
  - proposal
  - benchmarks
  - performance
  - optimization
generated:
  by: subagent
  at: "2026-10-31T02:36:51Z"
status: draft
---

# Proposal: Benchmark Program

## Context and goals

The [optimization spec](../../specs/optimization.md) puts measurement first (O1) and gates later
slices on a 10% regression. Weave has tests and calibration, but no reproducible corpus, end-to-end
harness or published numbers; this proposal designs measurement, not optimizations.

Goals: a deterministic corpus and query set; one registry of IDs, tiers and budgets; package
benchmarks plus `cmd/bench`; budgets mapped to IDs; advisory CI then a blocking >10% gate; stable
IDs for O2/O3. Success: every optimization PR reports before/after numbers without changing ranking,
contract or storage semantics.

## Current state

`go test ./...` and `scripts/verify.sh` cover unit, conformance, BDD, CLI and race paths.
Calibration is `engine/search/testdata/calibration.json`: 8 concept documents plus `index.md`,
2,011 bytes total, scored with NDCG@10/MRR@10.

The only Go benchmarks are `BenchmarkParseBundle` and `BenchmarkSearch` in `engine/bench_test.go`:
1,000 files of ~230 bytes, one link, no `keywords` or `description`, so the keyword index is empty.
No `-benchmem`, baseline or CI run exists, and other packages have no benchmarks.

No `benchmarks/` tree, `cmd/bench`, `performance.md` or budget file exists; `ci.yml` never passes
`-bench`, while `testing.md` only promises “budgets documented”.

## Proposed benchmark corpus

Package `benchmarks/corpus` and `benchmarks/corpus/gen`; bundles under
`benchmarks/corpus/{100,1k,10k}` are gitignored; `benchmarks/corpus/manifest.json` and
`benchmarks/queries.json` are committed.

| Corpus | Documents | Bytes | Primary use |
| --- | ---: | ---: | --- |
| 100 | 100 | ~0.3 MiB | smoke test |
| 1k | 1,000 | ~2.8 MiB | parse, microbenchmarks |
| 10k | 10,000 | ~28 MiB | budgets, O2/O3 |

- Frontmatter: `type`; 3–6 `keywords` from a 500-term Zipf vocabulary; 60–160 character
  description; 10% with `tags`/`status`. Body: 1–4 KiB (mean 2.5 KiB) of Markdown over a 2,000-term
  Zipf vocabulary; no clock-derived text.
- Links: 0–5 per document, mean 2.5 (60% inline, 40% `related`); 10% dangle, 5% are orphans.
  `dirty-10k` adds 2% missing keywords and 1% malformed frontmatter.
- Queries: 40 in `benchmarks/queries.json` — 10 single, 10 multi-token, 8 typo, 6 phrase, 3 negative, 3
  no-match — with expected empty/non-empty status.

Determinism: seed `20261031`, `math/rand/v2`, sorted traversal, LF, no map-order iteration. The
manifest pins counts, bytes, edges, keyword cardinality and a SHA-256; `gen -verify` regenerates
twice and fails on drift; CI caches by hash. Generator committed; corpus not.

## Harness design

| Layer | IDs | Tier |
| --- | --- | --- |
| Parser (`engine/okf`) | `WEAVE-B-OKF-PARSE-*` | M/S |
| Graph, search (`engine/graph`, `engine/search`) | `WEAVE-B-GRAPH-*`, `WEAVE-B-SEARCH-*` | M |
| Storage (`storage/sqlite`) | `WEAVE-B-STORE-*` | S |
| End to end (`cmd/bench`) | `WEAVE-B-CLI-*`, `WEAVE-B-SPACE-*`, `WEAVE-B-MEM-*` | E |

Go benchmarks use `b.Loop`, `b.ReportAllocs` and `b.SetBytes`, named
`BenchmarkParse/corpus=10k/cache=cold`; each package gets a `bench_test.go`, with loaders in
`benchmarks/benchutil`. Tiers are M (`-count=10`), S (`-count=3`) and E (20 samples, 2 warmups).
`benchmarks/registry.json` maps ID to runner, selector, tier, aggregate and budget;
`benchmarks/ids.go` exports constants.

`cmd/bench` owns corpus and query paths:

```sh
go run ./cmd/bench -corpus benchmarks/corpus/10k -scenario all -json
```

Scenarios: parse, ingest (zstd/lz4/none), sync (0/1/10%), search (four modes), explain, audit,
export. Flags: `-scenario`, `-samples`, `-weave`, `-baseline`, `-profile`, `-json`. Each scenario
gets a fresh DB; sync rewrites the pinned subset and calls `store.Sync`. CLI scenarios spawn the
binary with `--json`, so p95 includes startup and DB open; missing corpus exits 2, stale manifest 1.

- `MEM-INDEX-10K`: child builds, frees the bundle, GCs and reports the `HeapAlloc` delta; ratio is
  index/body bytes.
- `SPACE-DB-10K`: checkpoint WAL; report file, `-wal`, free pages and a split by documents, edges
  and FTS (`dbstat`, else `page_count`). `SPACE-BINARY` uses release flags;
  `MEM-PEAK-INGEST-10K` reads child `Maxrss`.

## Metrics and budgets

Spec budgets govern; derived targets gate at B2.

| ID | Budget |
| --- | --- |
| `WEAVE-B-CLI-VERSION-COLD` | p95 < 50 ms |
| `WEAVE-B-OKF-PARSE-1K` | p95 < 2 s |
| `WEAVE-B-STORE-INGEST-10K` | p50 < 30 s |
| `WEAVE-B-STORE-SYNC-10K-1PCT` | p50 < 2 s |
| `WEAVE-B-CLI-SEARCH-KEYWORD-10K` | p95 < 80 ms |
| `WEAVE-B-CLI-SEARCH-FUZZY-10K` | p95 < 120 ms |
| `WEAVE-B-CLI-SEARCH-BM25-10K` | p95 < 150 ms |
| `WEAVE-B-CLI-EXPLAIN-10K` | p95 < 150 ms |
| `WEAVE-B-MEM-INDEX-10K` | index / body < 3.0× |
| `WEAVE-B-SPACE-ZSTD-RATIO` | stored / raw < 60% |
| `WEAVE-B-SPACE-BINARY` | binary < 25 MB |

Derived: parse 10k < 20 s, hybrid < 180 ms, audit < 5 s, export < 30 s; DB and peak RSS are
baseline-only until O3. Search p95 is nearest-rank over 40 queries × 5 invocations; long scenarios
use p50/20; cold means no cache.

## CI design

Add `.github/workflows/perf.yml`, pinned to `ubuntu-24.04`, `GOMAXPROCS=4`, `GOGC=100` and the Go
version in `go.mod`. One job runs tiered `go test -bench` into `micro.txt`; one generates the
corpus, builds `weave` and runs `cmd/bench -json`; a gate runs `go tool benchstat` and
`cmd/bench -baseline`, fails on a >10% blocking-ID regression and uploads JSON, `micro.txt`,
`benchstat.txt`, `env.json` and profiles via `actions/upload-artifact`.

B1 is advisory on `main` and `perf`-labeled PRs. B2 makes `perf / gate` required for `engine/**`,
`storage/**`, `cmd/**`, `benchmarks/**` and `go.mod`. Deltas under 5% are noise; CV > 5% baselines
are “noisy” and ungated. Hosted runners vary ±10–20%, so the gate compares medians and repeats
borderline 10–15% deltas. Baselines change only through a `perf:` PR, including runner, Go or corpus
changes; budget 20 min; release runs only the functional gate.

## Baseline and reporting

- `benchmarks/baseline/10k.json` (n=20) and `benchmarks/baseline/micro.txt` are committed at B1.
- `benchmarks/schema/bench-v1.json` validates runner output in `go test` with the jsonschema
  dependency.
- `benchmarks/registry.json` is the single ID/budget source; `report_test.go` checks every ID appears
  once in `performance.md` and every budget has a row.
- `docs/architecture/performance.md` records environment, corpus hash, budget table, micro results,
  DB split, index memory, binary size and history; `cmd/bench -markdown` renders it on refresh.

JSON fields: `schema`, `commit`, `go`, `runner`, `gomaxprocs`,
`corpus{name,sha256,docs,body_bytes,edges}`, and `results[]{id,unit,aggregate,value,samples,budget,status}`.

## Sequencing

| Phase | Scope | Exit criteria |
| --- | --- | --- |
| B0 — local harness | Corpus, registry, benchmarks, `cmd/bench`, draft report | `gen -verify` matches twice; smoke passes; every ID yields a value; runs within 5%; tests green |
| B1 — advisory CI | `perf.yml`, baselines, artifacts, report | Three `main` runs within noise; a PR shows deltas; < 20 min; no required check |
| B2 — blocking gate | Registry gate, 10% threshold, required check, refresh policy | Seeded 15% fails and 5% passes; budgets map to IDs; report regenerates |

## Risks and non-goals

- Hosted-runner noise can exceed 10%; use medians, CV labels and repeats, not a looser gate.
- Corpus changes invalidate history; the manifest hash is part of every baseline.
- Microbenchmarks can be gamed and `cmd/bench` can drift; end-to-end budgets are authoritative,
  `-weave` spawns the real binary, and the smoke test guards the envelope.

Non-goals: MCP, watch, extensions, AI, crypto; cross-machine comparisons; `unsafe`, assembly, GC
tuning; committed corpora; auto-updated baselines; replacing calibration, goldens or BDD.

## Acceptance criteria

- The corpus deterministically generates 100/1K/10K clean bundles and a dirty 10K profile; the
  manifest pins counts, body size and link density.
- Every harness package has benchmarks; `cmd/bench` covers seven scenarios with schema-valid JSON,
  and every budget is in `registry.json` and `performance.md`.
- CI is advisory at B1 and blocking at B2 on >10%; the seeded test proves the gate, with no
  behavior, golden or exit-code changes.

## Hand-off: IDs for the time/space proposal

Time (O2): `WEAVE-B-CLI-VERSION-COLD`; `WEAVE-B-OKF-PARSE-{1K,10K,10K-COLD,10K-WARM}`;
`WEAVE-B-STORE-{INGEST-10K,SYNC-10K-NONE,SYNC-10K-1PCT,LOAD-10K,BM25-10K,EXPLAIN-10K}`;
`WEAVE-B-CLI-SEARCH-{KEYWORD,FUZZY,BM25,HYBRID}-10K`; `WEAVE-B-CLI-EXPLAIN-10K`;
`WEAVE-B-CLI-AUDIT-10K`; `WEAVE-B-CLI-EXPORT-RAW-10K`; `WEAVE-B-COMP-ENCODE-ZSTD-4K`;
`WEAVE-B-SEARCH-BUILD-10K`; `WEAVE-B-NAV-{LIST,SHOW,BACKLINKS}-10K`; `WEAVE-B-RELATE-10K`.

Space (O3): `WEAVE-B-SPACE-DB-10K`; `WEAVE-B-SPACE-DB-{DOCS,FTS,EDGES,FREE,WAL}-10K`;
`WEAVE-B-SPACE-ZSTD-RATIO`; `WEAVE-B-SPACE-BINARY`; `WEAVE-B-MEM-INDEX-10K`;
`WEAVE-B-MEM-PEAK-INGEST-10K`.

## References

- [Optimization spec](../../specs/optimization.md)
- [Maintainability research](./research-maintainability.md)
- [UX research](./research-ux.md)
- [Go testing benchmarks](https://pkg.go.dev/testing#hdr-Benchmarks)
- [benchstat](https://pkg.go.dev/golang.org/x/perf/cmd/benchstat)
- [SQLite DBSTAT](https://www.sqlite.org/dbstat.html)
- [github-action-benchmark](https://github.com/benchmark-action/github-action-benchmark) (runner
  variance; considered, not used for gating)
- [actions/upload-artifact](https://github.com/actions/upload-artifact)
