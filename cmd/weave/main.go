// Command weave is the CLI entrypoint for parsing, searching and syncing
// bundles.
package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine"
	"github.com/prulloac/weave/engine/operations"
	"github.com/prulloac/weave/engine/privacy"
)

var version = "dev"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		printHelp(stdout)
		return 0
	}

	if args[0] == "--help" || args[0] == "-h" {
		printHelp(stdout)
		return 0
	}
	if args[0] == "help" {
		if len(args) > 1 {
			printCommandHelp(stdout, args[1])
			return 0
		}
		printHelp(stdout)
		return 0
	}
	if args[0] == "--version" || args[0] == "-v" {
		fmt.Fprintf(stdout, "weave %s (contract 1)\n", version)
		return 0
	}

	command := args[0]
	if command == "config" {
		return runConfigCommand(args[1:], stdout, stderr)
	}
	if command == "mcp" {
		configPath, noConfig, strict, err := scanConfigArgs(args[1:])
		if err != nil {
			return usageError(stdout, stderr, false, err.Error())
		}
		cfg, warnings, err := loadAppConfig(configPath, noConfig, strict)
		if err != nil {
			return usageError(stdout, stderr, false, err.Error())
		}
		for _, warning := range warnings {
			fmt.Fprintln(stderr, warning)
		}
		return runMCP(stripConfigArgs(args[1:]), stdout, stderr, cfg)
	}
	jsonMode := contains(args[1:], "--json")
	flags, err := parseFlags(args[1:])
	if err != nil {
		return usageError(stdout, stderr, jsonMode || flags.json, err.Error())
	}
	if flags.help {
		printCommandHelp(stdout, command)
		return 0
	}
	if flags.configPath != "" && flags.noConfig {
		return usageError(stdout, stderr, flags.json, "--config and --no-config are mutually exclusive")
	}
	cfg, warnings, err := loadAppConfig(flags.configPath, flags.noConfig, flags.strictConfig)
	if err != nil {
		return usageError(stdout, stderr, flags.json, err.Error())
	}
	for _, warning := range warnings {
		fmt.Fprintln(stderr, warning)
	}
	applyConfigToFlags(&flags, cfg, command)

	request, exitCode, err := buildRequest(command, flags)
	if err != nil {
		return usageError(stdout, stderr, flags.json, err.Error())
	}
	if exitCode != 0 {
		return exitCode
	}
	request.DB = flags.db
	request.NoDB = flags.noDB
	request.Strict = flags.strict
	request.Permissive = flags.permissive
	request.Codec = flags.codec
	request.Decrypt = flags.decrypt
	request.Enricher = flags.enricher
	request.Rerank = flags.rerank
	if flags.timeoutSet {
		request.TimeoutMS = flags.timeout.Milliseconds()
	}
	if request.Enricher != "" {
		switch request.Op {
		case contractv1.OpIngest, contractv1.OpRebuild, contractv1.OpSync:
		default:
			return usageError(stdout, stderr, flags.json, "--with-enricher is only valid for ingest, rebuild and sync")
		}
	}
	if request.Rerank != "" && request.Op != contractv1.OpSearch {
		return usageError(stdout, stderr, flags.json, "--rerank is only valid for search")
	}
	if len(flags.sources) > 0 {
		switch request.Op {
		case contractv1.OpParse, contractv1.OpGraph, contractv1.OpSearch, contractv1.OpList,
			contractv1.OpShow, contractv1.OpBacklinks, contractv1.OpPath, contractv1.OpRelate,
			contractv1.OpExplain, contractv1.OpAudit, contractv1.OpIngest, contractv1.OpRebuild,
			contractv1.OpSync:
		default:
			return usageError(stdout, stderr, flags.json, "--source is not supported for this command")
		}
		specs, err := buildSourceSpecs(request.Path, flags.sources)
		if err != nil {
			return usageError(stdout, stderr, flags.json, err.Error())
		}
		request.Sources = specs
	}

	if flags.watch {
		if request.Op != contractv1.OpSync {
			return usageError(stdout, stderr, flags.json, "--watch is only valid for sync")
		}
		if flags.noDB || !flags.explicit["--db"] {
			return usageError(stdout, stderr, flags.json, "--watch requires --db")
		}
		if flags.events && !flags.json {
			return usageError(stdout, stderr, flags.json, "--events requires --json")
		}
		if flags.dryRun {
			return usageError(stdout, stderr, flags.json, "--watch does not support --dry-run")
		}
		if len(flags.sources) > 0 || flags.enricher != "" || flags.rerank != "" {
			return usageError(stdout, stderr, flags.json, "--watch does not support --source, --with-enricher or --rerank")
		}
	} else if flags.events {
		return usageError(stdout, stderr, flags.json, "--events requires --watch")
	}

	request.Out = flags.out
	request.Format = flags.format
	request.KeyID = flags.keyID
	request.Force = flags.force
	request.Confirmed = flags.yes
	request.Provider = flags.provider
	request.DryRun = flags.dryRun
	if flags.dryRun {
		switch request.Op {
		case contractv1.OpIngest, contractv1.OpRebuild, contractv1.OpSync, contractv1.OpExport:
		default:
			return usageError(stdout, stderr, flags.json, "--dry-run is only valid for ingest, rebuild, sync and export")
		}
	}
	if flags.noInput && request.Op != contractv1.OpExport {
		return usageError(stdout, stderr, flags.json, "--no-input is only valid for export")
	}
	if flags.provider != "" {
		switch request.Op {
		case contractv1.OpIngest, contractv1.OpRebuild, contractv1.OpSync, contractv1.OpShow,
			contractv1.OpAudit, contractv1.OpExport:
		default:
			return usageError(stdout, stderr, flags.json, "--provider is not supported for this command")
		}
	}
	if request.Op != contractv1.OpExport {
		if flags.out != "" || flags.format != "" || flags.keyID != "" || flags.force || flags.yes {
			return usageError(stdout, stderr, flags.json, "--out, --format, --key-id, --force and --yes are only valid for export")
		}
	} else {
		format := strings.ToLower(strings.TrimSpace(flags.format))
		if format == "" {
			return usageError(stdout, stderr, flags.json, "export requires --format raw|encrypted")
		}
		if format != "raw" && format != "encrypted" {
			return usageError(stdout, stderr, flags.json, "--format must be raw or encrypted")
		}
		if format == "encrypted" && flags.keyID == "" {
			return usageError(stdout, stderr, flags.json, "encrypted export requires --key-id")
		}
		if format == "raw" && !flags.yes && !flags.dryRun && !flags.noInput && !flags.json && stdinIsTerminal() {
			fmt.Fprintf(stderr, "WARNING: raw export writes plaintext document bodies to %s\nContinue? [y/N] ", flags.out)
			answer, _ := bufio.NewReader(os.Stdin).ReadString('\n')
			answer = strings.ToLower(strings.TrimSpace(answer))
			if answer != "y" && answer != "yes" {
				return usageError(stdout, stderr, false, "raw export cancelled")
			}
			request.Confirmed = true
		}
	}

	if flags.watch {
		mode := ""
		if flags.strict {
			mode = "strict"
		} else if flags.permissive {
			mode = "permissive"
		}
		watchContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		return runSyncWatch(watchContext, syncWatchOptions{
			Root:     request.Path,
			DB:       flags.db,
			Mode:     mode,
			Codec:    flags.codec,
			Provider: flags.provider,
			JSON:     flags.json,
			Events:   flags.events,
		}, stdout, stderr)
	}

	if !flags.json && request.Path != "" {
		if warning := privacy.SecurityFileWarning(request.Path); warning != "" {
			fmt.Fprintf(stderr, "weave: %s\n", warning)
		}
	}

	executor := operations.New(version, engine.Options{Cache: !flags.noCache, CacheDir: flags.cacheDir})
	executor.Warn = func(message string) {
		fmt.Fprintf(stderr, "weave: %s\n", message)
	}
	value := executor.Execute(context.Background(), request)

	switch envelope := value.(type) {
	case contractv1.SuccessEnvelope:
		if flags.json {
			if err := writeJSON(stdout, envelope); err != nil {
				fmt.Fprintf(stderr, "weave: %v\n", err)
				return 1
			}
			if exit := successExitCode(request.Op, envelope.Result); exit != 0 {
				return exit
			}
			if isEmptyResult(request.Op, envelope.Result) {
				fmt.Fprintln(stderr, "weave: no matches")
				return 1
			}
			return 0
		}
		return printHuman(stdout, stderr, request.Op, envelope.Result)
	case contractv1.FailureEnvelope:
		if flags.json {
			if err := writeJSON(stdout, envelope); err != nil {
				fmt.Fprintf(stderr, "weave: %v\n", err)
			}
		}
		printOperationError(stderr, envelope.Error.Code, envelope.Error.Message)
		return exitCodeFor(envelope.Error.Code)
	default:
		fmt.Fprintln(stderr, "weave: internal error")
		return 1
	}
}
