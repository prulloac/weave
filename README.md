# Weave

> To weave flat files into a living universe of knowledge.

Weave is a portable, CLI-driven visualization engine that unlocks the latent structure
inside [Open Knowledge Format (OKF)](#okf-bundles) bundles. Point it at a local directory
or Git repository and it parses the Markdown and YAML flat files into a traversable node
graph and search index — served instantly on `localhost`, strictly offline, with zero
pollution of your data.

## Features

- **One-command mount** — `weave mount <path>` builds the graph and serves it locally.
- **Pollution-free** — the target is wrapped in a temporary symlink layer; no copies, no
  writes into the repository. `git status` and file `mtime`s stay untouched.
- **Instant** — a fully traversable node graph and search index are generated in
  milliseconds.
- **Offline** — no network calls anywhere in the parse/build/serve path.
- **Local-first** — engineered for deep focus; everything runs on your filesystem.

## Requirements

- [Bun](https://bun.sh) (runtime for the CLI and development tooling)
- Node `>=22.12`

## Installation

```sh
bun install
```

To use the `weave` binary globally, link the package:

```sh
bun link
```

## Usage

```
weave mount <path> [--port <port>] [--build-only]
weave unmount <port>
weave status [--prune]
weave --version | --help
```

### `weave mount <path>`

Parse the OKF bundle at `<path>`, build the node graph and search index, and serve the
explorer at `http://localhost:4318` (or the first free port).

```sh
weave mount ~/notes
```

- `--port <port>` — request a specific port (falls back to the next free port if busy).
- `--build-only` — build the graph and index without starting a server.

### `weave unmount <port>`

Stop the server on `<port>` and remove all temporary artifacts.

### `weave status [--prune]`

List active mounts. `--prune` removes orphaned artifact directories left by crashed
processes.

## OKF Bundles

Weave consumes **OKF bundles**: portable collections of Markdown (`.md`) and YAML flat
files that follow the [Open Knowledge Format v0.2](docs/knowledge/concepts/okf-bundle.md).
Concepts are expressed as documents with YAML frontmatter (including a valid kebab-case
`metadata.id`), and graph edges come from bundle-relative Markdown links:

```md
---
type: Concept
id: users
title: "User Table"
---
See [User Table](/tables/users.md).
```

## Project Structure

```text
/
├── cli/                    # Standalone Weave CLI (Bun)
│   ├── index.ts            #   argument parsing + dispatch
│   ├── mount.ts            #   mount lifecycle
│   ├── artifacts.ts        #   temporary symlink layer
│   ├── server.ts           #   local static server
│   └── status.ts           #   mount registry
├── src/                    # Astro site
│   ├── lib/okf/            #   OKF bundle parser
│   │   ├── parser.ts       #     bundle walking + concept extraction
│   │   ├── frontmatter.ts  #     YAML frontmatter parsing
│   │   ├── links.ts        #     link extraction + resolution
│   │   ├── validate.ts     #     concept validation
│   │   └── types.ts
│   ├── lib/explorer-render.ts  # static HTML explorer renderer
│   └── pages/              #   Astro pages
├── docs/
│   ├── specs/              #   SDD specifications
│   └── knowledge/          #   OKF knowledge corpus
├── tests/
│   ├── unit/               #   bun test suites
│   └── e2e/                #   Playwright behavior tests
└── package.json
```

The CLI lives outside `src/` because `src/` is reserved for the Astro site — the CLI is a
standalone runtime tool that only imports the shared parser from `src/lib/okf/`.

## Development

| Command              | Action                                           |
| :------------------- | :----------------------------------------------- |
| `bun dev`            | Start the Astro dev server at `localhost:4321`   |
| `bun build`          | Build the production site to `./dist/`           |
| `bun preview`        | Preview the production build locally             |
| `bun test`           | Run unit tests (`bun test tests/unit`)           |
| `bun test:e2e`       | Run Playwright e2e behavior tests                |
| `bun run typecheck`  | Type-check the project (`tsc --noEmit`)          |
| `bun cli --help`     | Run the CLI from source                          |

## Roadmap

See [docs/backlog.md](docs/backlog.md) for the tracked feature pipeline
(Tier 1 foundation → Tier 2 engine → Tier 3 lens), and
[docs/knowledge/index.md](docs/knowledge/index.md) for the vision and architecture
concepts.
