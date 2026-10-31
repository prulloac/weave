---
type: specification
title: "Release Readiness: Regression, Smoke and Documentation Integrity"
description: "Release gate for v1.0 covering deterministic regression tests, built-binary smoke coverage and documentation integrity."
tags:
  - "specification"
  - "release"
  - "testing"
  - "documentation"
  - "v1"
generated:
  by: opencode/deepseek-v4.1-flash
  at: "2026-10-31T00:00:00Z"
status: stable
---

# Specification: Release Readiness

## 1. Overview

Weave is feature-complete for v1.0. Optimization is reclassified as a **stretch goal**: it may ship
after GA and must not block the release. The release gate is instead defined by two outcomes:

1. **We can prove the shipped binary still works.** A deterministic regression suite plus a
   built-binary smoke suite exercise every command, mode and error boundary from a clean environment.
2. **We can trust the documentation.** The docs describe only shipped behavior, have one source of
   truth per fact, use consistent numbering and status vocabularies, and cross-reference each other
   without dead links, ghost identifiers or contradictory claims.

This specification covers the test gate, the documentation contract and the release workflow that
consumes them. It does not add product features and does not change ranking, storage or the
`weave.cli/v1` contract.

Non-goals:

- Closing the remaining O4/O5 performance budgets.
- New commands, flags, schemas or storage migrations.
- Changing search weights, calibration expectations or privacy semantics.
- A UI, HTTP service, daemon or mount.
- Rewriting every prose document; the work is targeted reconciliation and structural enforcement.

Baseline at approval (the R1–R4 slices close these gaps):

| Area | Baseline |
| --- | --- |
| Local gate | `scripts/verify.sh` runs race, vet, build, gofmt and link checks only; staticcheck, govulncheck, smoke and `actionlint` are absent or run only in CI |
| Smoke | `InvokeBinary` is used only for `version`; no end-to-end built-binary scenario exists |
| Flake resistance | `progress_test.go` sleeps 600–650 ms; watch and sync-watch tests poll wall-clock deadlines |
| Fuzzing | No `func Fuzz` target exists despite parser and graph specs marking fuzzing as planned |
| Coverage | 70.6% CLI-driven; no trend report or release comparison |
| Docs lint | `cmd/docs-lint` checks only that linked `.md` files exist; it ignores anchors, frontmatter, status, numbering and register/backlog parity |
| Docs truth | Roadmap, backlog, specs, ADRs and architecture pages contradict each other on delivery status and numbering |

## 2. Scope and Definitions

- **Regression test:** an automated check that protects an existing observable behavior. It must be
  deterministic, isolated and fast enough for every pull request.
- **Smoke test:** a short end-to-end scenario against the built release binary, from a clean user
  environment, that proves the product starts, ingests, searches, navigates, syncs, exports and
  serves MCP.
- **Documentation integrity:** the mechanical and editorial rules in section 6. It is enforced by
  `cmd/docs-lint` where possible and by a review checklist where judgment is required.
- **Release gate:** the single command sequence that must pass locally, in CI and in the release
  workflow before a tag may publish.

This program is cross-cutting. It owns no production package and must not change runtime behavior
except where a test exposes a real defect.

## 3. Source of Truth

Each fact has exactly one owner. Other documents link to the owner instead of restating it.

| Fact | Authoritative location | Rule |
| --- | --- | --- |
| Feature delivery status | `ROADMAP.md` Feature Register | `shipped`, `stretch`, `deferred` or `planned`; no other document restates delivery status |
| Pipeline phase | `docs/backlog.md` | `backlog`, `in-progress`, `red`, `green`, `review`; `done` is set only by the repository owner |
| Behavioral contract | `docs/specs/*.md` | Present tense for shipped behavior; frontmatter `status` describes document maturity, not delivery |
| Architecture decision | `docs/architecture/adr/*.md` | Accepted ADRs are immutable; a later decision supersedes them |
| Current design | `docs/architecture/**` | Describes what exists today; deferred work lives in the roadmap, not here |
| Task guidance | `README.md` and `docs/guides/**` | User-facing instructions and runnable examples only |
| Behavior examples | `docs/bdd/*.md` | One scenario set per spec; links to the spec and the Go test file |
| Historical intent | `docs/knowledge/**` | Concepts and vision; never a status source |

Status vocabularies:

| Document type | Frontmatter `status` values | Body status line |
| --- | --- | --- |
| Specification | `draft`, `stable`, `deprecated` | Forbidden |
| ADR | `proposed`, `accepted`, `superseded`, `rejected` | Forbidden |
| Architecture or guide | `draft`, `stable` | Forbidden |
| BDD scenario set | `stable` | Forbidden |

A document must not say “Implemented”, “Proposed”, “pending” or “planned” in a body status line.
Delivery state belongs to the roadmap and backlog. Document maturity belongs to frontmatter.

## 4. R1 — Regression Suite

### 4.1 Required suites

Every pull request and release runs:

| Suite | Command | Purpose |
| --- | --- | --- |
| Unit and package | `go test ./...` | Package-level behavior and invariants |
| Race | `go test -race ./...` | Shared-state safety |
| Conformance and goldens | included above | Contract schemas, envelopes, CLI JSON and graph/search goldens |
| Calibration | included above | Keyword, fuzzy, BM25 and hybrid ranking baselines |
| CLI process | included above | `cmd/weave` suites invoke the real dispatch path with isolated HOME, cache and DB |
| Fuzz smoke | `go test -run '^$' -fuzz ... -fuzztime=10s` | Parser and link robustness; longer runs are scheduled, not per-PR |
| Coverage trend | `go test -coverpkg=./... -coverprofile` | Report total and per-package coverage; no percentage gate |

### 4.2 Determinism

Tests must not depend on wall-clock sleeps, network access, the developer's HOME, a shared database
path or execution order.

- Every test that needs a home, cache, config or database gets its own `t.TempDir()` and explicit
  environment overrides.
- No test reaches the public network. Extension and AI tests use local fixture subprocesses.
- No assertion polls with `time.Sleep` when a channel, callback or bounded condition is available.
  Where a watcher needs time, the interval is injected and the test waits on an event with a
  diagnostic deadline.
- Tests that install process-wide environment variables must not run in parallel with tests that
  read them.
- The gate runs with `-count=1 -shuffle=on` so cache and order dependence are visible.
- The gate sets an explicit `-timeout` and fails on leaked goroutines or child processes where the
  package already provides cleanup hooks.

### 4.3 Flake policy

- A test that fails once on `main` is quarantined or fixed before the next release tag; rerunning
  until green is not an accepted release practice.
- Known timing tests must be converted to injected intervals or event channels, not given larger
  sleeps.
- The release candidate is run three times with the full gate; all three runs must pass.

### 4.4 Fuzzing

Add fuzz targets for:

- frontmatter extraction and validation,
- inline link parsing and resolution,
- concept ID derivation.

Each target seeds the existing golden corpus, runs for at least 10 seconds in the normal CI gate,
and runs longer in a scheduled job. A crashing input becomes a committed regression fixture.

### 4.5 Coverage

Coverage is a trend, not a gate. The release report records total coverage, `cmd/weave` process
coverage and the packages with the lowest coverage. Coverage must not fall below the baseline
recorded at the start of this program without a written explanation in the release notes.

## 5. R2 — Smoke Suite

### 5.1 Invocation

The smoke suite builds the real binary with release flags (`CGO_ENABLED=0`, `-trimpath`,
`-ldflags "-s -w -X main.version=<version>"`) and runs it from a clean temporary HOME. It is a Go
test in `cmd/weave` so it works on Linux, macOS and Windows, and it accepts an external binary path
for release-artifact verification.

```sh
go test ./cmd/weave -run TestSmoke -count=1 -v
WEAVE_SMOKE_BINARY=/path/to/weave go test ./cmd/weave -run TestSmoke -count=1 -v
```

### 5.2 Scenario matrix

The suite covers the following in one linear scenario, with a fresh bundle and database:

| Step | Commands and assertions |
| --- | --- |
| Start | `version --json`, `contract --json`, top-level `help`, `help search` |
| Configuration | `config init`, `config show`, `config get`, `config paths`, `completion bash` |
| Ingest | `ingest --strict`, `ingest --permissive`, stored report and document counts |
| Search | `search` in `keyword`, `fuzzy`, `hybrid` and `bm25` modes; `--top-k`; JSON schema validation |
| Explain | `explain` for keyword, BM25 and hybrid; evidence fields present |
| Navigate | `list`, `show`, `backlinks`, `path`, `relate`, `audit` |
| Sync | no-op sync, modified file, deleted file, rename, report counts |
| Multi-source | `--source` search and ingest; qualified `show`; source attribution |
| Privacy | encrypt, tokenize, hash; `show --decrypt`; audit key warning |
| Extension | discovery, enrichment, rerank and timeout with local fixture executables |
| Export | raw with confirmation, encrypted with a key, re-ingest of the encrypted export |
| MCP | `--stdio` initialize, `tools/list`, `weave_search`, `weave_explain`, `weave_audit`, `resources/list`, redacted default |
| Errors | unknown command, unknown flag, missing bundle, unindexed bundle, missing key and invalid input; assert exit codes and remediation hints |
| Parity | the same representative query returns equivalent results with `--db` and `--no-db` |

### 5.3 Artifact smoke

The release workflow additionally:

1. extracts each archive,
2. checks the archive contains exactly the expected binary, `README.md` and `LICENSE` when a
   license exists,
3. runs the extracted binary with `version --json` and `contract --json`,
4. verifies `checksums.txt` and `buildinfo.txt`,
5. runs the Go smoke suite with `WEAVE_SMOKE_BINARY` pointing at the extracted Linux binary.

The artifact smoke runs on Linux, macOS and Windows for the architectures in the release matrix.
The tag build remains blocked if any target fails.

### 5.4 Runtime and isolation

- Target: under 90 seconds on a laptop and under 3 minutes on CI.
- No network, no real user config, no shared database, no leftover processes.
- Every temporary directory is removed by `t.Cleanup`.
- The suite prints the failing step and the full stdout/stderr for the command that failed.

## 6. R3 — Documentation Integrity

### 6.1 Information architecture

The documentation entry point is `docs/README.md`. It links, at minimum:

- getting started and task guides,
- the CLI and configuration reference,
- specifications,
- behavior scenarios,
- architecture and ADRs,
- the roadmap and backlog,
- contributing and the documentation style guide.

Add and maintain indexes for each collection:

| Index | Contents |
| --- | --- |
| `docs/README.md` | Audience-based reading paths and every top-level document |
| `docs/specs/README.md` | Every spec, its owner feature, its status and its BDD set |
| `docs/bdd/README.md` | Every scenario set, its spec and its Go test file |
| `docs/architecture/adr/README.md` | Every ADR, its status and the decision it records |

A document that no index reaches is an orphan and fails the docs gate.

### 6.2 Numbering

- **ADR:** files are `NNNN-kebab-title.md`, numbered contiguously from `0001`. The number in the
  filename, the title, the index and every reference must match. A superseded ADR keeps its number
  and is marked `superseded`.
- **Feature:** register IDs are contiguous from `1` and sorted ascending. A removed feature is
  renumbered; no gaps and no out-of-order rows.
- **Execution order:** every `Phase` and `Wave` referenced by the register exists as a heading or
  is removed from the vocabulary. Phase and wave numbers are not mixed in the same column.
- **Milestone:** MCP milestone IDs are defined once and referenced elsewhere; a partially delivered
  milestone is split or marked honestly.
- **Anchors:** every `#fragment` resolves to a real heading, including GitHub slug rules.

### 6.3 Cross-reference rules

- Every Feature Register row links one authoritative spec and one backlog row.
- Every backlog row maps to one register row, or is explicitly listed as cross-cutting.
- Every spec is owned by at least one register feature and appears in the spec index.
- Every BDD scenario set names its spec and the Go test file that implements it.
- Every ADR appears in the ADR index and is linked from the architecture page it affects.
- Every architecture page describes the current design; deferred work is linked, not restated.
- Every CLI example in `README.md` is checked by the smoke suite or a help test.
- Every error code listed in `weave contract --json` has a row in the error table and a test.

### 6.4 Content rules

- Write in present tense for shipped behavior. Do not describe a shipped feature as “will”,
  “planned”, “proposed” or “pending”.
- Keep future work in one roadmap section and one backlog row; reference docs link to them.
- Remove retired commands, frameworks and product surfaces from user-facing docs. Keep a short
  non-goals note where historical context is genuinely useful.
- Use one vocabulary: “bundle”, “document”, “concept”, “source”, “database”, “in-memory”.
- Every document opens with one sentence of purpose and names its audience.
- Prefer short paragraphs, tables and runnable examples over internal phase history.
- No duplicate status tables. Link the owner.
- No unexplained IDs such as M1, Phase 3 or feature 28 without a definition or link.

### 6.5 Automated documentation rules

Extend `cmd/docs-lint` with stable rule IDs and failing fixtures:

| Rule | Check |
| --- | --- |
| `DL001` | Markdown link target exists |
| `DL002` | Fragment target exists in the destination heading |
| `DL003` | Specs and ADRs have required frontmatter fields |
| `DL004` | Frontmatter status belongs to the allowed vocabulary for the document type |
| `DL005` | No body `Status:` line in specs, ADRs, architecture or BDD |
| `DL006` | ADR numbers are contiguous and filenames match titles |
| `DL007` | Feature register IDs are contiguous and sorted |
| `DL008` | Register and backlog rows are one-to-one |
| `DL009` | Every spec has a register owner and a BDD set, or an explicit exemption |
| `DL010` | Every document is reachable from an index |
| `DL011` | Referenced ADR, spec, phase, wave and milestone IDs exist |
| `DL012` | Code fences are balanced |

The linter prints the rule ID, file, line and a remediation hint. It exits non-zero on any
violation. Each rule has at least one fixture that must fail and one that must pass.

## 7. R4 — Release Gate

`scripts/verify.sh` becomes the single gate. It runs, in order:

1. `go mod tidy` and a clean `go.mod`/`go.sum` diff.
2. `go run ./cmd/contract-export` and a clean golden diff.
3. `go run ./cmd/schema-diff`.
4. `go test -count=1 -shuffle=on -race -timeout=15m ./...`.
5. `go vet ./...`.
6. staticcheck and govulncheck.
7. `CGO_ENABLED=0 go build` with release flags.
8. `go test ./cmd/weave -run TestSmoke -count=1`.
9. `go run ./cmd/docs-lint`.
10. `gofmt -l .` and `actionlint`.
11. Markdown lint, run by CI or by the documented Node-based command.

CI calls `scripts/verify.sh` instead of duplicating steps. The release workflow calls the same
script, then runs the artifact smoke and publishes a draft release. A local `make verify` or
`./scripts/verify.sh` must match CI.

The release checklist records:

- [ ] License decided and included in archives when applicable.
- [ ] Version and contract versions frozen.
- [ ] Full gate green three times on the release candidate.
- [ ] Smoke suite green against the extracted artifacts.
- [ ] Draft release reviewed: notes, checksums, buildinfo, all six archives.
- [ ] No `v*` tag or release exists for the version.
- [ ] Publishing is manual.

## 8. Data Flow

```text
developer -> scripts/verify.sh -> unit/race/smoke/docs -> CI
pull request -> CI -> scripts/verify.sh -> required checks
tag -> release.yml -> scripts/verify.sh -> build matrix -> package -> artifact smoke -> draft release
```

Documentation flow:

```text
ROADMAP (status) --links--> specs (contract) --links--> BDD (behavior) --links--> Go tests
      |                          |                          |
      +--> backlog (pipeline)    +--> ADRs (decisions)       +--> docs/specs/README.md
      |                          |                          |
      +--> docs/README.md <------+--> architecture (current) |
```

## 9. Validation and Errors

| Condition | Behavior |
| --- | --- |
| Regression failure | Gate fails, no tag, no release |
| Smoke step failure | Gate prints step, command, stdout and stderr; no tag |
| Artifact smoke failure | Release job fails; draft remains for inspection |
| Docs rule failure | Gate prints rule ID, file and line; no merge |
| Flaky test | Treated as a failure; fix or quarantine before the tag |
| Missing LICENSE | Artifact smoke fails; MIT requires the notice in distributions |
| Existing tag or release | Release workflow refuses to overwrite |
| Missing benchmark corpus | Stretch optimization only; does not block the release gate |

## 10. Testing the Gate

- `cmd/docs-lint` has table-driven tests for every rule, including a fixture that must fail.
- The smoke suite has a self-test that points at a deliberately broken fixture binary and asserts a
  non-zero, diagnostic failure.
- `scripts/verify.sh` is exercised by CI on every pull request.
- The release workflow is dry-run with `workflow_dispatch` before the first tag.
- The three-run release-candidate check is recorded in the release PR or checklist.

## 11. Acceptance Criteria

1. `./scripts/verify.sh` passes from a clean checkout and is the only command CI and the release
   workflow need for the functional gate.
2. `go test -count=1 -shuffle=on -race ./...` passes three consecutive times on the release
   candidate.
3. The smoke suite covers every shipped command, every search mode, every error-code class and the
   MCP stdio handshake against the built binary.
4. The artifact smoke passes on Linux, macOS and Windows release archives.
5. `cmd/docs-lint` enforces `DL001`–`DL012` with fixtures and reports zero violations.
6. There are no dead links or anchors; every document is reachable from an index.
7. ADR numbers, feature IDs, phases, waves and milestones have no gaps or dangling references.
8. No spec, ADR or architecture page contains a body status line or contradicts the roadmap.
9. Every shipped feature is described in present tense; deferred work appears only in the roadmap
   and backlog.
10. Every register row maps to one spec and one backlog row; every spec maps to a BDD set and a Go
    test file.
11. Fuzz targets exist for frontmatter, links and concept IDs and run in CI.
12. Coverage is reported and does not fall below the recorded baseline without an explanation.
13. A `workflow_dispatch` release dry run produces the full matrix, archives, checksums and build
    info without creating a release.
14. The MIT license is included in the repository and each archive; the release checklist is complete
    before the first tag.

## 12. Delivery Slices

| Slice | Scope | Depends on |
| --- | --- | --- |
| R0 | Approve this spec and reclassify optimization as stretch | owner |
| R1 | Documentation inventory, indexes and style guide | R0 |
| R2 | Extend `cmd/docs-lint` with rules and fixtures | R1 |
| R3 | Reconcile roadmap, backlog, specs, ADRs and architecture; fix all violations | R2 |
| R4 | Add smoke suite and wire it into the gate | R0 |
| R5 | Harden regression tests: injected time, shuffle, fuzz, coverage report | R4 |
| R6 | Unify `verify.sh`, CI and release workflow; dry-run the release | R3, R5 |
| R7 | License decision, release checklist, tag and publish | R6 |

Each slice is an atomic conventional commit or a stacked PR. R1–R3 may ship as one documentation
stack; R4–R5 as one test stack; R6–R7 as the release stack.

## 13. Decisions

| Decision | Outcome |
| --- | --- |
| License | Adopted: MIT; `LICENSE` is included in release archives |
| ADR numbering | Adopted: `0001`/`0002` contiguous, ADR index added, references updated |
| Feature IDs | Adopted: register contiguous from 1 and sorted ascending |
| Phase and wave axes | Adopted: phases 1–6 remain; wave 7–8 references use the wave names |
| Optimization | Adopted: post-GA stretch goal; no v1.0 gate |
| MCP M4 polish | Adopted: rate limits, metrics and service examples deferred until after GA |
| Markdown lint | Adopted: Node action in CI and release verification; `verify.sh` stays Go-first |

## 14. Related Documents

- [Release checklist](../release-checklist.md) — current v1.0.0 preparation state
- [Optimization program](./optimization.md) — stretch performance work
- [Release Automation](./release-automation.md) — tag, build matrix and draft release
- [Configuration](./configuration.md) — paths and provenance used by the smoke suite
- [Storage & Sync](./storage-and-sync.md) — ingest, sync and database behavior
- [MCP Server](./mcp-server.md) — stdio/TCP smoke handshake
- [Extension Subprocess IPC](./extension-ipc.md) — local fixture extensions
- [Architecture](../architecture/index.md) — current design and layer map
- [Roadmap](../../ROADMAP.md) — feature status
- [Backlog](../backlog.md) — pipeline status
