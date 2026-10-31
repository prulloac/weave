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
| `WEAVE-B-OKF-PARSE-10K` | p95 | 0.237 s | 20 s | pass |
| `WEAVE-B-OKF-PARSE-10K-COLD` | p95 | 0.237 s | 20 s | pass |
| `WEAVE-B-OKF-PARSE-10K-WARM` | p95 | 0.291 s | — | baseline |
| `WEAVE-B-STORE-INGEST-10K` | p50 | 8.088 s | 30 s | pass |
| `WEAVE-B-STORE-LOAD-10K` | p50 | 821.379 ms | — | baseline |
| `WEAVE-B-STORE-BM25-10K` | p95 | 60.063 ms | — | baseline |
| `WEAVE-B-STORE-EXPLAIN-10K` | p95 | 166.032 ms | — | baseline |
| `WEAVE-B-STORE-SYNC-10K-NONE` | p50 | 0.148 s | — | baseline |
| `WEAVE-B-STORE-SYNC-10K-1PCT` | p50 | 1.237 s | 2 s | **pass** |
| `WEAVE-B-CLI-VERSION-COLD` | p95 | 4.646 ms | 50 ms | pass |
| `WEAVE-B-CLI-SEARCH-KEYWORD-10K` | p95 | 58.721 ms | 80 ms | pass |
| `WEAVE-B-CLI-SEARCH-FUZZY-10K` | p95 | 179.934 ms | 120 ms | **fail** |
| `WEAVE-B-CLI-SEARCH-BM25-10K` | p95 | 74.491 ms | 150 ms | pass |
| `WEAVE-B-CLI-SEARCH-HYBRID-10K` | p95 | 143.758 ms | 180 ms | pass |
| `WEAVE-B-CLI-EXPLAIN-10K` | p95 | 397.159 ms | 150 ms | **fail** |
| `WEAVE-B-CLI-AUDIT-10K` | p95 | 0.992 s | 5 s | pass |
| `WEAVE-B-CLI-EXPORT-RAW-10K` | p95 | 1.391 s | 30 s | pass |

## Microbenchmarks

`benchmarks/baseline/micro.txt` is the raw `-benchtime=1x -benchmem` capture.

| Benchmark | Time | Allocated |
| --- | ---: | ---: |
| `ParseCorpus/corpus=1k/cache=off` | 55.7 ms | 43.8 MB |
| `ParseCorpus/corpus=1k/cache=on` | 28.9 ms | 14.3 MB |
| `ParseCorpus/corpus=10k/cache=off` | 235.3 ms | 382.9 MB |
| `ParseCorpus/corpus=10k/cache=on` | 202.5 ms | 144.2 MB |
| `SearchCorpus/mode=keyword` | 31.9 ms | 25.7 MB |
| `SearchCorpus/mode=fuzzy` | 224.7 ms | 49.6 MB |
| `SearchCorpus/mode=hybrid` | 233.3 ms | 49.6 MB |
| `SearchBuild10K` | 1.50 s | 893.4 MB |
| `Navigation10K/list` | 3.6 ms | 5.7 MB |
| `Navigation10K/show` | 0.06 ms | 1.1 KB |
| `Navigation10K/backlinks` | 0.05 ms | 0.9 KB |
| `Navigation10K/relate` | 0.11 ms | 49.6 KB |
| `IngestCorpus` | 10.04 s | 19.2 GB |
| `LoadCorpus` | 882 ms | 425 MB |
| `BM25Corpus` | 89.1 ms | 7.8 KB |
| `FuzzyHitsCorpus` | 134.2 ms | 35.9 MB |
| `SyncNoChangeCorpus` | 0.169 s | 44.2 MB |
| `Sync1PctCorpus` | 1.279 s | 693.3 MB |

## Space and memory

| Metric | Baseline | Budget | Status |
| --- | ---: | ---: | --- |
| SQLite database | 84.465 MB | — | baseline |
| Documents table | 17.230 MB | — | baseline |
| FTS tables | 47.195 MB | — | baseline |
| Edges table | 2.082 MB | — | baseline |
| Free pages | 0.508 MB | — | baseline |
| WAL after close | 0.000 MB | — | baseline |
| Stored / raw body ratio (zstd) | 0.245 | < 0.60 | pass |
| Release binary | 14.485 MB | < 25 MB | pass |
| Index / body ratio | 3.411× | < 3.0× | **fail** |
| Peak RSS during ingest | 451.074 MB | — | baseline |

## Findings for O4/O5

- **O4 — time.** Keyword answers from SQLite metadata in 59 ms. Fuzzy and hybrid use persistent
  token postings (`search_tokens`): hybrid is 144 ms and passes; fuzzy is 180 ms and is within 1.5×
  of its 120 ms budget. Explain uses persistent BM25 and fuzzy evidence: 397 ms, down from ~3.05 s,
  but still above 150 ms. Incremental sync now skips no-ops in 148 ms and applies a 1% change in
  1.24 s; both sync budgets pass.
- **O5 — space.** FTS dominates the database (47 MB of 84 MB); persistent token postings add about
  10 MB. Removing dead body/token projections and dropping eager trigrams cut in-memory index
  overhead from 6.8× to 3.4× body bytes; the 3× budget is close.
- **Keep.** Parse, ingest, BM25, keyword, hybrid, audit, export, version, zstd ratio and binary size
  meet their budgets.

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
| 2026-10-31 | O4 slice 2 | Persistent token postings and compact fuzzy/hybrid; hybrid budget passes |
| 2026-10-31 | O4 slice 3 | Persistent explain evidence; explain 3.05 s → 472 ms |
| 2026-10-31 | O4 slice 4 | Incremental sync; no-op 148 ms, 1% 1.24 s, both budgets pass |

## Related Documents

- [Optimization specification](../specs/optimization.md)
- [Benchmark proposal](optimization/proposal-benchmarks.md)
- [Time/space proposal](optimization/proposal-time-space.md)
- [Benchmark registry](../../benchmarks/registry.json)
- [Benchmark queries](../../benchmarks/queries.json)
