---
type: specification
title: "Configuration Specification"
description: "Global YAML configuration and WEAVE_* environment variables with deterministic precedence and resolved-value inspection."
tags:
  - "specification"
  - "configuration"
  - "cli"
  - "ux"
  - "v1"
generated:
  by: opencode/deepseek-v4.1-flash
  at: "2026-10-29T00:00:00Z"
status: draft
---

# Specification: Configuration

Status: **Implemented** (file/env precedence, `config show|get|path|init`, typed provenance,
platform paths, strict validation and MCP database defaults).
Target: **v1.0**

## 1. Overview

Operators repeat the same flags on every command (`--db`, `--codec`, `--mode`, `--timeout`). This
slice adds a global YAML configuration file and `WEAVE_*` environment variables with deterministic
precedence, plus commands to inspect and initialize configuration.

Precedence, highest first:

1. CLI flag explicitly set on the invocation.
2. Environment variable.
3. Global configuration file.
4. Built-in default.

Secrets are never read from the configuration file; privacy keys stay in `WEAVE_KEY_*` and
`.weave/security.yaml`.

Non-goals:

- Project-local configuration files in v1.0; `.weave/config.yaml` is deferred to v1.1 by
  [ADR 0004](../architecture/adr/0004-config-paths-and-db-location.md).
- Moving the default database to an XDG state directory in v1.0; `~/.weave/index.db` remains the
  storage invariant until a migration is specified.
- Configuring `--json`, `--help` or output formatting.
- Storing key material, tokens or provider credentials in the config file.
- Hot reload of configuration while a process is running.
- TUI or interactive configuration editor.

## 2. Contracts

### 2.1 File location

Resolution order:

1. `--config <file>`
2. `WEAVE_CONFIG`
3. The platform config directory:
   - Linux/Unix: `$XDG_CONFIG_HOME/weave/config.yaml` when `XDG_CONFIG_HOME` is absolute;
     otherwise `~/.config/weave/config.yaml`.
   - Windows: `%AppData%\weave\config.yaml`.
   - macOS: `~/Library/Application Support/weave/config.yaml`.

Platform locations come from `os.UserConfigDir`, with an explicit absolute-XDG check on
Linux/Unix. The old unconditional `AppData` check is removed, so a stray `AppData` value no longer
changes Unix paths. A relative `XDG_CONFIG_HOME` is ignored, as required by the XDG Base Directory
Specification.

- A missing file is not an error; defaults apply.
- `--no-config` disables file loading but keeps environment variables.
- `--config` and `--no-config` are mutually exclusive; both are global flags.
- The parse cache uses `os.UserCacheDir()/weave/okf` when `cache_dir` and `WEAVE_CACHE_DIR` are
  empty; Linux honors `XDG_CACHE_HOME`, Windows uses `%LocalAppData%`, and macOS uses
  `~/Library/Caches`. The old temporary directory is only a fallback.

### 2.2 File format

```yaml
db: ~/.weave/index.db
no_db: false
cache: true
cache_dir: ""
codec: zstd
search_mode: hybrid
top_k: 10
ingest_mode: permissive
timeout: 5s
enricher: ""
rerank: ""
```

| Key | Type | Default | CLI equivalent |
| --- | --- | --- | --- |
| `db` | path | `~/.weave/index.db` | `--db` |
| `no_db` | bool | `false` | `--no-db` |
| `cache` | bool | `true` | `--no-cache` inverted |
| `cache_dir` | path | empty (user cache dir) | env `WEAVE_CACHE_DIR` |
| `codec` | `zstd\|lz4\|none` | `zstd` | `--codec` |
| `search_mode` | `hybrid\|keyword\|fuzzy\|bm25` | `hybrid` | `--mode` |
| `top_k` | int `1..100` | `10` | `--top-k` |
| `ingest_mode` | `strict\|permissive` | `permissive` | `--strict`/`--permissive` |
| `timeout` | duration | `5s` | `--timeout` |
| `enricher` | name | empty | `--with-enricher` |
| `rerank` | name | empty | `--rerank` |

- Paths support `~` expansion.
- The default database stays `~/.weave/index.db` in v1.0. `--db`, `WEAVE_DB` and `db` move it;
  no automatic state-directory migration happens. See
  [ADR 0004](../architecture/adr/0004-config-paths-and-db-location.md).
- Unknown keys are ignored with the warning `CONFIG_UNKNOWN_KEY`; `--strict-config` turns them into
  exit `2`.
- Type errors, invalid enum values, non-positive timeout and out-of-range `top_k` exit `2` with
  `CONFIG_INVALID`.
- `keys`, `security`, `token` and `api_key` are reserved and always rejected.
- The file is read once per process before dispatch.

### 2.3 Environment variables

| Variable | Key |
| --- | --- |
| `WEAVE_DB` | `db` |
| `WEAVE_NO_DB` | `no_db` |
| `WEAVE_CACHE` | `cache` |
| `WEAVE_CACHE_DIR` | `cache_dir` |
| `WEAVE_CODEC` | `codec` |
| `WEAVE_SEARCH_MODE` | `search_mode` |
| `WEAVE_TOP_K` | `top_k` |
| `WEAVE_INGEST_MODE` | `ingest_mode` |
| `WEAVE_TIMEOUT` | `timeout` |
| `WEAVE_ENRICHER` | `enricher` |
| `WEAVE_RERANK` | `rerank` |
| `WEAVE_CONFIG` | config path |
| `WEAVE_STRICT_CONFIG` | strict config parsing |

- Booleans accept `1`, `0`, `true`, `false`, `yes`, `no` case-insensitively.
- Invalid values exit `2` with `CONFIG_INVALID` naming the variable.
- Empty `WEAVE_ENRICHER`/`WEAVE_RERANK` disables the configured extension, matching “flag absent”.
- `WEAVE_KEY_*` is reserved for [Privacy Directives](./privacy-directives.md); configuration never
  reads or prints it.

### 2.4 Resolution and visibility

```sh
weave config show --json
weave config get <key> --json
weave config path --json
weave config init [--force]
```

`config show` reports every key with its effective value, source and origin:

```json
{
  "config_path": "/home/user/.config/weave/config.yaml",
  "exists": true,
  "entries": [
    {
      "key": "db",
      "value": "/srv/weave/index.db",
      "source": "env",
      "origin": "env WEAVE_DB"
    },
    {
      "key": "top_k",
      "value": 10,
      "source": "default",
      "origin": "default"
    }
  ]
}
```

- `source` is `flag`, `env`, `file` or `default`. `weave config show` itself takes no command flags;
  `flag` appears when a command's effective value was overridden on that invocation.
- `origin` names the exact file or environment variable, for example
  `file /home/user/.config/weave/config.yaml` or `env WEAVE_TOP_K`. It is additive and optional in
  the schema.
- Values are JSON-native; durations use their Go string form.
- `config get <key>` returns one object with `key`, `value`, `source` and `origin`. An unknown key
  exits `2` with `INVALID_REQUEST` and lists the known keys.
- `config path` prints the resolved file path and whether it exists. It does not parse the file.
- `config init` writes the commented default file at the resolved global path; it refuses to
  overwrite without `--force` and never writes secrets. It does not parse an existing file, so
  `config init --force` can repair an invalid file. Its result includes
  `next_command: "weave config show"`, and human output ends with `next: weave config show`.
- `config show` never prints `WEAVE_KEY_*` values.
- The four result types are `ConfigShow`, `ConfigGet`, `ConfigPath` and `ConfigInit` in
  `contract/v1`, validated by `config.schema.json`.

### 2.5 Machine-output policy

`weave.cli/v1` is additive inside v1.0:

- no field is removed and no field changes JSON type;
- new fields are optional;
- enum values may be added but not removed;
- every `--json` success result validates against its schema, and every failure validates against
  the envelope schema;
- `weave sync --watch --events --json` writes one initial success envelope followed by the
  documented JSONL `WatchEvent` stream;
- stdout carries only the envelope or JSONL stream; warnings, hints and progress go to stderr;
- `--json` never prompts and never emits progress.

`contract/v1/golden/schema-baseline.json` records the additive contract. `go run ./cmd/schema-diff`
fails on removed definitions, properties or variants, changed property shapes, and newly required
properties. New optional fields and enum values pass. See
[ADR 0004](../architecture/adr/0004-config-paths-and-db-location.md).

### 2.6 Applicability

- Bundle commands inherit `db`, `no_db`, `cache`, `codec`, `search_mode`, `top_k`,
  `ingest_mode`, `timeout`, `enricher` and `rerank` when the corresponding flag is not set.
- Commands that do not expose a key ignore it.
- `weave mcp` reads `db` from configuration; transport and token flags remain explicit.
- Explicit CLI flags always win, including `--no-db` over `db`.
- `--strict-config` and `--no-config` apply to every command, including `mcp`.

## 3. Behavior

1. Parse the command and global flags (`--config`, `--no-config`, `--strict-config`); global flags
   may appear before or after the command.
2. Resolve the config path: `--config`, `WEAVE_CONFIG`, then the platform config directory.
3. Load the chosen file if present and record each file origin.
4. Merge defaults → file → environment → explicit CLI flags, recording the source and origin.
5. Validate the merged result; fail with `CONFIG_INVALID` before any bundle or database access.
6. Dispatch with the resolved values. `config show|get|path|init` consume the same resolved config.

Interactive confirmation behavior is unchanged; configuration never auto-confirms destructive
commands.

## 4. Data Flow

```text
platform paths (os.UserConfigDir / os.UserCacheDir)
defaults
  <- file (WEAVE_CONFIG / --config / platform config dir)
  <- environment (WEAVE_*)
  <- explicit CLI flags
  -> validated config with source + origin per key
  -> flag defaults for dispatch
  -> commands, operations, MCP
```

## 5. Validation and Errors

| Condition | Code | Behavior |
| --- | --- | --- |
| Unknown key | `CONFIG_UNKNOWN_KEY` | Warning; exit `2` with `--strict-config` |
| Invalid type/value | `CONFIG_INVALID` | Exit `2`, names key or variable |
| Reserved key present | `CONFIG_INVALID` | Exit `2` |
| `--config` and `--no-config` together | `INVALID_REQUEST` | Exit `2` |
| Config file unreadable | `CONFIG_INVALID` | Exit `2` |
| Reserved environment key | `CONFIG_INVALID` | Exit `2` |
| `config get` with an unknown key | `INVALID_REQUEST` | Exit `2`, lists known keys |

## 6. Testing

- Precedence matrix across defaults, file, environment and flags for each key.
- `WEAVE_CONFIG`, `--config`, `--no-config` and strict mode.
- `os.UserConfigDir` resolution on each platform; relative `XDG_CONFIG_HOME` is ignored; Unix
  never uses `AppData`.
- The parse cache uses `os.UserCacheDir` and honors `XDG_CACHE_HOME`.
- Unknown keys warn; strict mode fails.
- Reserved keys and malformed values fail with `CONFIG_INVALID`.
- `config show/get/path/init` JSON and human output, including source and origin attribution.
- `config get` rejects unknown keys with the known-key list.
- `config init` refuses overwrite; `--force` replaces; file mode is `0600`; output ends with
  `next: weave config show`.
- Config JSON validates against `config.schema.json`; CLI goldens cover show/get/path/init.
- No key material appears in output or error messages.
- Config-driven commands match the equivalent explicit flags (goldens unchanged).
- MCP reads `db` but not transport settings.
- Race suite and docs-lint.

## 7. Acceptance Criteria

- A global YAML file and `WEAVE_*` variables can replace repeated flags without changing behavior.
- Precedence is deterministic and visible through `weave config show` and `weave config get`.
- Config, cache and database paths follow one documented decision record across operating systems.
- Invalid or reserved configuration fails before any bundle or database access.
- Secrets are never read from or printed by configuration.
- `config init` produces a usable, commented file with restrictive permissions and a next command.
- Every `--json` result validates against its schema; additive-only changes are enforced by
  `cmd/schema-diff`.
- The full verification gate stays green.

## Related Documents

- [ADR 0004: Configuration Paths, Database Location and Machine Output](../architecture/adr/0004-config-paths-and-db-location.md)
- [Optimization Program](./optimization.md)
- [Storage & Sync](./storage-and-sync.md)
- [Privacy Directives](./privacy-directives.md)
- [MCP Server](./mcp-server.md)
- [Contract v1 README](../../contract/v1/README.md)
- [ROADMAP](../../ROADMAP.md)
