---
type: research
title: "User Experience of Local-First CLI Tools"
description: "Current practice for help, configuration, errors, safety, machine output and privacy in local-first CLIs, applied to Weave's command surface."
tags:
  - "research"
  - "ux"
  - "cli"
  - "optimization"
generated:
  by: subagent
  at: "2026-10-31T02:21:41Z"
status: draft
---

# Research: User Experience of Local-First CLI Tools

## Executive summary

Weave's base is strong: deterministic `weave.cli/v1` envelopes, documented `0/1/2/3` exit codes,
`config show` with per-key sources, and distinct `--yes`/`--force` semantics. Execution lags: help
still says "mount", command help is a one-line stub, human errors omit stable codes, and human/JSON
exits diverge for ingest and sync errors. Prioritize help, error/exit parity, dry-run safety and
config polish.

## State of practice (2024-2026)

- **Human-first is the baseline.** clig.dev and 12-factor CLI Apps prescribe flags over args, full
  subcommand help, examples, stdout/stderr separation and actionable errors.
- **Machine output is a product.** gh's field-selected `--json` plus `--jq`, kubectl's
  `-o json|yaml|name`, and restic's additive JSON/JSONL treat scripts and agents as consumers.
- **Provenance is expected.** `git config --show-origin/--show-scope` and `aws configure list` show
  value, source and location; XDG separates config, data, state and cache.
- **Safety is explicit.** Terraform uses 0/1/2 for no change/error/changes; kubectl offers
  `--dry-run=client`; restic returns 130 on cancel; Heroku requires `--confirm=<app>`.
- **Simple codes and local-first promises.** OpenBSD says sysexits "Do not use them"; gh uses 0/1/2/4
  and grep 0/1/2 for search. Data location, egress and NO_COLOR (2026) define the contract.

## Key findings

1. **Help drives discovery.** Group by task and lead with examples; command help
   lists defaults, env vars and config keys. `git help <cmd>`, `gh help formatting` and
   Heroku's `USAGE/FLAGS/EXAMPLES` blocks are the reference.
2. **Progressive disclosure needs topics.** `gh help exit-codes` and `git help tutorial` keep help
   short and contracts discoverable offline.
3. **Grammar builds trust.** Accept `--opt=value` and `--opt value`, `--`, order-independent flags and
   `-h` anywhere; git documents `--no-` negation and "options trump configuration".
4. **Errors are documentation.** State the cause and the fix; a stable code plus one command hint
   beats a paragraph.
5. **Safety and accessibility are explicit.** Prompt only on a TTY, split `--yes` from `--force`,
   offer dry-run; no color-only meaning, no spinner off-TTY, ASCII fallback for `→`/`·`.

## Applicability to Weave

**Help and grammar (must).** `weave --help` says "mount"; usage lines exceed 200 chars;
`weave search --help` is a stub; no `help search`, examples or exit-code topic. Parser rejects
`--db=/tmp/x.db` and `weave --json search`, and `weave config --help` errors. Rewrite the top level
into task groups with the onboarding pair (`weave ingest ~/notes`;
`weave search ~/notes "token rotation"`), add per-command help, topics and a man page, and accept both
option forms plus global flags on either side.

**Configuration (should).** `config show` reports sources, but only `XDG_CONFIG_HOME` is honored; the
DB is `~/.weave/index.db`; `defaultConfigPath` checks Windows `AppData` on Unix; there
is no project `.weave/config.yaml`; `WEAVE_CONFIG` is undocumented; `config show --json` is untyped.
Use `os.UserConfigDir`/`UserCacheDir`, put state under `XDG_STATE_HOME` with migration, add project
precedence and `config get|set|edit`.

**Errors and exit codes (must).** JSON has codes and `BUNDLE_NOT_INDEXED` has an ingest hint, but human
output prints only the message. `CONFIG_INVALID`, `EXTENSION_NOT_FOUND` and `PRIVACY_KEY_UNAVAILABLE`
lack file, path and key detail; `weave extensions list` and `weave show --decrypt` should name search
paths, key id and provider. Add a code/exit/hint/docs table, print `weave: CODE: message` plus
`hint:`, expose it in `contract --json`, and fix parity: human ingest/sync returns 0 when
`report.Errors > 0`; empty
`list` exits 1 but `backlinks` exits 0; watch Ctrl-C exits 0; cancelled export exits 2 — normalize both
to 130.

**Safety (must).** Export's TTY prompt, `--yes` and `--force` are right. Add `--dry-run` to ingest,
rebuild, sync and export, `--no-input`, and typed `--confirm <path>` before raw export.

**Feedback, onboarding and privacy (should).** A 10k ingest can take 30 s; add progress
on stderr, quiet/verbose, a watch banner, a pager and completion. Empty `list` should be 0 with a
hint; search no-match stays 1. State "no telemetry, no network by default"; show data paths; prefer
0600 `security.yaml` or a key file over `WEAVE_KEY_*`; AI prints its endpoint and requires
`privacy.ai.send` consent.

## Recommended practices

| Priority | Practice | Weave move |
| --- | --- | --- |
| Must | Help and error/exit contract | Task groups, topics, code/hint/docs, 130 on cancel |
| Must | Dry-run and non-TTY safety | All write commands; `--no-input`; no hidden prompts |
| Must | Config paths and provenance | XDG config/state/cache, project config, show origin |
| Should | Grammar, feedback and onboarding | `--opt=value`, global flags, progress, quiet/verbose, quickstart |
| Optional | Machine and i18n | `--fields`, `--jq`, NDJSON, `LC_MESSAGES`, `--ascii` |

## Anti-patterns to avoid

- Stale help ("mount"), stub subcommand help and usage walls without examples.
- Codes only in JSON; human errors with no fix or documentation link.
- One exit code for error, empty and cancellation; `--yes` as `--force`.
- Off-TTY prompts, silent operations, CI spinners, Unicode-only separators, color-only meaning.
- Telemetry or network calls without an explicit, visible opt-in.

## Actionable metrics

- First search from clean install: ≤2 commands and <60 s.
- Every non-zero exit prints code, hint and docs; human/JSON parity 100%.
- 0 prompts off-TTY; 0 destructive commands without `--dry-run`.
- Help under 40 lines (command help 60), latency <50 ms; config sources cover every key.
- JSON additive-only; every envelope golden-tested; no field removals.
- Five-user test: ≥90% complete "ingest then search" unaided; ≥90% explain a seeded error.

## References

- [Command Line Interface Guidelines](https://clig.dev/)
- [12 Factor CLI Apps](https://jdx.dev/posts/2018-10-08-12-factor-cli-apps/)
- [GNU command-line standards](https://www.gnu.org/prep/standards/html_node/Command_002dLine-Interfaces.html)
- [POSIX Utility Conventions](https://pubs.opengroup.org/onlinepubs/9699919799/basedefs/V1_chap12.html)
- [XDG Base Directory Specification](https://specifications.freedesktop.org/basedir-spec/latest/)
- [NO_COLOR](https://no-color.org/)
- [OpenBSD sysexits(3)](https://man.openbsd.org/sysexits)
- [GNU grep exit status](https://www.gnu.org/software/grep/manual/grep.html#Exit-Status)
- [git-config origins and scopes](https://git-scm.com/docs/git-config#Documentation/git-config.txt---show-origin)
- [git CLI conventions](https://git-scm.com/docs/gitcli)
- [gh exit codes](https://cli.github.com/manual/gh_help_exit-codes)
- [gh output formatting](https://cli.github.com/manual/gh_help_formatting)
- [Heroku CLI style guide](https://devcenter.heroku.com/articles/cli-style-guide)
- [Terraform plan exit codes](https://developer.hashicorp.com/terraform/cli/commands/plan)
- [kubectl usage conventions](https://kubernetes.io/docs/reference/kubectl/conventions/)
- [restic scripting and exit codes](https://restic.readthedocs.io/en/stable/075_scripting.html)
- [aws configure list](https://docs.aws.amazon.com/cli/latest/reference/configure/list.html)
- [Local-first software](https://www.inkandswitch.com/essay/local-first/)
- [Writing helpful error messages](https://developers.google.com/tech-writing/error-messages)
- [age(1)](https://man.archlinux.org/man/age.1.en)
- [Model Context Protocol](https://modelcontextprotocol.io/specification/2025-06-18)
- [Go os.UserConfigDir](https://pkg.go.dev/os#UserConfigDir)
- [Plain language principles](https://digital.gov/guides/plain-language/principles)

## Hand-off for proposal agent

1. **P0 — Rebuild help:** groups, examples, `help <cmd>`, topics, man page.
2. **P0 — Unify errors:** code/exit/hint/docs table, `contract --json`, parity, 130.
3. **P0 — Add dry-run safety:** preview writes, sync removals, `--no-input`; separate `--yes` from
   `--force`.
4. **P1 — Finish configuration:** XDG state/cache, project config, `get|set|edit`, validation, `init`
   hints.
5. **P1 — Pin output and feedback:** contract goldens, additive policy, progress, banner, pager.
6. **P2 — Improve onboarding:** quickstart, empty states, completion, `version --json` support.
7. **P2 — Make privacy and accessibility explicit:** no-telemetry statement, data locations, key file,
   AI preview, ASCII/NO_COLOR.
