package main

import (
	"fmt"
	"io"
)

type commandHelp struct {
	Usage    string
	Summary  string
	Flags    []string
	Examples []string
}

var commandHelpText = map[string]commandHelp{
	"search": {
		Usage:   "weave search <path> <query> [flags]",
		Summary: "Rank concepts with keyword, fuzzy, hybrid or BM25 search.",
		Flags: []string{
			"--mode hybrid|keyword|fuzzy|bm25   default hybrid",
			"--top-k <n>                        1..100, default 10",
			"--rerank <name>                    rerank with a weave-<name> extension",
			"--source <alias=path>              add a bundle source (repeatable)",
			"--timeout <duration>               extension timeout, default 5s",
			"--db <file> | --no-db              persistent or transient",
			"--json                             one weave.cli/v1 envelope",
		},
		Examples: []string{
			`weave search ~/notes "token rotation"`,
			`weave search ~/notes "oauth tokn" --mode fuzzy --top-k 5`,
			`weave search ~/notes "alpha" --mode bm25 --json`,
		},
	},
	"ingest": {
		Usage:   "weave ingest <path> [flags]",
		Summary: "Load a bundle into the persistent index.",
		Flags: []string{
			"--db <file>                        default ~/.weave/index.db",
			"--strict | --permissive            validation mode",
			"--codec zstd|lz4|none              body compression",
			"--with-enricher <name>             enrich documents during the write",
			"--dry-run                          preview counts without writing",
			"--provider <name>                  external crypto provider",
		},
		Examples: []string{
			`weave ingest ~/notes --strict --json`,
			`weave ingest ~/notes --codec lz4 --with-enricher ai`,
		},
	},
	"rebuild": {
		Usage:   "weave rebuild <path> [flags]",
		Summary: "Re-index one bundle, optionally changing the codec.",
		Flags: []string{
			"--db <file>                        persistent database",
			"--strict | --permissive            validation mode",
			"--codec zstd|lz4|none              re-encode every body",
		},
		Examples: []string{`weave rebuild ~/notes --codec zstd --json`},
	},
	"sync": {
		Usage:   "weave sync <path> [flags]",
		Summary: "Incrementally update one bundle from content hashes.",
		Flags: []string{
			"--db <file>                        persistent database",
			"--codec zstd|lz4|none              override the stored codec",
			"--watch [--events]                 poll and sync; JSONL events with --json",
			"--with-enricher <name>             enrich changed documents",
			"--dry-run                          preview counts without writing",
		},
		Examples: []string{
			`weave sync ~/notes --json`,
			`weave sync ~/notes --db ~/.weave/index.db --watch --json --events`,
		},
	},
	"show": {
		Usage:   "weave show <path> <id|file> [flags]",
		Summary: "Show one concept, redacting encrypted bodies by default.",
		Flags: []string{
			"--decrypt                          decrypt encrypted content",
			"--provider <name>                  external crypto provider",
			"--json                             include privacy state",
		},
		Examples: []string{`weave show ~/notes secret --decrypt --json`},
	},
	"list": {
		Usage:   "weave list <path> [flags]",
		Summary: "List concepts filtered by metadata.",
		Flags: []string{
			"--type <t> / --tag <t>             repeatable filters",
			"--status <s>                       draft|stable|deprecated",
			"--limit <n>                        default 20",
		},
		Examples: []string{`weave list ~/notes --type Concept --limit 5`},
	},
	"audit": {
		Usage:   "weave audit <path> [flags]",
		Summary: "Report conformance, link and key-availability findings.",
		Flags: []string{
			"--fail-on error|warning|none       failure threshold",
			"--verify-keys                      report KEY_UNAVAILABLE findings",
			"--provider <name>                  external crypto provider",
		},
		Examples: []string{`weave audit ~/notes --verify-keys --json`},
	},
	"export": {
		Usage:   "weave export <path> --out <dir> --format raw|encrypted [flags]",
		Summary: "Reconstruct bundle files from the index.",
		Flags: []string{
			"--format raw|encrypted             required",
			"--key-id <id>                      export key for encrypted format",
			"--yes                              confirm raw plaintext export",
			"--force                            write into a non-empty target",
			"--dry-run                          preview counts without writing",
			"--no-input                         fail instead of prompting",
			"--provider <name>                  external crypto provider",
		},
		Examples: []string{
			`weave export ~/notes --out ./raw --format raw --yes`,
			`weave export ~/notes --out ./sealed --format encrypted --key-id export-2026`,
		},
	},
	"config": {
		Usage:   "weave config show|path|init [flags]",
		Summary: "Inspect or initialize global configuration.",
		Flags: []string{
			"--config <file> / --no-config       select or ignore the file",
			"--strict-config                    reject unknown config keys",
			"--force                            overwrite on init",
		},
		Examples: []string{
			`weave config show --json`,
			`weave config init`,
		},
	},
	"extensions": {
		Usage:    "weave extensions list [<path>] [--json]",
		Summary:  "List discovered weave-<name> extensions.",
		Examples: []string{`weave extensions list ~/notes --json`},
	},
	"mcp": {
		Usage:   "weave mcp --bundle <path> [flags]",
		Summary: "Serve MCP tools and resources over stdio or TCP.",
		Flags: []string{
			"--source <alias=path>              additional bundle sources",
			"--db <file>                        persistent database",
			"--watch                            poll and sync; requires --db",
			"--allow-sensitive                  decrypt encrypted show/resources",
			"--stdio | --listen <addr>          transport",
		},
		Examples: []string{`weave mcp --bundle ~/notes --db ~/.weave/index.db --watch`},
	},
	"parse": {
		Usage:   "weave parse <path> [--source ...] [--json]",
		Summary: "Parse a bundle and report validation and index state.",
	},
	"graph": {
		Usage:   "weave graph <path> [--source ...] [--json]",
		Summary: "Print nodes, edges and dangling links.",
	},
	"backlinks": {
		Usage:   "weave backlinks <path> <id|file> [--json]",
		Summary: "List incoming references for a concept.",
	},
	"path": {
		Usage:   "weave path <path> <from> <to> [--json]",
		Summary: "Find a shortest directed path.",
	},
	"relate": {
		Usage:   "weave relate <path> <id|file> [--depth <n>] [--direction out|in|both]",
		Summary: "Show a bounded graph neighborhood.",
	},
	"explain": {
		Usage:   "weave explain <path> <query> [--id <id>] [--mode ...]",
		Summary: "Explain ranking evidence for one result.",
	},
	"contract": {
		Usage:   "weave contract [--json]",
		Summary: "Print contract versions, operations and error codes.",
	},
	"version": {
		Usage:   "weave version [--json]",
		Summary: "Print the binary and contract versions.",
	},
}

func printHelp(writer io.Writer) {
	fmt.Fprint(writer, `weave — local-first related-document database

usage:
  weave <command> [arguments] [flags]
  weave help <command>            detailed command help

read:
  parse graph search list show backlinks path relate explain audit
write:
  ingest rebuild sync export
configure & integrate:
  config extensions mcp contract version

global flags:
  --json                  one weave.cli/v1 envelope on stdout
  --db <file>             persistent database (~/.weave/index.db by default)
  --no-db                 transient in-memory parsing
  --config <file>         global config file (see weave config show)
  --no-config             ignore the config file; environment still applies
  --strict-config         reject unknown config keys
  --no-cache              disable the parse cache

examples:
  weave ingest ~/notes --strict
  weave search ~/notes "token rotation" --mode hybrid
  weave config show --json

exit codes: 0 success · 1 runtime/no match · 2 usage/confirmation · 3 permission/privacy
`)
}

func printCommandHelp(writer io.Writer, command string) {
	help, ok := commandHelpText[command]
	if !ok {
		fmt.Fprintf(writer, "weave %s: unknown command\n\nRun `weave --help` for the command list.\n", command)
		return
	}
	fmt.Fprintf(writer, "weave %s — %s\n\nusage:\n  %s\n", command, help.Summary, help.Usage)
	if len(help.Flags) > 0 {
		fmt.Fprint(writer, "\nflags:\n")
		for _, flag := range help.Flags {
			fmt.Fprintf(writer, "  %s\n", flag)
		}
	}
	if len(help.Examples) > 0 {
		fmt.Fprint(writer, "\nexamples:\n")
		for _, example := range help.Examples {
			fmt.Fprintf(writer, "  %s\n", example)
		}
	}
	fmt.Fprint(writer, "\nglobal: --json --db <file> --no-db --config <file> --no-config --strict-config\n")
}

func printConfigHelp(writer io.Writer) {
	fmt.Fprint(writer, `weave config — inspect or initialize global configuration

usage:
  weave config show [--json]
  weave config path [--json]
  weave config init [--force]

flags:
  --config <file>       select a config file
  --no-config           ignore the config file
  --strict-config       reject unknown keys
  --force               overwrite on init
  --json                machine-readable output

precedence: explicit CLI flag > WEAVE_* environment > config file > default
`)
}
