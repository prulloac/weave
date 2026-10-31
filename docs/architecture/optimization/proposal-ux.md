---
type: proposal
title: "User Experience Proposal: A Predictable, Discoverable CLI"
description: "Shippable CLI UX workstreams for help, errors, safety, configuration, machine output, feedback, onboarding and local-first trust."
tags:
  - proposal
  - ux
  - cli
  - configuration
  - optimization
generated:
  by: subagent
  at: "2026-10-31T02:45:10Z"
status: draft
---

# Proposal: User Experience

## Context and goals

Weave has no UI; the terminal is the product. Deterministic envelopes, documented exits, per-key
config provenance and gated plaintext export are strengths. But help advertises a retired "mount"
model, command help is a stub, human errors omit JSON codes, and paths and exits vary. Goals: first
search in ≤2 commands; every non-zero exit names a cause, code and next step; no surprise writes or
off-TTY prompts; additive, schema-pinned JSON; predictable paths. No TUI, web UI or daemon.

## Evidence

- **Stale, stubbed help.** `printHelp` says "mount OKF bundles locally" and usage walls
  (`main.go:1273`); `printCommandHelp` is one line (`main.go:1306`); `weave help search` ignores its
  argument (`main.go:39`); no help topics exist; `weave config --help` exits 2.
- **Grammar.** `--db=/tmp/x.db` is rejected by `parseFlags` (`main.go:296`); the command must come
  first, so `weave --json search` fails.
- **Exit drift.** Human ingest/rebuild/sync return 0 when `report.Errors > 0` (`main.go:1061`), JSON
  1 (`main.go:867`); empty `list` is 1 but empty `backlinks` 0; watch Ctrl-C is 0
  (`sync_watch.go:118`); cancelled export is 2 (`main.go:182`).
- **Safety gaps.** Export alone prompts (consent vs overwrite split is right); no `--dry-run` or
  `--no-input`; a 10k ingest can run ~30 s silently.
- **Config.** `config show` reports sources and `config path` works, but output is untyped
  (`config.go:373`), XDG support stops at `XDG_CONFIG_HOME`, `AppData` is checked on every OS
  (`config.go:65`), and `WEAVE_CONFIG` is missing from README.

## Proposed workstreams

Effort, risk and dependencies are per workstream.

| ID | Workstream | Effort | Risk | Depends on |
| --- | --- | --- | --- | --- |
| U1 | Help, grammar and discoverability | M | Help drift | — |
| U2 | Error and exit contract | M/L | Script breakage | U1 |
| U3 | Safety | M | Dry-run drift | U2 |
| U4 | Configuration UX | M | DB migration | U1, U5 |
| U5 | Machine output | M | Over-pinning | U2 |
| U6 | Feedback | S/M | CI noise | U5 |
| U7 | Onboarding | M | Shell drift | U1 |
| U8 | Local-first trust | S/M | Overclaiming | U2, U4 |

### U1 — Help, grammar and discoverability

**Scope.** Rewrite `--help` into task groups under 40 lines, leading with `ingest`/`search`, config
keys and exits. Generate per-command help, `weave help <cmd>`, topics (`exit-codes`, `config`, `json`)
and a man page from one table with defaults and examples. Accept `--opt=value` and global flags
before or after the command.

**Why.** Help is the only pre-action reference; today it teaches a retired command.

**Acceptance.** `help search` shows usage, defaults, keys, examples and exits; top-level help is ≤40
lines, has no "mount", runs under 50 ms; `--db=/tmp/x.db` and `weave --json search` parse; goldens
fail on drift.

### U2 — Unified error and exit contract

**Scope.** Publish one code/exit/message/hint/docs table; print `weave: CODE: message` plus
`hint: <command>` on stderr, naming the offending flag, path or key; expose the catalog in
`contract --json`. Align human/JSON exits, make empty `list` 0 with a hint (search no-match stays 1),
and return 130 on cancellation.

**Why.** Errors are the recovery path; JSON-only codes force re-runs.

**Acceptance.** Seeded `BUNDLE_NOT_INDEXED`, `CONFIG_INVALID`, `EXTENSION_NOT_FOUND` and
`PRIVACY_KEY_UNAVAILABLE` print code, exit and hint; human/JSON parity is 100%; every non-zero exit
has a hint; 130 is documented and tested.

### U3 — Safety: dry-run, no-input, confirmation

**Scope.** Add `--dry-run` to `ingest`, `rebuild`, `sync` and `export`, previewing counts and targets
without writes (exit 0, labelled `dry run`). Add `--no-input` to fail with `CONFIRMATION_REQUIRED`
and a hint. Keep `--yes` as consent, `--force` as overwrite, and add `--confirm <path>` as an explicit
alternative for raw export.

**Why.** A silent 30-second write gives no chance to catch a wrong bundle.

**Acceptance.** All four mutate nothing under `--dry-run`; off-TTY and `--no-input` never block; the
confirmation matrix is tested.

### U4 — Finish configuration UX

**Scope.** Use `os.UserConfigDir`/`UserCacheDir`, honor `XDG_STATE_HOME`, and fix the unconditional
`AppData` check. Decide the DB location (`~/.weave/index.db` or XDG state with migration) and the
project `.weave/config.yaml` precedence, or defer the latter. Add `config get`, typed output and
origins in `config show --json`, document `WEAVE_CONFIG`, and end `config init` with the next
command.

**Why.** Configuration stops repeated flags; path surprises cause support load.

**Acceptance.** One decision record; `config path` resolves correctly across OSes;
outputs are typed and tested; `init` prints a working next command.

### U5 — Machine-output stability and schema pinning

**Scope.** Add schemas and goldens for `config show|path|init` and watch JSONL; validate every
`--json` result, including error envelopes. Declare additive-only within `weave.cli/v1`: no removals
or type changes, new fields optional. Keep one envelope (or documented JSONL) on stdout and
diagnostics on stderr; `--json` implies no prompts or progress.

**Why.** Scripts and agents are first-class; untyped config maps are a hazard.

**Acceptance.** 100% of JSON validates; config goldens exist; a schema-diff check rejects removals;
stdout has no human text.

### U6 — Feedback: quiet, verbose, progress, NO_COLOR

**Scope.** Add `--quiet` (errors only) and `--verbose` (paths, sources, timings). Show progress on
stderr only on a TTY without `--json`; no carriage-return spinners off-TTY. Honor `NO_COLOR`, avoid
color-only meaning, add `--ascii` for `→`/`·`, and print a watch banner with bundle, DB and Ctrl-C
hint.

**Why.** Silence looks like a hang; spinners corrupt logs; Unicode separators exclude terminals.

**Acceptance.** A 10k ingest shows TTY progress and none in a pipe/JSON; `--quiet` omits non-errors;
`--verbose` shows the DB and sources; NO_COLOR/ASCII tests pass.

### U7 — Onboarding, empty states, completion

**Scope.** Align README and `--help` on one quickstart; make no-argument `weave` print "start here";
empty `list` exits 0 with an `ingest` hint; missing `.weave`, unindexed DB and no extensions name the
next step. Add `weave completion bash|zsh|fish` from the command table.

**Why.** The first five minutes decide adoption; empty output looks broken.

**Acceptance.** A scripted clean install reaches `ingest`→`search` in ≤2 commands and <60 s; empty
states have next steps; completion scripts pass smoke tests and are documented.

### U8 — Privacy and local-first messaging

**Scope.** State "no telemetry, no network by default" in help and version; add `weave config paths`
listing config, DB, cache, key-search and extension paths; let `extensions list` name search paths;
show the AI endpoint and consent; name key/provider in privacy errors.

**Why.** Local-first must be verifiable; opaque key errors erode trust.

**Acceptance.** The statement appears in help/version; `config paths` is tested; AI/extension docs
separate core from opt-in network use; no key material leaks.

## Sequencing

- **UX1 — Understand and recover (U1–U3).** Help, error/exit parity and dry-run remove the
  highest-friction failures and unblock trustworthy docs.
- **UX2 — Configure and automate (U4–U6).** Pins config and JSON before feedback, so progress cannot
  leak into machine output.
- **UX3 — Welcome and trust (U7–U8).** Completion needs U1 metadata; privacy messaging needs U4
  paths. Each slice is a stacked PR with docs and CLI tests.

## Metrics and accessibility

- **Task success:** ≥4 of 5 first-time users complete ingest→search unaided; ≥90% explain a seeded
  `BUNDLE_NOT_INDEXED` from the message alone.
- **Commands to first result:** ≤2 commands and <60 s from a clean install, verified by scripted
  onboarding.
- **Error recovery:** ≥90% of seeded errors resolved on first retry; 100% human/JSON exit parity;
  every non-zero exit prints code and hint; no off-TTY prompts.
- **Discoverability:** ≥80% find a flag or config key through help in <30 s; help latency <50 ms.
- **Accessibility:** plain language, no color-only meaning, `NO_COLOR`/`--ascii`, no off-TTY
  spinners, 80-column output; one screen-reader pass.

## Risks and non-goals

**Risks.** Exit-code changes can break scripts (document; keep codes stable); help/completion
drift (one table, goldens); project-config precedence confusion (decision record, origins); privacy
overclaiming (separate core from extensions); dry-run divergence (shared planning path).

**Non-goals.** TUI, web UI, daemon, mount, interactive config editor, telemetry, i18n, ranking
changes or breaking contract changes, and the internal file splits/error-table refactors owned by the
maintainability proposal. U2 defines the product contract without restructuring packages.

## References

- [Research: User Experience of Local-First CLI Tools](./research-ux.md)
- [Optimization specification](../../specs/optimization.md)
- [Configuration specification](../../specs/configuration.md)
- [Bundle Export specification](../../specs/bundle-export.md)
- [README](../../../README.md)
- [Command Line Interface Guidelines](https://clig.dev/)
- [XDG Base Directory Specification](https://specifications.freedesktop.org/basedir-spec/latest/)
- [NO_COLOR](https://no-color.org/)
- [gh exit codes](https://cli.github.com/manual/gh_help-exit-codes)
