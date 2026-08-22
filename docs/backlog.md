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
| OKF Bundle Parser | [okf-bundle-parser.md](specs/okf-bundle-parser.md) | [#1](https://github.com/prulloac/weave/pull/1) | x | - | - | done |
| Weave CLI Mount | [weave-cli.md](specs/weave-cli.md) | [#2](https://github.com/prulloac/weave/pull/2) / [#8](https://github.com/prulloac/weave/pull/8) | x | x | x | done |

## Tier 2 — Engine

| Feature | Spec | PR | Spec | BDD | TDD | Status |
| --- | --- | --- | --- | --- | --- | --- |
| Node Graph Engine | [node-graph-engine.md](specs/node-graph-engine.md) | - | x | - | - | in-progress |
| Search Index | [search-index.md](specs/search-index.md) | - | x | - | - | in-progress |
| Weave CLI Find & Navigate | [weave-cli-navigation.md](specs/weave-cli-navigation.md) | - | x | - | - | in-progress |

## Tier 3 — Lens

| Feature | Spec | PR | Spec | BDD | TDD | Status |
| --- | --- | --- | --- | --- | --- | --- |
| Visualization Lens (physics-based map) | [visualization-lens.md](specs/visualization-lens.md) | [#9](https://github.com/prulloac/weave/pull/9) / [#10](https://github.com/prulloac/weave/pull/10) / [#11](https://github.com/prulloac/weave/pull/11) | x | x | x | done |

## Cross-cutting

- [x] Knowledge corpus: vision + related concepts (`docs/knowledge/`)
- [x] okf-generator skill aligned to OKF v0.2
- [x] spec-writer skill actor convention aligned to OKF v0.2
- [x] Test infrastructure: `tests/e2e/` (Playwright), `tests/unit/` (bun test)
- [x] CI workflow (lint + unit + e2e on stack PRs)
- [x] `package.json` scripts: `weave` bin, `test`, `test:e2e`
- [x] CI workflow: lint (Biome) + typecheck + unit + e2e on PRs/main (`.github/workflows/ci.yml`)
- [x] BDD test case documentation: `docs/bdd/` with Given/When/Then scenarios
- [x] CONTRIBUTING.md: development process, worktrees, stacked PRs, PR template
- [x] PR template: `.github/PULL_REQUEST_TEMPLATE.md`

## Maintenance

Keep this file updated as pipeline phases advance. Checkboxes in the phase columns are toggled per feature as they complete.
