# Documentation Style Guide

Weave documentation is part of the release contract. A reader should be able to tell what is
shipped, what is deferred and which document owns each fact without opening more than two pages.

## Document classes

| Class | Location | Owns | Does not own |
| --- | --- | --- | --- |
| Task guide | `README.md`, `docs/guides/**` | Install, quickstart, commands and examples | Delivery status or internal design |
| Roadmap | `ROADMAP.md` | Feature delivery status and release scope | Behavioral contracts |
| Backlog | `docs/backlog.md` | Pipeline phase and tracked work | Delivery status |
| Specification | `docs/specs/**` | Behavioral contract | Delivery status or phase history |
| Behavior scenario | `docs/bdd/**` | Observable behavior examples | Algorithms or implementation |
| Architecture | `docs/architecture/**` | Current design and boundaries | Roadmap promises |
| ADR | `docs/architecture/adr/**` | Immutable decisions | Current task instructions |
| Knowledge | `docs/knowledge/**` | Vision and concepts | Status or contract |

## Status

- Specs use `draft`, `stable` or `deprecated` in frontmatter.
- ADRs use `proposed`, `accepted`, `superseded` or `rejected` in frontmatter.
- Do not add a `Status:` line to the body. Delivery state belongs to the roadmap and backlog.
- Write shipped behavior in the present tense. Put future work in the roadmap and backlog, not in a
  reference page.
- A document's status describes the document, not the feature. A shipped feature can have a `draft`
  spec while the contract is being reviewed, but that is an exception to close before release.

## Numbering

- ADR files are `NNNN-kebab-title.md`, contiguous from `0001`. The number in the filename, title,
  index and references must match.
- Feature Register IDs are contiguous from `1` and sorted ascending.
- Every `Phase`, `Wave` and milestone ID referenced by another document must exist.
- Anchors are checked; do not link to a heading that no longer exists.

## Cross-references

- Link the owner instead of restating a fact.
- Every Feature Register row links one authoritative spec and one backlog tier.
- Every spec appears in [`docs/specs/README.md`](specs/README.md).
- Every behavior set appears in [`docs/bdd/README.md`](bdd/README.md) and names its Go tests.
- Every ADR appears in [`docs/architecture/adr/README.md`](architecture/adr/README.md).
- Every document is reachable from [`docs/README.md`](README.md).

## Voice and structure

- Open with one sentence that states the document's purpose.
- Name the audience when it is not obvious.
- Use short paragraphs, tables and runnable examples.
- Use one vocabulary: bundle, document, concept, source, database, in-memory.
- Do not describe retired commands, frameworks or product surfaces. Keep a short non-goals note only
  when the history prevents a real misunderstanding.
- Prefer concrete commands and outputs over internal phase history.
- Do not use unexplained IDs. Link the definition.

## Links and examples

- Use relative Markdown links for repository documents.
- Link to headings only when the heading is stable; prefer linking the document.
- Keep examples copy-pasteable and free of placeholder secrets.
- Run `go run ./cmd/docs-lint` before opening a documentation pull request.
