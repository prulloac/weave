# AGENTS.md

## Topology

Astro · Bun · TypeScript · Playwright · Biome · GitHub (`gh stack`). All repo scripts live in `package.json`.

## Commands

- Install dependencies: `bun install`
- Dev server (always background): `astro dev --background`; manage with `astro dev status`, `astro dev logs`, `astro dev stop`
- Unit tests: `bun test`
- E2E tests: `bun x playwright test`
- Lint/format: `bun run lint` · Markdown lint: `bun run lint:md`
- Typecheck: `bun run typecheck`

## Architectural Invariants

1. **Strict pipeline order:** SDD → BDD → TDD. Never write implementation code before tests; never write tests before the specification.
2. **Approval gates:** Never advance a pipeline phase without explicit user approval of its artifact (the spec, then the BDD scenarios).
3. **Environment isolation:** For non-trivial features or spikes, ask the user first, then create a worktree: `git worktree add ../<feature-name> -b <feature-branch>`
4. **Astro islands:** Zero-JS by default; client hydration directives (`client:visible`, `client:idle`, ...) only when justified.
5. **Bun-native:** Always favor `bun test`, `bunx`, and native Bun APIs over Node.js polyfills, npm, or npx.
6. **Commits & delivery:** Atomic conventional commits only (`feat:`, `fix:`, `docs(spec):`, `test(e2e):`, ...), delivered as stacked PRs via `gh stack`.
7. **Backlog:** Update `docs/backlog.md` continuously as phases complete (`backlog` → `in-progress` → `red` → `green` → `review`) and commit backlog edits with the triggering work. **Never** set `merged` — that is a user-only action after manual PR merges.

## Skill Router (progressive disclosure)

Detailed step-by-step procedures live in `.agents/skills/<name>/SKILL.md`. Do not duplicate them here; load the matching skill via the `skill` tool only when its domain activates:

| Trigger domain | Skill |
| --- | --- |
| Feature specification phase (SDD) | `spec-writer` |
| BDD scenarios / E2E behavior tests | `bdd-tester` |
| Unit tests / implementation (TDD) | `tdd-coder` |
| Adding UI interactivity / hydration strategy | `astro-island-architect` |
| Running scripts, installing deps, writing tests | `bun-performance-guard` |
| Commits, stacked PRs, worktree management | `git-stack-reviewer` |
| Documenting architecture, data models, concepts | `okf-generator` |
| Unexpected test failure or broken build | `systematic-debugging` |
