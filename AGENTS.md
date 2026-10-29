# AGENTS.md

## Topology

Go · GitHub Actions · GitHub (`gh stack`). Weave is a local-first related-document database with a
CLI-first surface and no UI. The core is in-memory; v1.0 adds optional SQLite persistence.

## Commands

- Build: `go build -o weave ./cmd/weave`
- Unit/conformance/CLI tests: `go test ./...`
- Race suite: `go test -race ./...`
- Vet: `go vet ./...`
- Regenerate goldens: `go run ./cmd/contract-export`
- Calibration: `go test ./engine/search -run TestCalibration -v`
- Docs link check: `go run ./cmd/docs-lint`
- Markdown lint (CI): `DavidAnson/markdownlint-cli2-action`
- Full gate: `./scripts/verify.sh`

## Architectural Invariants

1. **Strict pipeline order:** SDD → BDD → TDD. Never write implementation code before tests; never
   write tests before the specification.
2. **Approval gates:** Never advance a pipeline phase without explicit user approval of its
   artifact (spec, then BDD, then implementation).
3. **CLI-first:** `cmd/weave` is the integration boundary. No HTTP, daemon, mount or browser UI.
4. **Data contract:** `contract/v1` is authoritative. Documents require `type` and `keywords`.
   JSON Schema drives hand-written Go structs and goldens.
5. **Storage:** bundle reads use a bundle-scoped `~/.weave/index.db` by default. `--no-db` forces
   transient in-memory parsing; `--db <file>` overrides the path.
6. **Search:** shipped keyword (exact + prefix, AND), weighted fuzzy (title 1.00, description 0.60,
   body 0.30), BM25 via SQLite FTS5 (`--mode bm25`), and persistent hybrid BM25 + fuzzy.
   `--no-db` keeps legacy keyword + fuzzy hybrid. `search` replaces `find`. Default K=10.
7. **Calibration:** ranking changes must update or pass the labeled corpus and baseline.
8. **Go-native:** favor the standard library and Go idioms; use `yaml.v3` and `goldmark` only where
   specified. No UI dependencies.
9. **Commits & delivery:** atomic conventional commits only (`feat:`, `fix:`, `docs(spec):`,
   `test:`, `perf:`), delivered as stacked PRs via `gh stack`.
10. **Backlog:** update `docs/backlog.md` as phases advance (`backlog` → `in-progress` → `red` →
    `green` → `review`). **Never** set `merged`; that is a user-only action.

## Skill Router (progressive disclosure)

| Trigger domain | Skill |
| --- | --- |
| Feature specification phase (SDD) | `spec-writer` |
| BDD scenarios / behavior tests | `bdd-tester` |
| Unit tests / implementation (TDD) | `tdd-coder` |
| Go engine/search work | `tdd-coder` |
| Commits, stacked PRs, worktrees | `git-stack-reviewer` |
| Documenting architecture, data models, concepts | `okf-generator` |
| Unexpected test failure or broken build | `systematic-debugging` |
