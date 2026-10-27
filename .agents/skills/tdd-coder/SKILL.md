---
name: tdd-coder
description: Executes Test-Driven Development in Go using the standard testing package, contract goldens, CLI process tests and search calibration.
---

# Test-Driven Development (TDD) & Implementation Protocol

You are in **Implementation Engineer Mode**. Write failing Go tests, implement the minimal code,
and keep the conformance and calibration suites green.

## 1. Input Handoff & Contract Retrieval

- Read the approved spec in `docs/specs/<feature-id>.md`.
- Read `contract/v1/schema` and `contract/v1/types.go`.
- Read the relevant behavior tests in `docs/bdd/` and the existing Go tests.

## 2. Red Phase

Write tests in the package that owns the behavior:

- `contract/v1/types_test.go` for schema/type contracts.
- `engine/okf/*_test.go` for parsing, links and cache.
- `engine/conformance_test.go` for bundle/graph/search goldens.
- `engine/search/*_test.go` for ranking and calibration.
- `cmd/weave/main_test.go` for CLI envelopes and exit codes.

Run:

```sh
go test ./<package> -run <Feature>
```

and confirm failure.

## 3. Green Phase

Implement the minimal Go code to pass. Respect the dependency direction:

```text
contract -> okf/graph -> search/nav -> operations -> cmd/weave
```

Do not add UI, HTTP, Astro, Preact or Playwright dependencies.

## 4. Verification

```sh
go test ./...
go test -race ./...
go vet ./...
gofmt -l cmd contract engine
go run ./cmd/docs-lint
./scripts/verify.sh
```

Ranking changes must also pass `TestCalibration` and must not lower the committed baseline.

## 5. Refactor

Clean imports and names without changing behavior. Keep results deterministic and golden output
stable.

## Hand-off

Report:

- package tests green,
- conformance green,
- calibration baseline met,
- CLI process tests green,
- `./scripts/verify.sh` green.

Then hand off to `git-stack-reviewer` for atomic commits and stacked PRs.
