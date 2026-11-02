# BDD: Onboarding and Completion

Human-readable behavior cases for the start screen, empty states and shell completion.

**Spec reference:** [optimization.md § O2](https://github.com/prulloac/weave-docs/blob/main/specs/optimization.md)
**Go tests:** `cmd/weave/onboarding_bdd_test.go`

---

## Feature: Start here

### Scenario: No arguments show the quickstart

- **When** I run `weave` with no arguments
- **Then** stdout contains `start here`
- **And** shows `weave ingest` and `weave search`
- **And** does not dump the full command reference

### Scenario: Help still shows the full reference

- **When** I run `weave --help` or `weave help`
- **Then** the grouped command reference is printed

---

## Feature: Empty states

### Scenario: Empty list suggests ingest

- **When** no concepts match the filters
- **Then** the command exits `0`
- **And** the hint mentions `weave ingest <path>`

### Scenario: No extensions suggests installation

- **When** `weave extensions list` finds nothing
- **Then** the command exits `0`
- **And** the hint names `.weave/bin` or `PATH`

### Scenario: Unindexed bundle names the next command

- **When** a read targets an unindexed bundle
- **Then** the error code is `BUNDLE_NOT_INDEXED`
- **And** the hint contains `weave ingest <path>`

---

## Feature: Completion

### Scenario: Supported shells print scripts

- **When** I run `weave completion bash|zsh|fish`
- **Then** the matching completion script is printed on stdout
- **And** it includes the command names

### Scenario: Unknown shell fails

- **When** I run `weave completion powershell`
- **Then** the command exits `2`

### Scenario: Completion help

- **When** I run `weave completion --help` or `weave help completion`
- **Then** usage and examples are printed
