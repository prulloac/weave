## Description

<!-- What does this PR do? Keep it concise. -->

## Stack Layer

<!-- Stacked PRs should follow: Layer 1 (Spec) → Layer 2 (Behavior tests) → Layer 3 (Implementation). -->

- [ ] Layer 1 — Specification (`docs/specs/`)
- [ ] Layer 2 — Behavior tests (contract/engine/CLI)
- [ ] Layer 3 — Implementation + unit tests
- [ ] Standalone (not part of a stack)

## Spec Reference

<!-- Link to the spec this PR implements, if applicable. -->

## Checklist

- [ ] `go test ./...` passes
- [ ] `go test -race ./...` passes
- [ ] `go vet ./...` and `gofmt` are clean
- [ ] `go run ./cmd/contract-export` produces no diff
- [ ] `bun run lint:md` passes (if docs changed)
- [ ] Commit messages follow [Conventional Commits](https://www.conventionalcommits.org/)
- [ ] Changes are scoped — no unrelated modifications

## Notes

<!-- If this changes ranking, include the calibration diff. -->
