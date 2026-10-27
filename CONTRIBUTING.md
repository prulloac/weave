# Contributing to Weave

Weave is a Go engine and CLI for keyword and weighted fuzzy search over OKF bundles. There is no
browser UI.

## Prerequisites

- Go 1.27+
- [GitHub CLI](https://cli.github.com/) with the [`gh stack`](https://github.com/dolmen/gh-stack)
  extension for stacked PRs

## Getting Started

```sh
git clone https://github.com/prulloac/weave.git
cd weave
go build -o weave ./cmd/weave
./weave --help
```

## Development Pipeline

We follow a strict **SDD → BDD → TDD** pipeline. Never write implementation code before tests, and
never write tests before the specification.

### Phase 1: Specification (SDD)

1. Write a markdown spec in `docs/specs/<feature-name>.md`.
2. Get the spec approved before proceeding.

### Phase 2: Behavior Testing (BDD)

1. Write Given/When/Then scenarios in `docs/bdd/<feature-name>.md`.
2. Get the BDD cases approved.
3. Encode them as Go tests (contract, engine, or CLI process tests) and confirm they fail.

### Phase 3: Unit Testing and Implementation (TDD)

1. Add Go tests under the relevant package.
2. Implement the minimal code to pass.
3. Refactor with the full suite green.

## Commands

| Command | Action |
| --- | --- |
| `go test ./...` | Unit, contract, conformance and CLI tests |
| `go test -race ./...` | Race suite |
| `go vet ./...` | Static checks |
| `go build ./cmd/weave` | Build the CLI |
| `go run ./cmd/contract-export` | Regenerate golden output |
| `go test ./engine/search -run TestCalibration -v` | Ranking calibration |
| `go run ./cmd/docs-lint` | Check relative Markdown links |
| `./scripts/verify.sh` | Full local gate |

## Code Style

- `gofmt` and `goimports` formatting; tabs.
- Conventional commits: `feat:`, `fix:`, `test:`, `docs:`, `perf:`, `build:`.
- Keep packages one-directional: `contract` → `okf`/`graph` → `search`/`nav` → `operations` →
  `cmd/weave`.
- Do not reintroduce UI, HTTP, Astro, Preact or Playwright dependencies.

## Tests and Calibration

- Golden corpus: `contract/v1/golden`.
- Conformance: `engine/conformance_test.go`.
- Calibration: `engine/search/testdata/calibration.json` and
  `engine/search/calibration_test.go`.
- CLI process tests: `cmd/weave/main_test.go`.

Ranking changes must include a calibration diff. Never lower the baseline to make a failing test
pass without an explicit review.

## Pull Requests

- Keep changes atomic.
- Update the relevant docs and `docs/backlog.md`.
- Run `./scripts/verify.sh` before pushing.
- Use stacked PRs for multi-phase work.
