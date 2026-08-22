# BDD: Weave CLI Find & Navigate

Human-readable behavior test cases for the stateless navigation commands (`find`, `list`, `show`, `backlinks`, `path`).

**Spec reference:** [weave-cli-navigation.md](../specs/weave-cli-navigation.md)
**Playwright spec:** `tests/e2e/weave-cli-navigation.spec.ts`

---

## Feature: Find Concepts With Ranked Search

As a terminal user, I want grep-style full-text search over a bundle without starting a server.

### Scenario: Find prints ranked human-readable results

- **Given** the test fixture bundle on disk
- **When** I run `weave find <bundle-path> okf`
- **Then** the process exits with code 0
- **And** each output line matches the shape `score title [path]`
- **And** results are ordered highest score first

### Scenario: Limit caps the number of results

- **Given** a fixture bundle where more than 2 concepts match
- **When** I run `weave find <bundle-path> <term> --limit 2`
- **Then** exactly 2 result lines are printed

### Scenario: JSON output is machine-parseable

- **Given** the test fixture bundle on disk
- **When** I run `weave find <bundle-path> okf --json`
- **Then** stdout parses as a JSON array of `FindResult` objects
- **And** each object exposes keys in stable order: `id`, `title`, `path`, `type`, `score`
- **And** two consecutive runs produce identical output

---

## Feature: Filter Concepts by Metadata Only

As a terminal user, I want to filter concepts by frontmatter metadata without paying for full-text scoring.

### Scenario: Filter by type

- **Given** the test fixture bundle containing concepts of mixed types
- **When** I run `weave list <bundle-path> --type concept`
- **Then** the process exits with code 0
- **And** every output line matches the shape `title [path]`
- **And** only concepts whose frontmatter type is `concept` are listed, sorted by id ascending

### Scenario: Repeated tags AND-combine

- **Given** fixture concepts where some carry tag `cli`, some `search`, and one carries both
- **When** I run `weave list <bundle-path> --tag cli --tag search`
- **Then** exactly the concept carrying both tags is listed

### Scenario: JSON listing is stable and ordered

- **When** I run `weave list <bundle-path> --json`
- **Then** stdout parses as a JSON array of `ListResult` objects with keys `id`, `title`, `path`, `type`, `status`, `tags`
- **And** the array is sorted by id ascending
- **And** two consecutive runs produce identical output

### Scenario: No matches exits 1

- **Given** no concept in the bundle carries status `deprecated`
- **When** I run `weave list <bundle-path> --status deprecated`
- **Then** the process exits with code 1
- **And** no result lines are printed

---

## Feature: Show a Concept

As a terminal user, I want to read a concept with its metadata and connections inline.

### Scenario: Show renders metadata, backlinks and body

- **Given** the test fixture bundle where `concepts/okf-bundle` is referenced by other concepts
- **When** I run `weave show <bundle-path> concepts/okf-bundle`
- **Then** the output contains the concept title
- **And** the output contains the frontmatter tags and status
- **And** the output contains a backlink count greater than 0
- **And** the output contains the outgoing links
- **And** the output contains the body content

### Scenario: Id-or-path resolution accepts multiple input forms

- **Given** the test fixture bundle on disk
- **When** I run `weave show <bundle-path> concepts/okf-bundle`
- **And** I run `weave show <bundle-path> concepts/okf-bundle.md`
- **And** I run `weave show <bundle-path> <absolute-path>/concepts/okf-bundle.md`
- **Then** all three invocations exit with code 0
- **And** all three render the same concept

### Scenario: JSON show emits the full ShowResult

- **Given** the test fixture bundle on disk
- **When** I run `weave show <bundle-path> concepts/okf-bundle --json`
- **Then** stdout parses as a single JSON object with keys `id`, `path`, `type`, `body`, `links`, `backlinks`

---

## Feature: List Backlinks

As a terminal user, I want to know what references a concept.

### Scenario: Backlinks lists incoming edges with titles

- **Given** the test fixture bundle where two concepts link to `concepts/okf-bundle`
- **When** I run `weave backlinks <bundle-path> concepts/okf-bundle`
- **Then** the process exits with code 0
- **And** the output lists both referencing concept ids with their titles
- **When** I run the same command with `--json`
- **Then** stdout parses as `{ id, backlinks }` with 2 entries

---

## Feature: Path Between Concepts

As a terminal user, I want the shortest route between two concepts.

### Scenario: Shortest path is printed as a hop sequence

- **Given** a fixture bundle forming the chain `a → b → c`
- **When** I run `weave path <bundle-path> concepts/a concepts/c`
- **Then** the process exits with code 0
- **And** the printed route is `concepts/a → concepts/b → concepts/c`
- **When** I run the same command with `--json`
- **Then** stdout parses as `{ from, to, path }` where `path` has 3 entries

---

## Feature: Errors Follow the Mount Convention

As a script author, I rely on exit codes instead of parsing prose.

### Scenario: Empty query exits 1

- **Given** the test fixture bundle on disk
- **When** I run `weave find <bundle-path> ""`
- **Then** the process exits with code 1
- **And** no result lines are printed

### Scenario: Unknown concept exits 1

- **Given** the test fixture bundle on disk
- **When** I run `weave show <bundle-path> concepts/nonexistent`
- **Then** the process exits with code 1
- **And** the output mentions the concept was not found

### Scenario: Disconnected path exits 1 with null route

- **Given** a fixture bundle with two concepts in different components
- **When** I run `weave path <bundle-path> concepts/a concepts/z`
- **Then** the process exits with code 1
- **And** the `--json` variant reports `path: null`

### Scenario: Missing target directory exits 2

- **Given** a target path that does not exist on the filesystem
- **When** I run `weave find <nonexistent-path> anything`
- **Then** the process exits with code 2

### Scenario: Empty bundle exits 1 for every command

- **Given** a directory containing no valid OKF concepts
- **When** I run `weave find`, `weave list`, `weave show`, and `weave backlinks` against it
- **Then** every invocation exits with code 1

---

## Feature: Stateless Determinism

As an AI agent scripting Weave, I need identical invocations to produce identical output.

### Scenario: Repeated invocations never drift

- **Given** the test fixture bundle on disk
- **When** I run the same `weave find --json` command three times
- **Then** all three outputs are byte-identical
- **And** no files inside the target bundle were modified (clean `git status --porcelain`)
