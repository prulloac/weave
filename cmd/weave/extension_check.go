package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine/codecs"
	"github.com/prulloac/weave/engine/extensions"
	"github.com/prulloac/weave/engine/operations"
)

func runExtensionCheck(args []string, stdout, stderr io.Writer) int {
	flags, err := parseFlags(args)
	if err != nil {
		return usageError(stdout, stderr, contains(args, "--json"), err.Error())
	}
	if flags.help || len(flags.positionals) == 0 {
		printExtensionCheckHelp(stdout)
		return 0
	}
	if flags.short {
		return usageError(stdout, stderr, flags.json, "--short is only valid for extensions list")
	}
	if len(flags.positionals) < 2 || len(flags.positionals) > 3 {
		return usageError(stdout, stderr, flags.json, "usage: weave extensions check <name> [<path>] [--timeout 5s] [--json]")
	}
	name := flags.positionals[1]
	root := ""
	if len(flags.positionals) == 3 {
		root = flags.positionals[2]
	}
	timeout := extensions.DefaultTimeout
	if flags.timeoutSet {
		timeout = flags.timeout
	}

	var report contractv1.ExtensionCheckReport
	if strings.HasPrefix(name, "codec-") {
		registry := codecs.New(root, timeout)
		report, err = registry.Check(context.Background(), strings.TrimPrefix(name, "codec-"))
	} else {
		report, err = extensions.Check(context.Background(), root, name, timeout)
	}
	if err != nil {
		code := contractv1.ErrInternal
		if errors.Is(err, extensions.ErrNotFound) {
			code = contractv1.ErrExtensionMissing
		}
		message := err.Error()
		if flags.json {
			_ = writeJSON(stdout, contractv1.FailureEnvelope{
				Contract: operations.ContractVersion,
				CLI:      operations.CLIVersion,
				OK:       false,
				Error:    contractv1.OperationError{Code: code, Message: message},
			})
		}
		printOperationError(stderr, code, message)
		if code == contractv1.ErrExtensionMissing {
			return 2
		}
		return 1
	}

	if flags.json {
		_ = writeJSON(stdout, successEnvelope(report))
	} else {
		printExtensionCheck(stdout, report)
	}
	if !report.OK {
		return 1
	}
	return 0
}

func printExtensionCheck(writer io.Writer, report contractv1.ExtensionCheckReport) {
	failed := 0
	for _, check := range report.Checks {
		status := string(check.Status)
		line := fmt.Sprintf("%-20s %s", check.Check, status)
		if check.Message != "" {
			line += "  " + check.Message
		}
		fmt.Fprintln(writer, strings.TrimRight(line, " "))
		if check.Status == contractv1.ExtensionCheckFail {
			failed++
		}
	}
	if failed == 0 {
		fmt.Fprintf(writer, "%s: ok (%d checks)\n", report.Extension, len(report.Checks))
		return
	}
	fmt.Fprintf(writer, "%s: failed (%d of %d checks)\n", report.Extension, failed, len(report.Checks))
}

func printExtensionCheckHelp(writer io.Writer) {
	fmt.Fprintln(writer, `weave extensions check — validate one extension against the protocol contract

usage:
  weave extensions check <name> [<path>] [--timeout 5s] [--json]

The check reads the manifest, evaluates requires and runs one canned request per
declared role. It never reads bundle documents or writes to the bundle.`)
}
