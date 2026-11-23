package main

import (
	"errors"
	"fmt"
	"io"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine/extensions"
)

func runExtensionTrust(args []string, stdout, stderr io.Writer) int {
	flags, err := parseFlags(args)
	if err != nil {
		return usageError(stdout, stderr, contains(args, "--json"), err.Error())
	}
	if flags.help || len(flags.positionals) == 0 {
		printExtensionTrustHelp(stdout)
		return 0
	}
	if flags.short {
		return usageError(stdout, stderr, flags.json, "--short is only valid for extensions list")
	}
	switch flags.positionals[0] {
	case "trust":
		if len(flags.positionals) < 2 || len(flags.positionals) > 3 {
			return usageError(stdout, stderr, flags.json, "usage: weave extensions trust <name> [<path>]")
		}
		root := "."
		if len(flags.positionals) == 3 {
			root = flags.positionals[2]
		}
		name := flags.positionals[1]
		record, err := extensions.TrustExtension(root, name)
		if err != nil {
			code := contractv1.ErrInvalidRequest
			exit := 2
			if errors.Is(err, extensions.ErrNotFound) {
				code = contractv1.ErrExtensionMissing
			}
			return trustFailure(stdout, stderr, flags.json, code, err.Error(), exit)
		}
		if flags.json {
			_ = writeJSON(stdout, successEnvelope(map[string]any{
				"name": name, "path": record.Path, "sha256": record.SHA256,
			}))
			return 0
		}
		fmt.Fprintf(stdout, "trusted %s (%s)\n", name, record.SHA256)
		return 0
	case "untrust":
		if len(flags.positionals) != 2 {
			return usageError(stdout, stderr, flags.json, "usage: weave extensions untrust <name>")
		}
		name := flags.positionals[1]
		if err := extensions.UntrustExtension(name); err != nil {
			return trustFailure(stdout, stderr, flags.json, contractv1.ErrInternal, err.Error(), 1)
		}
		if flags.json {
			_ = writeJSON(stdout, successEnvelope(map[string]any{"name": name}))
			return 0
		}
		fmt.Fprintf(stdout, "untrusted %s\n", name)
		return 0
	case "trusted":
		states, err := extensions.TrustStates()
		if err != nil {
			return trustFailure(stdout, stderr, flags.json, contractv1.ErrInternal, err.Error(), 1)
		}
		if flags.json {
			_ = writeJSON(stdout, successEnvelope(map[string]any{"extensions": trustStatesJSON(states)}))
			return 0
		}
		for _, state := range states {
			status := "matches"
			if !state.Matches {
				status = "changed"
			}
			fmt.Fprintf(stdout, "%s %s %s %s\n", state.Name, state.Path, state.SHA256, status)
		}
		return 0
	default:
		return usageError(stdout, stderr, flags.json, "usage: weave extensions trust|untrust|trusted [flags]")
	}
}

func trustStatesJSON(states []extensions.TrustState) []map[string]any {
	items := make([]map[string]any, 0, len(states))
	for _, state := range states {
		items = append(items, map[string]any{
			"name": state.Name, "path": state.Path, "sha256": state.SHA256, "matches": state.Matches,
		})
	}
	return items
}

func trustFailure(stdout, stderr io.Writer, jsonOut bool, code contractv1.ErrorCode, message string, exit int) int {
	if jsonOut {
		_ = writeJSON(stdout, contractv1.FailureEnvelope{
			Contract: "weave.contract/v1",
			CLI:      "weave.cli/v1",
			OK:       false,
			Error:    contractv1.OperationError{Code: code, Message: message},
		})
	}
	printOperationError(stderr, code, message)
	return exit
}

func printExtensionTrustHelp(writer io.Writer) {
	fmt.Fprintln(writer, `weave extensions trust — manage bundle-local extension trust

usage:
  weave extensions trust <name> [<path>]
  weave extensions untrust <name>
  weave extensions trusted [<path>] [--json]

Trust is recorded per user in ~/.weave/trust.yaml with the executable path and SHA-256.
Bundle files cannot grant trust. Extensions on PATH are not gated.`)
}
