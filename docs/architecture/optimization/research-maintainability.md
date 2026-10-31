---
type: research
title: "Maintainability of Go CLI and Data Systems"
description: "Evidence and practice for maintaining Go CLI/data systems, applied to Weave."
tags:
  - "research"
  - "maintainability"
  - "go"
  - "optimization"
generated:
  by: subagent
  at: "2026-10-31T02:20:00Z"
status: draft
---

# Research: Maintainability for a Go CLI/Data System

## Executive summary

Weave already defines its maintainability policy in the
[optimization spec](../../specs/optimization.md): a 1,200-line cap, package comments, one error
table per boundary, a shared CLI harness and a dependency-direction check. Enforcement is missing. `storage/sqlite/store.go` is 1,641 lines, `cmd/weave/main.go` 1,308
and `engine/operations/operations.go` 1,136; 9 of 21 packages lack a package comment; CI runs only
race tests, `go vet`, gofmt and docs-lint, with no static analysis beyond vet. At 14,271 non-test
LOC (10,607 test LOC), the fixes are days-scale. Practice points to enforced gates and size-driven splits.

## State of practice (2024-2026)

- **Style is settled.** Google's Go Style Guide and Go Code Review Comments agree on naming, doc
  comments and interface placement; Best Practices rejects files with "many thousands of lines".
- **Linting consolidated.** golangci-lint v2 (`version: "2"`, `linters.default: none`, separate
  `formatters`) wraps go vet, staticcheck, errcheck and depguard; staticcheck remains the
  highest-signal analyzer. `go test` runs only a small vet subset, plus `stdversion`
  since Go 1.27.
- **Supply chain is a pinned tool.** `go get -tool .../govulncheck` (Go 1.24+) pins it through a
  `tool` directive; `-format json` exits 0 even on findings, so CI must use the default format.
- **Toolchain features landed.** Go 1.26 added `go fix` modernizers and `T.ArtifactDir`; Go 1.27
  added `stdversion` and `httptest.NewTestServer`. Process-test coverage works since Go 1.20
  (`go build -cover`, `GOCOVERDIR`, `go tool covdata`).
- **Architecture rules are executable.** depguard, `go-arch-lint` v3 and goarchtest compare imports
  to a declared graph; ADRs are immutable and superseded. Evidence favors size: methods of 24 SLOC
  or fewer are less maintenance-prone (MSR 2022), size thresholds flag defect-prone files better
  than complexity thresholds (QRS 2016), and cognitive complexity is a modest proxy.

## Key findings

1. **Enforcement is the gap.** The line cap, package comments and "one error table per boundary"
   are unchecked: `main.go` spreads exit decisions over `exitCodeFor`, `successExitCode`,
   `isEmptyResult` and `usageError`, while `operations` maps errors twice.
2. **Dispatch is a god function.** `Executor.Execute` chains ten sequential `if request.Op == ...`
   tests over ~250 lines (94-352), mixing dispatch with source and path validation.
3. **One file owns six storage concerns.** `store.go` mixes schema/migrations, ingest and sync
   transactions, scans, FTS query construction, BM25 search/explain and enrichment merging.
4. **The CLI harness is duplicated.** Four invokers and seven per-suite envelope decoders span 21
   `cmd/weave` test files, plus raw-DB helpers (`extensionRaw`, `compressionRaw`).
5. **Dependency direction is prose.** Go forbids cycles only; the `contract -> okf/graph -> search
   -> operations -> cmd/weave` rule is unenforced.
6. **Coverage is unmeasured** although 43% of Go lines are tests; refactors are validated by
   goldens, not executed paths.
7. **Foundations are strong:** versioned `weave.cli/v1` envelopes, deterministic JSON, exit codes
   0/1/2/3, a 17-code error enum and documented config precedence (flag > env > file > default).

## Applicability to Weave

- **Split by responsibility, not line count.** `store.go` becomes `schema.go` (create/migrate),
  `write.go` (ingest/sync/insert), `read.go` (LoadBundle/scanConcept), `fts.go` (query building,
  SearchBM25/ExplainBM25) and `enrichment.go`. `main.go` becomes `flags.go`, `request.go`,
  `output.go`. `operations.go` becomes an operation table plus `search.go`/`explain.go`.
- **Turn `Execute` into a table.** `map[OperationName]func(ctx, request) any` with validation
  middleware removes the branch chain and lets `weave contract` descriptors derive from it.
- **Collapse the test harness.** One `clicase` helper exposing `Invoke`, `Envelope` and `OpenDB`
  replaces duplicated invokers and decoders.
- **Make the dependency rule a test.** `go list -deps` (no new dependency) or depguard, plus the
  failing fixture the spec requires.
- **Extend what works.** Add `-v` levels and `--timings` to the existing envelope instead of
  ad-hoc logs.

## Recommended practices

**Must do**

1. Add golangci-lint v2 with `default: none` and an explicit set: `govet`, `staticcheck`,
   `errcheck`, `ineffassign`, `unused`, `misspell`, `revive`, `depguard`. Pin the version.
2. Add govulncheck as a `go.mod` tool and CI step, default format.
3. Block on the 1,200-line cap and ST1000 in that same job.
4. Add the dependency-direction test with a failing fixture.
5. Track coverage twice: `go test -coverprofile` for packages and `go build -cover` +
   `go tool covdata textfmt` for process tests. No percentage gate.
6. Route each boundary through one error table; test that all 17 contract codes map to one exit code.

**Optional**

- `funlen`/`gocognit` advisory output; `go fix` modernizers and `-shuffle=on`; `testscript`/`txtar`
  and `T.ArtifactDir`; `-v` slog levels and a `weave doctor` command.

## Anti-patterns to avoid

- Mechanical line-count splitting that creates pass-through wrappers.
- `linters.default: all`, which turns every upgrade into a breaking change.
- Interfaces declared next to implementations "for mocking" (define at the consumer, once used).
- Replacing golden/BDD suites with unit tests; goldens are the behaviour contract.
- Editing accepted ADRs or changing JSON fields and exit codes as "cleanup"; the CLI is the API.

## Actionable metrics

| Metric | Target | Tool | Caveat |
| --- | --- | --- | --- |
| Non-test file length | <= 1,200 lines | script + review | split by responsibility |
| Longest function | flag > 80 lines | `funlen` (advisory) | 24 SLOC is the evidence ideal |
| Cognitive complexity | flag > 20 | `gocognit` | triage only, not a gate |
| Package comment | 100% | staticcheck ST1000 | `// Command` allowed for main |
| Dependency violations | 0 | `go list` / depguard | needs a failing fixture |
| Coverage | trend, no gate | `go test -cover`, `covdata` | process tests matter most |

Duplication (`dupl`) and coupling counts are noisy in Go; use them to start conversations, not to
fail builds.

## References

- [Effective Go](https://go.dev/doc/effective_go)
- [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments)
- [Google Go Style: Decisions](https://google.github.io/styleguide/go/decisions)
- [Google Go Style: Best Practices](https://google.github.io/styleguide/go/best-practices)
- [Staticcheck checks](https://staticcheck.dev/docs/checks/)
- [golangci-lint v2 configuration](https://golangci-lint.run/docs/configuration/file/)
- [Go security best practices](https://go.dev/doc/security/best-practices)
- [Coverage for integration tests](https://go.dev/doc/build-cover)
- [Go 1.26 release notes](https://go.dev/doc/go1.26)
- [Go 1.27 release notes](https://go.dev/doc/go1.27)
- [Using go fix](https://go.dev/blog/gofix)
- [go-arch-lint](https://github.com/fe3dback/go-arch-lint)
- [Fitness functions (InfoQ, 2025)](https://www.infoq.com/articles/fitness-functions-architecture/)
- [Architecture Decision Records](https://adr.github.io/)
- [Method size and maintenance (MSR 2022)](https://arxiv.org/pdf/2205.01842)
- [Metric thresholds (QRS 2016)](https://posl.ait.kyushu-u.ac.jp/~kamei/publications/Yamashita_QRS2016.pdf)
- [Cognitive complexity](https://www.sonarsource.com/docs/CognitiveComplexity.pdf)

## Hand-off for proposal agent

1. **CI analysis gate**: golangci-lint v2 core set plus govulncheck tool directive.
2. **Split `storage/sqlite/store.go`** into schema/write/read/fts/enrichment.
3. **Unify the CLI test harness** before touching `main.go`.
4. **Convert `Executor.Execute` to an operation table.**
5. **Split `cmd/weave/main.go`** with byte-identical goldens.
6. **Dependency-direction test** with the spec's failing fixture.
7. **Coverage trend job**, non-blocking, plus an ADR for the gate.
