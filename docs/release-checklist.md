# v1.0.0 Release Checklist

This checklist records the release state. The tag and publish steps are owner actions.

## Complete

- [x] Feature register shipped, with optimization marked post-GA stretch.
- [x] MIT license added and included in release archives.
- [x] Documentation gate: `docs-lint` passes with 0 violations.
- [x] Markdown lint: 0 issues.
- [x] Regression gate: shuffled race suite, vet, staticcheck, govulncheck, fuzzing and built-binary
  smoke.
- [x] CI on `main`: run `41225010841` passed.
- [x] Release dry run `v1.0.0-rc.3`: run `41225227470` passed on Linux, macOS and Windows.
- [x] Dry run produced all six archives, checksums and build info without creating a release.
- [x] Linux archive inspected: `weave`, `README.md`, `LICENSE`.

## Owner steps

- [ ] Decide the final version (`v1.0.0` unless a release candidate is preferred).
- [ ] Create an annotated tag:
      `git tag -a v1.0.0 -m "Weave v1.0.0"`.
- [ ] Push the tag: `git push origin v1.0.0`.
- [ ] Watch the release workflow. It verifies, builds the matrix, runs artifact smoke and creates a
      **draft** release.
- [ ] Review the draft: notes, six archives, checksums and build info.
- [ ] Publish manually with `gh release edit v1.0.0 --draft=false`.

## Notes

- Publishing is intentionally manual; a tag push never publishes directly.
- If the draft is wrong, delete it and re-run after fixing the cause. The workflow refuses to
  overwrite an existing release.
- The performance optimization program is a post-GA stretch goal and does not block this tag.
