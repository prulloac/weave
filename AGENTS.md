# AGENTS.md

Weave is a local-first related-document database for OKF bundles: a Go CLI, an in-memory engine and
an optional SQLite index. There is no browser UI or HTTP API; `cmd/weave` is the integration
boundary, and MCP is an additional foreground transport. Go 1.27.3+ (see `go.mod`); the
`.devcontainer/` pins Go 1.27. Project documentation (roadmap, backlog, specs, architecture, ADRs,
knowledge and style) lives in the sibling `../weave-docs/` repository; this repo keeps end-user docs,
BDD scenarios and tests.

## Commands

- Build: `go build -o weave ./cmd/weave`
- All tests: `go test ./...`
- One test (example): `go test ./cmd/weave -run TestVersionJSON -v`
- Race: `go test -race ./...`
- Import-direction check: `go test ./internal/archcheck -run TestDependencyDirection -v`
- Vet / format: `go vet ./...`; `gofmt -l .` must print nothing
- Contract package: `go test ./contract/v1`
- Regenerate contract goldens: `go run ./cmd/contract-export`
- Additive schema check: `go run ./cmd/schema-diff` (`-write` only after a reviewed addition)
- Calibration: `go test ./engine/search -run 'TestCalibration|TestBM25Calibration' -v`
- Docs gate (this repo): `go run ./cmd/docs-lint`
- Project docs: see `../weave-docs/AGENTS.md`; run its link and Markdown checks there
- Built-binary smoke: `go test ./cmd/weave -run '^TestSmoke$'`
  (`WEAVE_SMOKE_BINARY` reuses a prebuilt binary; `WEAVE_SMOKE_VERSION` sets the expected version)
- Parser fuzz smoke: `go test ./engine/okf -run '^$' -fuzz '^FuzzExtractFrontmatter$' -fuzztime=5s`
- Corpus: `go run ./benchmarks/corpus/gen -out benchmarks/corpus`
- Benchmark binary: `go build -o /tmp/weave ./cmd/weave`
- Benchmarks: `go run ./cmd/bench -corpus benchmarks/corpus/10k -scenario all -samples 1 -weave /tmp/weave -baseline= -json`
- Full gate: `./scripts/verify.sh`

`verify.sh` is the CI and release gate. It runs `go mod tidy` and fails on `go.mod`/`go.sum` drift,
regenerates contract goldens and fails on drift, checks schemas, runs the race suite with
`-count=1 -shuffle=on`, vet, staticcheck/govulncheck/actionlint (pinned versions downloaded on
demand), three fuzzers, docs-lint and gofmt. It needs bash and network: use WSL or the
`.devcontainer/` on Windows, and run it before pushing rather than after every edit.

## Layout and generated files

- `contract/v1/schema/defs.schema.json` is authoritative; `contract/v1/types.go` is hand-written and
  schema-tested. Change schema → types → fixtures → `contract-export` → review the diff →
  `go test ./contract/v1`.
- `contract/v1/golden/expected/` is generated; never hand-edit it. `golden/schema-baseline.json` is
  also generated (use `go run ./cmd/schema-diff -write` after a reviewed additive change).
- Contract changes are additive inside v1.0. Until GA, `contract/v1` may evolve in place; after GA,
  breaking changes need `contract/v2` / `cli/v2`.
- Golden output is deterministic: POSIX separators, sorted nodes/edges/results, scores rounded to
  four decimals.
- Core: `engine/` (parse, graph, search, nav, audit, relate, privacy, compression, extensions).
  `storage/sqlite/` is the persistent adapter; `engine/operations/` is the transport-neutral layer
  shared by `cmd/weave` and `mcp/`. `watch/` polls incremental sync.
- `docs/` holds end-user documentation and BDD scenarios. Project docs live in `../weave-docs/`
  (`specs/`, `architecture/`, `knowledge/`, `ROADMAP.md`, `backlog.md`, `STYLE.md`,
  `release-checklist.md`).
- Import direction is enforced by `internal/archcheck` (`TestDependencyDirection`): `contract` is a
  leaf; `engine/*` (except `engine/operations`) must not import `storage`, `internal` or transports
  (`cmd`, `mcp`, `watch`); `storage` must not import `operations` or transports; `operations` must
  not import transports; `mcp` must not import `storage`, `watch` or `cmd`; `watch` must not import
  `mcp` or `cmd`. `cmd/weave` composes them.
- Dependencies are deliberately small: stdlib first, plus `yaml.v3`, `jsonschema/v6`,
  `klauspost/compress`, `lz4/v4` and pure-Go `modernc.org/sqlite`; releases are static
  `CGO_ENABLED=0` builds. No UI, HTTP framework or CGO dependency.

## Invariants

1. **Pipeline order:** SDD → BDD → TDD. Write the spec in `../weave-docs/specs/`, get explicit user
   approval, write Given/When/Then scenarios in `docs/bdd/`, get explicit user approval, then write
   failing Go tests, then implement. Never write implementation before tests or tests before the spec.
2. **CLI-first:** no HTTP API, daemon, mount or browser UI. MCP is an additional transport over
   `engine/operations`, not a second product surface.
3. **Documents:** the Weave profile requires `type` and `keywords`; missing keywords invalidate the
   document for keyword search but it remains available for graph/fuzzy. `contract/v1` is
   authoritative; do not invent fields.
4. **Storage:** bundle reads default to the bundle-scoped `~/.weave/index.db`; `--no-db` forces
   transient in-memory parsing and is mutually exclusive with `--db`.
5. **Search:** keyword is exact + prefix with AND semantics; fuzzy weights are title 1.00,
   description 0.60, body 0.30; persistent hybrid is 50/50 BM25 + fuzzy, while `--no-db` keeps
   keyword + fuzzy; default K=10, max 100. `search` replaced `find`. BM25 requires the database.
6. **Calibration:** ranking changes must pass the labeled corpus and must not lower the committed
   baseline without explicit review. See `engine/search/testdata/calibration.json`.
7. **Commits:** atomic conventional commits (`feat:`, `fix:`, `docs(spec):`, `test:`, `perf:`),
   delivered as stacked PRs via `gh stack`.
8. **Backlog:** update `../weave-docs/backlog.md` as phases advance (`backlog` → `in-progress` →
   `red` → `green` → `review` → `done`). Never set `done`; only the user closes merged work.

## Documentation split

- This repo owns end-user docs (`README.md`, `docs/`) and BDD behavior scenarios (`docs/bdd/`).
  Project docs — roadmap, backlog, specifications, architecture, ADRs, knowledge, style and release
  checklist — live in `../weave-docs/`.
- Commit project docs in `../weave-docs/`, not here. A cross-repo change needs a commit and PR in
  each repo; link the spec from the BDD/implementation PR.
- `go run ./cmd/docs-lint` checks links, anchors and the remaining docs here. It no longer checks
  specs, ADRs or the roadmap; run the `weave-docs` checks (`python3 scripts/check-links.py`,
  `npx --yes markdownlint-cli2 "**/*.md"`) from that repo.
- Specs use frontmatter `status: draft|stable|deprecated`; ADRs use
  `proposed|accepted|superseded|rejected`. Never add a body `Status:` line; delivery status belongs
  in `../weave-docs/ROADMAP.md` and `../weave-docs/backlog.md`.
- A new roadmap feature needs a Feature Register row in `../weave-docs/ROADMAP.md` and a backlog row.
  Every spec needs a row in `../weave-docs/specs/README.md`; every behavior set belongs in
  `docs/bdd/README.md`.
- Keep docs reachable from a documentation index; orphan docs fail the lint.
- Style rules: `../weave-docs/STYLE.md`. CI also runs markdownlint
  (`.markdownlint-cli2.jsonc`); `docs-lint` does not cover Markdown style.
- Cross-repo links use absolute GitHub URLs: `https://github.com/prulloac/weave-docs/blob/main/...`
  from here, and `https://github.com/prulloac/weave/blob/main/...` from `weave-docs`.

## Testing quirks

- Most CLI behavior tests run in-process via the shared harness in `cmd/weave/harness_test.go`
  (`Invoke`, `InvokeRead`, `InvokePersistent`); release and smoke tests build the real binary.
  Reuse the harness instead of adding invokers or database openers.
- `TestSmoke` copies fixtures from `cmd/weave/testdata/` and isolates HOME/XDG/WEAVE_* env, so it
  does not touch your real `~/.weave/index.db`.
- `contract/v1/golden/bundles/symlinked-dirs` uses committed symlinks; Windows checkouts need
  Developer Mode (or use WSL).
- Benchmark corpora under `benchmarks/corpus/{100,1k,10k,dirty-10k}` are generated and gitignored.

## Repo skills

These are plain instruction files under `.agents/skills/`, not necessarily registered with the
`skill` tool. Read the matching file before starting.

| Trigger | Skill file |
| --- | --- |
| Feature specification (SDD) | `.agents/skills/spec-writer/SKILL.md` |
| BDD scenarios | `.agents/skills/bdd-tester/SKILL.md` |
| Unit tests / implementation (TDD), Go engine/search | `.agents/skills/tdd-coder/SKILL.md` |
| Commits, stacked PRs, worktrees | `.agents/skills/git-stack-reviewer/SKILL.md` |
| Architecture / data-model docs | `.agents/skills/okf-generator/SKILL.md` |
| Unexpected test failure or broken build | `.agents/skills/systematic-debugging/SKILL.md` |
