---
type: report
title: "Weave Performance Baseline"
description: "Deterministic 10k corpus, end-to-end scenarios, microbenchmarks and the initial budget results that drive O4/O5."
tags:
  - "performance"
  - "benchmarks"
  - "optimization"
  - "baseline"
generated:
  by: opencode/deepseek-v4.1-flash
  at: "2026-10-31T00:00:00Z"
status: draft
---

# Weave Performance Baseline

## Status

B0 (local harness) is delivered. B1 (advisory CI) is wired through
`.github/workflows/perf.yml`; it generates the corpus, runs microbenchmarks, builds `weave`, runs
`cmd/bench` and uploads artifacts. The gate is advisory until O4/O5 have moved the failing budgets.

## Environment

| Field | Value |
| --- | --- |
| Runner | `linux/amd64` |
| CPU | AMD Ryzen 9 6900HX |
| Go | `go1.27.3` |
| GOMAXPROCS | 16 (local); CI pins 4 |
| GOGC | 100 |
| Commit | `28046fa` (O4 slice on top) |
| Seed | `20261031` |

## Corpus

The generator lives in `benchmarks/corpus/gen`; generated bundles are gitignored and pinned by
`benchmarks/corpus/manifest.json`.

| Profile | Documents | Body bytes | Edges | Keywords | SHA-256 |
| --- | ---: | ---: | ---: | ---: | --- |
| 100 | 100 | 274,728 | 210 | 149 | `2fb0abf9…f7ffd` |
| 1k | 1,000 | 2,734,807 | 2,116 | 431 | `50f974b1…077c` |
| 10k | 10,000 | 27,269,314 | 21,689 | 500 | `b1e3aaf4…338fe` |
| dirty-10k | 10,000 | 27,269,314 | 21,689 | 500 | `7f01c967…a4a6d` |

`go run ./benchmarks/corpus/gen -out benchmarks/corpus -verify` regenerates twice and compares the
committed manifest.

## Budget results

`n=1` for the 10k baseline on the developer machine; slow CLI scenarios take ~8 s per invocation.
CI refreshes the baseline on the pinned runner with higher sample counts.

| ID | Aggregate | Baseline | Budget | Status |
| --- | --- | ---: | ---: | --- |
| `WEAVE-B-OKF-PARSE-10K` | p95 | 0.218 s | 20 s | pass |
| `WEAVE-B-OKF-PARSE-10K-COLD` | p95 | 0.218 s | 20 s | pass |
| `WEAVE-B-OKF-PARSE-10K-WARM` | p95 | 0.217 s | — | baseline |
| `WEAVE-B-STORE-INGEST-10K` | p50 | 7.573 s | 30 s | pass |
| `WEAVE-B-STORE-LOAD-10K` | p50 | 771.356 ms | — | baseline |
| `WEAVE-B-STORE-BM25-10K` | p95 | 59.207 ms | — | baseline |
| `WEAVE-B-STORE-EXPLAIN-10K` | p95 | 157.093 ms | — | baseline |
| `WEAVE-B-STORE-SYNC-10K-NONE` | p50 | 6.343 s | — | baseline |
| `WEAVE-B-STORE-SYNC-10K-1PCT` | p50 | 5.549 s | 2 s | **fail** |
| `WEAVE-B-CLI-VERSION-COLD` | p95 | 4.165 ms | 50 ms | pass |
| `WEAVE-B-CLI-SEARCH-KEYWORD-10K` | p95 | 52.483 ms | 80 ms | pass |
| `WEAVE-B-CLI-SEARCH-FUZZY-10K` | p95 | 6,568.647 ms | 120 ms | **fail** |
| `WEAVE-B-CLI-SEARCH-BM25-10K` | p95 | 69.830 ms | 150 ms | pass |
| `WEAVE-B-CLI-SEARCH-HYBRID-10K` | p95 | 6,448.245 ms | 180 ms | **fail** |
| `WEAVE-B-CLI-EXPLAIN-10K` | p95 | 7,032.025 ms | 150 ms | **fail** |
| `WEAVE-B-CLI-AUDIT-10K` | p95 | 0.908 s | 5 s | pass |
| `WEAVE-B-CLI-EXPORT-RAW-10K` | p95 | 1.267 s | 30 s | pass |

## Microbenchmarks

`benchmarks/baseline/micro.txt` is the raw `-benchtime=1x -benchmem` capture.

| Benchmark | Time | Allocated |
| --- | ---: | ---: |
| `ParseCorpus/corpus=1k/cache=off` | 33.4 ms | 41.1 MB |
| `ParseCorpus/corpus=1k/cache=on` | 22.3 ms | 14.3 MB |
| `ParseCorpus/corpus=10k/cache=off` | 198.1 ms | 383.9 MB |
| `ParseCorpus/corpus=10k/cache=on` | 190.7 ms | 144.2 MB |
| `SearchCorpus/mode=keyword` | 29.8 ms | 15.2 MB |
| `SearchCorpus/mode=fuzzy` | 197.8 ms | 50.1 MB |
| `SearchCorpus/mode=hybrid` | 196.7 ms | 50.1 MB |
| `SearchBuild10K` | 5.72 s | 1.57 GB |
| `Navigation10K/list` | 3.2 ms | 5.7 MB |
| `Navigation10K/show` | 0.05 ms | 1.1 KB |
| `Navigation10K/backlinks` | 0.05 ms | 0.9 KB |
| `Navigation10K/relate` | 0.11 ms | 49.6 KB |
| `IngestCorpus` | 7.86 s | 19.1 GB |
| `LoadCorpus` | 801 ms | 425 MB |
| `BM25Corpus` | 83.7 ms | 7.8 KB |
| `SyncNoChangeCorpus` | 5.71 s | 1.23 GB |

## Space and memory

| Metric | Baseline | Budget | Status |
| --- | ---: | ---: | --- |
| SQLite database | 74.324 MB | — | baseline |
| Documents table | 17.230 MB | — | baseline |
| FTS tables | 47.195 MB | — | baseline |
| Edges table | 2.082 MB | — | baseline |
| Free pages | 0.508 MB | — | baseline |
| WAL after close | 0.000 MB | — | baseline |
| Stored / raw body ratio (zstd) | 0.245 | < 0.60 | pass |
| Release binary | 14.329 MB | < 25 MB | pass |
| Index / body ratio | 4.617× | < 3.0× | **fail** |
| Peak RSS during ingest | 480.754 MB | — | baseline |

## Findings for O4/O5

- **O4 — time.** Keyword now answers from SQLite metadata in 52 ms; audit dropped to 0.9 s after
  making the search index lazy. Fuzzy, hybrid and explain still rebuild the full in-memory index
  per CLI invocation (~6.5–7.0 s). Sync re-hashes the full 10k tree even when 1% changed.
- **O5 — space.** FTS dominates the database (47 MB of 74 MB). Removing the dead
  `Document.Body`/`FieldTokens` projections cut in-memory index overhead from 6.8× to 4.6× body
  bytes; the 3× budget still needs a compact postings or trigram representation.
- **Keep.** Parse, ingest, BM25, export, version, zstd ratio and binary size already meet their
  budgets.

## Reproduce

```sh
go run ./benchmarks/corpus/gen -out benchmarks/corpus
go run ./cmd/bench -corpus benchmarks/corpus/10k -scenario all \
  -samples 1 -weave /tmp/weave -baseline= -json > benchmarks/baseline/10k.json || true
go test -run '^$' -bench 'BenchmarkParseCorpus|BenchmarkSearchCorpus' \
  -benchtime=1x -benchmem ./engine/okf ./engine/search
```

The CI gate compares blocking IDs against `benchmarks/baseline/10k.json` and fails on a >10%
regression. Baselines change only through a `perf:` PR.

## History

| Date | Commit | Change |
| --- | --- | --- |
| 2026-10-31 | `c31dbc6` | Initial B0 corpus, harness, microbenchmarks and baseline |
| 2026-10-31 | O4 slice | Metadata keyword search, lazy search, slimmer index; keyword and audit budgets pass |

## Related Documents

- [Optimization specification](../specs/optimization.md)
- [Benchmark proposal](optimization/proposal-benchmarks.md)
- [Time/space proposal](optimization/proposal-time-space.md)
- [Benchmark registry](../../benchmarks/registry.json)
- [Benchmark queries](../../benchmarks/queries.json)
