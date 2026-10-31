package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine/operations"
	"github.com/prulloac/weave/engine/privacy"
	sqlitestore "github.com/prulloac/weave/storage/sqlite"
	"github.com/prulloac/weave/watch"
)

type syncWatchOptions struct {
	Root     string
	DB       string
	Mode     string
	Codec    string
	Provider string
	JSON     bool
	Events   bool
}

func runSyncWatch(ctx context.Context, options syncWatchOptions, stdout, stderr io.Writer) int {
	dbPath, err := sqlitestore.ExpandPath(options.DB)
	if err != nil {
		return syncWatchFailure(stdout, stderr, options.JSON, contractv1.ErrInvalidRequest, err.Error())
	}
	store, err := sqlitestore.OpenExisting(dbPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return syncWatchFailure(stdout, stderr, options.JSON, contractv1.ErrBundleNotIndexed,
				"database does not exist; run: weave ingest "+options.Root)
		}
		return syncWatchFailure(stdout, stderr, options.JSON, contractv1.ErrInvalidRequest, err.Error())
	}
	defer store.Close()

	indexed, err := store.HasBundle(ctx, options.Root)
	if err != nil {
		return syncWatchFailure(stdout, stderr, options.JSON, contractv1.ErrInternal, err.Error())
	}
	if !indexed {
		return syncWatchFailure(stdout, stderr, options.JSON, contractv1.ErrBundleNotIndexed,
			"bundle is not indexed; run: weave ingest "+options.Root)
	}

	provider, err := privacy.ResolveProvider(options.Root, options.Provider)
	if err != nil {
		return syncWatchFailure(stdout, stderr, options.JSON, contractv1.ErrPrivacyKey, err.Error())
	}
	if provider != nil {
		defer provider.Close()
	}

	initial := true
	watchOptions := watch.Options{
		Root:     options.Root,
		Store:    store,
		Mode:     options.Mode,
		Codec:    options.Codec,
		Provider: provider,
		OnSync: func(report contractv1.SyncReport) {
			if initial {
				initial = false
				if options.JSON {
					_ = writeJSON(stdout, successEnvelope(report))
					return
				}
				printSyncReport(stdout, report)
				return
			}
			if options.JSON {
				if options.Events {
					event := contractv1.WatchEvent{
						Event:   "sync",
						At:      time.Now().UTC().Format(time.RFC3339),
						Added:   report.Added,
						Updated: report.Updated,
						Removed: report.Removed,
						Errors:  report.Errors,
					}
					_ = writeJSON(stdout, event)
				}
				return
			}
			fmt.Fprintf(stdout, "synced documents=%d added=%d updated=%d removed=%d errors=%d\n",
				report.Documents, report.Added, report.Updated, report.Removed, report.Errors)
		},
		OnError: func(err error) {
			fmt.Fprintf(stderr, "weave sync: watch: %s\n", err)
		},
	}
	watcher, err := watch.New(watchOptions)
	if err != nil {
		return syncWatchFailure(stdout, stderr, options.JSON, contractv1.ErrInvalidRequest, err.Error())
	}
	if err := watcher.Run(ctx); err != nil {
		switch {
		case errors.Is(err, privacy.ErrKeyUnavailable):
			return syncWatchFailure(stdout, stderr, options.JSON, contractv1.ErrPrivacyKey, err.Error())
		case errors.Is(err, sqlitestore.ErrBundleNotIndexed):
			return syncWatchFailure(stdout, stderr, options.JSON, contractv1.ErrBundleNotIndexed,
				"bundle is not indexed; run: weave ingest "+options.Root)
		default:
			return syncWatchFailure(stdout, stderr, options.JSON, contractv1.ErrInternal, err.Error())
		}
	}
	return 0
}

func syncWatchFailure(stdout, stderr io.Writer, jsonMode bool, code contractv1.ErrorCode, message string) int {
	if jsonMode {
		_ = writeJSON(stdout, contractv1.FailureEnvelope{
			Contract: operations.ContractVersion,
			CLI:      operations.CLIVersion,
			OK:       false,
			Error: contractv1.OperationError{
				Code:    code,
				Message: message,
			},
		})
	}
	fmt.Fprintf(stderr, "weave: %s\n", message)
	return exitCodeFor(code)
}

func printSyncReport(writer io.Writer, report contractv1.SyncReport) {
	fmt.Fprintf(writer, "%d documents, added=%d updated=%d removed=%d skipped=%d warnings=%d errors=%d\n",
		report.Documents, report.Added, report.Updated, report.Removed,
		report.Skipped, report.Warnings, report.Errors)
}
