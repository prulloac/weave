---
type: Architecture Decision Record
title: "ADR 0004: Configuration Paths, Database Location and Machine Output"
description: "Uses platform config and cache directories, keeps the v1.0 database at ~/.weave/index.db, and pins additive machine output."
tags:
  - "architecture"
  - "adr"
  - "configuration"
  - "paths"
  - "json"
  - "ux"
generated:
  by: opencode/deepseek-v4.1-flash
  at: "2026-10-31T00:00:00Z"
status: accepted
---

# ADR 0004: Configuration Paths, Database Location and Machine Output

## Status

Accepted on 2026-10-31 for v1.0. It complements [ADR 0003](0003-keyword-fuzzy-search-and-ui-retirement.md)
and the [Configuration specification](../../specs/configuration.md).

## Context

Configuration already supported a global YAML file, `WEAVE_*` variables and
`weave config show|path|init`. Four problems remained:

- `defaultConfigPath` checked Windows `AppData` on every OS and only honored `XDG_CONFIG_HOME`.
- the parse cache defaulted to `os.TempDir()`, which is not a durable user cache.
- the database path (`~/.weave/index.db`) was never compared with an XDG state directory.
- `config show` returned untyped maps and no per-key origin, and `config init` stopped after
  writing the file.

At the same time, scripts and agents consume `--json`. Output shapes need an explicit
compatibility policy, schemas and a regression check rather than informal stability.

## Decision Summary

| ID | Decision |
| --- | --- |
| D1 | Resolve config with the platform config directory |
| D2 | Resolve the parse cache with the platform cache directory |
| D3 | Keep `~/.weave/index.db` as the v1.0 default database |
| D4 | Defer project-local `.weave/config.yaml` to v1.1 |
| D5 | Type config results, add `config get`, and print the next command after `init` |
| D6 | Make `weave.cli/v1` output additive-only with schemas, goldens and a schema diff |

## D1 — Platform config directory

### Decision

- `--config <file>` wins.
- `WEAVE_CONFIG` wins next.
- On Linux and other Unix systems, an **absolute** `XDG_CONFIG_HOME` resolves
  `$XDG_CONFIG_HOME/weave/config.yaml`; a relative value is ignored as the XDG specification
  requires, falling back to `~/.config/weave/config.yaml`.
- Windows uses `%AppData%\weave\config.yaml`; macOS uses
  `~/Library/Application Support/weave/config.yaml`. Both come from `os.UserConfigDir`, not a
  hand-written `AppData` check.

### Consequences

The Windows environment variable no longer changes Unix resolution. A missing file remains a
non-error, and `--no-config` still disables file loading.

## D2 — Platform cache directory

### Decision

When `cache_dir` and `WEAVE_CACHE_DIR` are empty, the parse cache uses
`os.UserCacheDir()/weave/okf`. On Linux that honors `XDG_CACHE_HOME` and otherwise uses
`~/.cache`; Windows uses `%LocalAppData%`; macOS uses `~/Library/Caches`. If the platform cache
directory cannot be resolved, the old temporary directory remains the fallback.

### Consequences

Cache files survive reboots and are separated by platform convention. `--no-cache` and explicit
cache directories are unchanged.

## D3 — Database location

### Decision

v1.0 keeps the bundle-scoped SQLite default at `~/.weave/index.db`.

- `--db`, `WEAVE_DB` and the config file `db` key continue to override it.
- No automatic move, copy or migration of an existing database happens in v1.0.
- An XDG state directory (`$XDG_STATE_HOME/weave/index.db` or
  `~/.local/state/weave/index.db`) is **not** adopted in v1.0.

### Rationale

The default database path is a public storage invariant used by documentation, MCP configuration
and the sync/watch contract. Moving it during the v1.0 release would require a migration for
existing indexes and would make an otherwise read-only upgrade mutate user data. A future release
can add a state directory with an explicit, reversible migration; that requires its own decision
record.

### Consequences

`XDG_STATE_HOME` is not used for storage in v1.0. `weave config show` reports the effective
database path and `weave config get db` makes the value scriptable.

## D4 — Project configuration

### Decision

v1.0 keeps one global configuration file. A project-local `.weave/config.yaml` is deferred to
v1.1. Precedence remains explicit CLI flag, then environment, then file, then default.

### Rationale

Project configuration adds a second scope to every command, including write and privacy commands.
It needs its own precedence, discovery and trust model. Shipping it without that model would make
the existing provenance output ambiguous.

## D5 — Typed configuration results

### Decision

- `weave config show --json` returns `config_path`, `exists` and typed entries with `key`, `value`,
  `source` and `origin`.
- `weave config get <key> --json` returns one typed value with the same provenance fields.
- `weave config path --json` returns `path` and `exists`.
- `weave config init --json` returns `path`, `created` and `next_command`; human output ends with
  `next: weave config show`.

`source` remains `default`, `file`, `env` or `flag`. `origin` names the exact file or environment
variable, for example `file /home/user/.config/weave/config.yaml` or `env WEAVE_TOP_K`.

### Consequences

Existing scripts that read `config_path` and `entries[].source` keep working. New fields are
additive. `config show` does not expose `WEAVE_KEY_*` values.

## D6 — Additive machine output

### Decision

Within `weave.cli/v1`:

- no result field is removed or changes JSON type;
- new fields are optional;
- enum values may be added but not removed;
- every result has a JSON Schema and a golden where the output is deterministic;
- one envelope is written to stdout, or the documented JSONL stream for
  `weave sync --watch --events --json`;
- diagnostics go to stderr, and `--json` never prompts or prints progress.

A committed schema baseline (`contract/v1/golden/schema-baseline.json`) is checked by
`cmd/schema-diff`. The check rejects removed definitions, properties or schema variants, changed
property shapes, and newly required properties.

### Consequences

Breaking changes require `weave.contract/v2` or `weave.cli/v2`. Additive fields need a schema and
golden update in the same change.

## Related Documents

- [Configuration specification](../../specs/configuration.md)
- [Optimization program](../../specs/optimization.md)
- [UX proposal](../optimization/proposal-ux.md)
- [Contract README](../../../contract/v1/README.md)
- [ROADMAP](../../../ROADMAP.md)
