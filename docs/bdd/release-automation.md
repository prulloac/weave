# BDD: Release Automation

Human-readable behavior cases for the tag-driven release workflow and its packaging helpers.

**Spec reference:** [release-automation.md](../specs/release-automation.md)
**Go tests:** `release/release_test.go`, `cmd/weave/release_bdd_test.go`

The workflow verifies the repository, builds six static targets, packages archives, writes sorted
checksums and creates a **draft** GitHub Release. Packaging helpers are covered directly.

---

## Feature: Version Injection

### Scenario: Built binary reports the release version

- **Given** a build with `-X main.version=v9.9.9`
- **When** I run `weave version --json`
- **Then** `binary_version` is `v9.9.9`
- **And** `weave --version` prints the same version

### Scenario: Development builds report dev

- **Given** a plain `go build` without ldflags
- **When** I run `weave version`
- **Then** the version is `dev`

---

## Feature: Artifact Packaging

### Scenario: Archive names follow the contract

- **When** I package linux/amd64, darwin/arm64 and windows/amd64
- **Then** names are `weave_<version>_<os>_<arch>.tar.gz`
- **And** Windows uses `.zip`

### Scenario: Archives contain the binary and docs

- **Given** a built binary
- **When** I package it
- **Then** the archive contains `weave` (or `weave.exe`), `README.md` and `LICENSE`
- **And** the binary is executable inside tar archives

### Scenario: Checksums are sorted and correct

- **Given** several archives
- **When** I write `weave_<version>_checksums.txt`
- **Then** lines are sorted by filename
- **And** each SHA-256 matches the archive bytes

### Scenario: Build info records provenance

- **When** I render build info
- **Then** it contains version, commit, Go version and build timestamp

---

## Feature: Workflow Contract

### Scenario: Tag pushes trigger a verified build

- **Given** `.github/workflows/release.yml`
- **Then** it listens to `v*` tag pushes and `workflow_dispatch`
- **And** the `build` job needs `verify`
- **And** `release` needs `build`
- **And** verification runs `./scripts/verify.sh`

### Scenario: The build matrix covers six targets

- **Then** the matrix includes linux, darwin and windows on amd64 and arm64
- **And** builds use `CGO_ENABLED=0` and `-trimpath`
- **And** version injection uses `-X main.version`

### Scenario: Releases are drafts

- **Then** the release job runs only for `refs/tags/v*`
- **And** it creates a draft GitHub Release with `contents: write`
- **And** pre-release tags are marked prerelease

### Scenario: Dry runs do not publish

- **Given** a `workflow_dispatch` run
- **Then** artifacts are uploaded to the workflow
- **And** no GitHub Release is created
