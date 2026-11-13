---
type: behavior
title: "Weave Git Source Behavior Scenarios"
description: "Behavior cases for git source providers, commit-pinned snapshots, repository nodes, provenance and workspace integration."
tags:
  - "behavior"
  - "git"
  - "source-provider"
  - "repository"
  - "graph"
status: draft
generated:
  by: opencode/deepseek-v4.1-flash
---

# Behavior Scenarios: Weave Git Source

Specification: [weave-git.md](https://github.com/prulloac/weave-docs/blob/main/specs/weave-git.md)

Architecture decision:
[git repositories as source providers](https://github.com/prulloac/weave-docs/blob/main/architecture/adr/0011-git-repositories-as-source-providers.md)

Planned Go tests: `cmd/weave/weave_git_bdd_test.go`, `engine/extensions/source_test.go`,
`engine/operations/source_test.go`, `storage/sqlite/schema_v9_test.go`, `mcp/workspace_test.go`, and
the standalone `weave-git` repository tests.

These scenarios describe observable behavior. They do not prescribe implementation details.

---

## Feature: Source discovery and acquisition

### Scenario: A local git repository is ingested explicitly

- **Given** a local git repository with a README and Go source
- **When** I run `weave ingest <path> --git --with-enricher code`
- **Then** the commit is resolved and a snapshot is materialized
- **And** the repository becomes a `Repository` document
- **And** the Go file becomes a `Code` document

### Scenario: A remote-style git URI is ingested explicitly

- **Given** a reachable `git+file://` fixture repository
- **When** I run `weave ingest git+file:///path/to/repo --with-enricher code`
- **Then** the provider fetches the repository into the cache
- **And** the repository and source files are indexed
- **And** nothing is written inside the fixture repository

### Scenario: Network access is never implicit

- **Given** a workspace that contains a git source
- **When** I open the workspace through MCP
- **Then** MCP serves the last indexed snapshot
- **And** it does not fetch, clone or contact the remote

### Scenario: An unknown git host works

- **Given** a reachable repository on a self-hosted git server
- **When** I ingest it by URI
- **Then** ingestion succeeds without a GitHub-specific API
- **And** repository metadata names the detected host when available

### Scenario: A missing provider fails clearly

- **Given** no `weave-git` provider is installed
- **When** I ingest a git URI
- **Then** the command fails with `EXTENSION_NOT_FOUND`
- **And** no partial source is indexed

## Feature: Ref resolution and stable identity

### Scenario: Branch, tag and commit refs resolve

- **Given** a repository with a branch, a tag and a known commit
- **When** I ingest each ref explicitly
- **Then** each snapshot records the resolved full commit
- **And** the requested ref is preserved in provenance

### Scenario: An unknown ref fails

- **Given** a repository without the requested branch
- **When** I ingest with that ref
- **Then** the command fails with `INVALID_REQUEST`
- **And** it does not fall back to another branch

### Scenario: The same commit is a no-op

- **Given** a repository already indexed at commit A
- **When** I ingest the same source and commit again
- **Then** the existing snapshot is reused
- **And** the graph is unchanged

### Scenario: A branch update reuses the source identity

- **Given** a repository indexed at commit A on `main`
- **When** `main` advances to commit B and I ingest again
- **Then** the snapshot worktree path stays the same
- **And** provenance records commit B
- **And** the previous documents and edges for that source are replaced
- **And** no stale bundle root remains

### Scenario: A ref change reuses the worktree

- **Given** a repository indexed at `main`
- **When** I ingest the same source at `v0.8.0`
- **Then** the same worktree path is used
- **And** provenance records the new ref and commit

### Scenario: Offline reuse works

- **Given** a repository already cached at commit A
- **When** the remote is unavailable and I request commit A
- **Then** ingestion succeeds from cache
- **And** no network call is required

### Scenario: A failed update keeps the previous snapshot

- **Given** an indexed source at commit A
- **When** an update to commit B fails before the worktree swap
- **Then** the source remains served at commit A
- **And** the source status reports `degraded` in permissive mode
- **And** no partial graph is written

### Scenario: Worktree mode is local-only

- **Given** a local repository with uncommitted changes
- **When** I ingest it with `--git --worktree`
- **Then** the current working tree is indexed
- **And** provenance records the current `HEAD` and a dirty flag
- **When** I request worktree mode for a remote URI
- **Then** the command fails with `INVALID_REQUEST`

## Feature: Snapshot cache

### Scenario: Snapshots live outside the bundle

- **Given** a git source
- **When** it is ingested
- **Then** the snapshot is stored under `WEAVE_CACHE_DIR`
- **And** the source bundle is not modified

### Scenario: The worktree is a real directory

- **Given** an indexed git source
- **When** core scans the source
- **Then** the bundle root is a real `worktree` directory
- **And** it is traversed like any local bundle root

### Scenario: Cache cleanup preserves active sources

- **Given** an active git source
- **When** cache garbage collection runs
- **Then** its worktree is retained
- **And** unreferenced git objects may be removed

## Feature: Repository document

### Scenario: The repository is a graph node

- **Given** a repository with a root README
- **When** it is ingested
- **Then** a `Repository` document with ID `_repo` exists
- **And** its body is the README Markdown
- **And** its title comes from the README or repository name

### Scenario: Description comes from README frontmatter

- **Given** a README with a `description` field
- **When** it is ingested
- **Then** the repository description is the frontmatter value

### Scenario: Description falls back to the first paragraph

- **Given** a README without frontmatter description and with a heading then a paragraph
- **When** it is ingested
- **Then** the repository description is the first paragraph

### Scenario: Missing README still creates the repository

- **Given** a repository without a root README
- **When** it is ingested
- **Then** the `_repo` document exists with an empty body
- **And** one warning reports the missing README

### Scenario: Repository provenance is complete

- **Given** an indexed git source
- **When** I show the repository document
- **Then** it records the URI, ref, commit, host when available, and worktree flag

## Feature: Graph relations

### Scenario: Repository contains source files

- **Given** an indexed repository
- **When** I inspect the graph
- **Then** `contains` edges link `_repo` to repository files
- **And** default traversal reaches the files from `_repo`

### Scenario: Existing code relations are unchanged

- **Given** an indexed repository with imports and calls
- **When** I inspect the graph
- **Then** `imports`, `declares`, `calls`, `references`, `inherits` and `implements` behave as before

## Feature: Workspace and MCP

### Scenario: A workspace declares a git source

- **Given** `weave.workspace.yaml` with a `git:` source object and `ref`
- **When** I run `weave ingest --workspace <dir>`
- **Then** the git source is resolved and indexed with the other sources
- **And** the existing string-path source form still works

### Scenario: Source commands report git provenance

- **Given** an active git source
- **When** I run `weave source list --json` or `weave source show <alias> --json`
- **Then** the output includes provider, URI, ref and commit

### Scenario: MCP exposes git provenance

- **Given** a workspace with a git source
- **When** an MCP client reads `weave://workspace/sources`
- **Then** the source includes provider, URI, ref and commit
- **And** its status is `active` or `degraded`

### Scenario: MCP never fetches

- **Given** an MCP server serving a git-backed workspace
- **When** the client reads any resource
- **Then** the server performs no network operation
- **And** it serves only the persisted snapshot

## Feature: Security and limits

### Scenario: Credentials never leak

- **Given** a repository requiring authentication
- **When** authentication fails
- **Then** the error names the provider and failure class
- **And** the token, credential helper output and repository contents are absent from logs and JSON

### Scenario: Unsafe symlinks are skipped

- **Given** a repository with a symlink escaping the worktree
- **When** it is ingested
- **Then** the symlink is skipped with a warning
- **And** the rest of the repository is indexed

### Scenario: Submodules and LFS are opt-in

- **Given** a repository with submodules and LFS objects
- **When** it is ingested with defaults
- **Then** submodules are skipped and LFS pointers are not expanded
- **And** `--git --submodules` materializes committed submodule trees
- **And** `--git --lfs` expands LFS objects when `git-lfs` is available
- **And** enabling either option changes only that behavior

### Scenario: Oversized repositories fail safely

- **Given** a repository exceeding the configured project or file limits
- **When** it is ingested
- **Then** the source fails with a limit warning or error
- **And** no partial graph is written
