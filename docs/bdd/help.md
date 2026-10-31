# BDD: Help and CLI Grammar

Human-readable behavior cases for help discoverability and `--flag=value` parsing.

**Spec reference:** [optimization.md § O2](../specs/optimization.md)
**Go tests:** `cmd/weave/help_bdd_test.go`

---

## Feature: Discoverability

### Scenario: Top-level help is current

- **When** I run `weave help`
- **Then** it says Weave is a local-first database
- **And** it groups read, write and integration commands
- **And** it no longer mentions `mount`

### Scenario: Per-command help is available

- **When** I run `weave help search` or `weave search --help`
- **Then** I see usage, flags and examples
- **When** I run `weave config --help`
- **Then** I see config commands and precedence

### Scenario: Help lists global configuration

- **Then** top-level help lists `--config`, `--no-config` and exit codes

---

## Feature: Grammar

### Scenario: `--flag=value` is accepted

- **When** I run `weave list <path> --type=Concept --limit=1 --no-db --json`
- **Then** the command succeeds with one result

### Scenario: Config paths accept equals syntax

- **When** I run `--config=<file>`
- **Then** the config file is applied
