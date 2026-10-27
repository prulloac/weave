---
type: assessment
title: "OKF Parsing & Ingestion Assessment"
description: "Findings and remediation plan for the OKF bundle parser, its consumers, and the mount ingestion path."
tags:
  - "okf"
  - "parser"
  - "ingestion"
  - "assessment"
  - "quality"
generated:
  by: opencode/deepseek-v4.1-flash
  at: "2026-10-25T00:00:00Z"
status: draft
---

# OKF Parsing & Ingestion Assessment

> **Historical.** This report describes the TypeScript/Bun implementation that was removed in the
> Go-only search v1 rewrite. See [ADR 0003](../architecture/adr/0003-keyword-fuzzy-search-and-ui-retirement.md).

## Summary

The OKF parsing core is implemented, tested, and integrated. All 23 parser unit tests and
96/96 total unit tests pass, and `tsc --noEmit` is clean. The parser feeds the graph engine,
the BM25 search index, the CLI navigation commands, and the mount explorer.

Ingestion works but is minimal: `weave mount` materializes a temporary symlink layer, parses
the bundle once, and serves one static HTML string for every request. There is no caching,
watching, incremental re-ingest, or persistent index. The richer mounted-site ingestion
(`/`, `/lens`, `/concept/<id>`) is specced and covered by red BDD scenarios, but not yet
implemented.

This report records verified defects in the parser, documentation inconsistencies, and a
phased plan to fix them. Findings marked **Verified** were reproduced against the current
code, not inferred from the specification.

## Remediation Status

All phases were implemented on `fix/okf-parsing-hardening` on 2026-10-25.

| Phase | Findings | State |
| --- | --- | --- |
| 1 — Correctness hotfixes | F1, F2, F4, F12 | Fixed with regression tests |
| 2 — Parser robustness | F3, F5, F6 | Fixed with regression tests |
| 3 — Documentation alignment | F7, F8, F11 | Fixed |
| 4 — Performance & ingestion | F9, F10 | Benchmark added; disk cache added; live reload remains out of scope |
| 5 — Feature work | Lens mount integration | Implemented and green (14 E2E scenarios) |

Verification at hand-off: `bun test tests/unit` → 115 pass; `bun run bench` → ~0.04 ms/file
(~410 ms for 10k files); full Playwright suite → 80 pass; `tsc --noEmit` clean.

The decisions listed at the end of this report were resolved as follows: link anchors are
dropped from the resolved path (the raw target is preserved); nested `index.md` files are
ignored as concepts and do not alter bundle metadata; live reload is out of scope and
`ParseOptions.cache` speeds up repeated CLI reads; the hand-rolled YAML parser was retained
and made recursive; the 100 ms target was replaced by a measured 5 s regression budget.

## Scope & Method

- **Commit:** `3b88787` (`docs(spec): define server-rendered lens index in lens shell`)
- **Working tree:** clean
- **Runtime:** Bun `1.3.14` on WSL Ubuntu
- **Commands run:**
  - `bun test tests/unit` → 96 pass, 0 fail
  - `bun test tests/unit/okf-bundle-parser.test.ts` → 23 pass, 0 fail
  - `bunx tsc --noEmit` → clean
  - `bunx biome check src/lib/okf cli` → 1 pre-existing error (`cli/index.ts:339`, implicit `any`)
  - Ad-hoc parse probes over temporary bundles and over `docs/knowledge/concepts/*.md`
- **Out of scope:** the Lighthouse/design pass, live reload, and the unbuilt lens mount routes,
  except where they affect ingestion.

## Current State

| Area | State | Location |
| --- | --- | --- |
| Frontmatter extraction + YAML subset | Working, limited | `src/lib/okf/frontmatter.ts` |
| Markdown link extraction/resolution | Working, limited | `src/lib/okf/links.ts` |
| Bundle walker, index, validation, UTF-8 handling | Working | `src/lib/okf/parser.ts` |
| Types and validation helpers | Working | `src/lib/okf/types.ts`, `src/lib/okf/validate.ts` |
| Graph ingestion | Working | `src/lib/graph/build.ts` |
| Search ingestion (BM25) | Working | `src/lib/search/build.ts` |
| CLI consumers | Working | `cli/find.ts`, `cli/list.ts`, `cli/show.ts`, `cli/path.ts`, `cli/resolve.ts` |
| Mount ingestion | Working but static | `cli/artifacts.ts`, `cli/mount.ts`, `cli/server.ts`, `src/lib/explorer-render.ts` |
| Lens mount integration | Red (spec + BDD only) | `docs/specs/lens-mount-integration.md` |

### Ingestion flow today

```text
weave mount <dir>
  -> createArtifactLayer()   temp dir of symlinks (hardlink fallback on Windows)
  -> parseBundle()           full recursive walk, one sequential read per file
  -> renderExplorer(bundle)  one static HTML string
  -> Bun.serve()             same HTML for every route; no watch, no reload
```

CLI read commands (`find`, `list`, `show`, `backlinks`, `path`) skip the artifact layer and
call `parseBundle()` directly on the target path on every invocation. Every command therefore
performs a full walk and full file read set.

## Findings

### F1 — Fragment links are falsely reported as broken (High)

- **Status:** Verified
- **Location:** `src/lib/okf/links.ts:25-34`, `src/lib/okf/parser.ts:166-177`
- **Observed:** A link `[x](b.md#section)` resolves to `b.md#section`. The resolver then checks
  `concepts.has("b.md#section")`, fails, and emits `broken link: b.md#section`.
- **Expected:** The fragment is a navigation anchor, not part of the concept identity. The edge
  should resolve to `b.md` (and optionally retain the anchor for rendering).
- **Why it matters:** Anchors are common in Markdown; every anchored link currently becomes a
  dangling edge and a false validation warning, which pollutes graph and mount output.
- **Reproduction:**

  ```ts
  const root = mkdtempSync(join(tmpdir(), "probe-"))
  writeFileSync(join(root, "a.md"), "---\ntype: Concept\n---\n[x](b.md#section)\n")
  writeFileSync(join(root, "b.md"), "---\ntype: Concept\n---\n")
  const bundle = await parseBundle(root)
  // observed: a.links[0].resolved === "b.md#section", resolvesInBundle === false
  ```

**Fix steps**

1. Split the destination before resolving: drop `?query` and `#fragment` for path resolution.
2. Preserve the anchor in a new optional `anchor?: string` field on `OkfLink` if the renderer
   needs it; do not include it in the concept id.
3. Add unit tests for `a.md#section`, `a.md?x=1`, `#local-anchor`, and external URLs with
   fragments.
4. Add a parser test asserting an anchored link to an existing concept is `resolvesInBundle: true`
   with no broken-link warning.

**Acceptance criteria:** Anchored and query-suffixed links resolve to the underlying concept;
`resolvesInBundle` is `true`; no broken-link warning is emitted.

---

### F2 — Nested `index.md` overwrites the bundle index and version (High)

- **Status:** Verified
- **Location:** `src/lib/okf/parser.ts:131-149`
- **Observed:** Reserved filenames are matched by basename. Any `sub/index.md` is parsed as the
  bundle index, replacing `bundle.index` and potentially `bundle.okfVersion` with the nested
  file's data. In a probe with a root index and `sub/index.md`, `bundle.index` ended as `[]`
  (the nested file's listing).
- **Expected:** Per the `okf-generator` skill, `index.md` and `log.md` are reserved at any level.
  Only the **bundle-root** `index.md` may set `okfVersion` and the bundle-wide index. Nested
  listings must not replace root metadata.
- **Why it matters:** A directory listing silently destroys the root index listing and can
  downgrade or clear `okfVersion`.

**Fix steps**

1. Compute the bundle-relative path (`rel`) before the reserved-name branch.
2. Treat `isRootIndex = rel === "index.md"`. Only that file sets `index` and `okfVersion`.
3. Skip other `index.md`/`log.md` files from concept mapping (they remain non-concepts).
4. If per-directory listings are wanted later, add a separate `directoryIndexes` map instead of
   reusing `bundle.index`.
5. Add tests: root + nested `index.md`; root version preserved; nested listing not leaked;
   `index.md` in a subdirectory does not become a concept.

**Acceptance criteria:** `bundle.index` and `bundle.okfVersion` reflect only the root file;
nested reserved files are ignored without warnings.

---

### F3 — YAML subset corrupts lists of mappings (High)

- **Status:** Verified
- **Location:** `src/lib/okf/frontmatter.ts:94-128`
- **Observed:** The `verified` block used throughout the project's own knowledge corpus:

  ```yaml
  verified:
    - by: human:developer
      at: "2026-08-06T00:00:00Z"
  ```

  parses to `["by: \"human:developer\""]`. The `at` value is lost and the list item is a string
  instead of an object.
- **Expected:** `verified: [{ by: "human:developer", at: "2026-08-06T00:00:00Z" }]`, per the
  `okf-generator` trust-frontmatter rules.
- **Why it matters:** Trust metadata (`generated`/`verified`), nested resources, and any
  structured list are silently depleted. Consumers cannot rely on `frontmatter` beyond scalars.

**Fix steps**

1. Replace the single-level block parser with a recursive indentation-based parser:
   - Track an indent stack.
   - Parse sequences and mappings recursively.
   - Support list items that are mappings (`- key: value` followed by sibling keys at the item
     indent).
2. Keep the existing YAML subset surface (scalars, flow arrays, nested maps, comments) and add
   list-of-mappings plus nested sequences.
3. Add table-driven tests for `verified` as a list of mappings, a bare mapping (`verified: { by, at }`),
   nested `related` lists, and deeper-than-one-level mappings.
4. Consider replacing the hand-rolled parser with a maintained small YAML parser if the subset
   keeps growing; record the decision before adding block scalars (`|`, `>`), multi-line strings,
   anchors, or tags.

**Acceptance criteria:** Parsing `docs/knowledge/concepts/okf-bundle.md` yields
`verified[0].by === "human:developer"` and `verified[0].at` as the original timestamp.

---

### F4 — `#` truncates unquoted scalar values (Medium)

- **Status:** Verified
- **Location:** `src/lib/okf/frontmatter.ts:17-35`
- **Observed:** `url: https://example.com/page#frag` parses as `https://example.com/page`.
- **Expected:** A YAML comment starts only when `#` is preceded by whitespace (or begins the
  line). `#` inside a URI is data.
- **Why it matters:** `resource`, `url`, and similar frontmatter fields lose fragments, and
  copied values can silently change meaning.

**Fix steps**

1. In `stripComment`, only treat `#` as a comment if `i === 0` or the previous character is
   whitespace, and the parser is outside quotes and flow collections.
2. Add tests for `https://example.com/page#frag`, `value # comment`, `# leading comment`, and
   `"quoted # data"`.

**Acceptance criteria:** Unquoted URLs retain fragments; trailing comments still strip correctly.

---

### F5 — Markdown link extraction misses valid links (Medium)

- **Status:** Verified
- **Location:** `src/lib/okf/links.ts:4`
- **Observed:** The pattern `(?<!!)\[([^\]]*)\]\(([^()\s]+)\)` does not match:
  - reference-style links `[x][ref]` / `[x]`
  - titled inline links `[x](a.md "Title")`
  - destinations containing balanced parentheses or encoded spaces
- **Expected:** These are standard Markdown link forms and should become graph edges.
- **Why it matters:** Whole classes of edges disappear, so graph, backlinks, and search context
  are incomplete with no warning.

**Fix steps**

1. Extend inline-link matching to allow an optional title after the destination:
   `\(([^()\s]+)(?:\s+(?:"[^"]*"|'[^']*'|\([^()]*\)))?\)`.
2. Add a second pass for reference definitions (`[id]: /path`) and reference uses
   (`[text][id]`, `[text][]`, `[text]`); resolve each definition once per document.
3. Skip fenced code blocks and inline code spans so examples are not treated as edges.
4. Add tests for each form plus image embeds, code fences, and nested brackets in link text.

**Acceptance criteria:** Titled and reference-style links are extracted; code samples are not.

---

### F6 — Symlinked directories are never followed (Medium)

- **Status:** Verified
- **Location:** `src/lib/okf/parser.ts:24-46`
- **Observed:** Symlink entries fall through to the file branch. A symlink to a directory is
  stat'ed, found not to be a file, and skipped. The spec says symlinked directories are followed
  at most one level with a visited-set guard.
- **Expected:** Follow one level of directory symlinks, skip cycles via `realpath`, and never
  follow `node_modules`/`.git`.
- **Why it matters:** Bundles assembled from symlinked sources silently parse fewer concepts.

**Fix steps**

1. Use `lstat`/`Dirent` to detect symlinks, then `stat` the target.
2. If the target is a directory, recurse with an incremented depth (max 1 beyond the root) and
   the existing `realpath` visited set.
3. Seed the visited set with the resolved root to guard root cycles.
4. Add tests for a symlinked directory, a self-referential cycle, and a symlinked file.
5. Decide whether the depth limit applies to nested symlinks only; encode the decision in the
   spec and tests.

**Acceptance criteria:** Concepts inside one level of symlinked directories are parsed exactly
once; cycles terminate.

---

### F7 — Identity documentation contradicts the implementation (Medium)

- **Status:** Verified
- **Location:** `README.md:72-73`, `docs/knowledge/concepts/okf-bundle.md:33`,
  `src/lib/okf/parser.ts:52-54`
- **Observed:** The README and the OKF Bundle concept require a "valid kebab-case `metadata.id`".
  The parser ignores `metadata.id` and derives the concept id from the bundle-relative path. The
  `okf-generator` skill explicitly states "There is NO `metadata.id` field".
- **Expected:** One identity rule everywhere: concept id = path minus `.md`.
- **Why it matters:** Authors following the README will add a field the parser ignores, and can
  disagree with generated OKF documents.

**Fix steps**

1. Update `README.md` to state the path-based identity rule and remove `metadata.id` from the
   example.
2. Update the OKF Bundle concept requirements table accordingly.
3. Add a parser test asserting `metadata.id` is preserved as opaque frontmatter but does not
   change the concept id.
4. Grep for remaining `metadata.id` references and align them.

**Acceptance criteria:** No documentation requires `metadata.id`; identity behavior is covered by
a test.

---

### F8 — Index entry descriptions are never populated (Low)

- **Status:** Verified
- **Location:** `src/lib/okf/parser.ts:107-119`, `src/lib/okf/types.ts:29-33`
- **Observed:** `readIndex` maps only link text and target. `IndexEntry.description` is always
  `undefined`, even when the listing has `- [Title](path) - description`.
- **Expected:** Parse the trailing description if present.
- **Why it matters:** The mount overview loses the human-readable summary the format allows.

**Fix steps**

1. Parse the text following the link on the same list item, stripping a leading `-`, `--`, or
   `—` separator.
2. Add unit tests with and without descriptions, and with inline formatting to strip.
3. Extend `renderExplorer` to display descriptions once parsed.

**Acceptance criteria:** Descriptions round-trip from `index.md` into `IndexEntry` and the
rendered explorer.

---

### F9 — No performance evidence for the 10k-file target (Low)

- **Status:** Open risk
- **Location:** `docs/specs/okf-bundle-parser.md:135`, `src/lib/okf/parser.ts:131-164`
- **Observed:** The spec targets parse + validate of a 10k-file bundle in under 100 ms. Files are
  read sequentially, frontmatter is extracted twice per file (`parser.ts:156` and
  `parser.ts:160`), and there is no benchmark in the repo.
- **Expected:** A repeatable benchmark and evidence against the budget, or a revised budget.
- **Why it matters:** The vision promises millisecond graph generation; without a benchmark the
  claim is unverifiable and regressions are invisible.

**Fix steps**

1. Add `tests/bench/parse-bundle.bench.ts` that generates a 10k-file synthetic bundle and reports
   walk, parse, link-resolution, and validation timings.
2. Remove the double frontmatter extraction by parsing once and passing the result through.
3. Introduce bounded-concurrency reads (for example, 32 in flight) and measure the difference on
   local SSD.
4. If the budget is infeasible in Bun-on-WSL, record the measured number and revise the spec
   rather than leaving a false target.
5. Add a CI smoke benchmark with a generous ceiling to catch order-of-magnitude regressions.

**Acceptance criteria:** A committed benchmark produces a stable number; the spec target and the
measured result agree or the spec is amended.

---

### F10 — Ingestion has no cache, watch, or incremental path (Low, by design)

- **Status:** Open risk / product gap
- **Location:** `cli/mount.ts:67-84`, `cli/server.ts:11-32`, `cli/find.ts`, `cli/list.ts`,
  `cli/show.ts`, `cli/path.ts`
- **Observed:** `parseBundle()` re-reads the whole bundle on every CLI invocation. The mount
  parses once and never reloads. No watch mode exists; the lens integration spec lists live
  reload as a non-goal.
- **Expected:** Acceptable for small bundles; needs a decision before large-repo use.
- **Why it matters:** Interactive CLI use on a large repository pays the full parse cost per
  command; mounted sites go stale after edits until remount.

**Fix steps**

1. Add a parse cache keyed by bundle root plus file `(mtimeMs, size)`, stored under the mount
   artifact layer or a local cache dir; invalidate per file.
2. Reuse the cache across `find`/`list`/`show`/`backlinks`/`path`.
3. Decide whether `weave mount --watch` is in scope; if not, document that remount is required.
4. Add a test that a second parse of an unchanged bundle avoids re-reading bodies and that a
   touched file invalidates only that entry.

**Acceptance criteria:** Repeated CLI commands on an unchanged bundle are materially faster; stale
behavior is documented or watch is implemented.

---

### F11 — Backlog phase columns for the parser are inconsistent (Low)

- **Status:** Verified
- **Location:** `docs/backlog.md:23`
- **Observed:** The OKF Bundle Parser row shows BDD `-` and TDD `-`, but a BDD document exists
  (`docs/bdd/okf-bundle-explorer.md`), Playwright scenarios exist, and 23 unit tests pass.
- **Expected:** Phase columns reflect reality.
- **Fix steps:** Set BDD and TDD to `x` for the parser row; keep status `done`. Verify the other
  Tier 1 row at the same time.

**Acceptance criteria:** Backlog matches artifacts and tests.

---

### F12 — Unrelated lint failure in the CLI (Trivial)

- **Status:** Verified
- **Location:** `cli/index.ts:339`
- **Observed:** `biome check` fails on `let info` (`noImplicitAnyLet`).
- **Fix steps:** Type the binding, for example
  `let info: ReturnType<typeof statSync>`; re-run `bun run lint`.

**Acceptance criteria:** `bun run lint` passes on a clean tree.

## Remediation Plan

Execute in this order; each phase is independently shippable and test-backed.

### Phase 1 — Correctness hotfixes

- [ ] F1: strip fragment/query before resolving links; optional `anchor` field.
- [ ] F2: scope root index/version to the bundle root only.
- [ ] F4: correct YAML comment detection.
- [ ] F12: fix the implicit-`any` lint error.
- [ ] Add regression tests for each.
- [ ] Run `bun test tests/unit`, `bun run typecheck`, `bun run lint`.

### Phase 2 — Parser robustness

- [ ] F3: recursive YAML subset (list-of-mappings first).
- [ ] F5: titled and reference-style links; ignore code fences/spans.
- [ ] F6: one-level symlinked directory traversal with cycle guard.
- [ ] Re-parse `docs/knowledge/` and assert trust metadata round-trips.
- [ ] Expand fixtures: `anchors/`, `nested-index/`, `verified/`, `links/`, `symlinks/`.

### Phase 3 — Documentation alignment

- [ ] F7: path-based identity in README and OKF Bundle concept.
- [ ] F8: index descriptions (parser + renderer) if still desired.
- [ ] F11: correct backlog columns.
- [ ] Add this report to the relevant spec's "Related Concepts" or backlog cross-cutting list.

### Phase 4 — Performance & ingestion

- [ ] F9: 10k benchmark, single frontmatter parse, bounded-concurrency reads.
- [ ] F10: mtime/size parse cache across CLI commands; decide watch scope.
- [ ] Record measured budgets in `docs/specs/okf-bundle-parser.md`.

### Phase 5 — Feature work (not defects)

- [ ] Implement the red `lens-mount-integration` scenarios (routes, concept pages, lens shell).
- [ ] Keep `/` and `/concept/<id>` zero-JS; hydrate only `/lens`.

## Verification Checklist

Run after each phase:

```sh
bun test tests/unit
bun run typecheck
bun run lint
bun run lint:md
bun x playwright test
```

Targeted new tests to add:

| Finding | Test file | Cases |
| --- | --- | --- |
| F1 | `tests/unit/okf-bundle-parser.test.ts` | anchor, query, local anchor, external fragment |
| F2 | `tests/unit/okf-bundle-parser.test.ts` | root vs nested index, version preservation |
| F3 | `tests/unit/okf-bundle-parser.test.ts` | list-of-mappings, bare mapping, nested depth |
| F4 | `tests/unit/okf-bundle-parser.test.ts` | URL fragment, trailing comment, quoted `#` |
| F5 | `tests/unit/okf-bundle-parser.test.ts` | titled link, reference link, code fence exclusion |
| F6 | `tests/unit/okf-bundle-parser.test.ts` | symlinked dir, cycle, symlinked file |
| F7 | `tests/unit/okf-bundle-parser.test.ts` | `metadata.id` ignored for identity |
| F8 | `tests/unit/okf-bundle-parser.test.ts` | index description present/absent |
| F9 | `tests/bench/parse-bundle.bench.ts` | 10k-file timing breakdown |
| F10 | `tests/unit/weave-cli-navigation.test.ts` | cache hit/invalidation |

## Decisions Needed

1. Should `OkfLink` expose the anchor, or is dropping it acceptable for the graph?
2. Should nested `index.md` be ignored entirely, or exposed as per-directory listings?
3. Is live reload in scope for mount, or is remount the documented workflow?
4. Do we keep the hand-rolled YAML parser, or adopt a small maintained one once block scalars
   and nested sequences are required?
5. Is the 100 ms / 10k-file budget a hard requirement or an aspiration?

## Related Documents

- [OKF Bundle Parser Specification](../specs/okf-bundle-parser.md)
- [OKF Bundle Concept](../knowledge/concepts/okf-bundle.md)
- [Weave Backlog](../backlog.md)
- [ADR 0003](../architecture/adr/0003-keyword-fuzzy-search-and-ui-retirement.md)
