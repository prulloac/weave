# Weave Benchmarks

Deterministic corpus, package benchmarks and the end-to-end `cmd/bench` harness.

## Corpus

```sh
go run ./benchmarks/corpus/gen -out benchmarks/corpus
go run ./benchmarks/corpus/gen -out benchmarks/corpus -verify
```

Profiles: `100`, `1k`, `10k`, `dirty-10k`. Generated bundles are gitignored; the committed
`corpus/manifest.json` pins counts, body bytes, edges, keywords and a SHA-256. Generation uses seed
`20261031` and `math/rand/v2`.

## Registry and queries

- `registry.json` is the single source for benchmark IDs, runner, selector, tier, aggregate, unit
  and budget.
- `ids.go` exports the same IDs to Go callers.
- `queries.json` contains 40 queries across keyword, fuzzy, hybrid and BM25.
- `schema/bench-v1.json` validates the `cmd/bench -json` report.
- `go test ./benchmarks/... ./cmd/bench` checks registry uniqueness, ID coverage, query shape and
  report schema.

## Harness

```sh
go build -o /tmp/weave ./cmd/weave
go run ./cmd/bench -corpus benchmarks/corpus/10k -scenario all \
  -samples 1 -weave /tmp/weave -baseline= -json
```

Scenarios: `parse`, `ingest`, `sync`, `search`, `explain`, `audit`, `export`, `space`.
The CLI family (`search`, `explain`, `audit`, `export`) shares one prepared database and runs
together; `ingest` also prepares the sync scenarios.
`-baseline benchmarks/baseline/10k.json` compares blocking IDs and fails on a >10% regression.
`-markdown` prints the report table; `-profile` writes a CPU profile.

## Baselines

- `baseline/10k.json` is the committed end-to-end baseline.
- `baseline/micro.txt` is the raw `go test -bench` capture.
- Refresh only through a `perf:` PR. The advisory workflow is
  `.github/workflows/perf.yml`; the blocking gate is B2.

## Reports

[Performance baseline](../docs/architecture/performance.md) records the current numbers, budgets
and findings for O4/O5.
