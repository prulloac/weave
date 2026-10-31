# BDD: Dry Run and Non-Interactive Safety

Human-readable behavior cases for write-command previews and `--no-input`.

**Spec reference:** [optimization.md § O2](../specs/optimization.md)
**Go tests:** `cmd/weave/dryrun_bdd_test.go`

---

## Feature: Dry Run

### Scenario: Ingest previews without a database

- **When** I run `weave ingest --dry-run --json`
- **Then** the report has `dry_run: true`
- **And** document and edge counts are present
- **And** no database file is created

### Scenario: Sync previews without writing

- **Given** an indexed bundle and a modified document
- **When** I run `weave sync --dry-run`
- **Then** the report has `dry_run: true`
- **And** the stored document is unchanged

### Scenario: Export previews without confirmation or keys

- **When** I run raw export with `--dry-run`
- **Then** the report has `dry_run: true`
- **And** no target directory is created
- **When** I preview encrypted export without a key
- **Then** it still succeeds

### Scenario: Human output labels previews

- **When** I run a dry run without `--json`
- **Then** stdout starts with `dry run: no changes written` or `no files written`

### Scenario: Dry run is limited to writes

- **When** I pass `--dry-run` to `search`
- **Then** the command exits `2`

---

## Feature: No Input

### Scenario: Raw export fails instead of prompting

- **When** I run raw export with `--no-input` and without `--yes`
- **Then** the command exits `2`
- **And** the error code is `CONFIRMATION_REQUIRED`
- **And** no prompt is shown

### Scenario: No input is limited to export

- **When** I pass `--no-input` to another command
- **Then** the command exits `2`
