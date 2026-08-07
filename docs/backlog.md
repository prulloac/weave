# Weave Backlog

Tracked work queue for the Weave vision: "To weave flat files into a living universe of knowledge."

Status legend:
- `backlog` — not started
- `in-progress` — actively worked
- `red` — tests written, failing (BDD/TDD phase)
- `green` — tests passing
- `review` — PR submitted, awaiting review/merge
- `done` — merged

Pipeline phase legend (per SDD -> BDD -> TDD):
- `-` — not applicable / not started
- `x` — complete

## Tier 1 — Foundation

| Feature | Spec | PR | Spec | BDD | TDD | Status |
| --- | --- | --- | --- | --- | --- | --- |
| OKF Bundle Parser | [okf-bundle-parser.md](specs/okf-bundle-parser.md) | [#1](https://github.com/prulloac/weave/pull/1) | x | - | - | review |
| Weave CLI Mount | [weave-cli.md](specs/weave-cli.md) | [#2](https://github.com/prulloac/weave/pull/2) / [#8](https://github.com/prulloac/weave/pull/8) | x | x | x | review |

## Tier 2 — Engine

| Feature | Spec | PR | Spec | BDD | TDD | Status |
| --- | --- | --- | --- | --- | --- | --- |
| Node Graph Engine | - | - | - | - | - | backlog |
| Search Index | - | - | - | - | - | backlog |

## Tier 3 — Lens

| Feature | Spec | PR | Spec | BDD | TDD | Status |
| --- | --- | --- | --- | --- | --- | --- |
| Visualization Lens (physics-based map) | - | - | - | - | - | backlog |

## Cross-cutting

- [x] Knowledge corpus: vision + related concepts (`docs/knowledge/`)
- [x] okf-generator skill aligned to OKF v0.2
- [x] spec-writer skill actor convention aligned to OKF v0.2
- [ ] Test infrastructure: `tests/e2e/` (Playwright), `tests/unit/` (bun test)
- [ ] CI workflow (lint + unit + e2e on stack PRs)
- [x] `package.json` scripts: `weave` bin, `test`, `test:e2e`

## Maintenance

Keep this file updated as pipeline phases advance. Checkboxes in the phase columns are toggled per feature as they complete.
