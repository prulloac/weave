# BDD: Machine Output

Human-readable behavior cases for `weave.cli/v1` envelopes, config output, watch JSONL and the
additive-only schema contract.

**Spec reference:** [configuration.md § 2.5](../specs/configuration.md#25-machine-output-policy)
**Go tests:** `cmd/weave/output_schema_bdd_test.go`, `cmd/weave/config_golden_test.go`,
`cmd/schema-diff/main_test.go`, `cmd/weave/sync_watch_bdd_test.go`

---

## Feature: Envelope validation

### Scenario: Every operation result validates

- **When** I run any command with `--json`
- **Then** stdout is exactly one `weave.cli/v1` envelope
- **And** the envelope validates against `envelope.schema.json`
- **And** a success result validates against its operation schema

Covered operations: `contract`, `version`, `parse`, `graph`, `search`, `list`, `show`,
`backlinks`, `path`, `audit`, `relate`, `explain`, `ingest`, `rebuild`, `sync`, `extensions`,
`export`, plus multi-source `parse`/`graph` and `config show|get|path|init`.

### Scenario: Error envelopes validate

- **When** a command fails with `--json`
- **Then** stdout is one failure envelope
- **And** the error validates against the envelope schema
- **And** diagnostics go to stderr

### Scenario: JSON mode never prompts

- **When** I run raw export with `--json` and without `--yes`
- **Then** no prompt is shown
- **And** the command fails with `CONFIRMATION_REQUIRED`

### Scenario: Watch emits documented JSONL

- **When** I run `weave sync --watch --json --events`
- **Then** the first line is a success envelope with a `SyncReport`
- **And** later lines are `WatchEvent` objects
- **And** every line validates against its schema

---

## Feature: Additive schema policy

### Scenario: Optional additions pass

- **When** a schema adds an optional property
- **Then** `go run ./cmd/schema-diff` succeeds

### Scenario: Removals and type changes fail

- **When** a definition, property or schema variant is removed
- **Then** the schema diff fails
- **When** an existing property changes type or shape
- **Then** the schema diff fails
- **When** a new property is required
- **Then** the schema diff fails

### Scenario: Config goldens exist

- **When** I run `config show|get|path|init --json`
- **Then** the normalized result matches `contract/v1/golden/config/*.json`
- **And** the golden validates against `config.schema.json`
