# Contributing to Weave

Thanks for your interest in contributing! This guide covers the development process, tooling, and conventions used in this project.

## Prerequisites

- [Bun](https://bun.sh) (runtime, package manager, test runner)
- Node `>=22.12`
- [GitHub CLI](https://cli.github.com/) with the [`gh stack`](https://github.com/dolmen/gh-stack) extension (for stacked PRs)

## Getting Started

```sh
git clone https://github.com/prulloac/weave.git
cd weave
bun install
```

Start the Astro dev server:

```sh
bun dev
```

The site runs at `http://localhost:4321`. The CLI can be run from source with:

```sh
bun cli --help
```

## Development Pipeline

We follow a strict **SDD → BDD → TDD** pipeline. Never write implementation code before tests, and never write tests before the specification.

### Phase 1: Specification (SDD)

1. Write a markdown specification in `docs/specs/<feature-name>.md`.
2. Get the spec approved before proceeding.

### Phase 2: Behavior Testing (BDD)

1. Write human-readable BDD test cases in `docs/bdd/<feature-name>.md` using Given/When/Then scenarios.
2. Get the BDD test cases approved before proceeding.
3. Translate the approved scenarios into Playwright end-to-end tests in `tests/e2e/`.
4. Run them to confirm they fail (red phase):

```sh
bun test:e2e
```

### Phase 3: Unit Testing & Implementation (TDD)

1. Write unit tests in `tests/unit/` using `bun test` and confirm they fail.
2. Implement the code to make all tests pass.
3. Refactor while keeping all test suites green.

## Available Scripts

| Command             | Action                                           |
| :------------------ | :----------------------------------------------- |
| `bun dev`           | Start the Astro dev server at `localhost:4321`   |
| `bun build`         | Build the production site to `./dist/`           |
| `bun preview`       | Preview the production build locally             |
| `bun test`          | Run unit tests (`tests/unit/`)                   |
| `bun test:e2e`      | Run Playwright e2e behavior tests                |
| `bun run typecheck` | Type-check the project (`tsc --noEmit`)          |
| `bun run lint`      | Lint and format with Biome                       |
| `bun run lint:md`   | Lint markdown files                              |
| `bun cli --help`    | Run the CLI from source                          |

## Code Style

This project uses [Biome](https://biomejs.dev) for linting and formatting:

- **Tabs** for indentation (not spaces)
- **Single quotes** for JavaScript/TypeScript strings
- **No semicolons** (ASI style)
- **100-character** line width
- **Conventional commits** (see below)

Run `bun run lint` before committing to ensure your code passes.

## Git Worktrees

For non-trivial features, use a [Git worktree](https://git-scm.com/docs/git-worktree) to isolate your work without dirtying the main working directory:

```sh
# Create a worktree for a new feature
git worktree add ../<feature-name> -b <feature-branch>

# List active worktrees
git worktree list

# Remove a worktree after merging
git worktree remove ../<feature-name>
```

Worktrees share the same `.git` history but have independent working directories, so you can work on multiple features concurrently without stashing or switching branches.

## Stacked Pull Requests

We use [`gh stack`](https://github.com/dolmen/gh-stack) to structure changes into small, dependent, reviewable PRs. Each feature follows a 4-layer stack:

| Layer | Content              | Commit prefix      |
| :---- | :------------------- | :----------------- |
| 1     | Specification        | `docs(spec):`      |
| 2     | BDD test cases       | `docs(bdd):`       |
| 3     | E2E behavior tests   | `test(e2e):`       |
| 4     | Implementation + unit tests | `feat(<scope>):` / `fix(<scope>):` |

### Workflow

```sh
# After completing a phase, save it to the stack
gh stack save -m "docs(spec): add spec for <feature>"

# Check stack status
gh stack status

# After all layers are ready, push and create PRs
gh stack submit

# Sync after upstream changes
gh stack sync
```

Each `gh stack save` creates a new branch and commit on top of the previous layer. `gh stack submit` pushes all branches and opens PRs on GitHub with the correct parent-child relationships.

## Commit Convention

We use [Conventional Commits](https://www.conventionalcommits.org/). Stage files selectively (never `git add .`):

```
docs(spec): add specification for node graph engine
docs(bdd): add BDD test cases for search index
test(e2e): add failing behavior tests for search index
test(unit): add unit tests for frontmatter parser
feat(parser): implement OKF bundle walker
fix(server): handle port conflict fallback
refactor(artifacts): simplify symlink creation
chore(ci): update playwright version
```

## Running Tests

```sh
# Unit tests
bun test

# E2E tests (requires Playwright browsers installed)
bun test:e2e

# Install Playwright browsers (first time only)
bunx playwright install chromium
```

## Project Structure

```
weave/
├── cli/                    # Standalone CLI (Bun runtime)
├── src/                    # Astro site + shared parser library
│   └── lib/okf/            #   OKF bundle parser (hand-rolled, no deps)
├── docs/
│   ├── specs/              #   SDD specifications
│   ├── bdd/                #   BDD test cases (Given/When/Then)
│   ├── knowledge/          #   OKF knowledge corpus
│   └── backlog.md          #   Feature pipeline tracker
├── tests/
│   ├── unit/               #   Bun test suites
│   ├── e2e/                #   Playwright behavior tests
│   └── fixtures/           #   Test fixture bundles
└── .github/workflows/      # CI pipelines
```

## Questions?

Open an issue or start a discussion on [GitHub](https://github.com/prulloac/weave).
