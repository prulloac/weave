# BDD: Global Configuration

Human-readable behavior cases for the global config file, environment variables and inspection
commands.

**Spec reference:** [configuration.md](../specs/configuration.md)
**Go tests:** `cmd/weave/config_bdd_test.go`

Precedence: explicit CLI flag → `WEAVE_*` environment variable → config file → default.

---

## Feature: Precedence

### Scenario: File values apply when flags are absent

- **Given** a config file with `search_mode: keyword` and `top_k: 2`
- **When** I run search without those flags
- **Then** the response mode is `keyword`
- **And** top-K is `2`

### Scenario: Environment overrides the file

- **When** `WEAVE_SEARCH_MODE=hybrid` and `WEAVE_TOP_K=1`
- **Then** search uses those values

### Scenario: CLI flags override the environment

- **When** I pass `--mode fuzzy --top-k 3`
- **Then** the response uses fuzzy mode and top-K 3

### Scenario: Config database is used by default

- **Given** `db: <path>` in the config file
- **When** I ingest without `--db`
- **Then** the report points at that database

### Scenario: Environment database overrides the file

- **When** `WEAVE_DB` is set
- **Then** ingest uses the environment database

---

## Feature: Validation

### Scenario: Invalid values fail

- **When** `top_k` is outside `1..100`
- **Then** the command exits `2`
- **And** names the invalid key

### Scenario: Reserved keys fail

- **When** the file contains `keys`, `security`, `token` or `api_key`
- **Then** the command exits `2`

### Scenario: Unknown keys warn

- **When** the file contains an unknown key
- **Then** the command succeeds
- **And** stderr contains `CONFIG_UNKNOWN_KEY`

### Scenario: Strict config fails unknown keys

- **When** I pass `--strict-config`
- **Then** the command exits `2`

### Scenario: Config controls are exclusive

- **When** I pass `--config` and `--no-config`
- **Then** the command exits `2`

### Scenario: No-config ignores the file

- **When** I pass `--no-config`
- **Then** file values are ignored
- **And** environment variables still apply

---

## Feature: Inspection

### Scenario: Show reports values and sources

- **When** I run `weave config show --json`
- **Then** entries carry `key`, `value` and `source`
- **And** environment values report `source: "env"`

### Scenario: Path reports the resolved file

- **When** I run `weave config path --json`
- **Then** it reports the path and whether it exists

### Scenario: Init writes a default file

- **When** I run `weave config init`
- **Then** the file is created with `0600` permissions
- **When** I run it again
- **Then** it exits `2`
- **When** I pass `--force`
- **Then** the file is replaced
