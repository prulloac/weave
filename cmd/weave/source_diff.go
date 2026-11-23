package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine/extensions"
	"github.com/prulloac/weave/engine/operations"
)

func runSourceDiff(
	alias, workspaceArg, from, to, format, pathFilter, outFile string,
	timeout time.Duration,
	jsonOut bool,
	stdout, stderr io.Writer,
) int {
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" {
		format = "json"
	}
	if format != "json" && format != "patch" {
		return sourceDiffError(stdout, stderr, jsonOut, contractv1.ErrInvalidRequest, "--format must be json or patch", 2)
	}
	if strings.TrimSpace(from) == "" || strings.TrimSpace(to) == "" {
		return sourceDiffError(stdout, stderr, jsonOut, contractv1.ErrInvalidRequest, "--from and --to are required", 2)
	}
	if format == "patch" && jsonOut {
		return sourceDiffError(stdout, stderr, jsonOut, contractv1.ErrInvalidRequest, "--format patch and --json are mutually exclusive", 2)
	}
	if format != "patch" && outFile != "" {
		return sourceDiffError(stdout, stderr, jsonOut, contractv1.ErrInvalidRequest, "--out requires --format patch", 2)
	}

	ws, err := loadWorkspace(workspaceArg)
	if err != nil {
		return sourceDiffError(stdout, stderr, jsonOut, contractv1.ErrInvalidRequest, err.Error(), 2)
	}
	record, ok, err := sourceRecord(ws.Index, alias)
	if err != nil {
		return sourceDiffError(stdout, stderr, jsonOut, contractv1.ErrInternal, err.Error(), 1)
	}
	if !ok {
		return sourceDiffError(stdout, stderr, jsonOut, contractv1.ErrSourceNotFound, "source not registered: "+alias, 1)
	}
	provider := sourceProvider(record)
	uri := sourceURI(record)
	sourceID := extensions.WorkspaceSourceID(ws.Dir, alias, operations.CanonicalSourceIdentity(uri))

	result, err := operations.DiffSource(context.Background(), operations.SourceDiffOptions{
		WorkspaceRoot: ws.Dir,
		Alias:         alias,
		Provider:      provider,
		URI:           uri,
		SourceID:      sourceID,
		CacheDir:      operations.SourceCacheDir(),
		From:          strings.TrimSpace(from),
		To:            strings.TrimSpace(to),
		Format:        map[string]string{"json": "changes", "patch": "patch"}[format],
		Path:          pathFilter,
		Timeout:       timeout,
		Warn: func(message string) {
			fmt.Fprintf(stderr, "weave: %s\n", message)
		},
	})
	if err != nil {
		code, exit := sourceDiffErrorCode(err)
		return sourceDiffError(stdout, stderr, jsonOut, code, err.Error(), exit)
	}

	if format == "patch" {
		var patch strings.Builder
		for _, change := range result.Changes {
			if change.Patch != "" {
				patch.WriteString(change.Patch)
				if !strings.HasSuffix(change.Patch, "\n") {
					patch.WriteString("\n")
				}
			}
		}
		data := []byte(patch.String())
		if outFile != "" {
			if err := writeFileAtomic(outFile, data); err != nil {
				return sourceDiffError(stdout, stderr, jsonOut, contractv1.ErrInternal, err.Error(), 1)
			}
			return 0
		}
		fmt.Fprint(stdout, string(data))
		return 0
	}

	if jsonOut {
		if err := writeJSON(stdout, successEnvelope(result)); err != nil {
			return 1
		}
		return 0
	}
	fmt.Fprintf(stdout, "%s -> %s\n", result.From, result.To)
	for _, change := range result.Changes {
		line := change.Kind + " " + change.Path
		if change.PreviousPath != "" {
			line += " (from " + change.PreviousPath + ")"
		}
		if change.Binary {
			line += " [binary]"
		}
		fmt.Fprintln(stdout, line)
	}
	return 0
}

func sourceDiffErrorCode(err error) (contractv1.ErrorCode, int) {
	switch {
	case errors.Is(err, operations.ErrSourceUnsupported):
		return contractv1.ErrSourceUnsupported, 2
	case errors.Is(err, extensions.ErrNotFound):
		return contractv1.ErrExtensionMissing, 2
	case errors.Is(err, extensions.ErrRequirementUnmet):
		return contractv1.ErrExtensionRequirement, 1
	case errors.Is(err, extensions.ErrTimeout):
		return contractv1.ErrExtensionTimeout, 1
	case errors.Is(err, extensions.ErrFailed):
		return contractv1.ErrExtensionFailed, 1
	case errors.Is(err, extensions.ErrProtocol):
		return contractv1.ErrExtensionProto, 1
	default:
		return contractv1.ErrInternal, 1
	}
}

func sourceDiffError(stdout, stderr io.Writer, jsonOut bool, code contractv1.ErrorCode, message string, exit int) int {
	if jsonOut {
		_ = writeJSON(stdout, contractv1.FailureEnvelope{
			Contract: operations.ContractVersion,
			CLI:      operations.CLIVersion,
			OK:       false,
			Error:    contractv1.OperationError{Code: code, Message: message},
		})
	}
	printOperationError(stderr, code, message)
	return exit
}
