package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine/extensions"
	sqlitestore "github.com/prulloac/weave/storage/sqlite"
)

type extensionDataResult struct {
	Extension string          `json:"extension"`
	DocID     string          `json:"doc_id"`
	Key       string          `json:"key"`
	MediaType string          `json:"media_type,omitempty"`
	Size      int64           `json:"size"`
	SHA256    string          `json:"sha256"`
	Metadata  json.RawMessage `json:"metadata,omitempty"`
}

type extensionDataListResult struct {
	Data []extensionDataResult `json:"data"`
}

type extensionDataGCResult struct {
	Removed int  `json:"removed"`
	DryRun  bool `json:"dry_run"`
}

func runExtensionData(args []string, stdout, stderr io.Writer) int {
	flags, err := parseFlags(args)
	if err != nil {
		return usageError(stdout, stderr, false, err.Error())
	}
	if flags.help || len(flags.positionals) == 0 {
		printExtensionDataHelp(stdout)
		return 0
	}
	action := flags.positionals[0]
	if action == "get" && len(flags.positionals) == 4 && strings.TrimSpace(flags.out) == "" {
		return usageError(stdout, stderr, flags.json, "OUTPUT_REQUIRED: --out <file> is required")
	}
	cfg, warnings, err := loadAppConfig(flags.configPath, flags.noConfig, flags.strictConfig)
	if err != nil {
		return usageError(stdout, stderr, flags.json, err.Error())
	}
	for _, warning := range warnings {
		fmt.Fprintln(stderr, warning)
	}
	applyConfigToFlags(&flags, cfg, "extensions")
	if flags.noDB {
		return usageError(stdout, stderr, flags.json, "extension data requires persistent storage")
	}
	dbPath := flags.db
	if dbPath == "" {
		dbPath, err = sqlitestore.DefaultPath()
		if err != nil {
			return extensionDataError(stdout, stderr, flags.json, err)
		}
	}
	dbPath, err = sqlitestore.ExpandPath(dbPath)
	if err != nil {
		return extensionDataError(stdout, stderr, flags.json, err)
	}
	store, err := sqlitestore.OpenExisting(dbPath)
	if err != nil {
		return extensionDataError(stdout, stderr, flags.json, err)
	}
	defer store.Close()

	action = flags.positionals[0]
	switch action {
	case "list":
		if len(flags.positionals) != 2 {
			return usageError(stdout, stderr, flags.json, "usage: weave extensions data list <path> [--extension <name>] [--doc <id>]")
		}
		records, err := store.ListExtensionData(context.Background(), flags.positional(1), sqlitestore.ExtensionDataFilter{
			Extension: flags.extensionName,
			DocID:     flags.docID,
		})
		if err != nil {
			return extensionDataError(stdout, stderr, flags.json, err)
		}
		result := extensionDataListResult{Data: make([]extensionDataResult, 0, len(records))}
		for _, record := range records {
			result.Data = append(result.Data, extensionDataResult{
				Extension: record.Extension,
				DocID:     record.DocID,
				Key:       record.Key,
				MediaType: record.MediaType,
				Size:      record.Size,
				SHA256:    record.SHA256,
				Metadata:  record.Metadata,
			})
		}
		if flags.json {
			_ = writeJSON(stdout, successEnvelope(result))
			return 0
		}
		if len(result.Data) == 0 {
			fmt.Fprintln(stderr, "weave: no extension data found")
			return 0
		}
		for _, item := range result.Data {
			fmt.Fprintf(stdout, "%-16s %-24s %-32s %8d %s\n", item.Extension, item.DocID, item.Key, item.Size, item.SHA256)
		}
		return 0
	case "get":
		if len(flags.positionals) != 4 {
			return usageError(stdout, stderr, flags.json, "usage: weave extensions data get <path> <extension> <key> --out <file> [--force]")
		}
		if strings.TrimSpace(flags.out) == "" {
			return usageError(stdout, stderr, flags.json, "OUTPUT_REQUIRED: --out <file> is required")
		}
		var data []byte
		var record sqlitestore.ExtensionDataRecord
		if flags.docID != "" {
			data, record, err = store.ReadExtensionData(
				context.Background(),
				flags.positional(1),
				flags.positional(2),
				flags.docID,
				flags.positional(3),
			)
		} else {
			data, record, err = store.ReadExtensionDataByKey(
				context.Background(),
				flags.positional(1),
				flags.positional(2),
				flags.positional(3),
			)
		}
		if err != nil {
			return extensionDataError(stdout, stderr, flags.json, err)
		}
		if _, err := os.Stat(flags.out); err == nil && !flags.force {
			return usageError(stdout, stderr, flags.json, "OUTPUT_EXISTS: "+flags.out+" already exists; use --force")
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			return extensionDataError(stdout, stderr, flags.json, err)
		}
		if err := os.WriteFile(flags.out, data, 0o600); err != nil {
			return extensionDataError(stdout, stderr, flags.json, err)
		}
		result := extensionDataResult{
			Extension: record.Extension,
			DocID:     record.DocID,
			Key:       record.Key,
			MediaType: record.MediaType,
			Size:      record.Size,
			SHA256:    record.SHA256,
			Metadata:  record.Metadata,
		}
		if flags.json {
			_ = writeJSON(stdout, successEnvelope(result))
			return 0
		}
		fmt.Fprintf(stdout, "wrote %d bytes to %s\n", len(data), flags.out)
		return 0
	case "gc":
		if len(flags.positionals) != 2 {
			return usageError(stdout, stderr, flags.json, "usage: weave extensions data gc <path> [--dry-run]")
		}
		removed, err := store.GCExtensionDataWithOptions(context.Background(), flags.positional(1), flags.dryRun)
		if err != nil {
			return extensionDataError(stdout, stderr, flags.json, err)
		}
		result := extensionDataGCResult{Removed: removed, DryRun: flags.dryRun}
		if flags.json {
			_ = writeJSON(stdout, successEnvelope(result))
			return 0
		}
		if flags.dryRun {
			fmt.Fprintf(stdout, "would remove %d orphaned blobs\n", removed)
		} else {
			fmt.Fprintf(stdout, "removed %d orphaned blobs\n", removed)
		}
		return 0
	default:
		return usageError(stdout, stderr, flags.json, "unknown extensions data command "+action)
	}
}

func extensionDataError(stdout, stderr io.Writer, jsonMode bool, err error) int {
	code := contractv1.ErrInternal
	switch {
	case errors.Is(err, sqlitestore.ErrExtensionDataNotFound):
		code = contractv1.ErrExtensionDataNotFound
	case errors.Is(err, sqlitestore.ErrExtensionDataAmbiguous):
		code = contractv1.ErrInvalidRequest
	case errors.Is(err, extensions.ErrDataInvalid):
		code = contractv1.ErrExtensionDataInvalid
	case errors.Is(err, extensions.ErrDataTooLarge):
		code = contractv1.ErrExtensionDataTooLarge
	case errors.Is(err, extensions.ErrDataQuota):
		code = contractv1.ErrExtensionDataQuota
	}
	message := err.Error()
	if jsonMode {
		_ = writeJSON(stdout, contractv1.FailureEnvelope{
			Contract: "weave.contract/v1",
			CLI:      "weave.cli/v1",
			OK:       false,
			Error: contractv1.OperationError{
				Code:    code,
				Message: message,
			},
		})
	}
	printOperationError(stderr, code, message)
	return 1
}

func printExtensionDataHelp(writer io.Writer) {
	fmt.Fprintln(writer, `weave extensions data — read extension-owned opaque artifacts

usage:
  weave extensions data list <path> [--extension <name>] [--doc <id>] [--json]
  weave extensions data get <path> <extension> <key> --out <file> [--force] [--json]
  weave extensions data gc <path> [--dry-run] [--json]

The commands never parse or index the bytes. Extension data is produced by
weave-<name> enrichers through their workspace manifest.`)
}
