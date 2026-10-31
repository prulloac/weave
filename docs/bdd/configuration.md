# BDD: Global Configuration

Human-readable behavior cases for the global config file, environment variables and inspection
commands.

**Spec reference:** [configuration.md](../specs/configuration.md)
**Go tests:** `cmd/weave/config_bdd_test.go`, `cmd/weave/config_ux_bdd_test.go`,
`cmd/weave/config_golden_test.go`

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

## Feature: Paths

### Scenario: Config uses the platform config directory

- **Given** no `--config` or `WEAVE_CONFIG`
- **When** I run `weave config path --json`
- **Then** the path is under `os.UserConfigDir()/weave/config.yaml`
- **And** on Unix a stray `AppData` value is ignored

### Scenario: Relative XDG_CONFIG_HOME is ignored

- **Given** `XDG_CONFIG_HOME` is a relative path
- **When** I resolve the config path
- **Then** the path falls back to `~/.config/weave/config.yaml` on Linux/Unix

### Scenario: WEAVE_CONFIG selects the file

- **Given** `WEAVE_CONFIG` points at a file
- **When** I run `weave config path --json`
- **Then** that file is reported

### Scenario: Parse cache uses the user cache directory

- **Given** `cache_dir` and `WEAVE_CACHE_DIR` are empty
- **When** the parser caches a bundle
- **Then** the cache lives under `os.UserCacheDir()/weave/okf`

### Scenario: Database stays at the v1.0 path

- **Given** no `--db`, `WEAVE_DB` or `db` key
- **Then** the database is `~/.weave/index.db`
- **And** no state-directory migration runs

---

## Feature: Inspection

### Scenario: Show reports typed values, sources and origins

- **When** I run `weave config show --json`
- **Then** entries carry `key`, `value`, `source` and `origin`
- **And** the result carries `config_path` and `exists`
- **And** environment values report `source: "env"` and `origin: "env WEAVE_<NAME>"`
- **And** file values report `origin: "file <path>"`

### Scenario: Get returns one key

- **When** I run `weave config get top_k --json`
- **Then** it reports the effective value, source and origin
- **When** the key is unknown
- **Then** it exits `2`
- **And** lists the known keys

### Scenario: Path reports the resolved file

- **When** I run `weave config path --json`
- **Then** it reports the path and whether it exists
- **And** it does not parse an invalid file

### Scenario: Init repairs an invalid file with force

- **Given** the config file contains an invalid value
- **When** I run `weave config init --force --json`
- **Then** it succeeds and replaces the file

### Scenario: Init writes a default file and next command

- **When** I run `weave config init`
- **Then** the file is created with `0600` permissions
- **And** output ends with `next: weave config show`
- **And** JSON includes `next_command`
- **When** I run it again
- **Then** it exits `2`
- **When** I pass `--force`
- **Then** the file is replaced

### Scenario: Config JSON matches goldens

- **When** I run `config show`, `config get`, `config path` or `config init` with `--json`
- **Then** stdout is one envelope
- **And** the result validates against `config.schema.json`
- **And** show/get/path/init match their committed goldens after path normalization
