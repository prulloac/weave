---
type: specification
title: "Release Automation Specification"
description: "Tag-driven GitHub Actions release workflow: verified builds, artifact matrix, checksums and draft GitHub releases."
tags:
  - "specification"
  - "release"
  - "ci"
  - "github-actions"
  - "v1"
generated:
  by: opencode/deepseek-v4.1-flash
  at: "2026-10-29T00:00:00Z"
status: draft
---

# Specification: Release Automation

Status: **Implemented** (workflow, packaging helpers and version injection).
Target: **v1.0**

## 1. Overview

Releases are produced by one GitHub Actions workflow triggered by an annotated `v*` tag. The
workflow verifies the repository, builds a fixed OS/architecture matrix, publishes checksums and
attaches everything to a **draft** GitHub Release. A maintainer reviews and publishes; no registry,
package manager or container image is involved.

Non-goals:

- Code signing, notarization or SLSA attestations.
- Homebrew, apt, Docker Hub or module proxy publishing.
- Automatic changelog curation beyond generated release notes.
- Release branches or long-term support lines.

## 2. Trigger and Permissions

```yaml
on:
  push:
    tags: ["v*"]
  workflow_dispatch:
    inputs:
      version:
        description: "Version for a dry-run build (e.g. v1.2.3)"
        required: true
```

- Tags must match `v<semver>` (`v1.2.3`, `v1.2.3-rc.1`); other tags are ignored.
- `workflow_dispatch` builds and uploads workflow artifacts only; it never creates a release.
- Permissions: `contents: write` for the release job, `contents: read` for verification/build jobs.
- The workflow uses the repository `GITHUB_TOKEN`; no personal tokens.
- Concurrency group `release-<ref>` cancels superseded runs.

## 3. Pipeline

1. **Verify** (ubuntu-latest): checkout, install the Go version from `go.mod`, run `gofmt -l`,
   `go vet ./...`, `go test ./...`, `go test -race ./...`, contract goldens, docs-lint and
   markdownlint via `./scripts/verify.sh`.
2. **Build** (matrix): compile static binaries with `CGO_ENABLED=0`, `-trimpath` and
   `-ldflags "-s -w -X main.version=<version>"`.
3. **Package**: archive each binary, generate SHA-256 checksums and a build-info file.
4. **Release** (ubuntu-latest, tag events only): create a draft GitHub Release and attach all
   artifacts; generate notes from commits since the previous tag.
5. **Verify artifacts**: download the linux/amd64 archive and run `weave version --json` and
   `weave contract --json` as a smoke test.

Builds fail closed: a failed verification job blocks build and release.

## 4. Build Matrix and Artifacts

| OS | Architectures | Archive |
| --- | --- | --- |
| linux | amd64, arm64 | `.tar.gz` |
| darwin | amd64, arm64 | `.tar.gz` |
| windows | amd64, arm64 | `.zip` |

Artifact names:

```text
weave_<version>_<os>_<arch>.tar.gz
weave_<version>_<os>_<arch>.zip
weave_<version>_checksums.txt
weave_<version>_buildinfo.txt
```

- `<version>` omits the leading `v` (`v1.2.3` → `weave_1.2.3_linux_amd64.tar.gz`).
- Archives contain a single `weave` binary (`weave.exe` on Windows) and `README.md`; `LICENSE` is
  included when the repository has one. Weave currently ships without a license file, so release
  archives omit it until licensing is decided.
- `checksums.txt` lists every archive with lowercase SHA-256, sorted by filename.
- `buildinfo.txt` records version, commit SHA, Go version and build timestamp.
- `weave version` prints the tag version, never `dev`.

## 5. Release Notes and Publishing

- Notes are generated from conventional commits since the previous tag, grouped as `feat`, `fix`,
  `docs`, `test` and `perf`.
- The release is created as `draft: true` with `prerelease: true` when the tag contains `-rc`/`-beta`.
- The draft title is the tag; the body starts with a one-line summary and installation commands.
- A maintainer publishes manually after checking artifacts; publishing is not automated by tag push.
- Re-running a tag build is idempotent: if a release for the tag exists, the job fails with a clear
  message instead of overwriting assets.
- A failed publish can be retried with `workflow_dispatch` plus a manual `gh release upload`.

## 6. Configuration and Secrets

- No repository secrets are required for a normal release.
- Optional `release` environment protection adds reviewer approval before the release job.
- Go module downloads use the public proxy; no vendor directory is committed.
- Builds must be reproducible for the same commit, Go version and version string; timestamps are the
  only nondeterministic field and live in `buildinfo.txt`.

## 7. Validation and Errors

| Condition | Behavior |
| --- | --- |
| Tag not matching `v<semver>` | Workflow exits successfully without releasing |
| Verification or race failure | Build and release skipped |
| Build failure for one target | Whole release fails; no partial release |
| Existing release for tag | Fail with a diagnostic; assets untouched |
| Missing `contents: write` | Release job fails before creating anything |
| Smoke test failure | Draft release remains for inspection, job fails |

## 8. Testing

- Workflow syntax validated with `actionlint` in CI.
- A `workflow_dispatch` dry run builds the full matrix and uploads workflow artifacts.
- Unit coverage for the version-injection path: `weave version --json` reports the injected value.
- Archive layout and checksum sorting verified by a packaging script test.
- Release notes grouping verified against a fixture commit range.
- Docs-lint covers workflow and release documentation links.

## 9. Acceptance Criteria

- Pushing an annotated `v<semver>` tag produces a draft GitHub Release with six archives, checksums
  and build info after the full verification gate passes.
- `weave version --json` reports the tag version in every artifact.
- A dry run with `workflow_dispatch` publishes artifacts without creating a release.
- No repository secrets are needed; permissions are minimal and explicit.
- Failed verification or builds never publish a partial release.

## Related Documents

- [Storage & Sync](./storage-and-sync.md)
- [Extension Subprocess IPC](./extension-ipc.md)
- [Architecture](../architecture/index.md)
- [ROADMAP](../../ROADMAP.md)
