package operations

import (
	"context"
	"errors"
	"os"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine/move"
	sqlitestore "github.com/prulloac/weave/storage/sqlite"
)

// move implements `weave mv`: plan the file rewrites, apply them, then update the index in one
// transaction. A failed index update restores every touched file.
func (e *Executor) move(ctx context.Context, request contractv1.OperationRequest) any {
	plan, err := move.Build(request.Path, request.From, request.To)
	if err != nil {
		return moveFailure(err, request)
	}
	result := contractv1.MoveResult{
		From:      plan.From,
		To:        plan.To,
		DryRun:    request.DryRun,
		Rewritten: plan.Rewritten,
	}
	if request.DryRun {
		return success(result)
	}

	dbPath, err := resolveDBPath(request.DB)
	if err == nil {
		if _, statErr := os.Stat(dbPath); statErr != nil {
			dbPath = ""
		}
	} else {
		dbPath = ""
	}
	applyErr := plan.Apply(func() error {
		if dbPath == "" {
			return nil
		}
		store, err := sqlitestore.OpenExisting(dbPath)
		if err != nil {
			return err
		}
		defer store.Close()
		indexed, err := store.HasBundle(ctx, request.Path)
		if err != nil {
			return err
		}
		if !indexed {
			return nil
		}
		tokenizer, err := e.writeTokenizer(ctx, store, request, extensionTimeout(request))
		if err != nil {
			return err
		}
		_, err = store.Sync(ctx, request.Path, sqlitestore.SyncOptions{Tokenizer: tokenizer})
		return err
	})
	if applyErr != nil {
		return failure(contractv1.ErrMoveFailed, applyErr.Error(), map[string]any{
			"from": plan.From,
			"to":   plan.To,
		})
	}
	return success(result)
}

func moveFailure(err error, request contractv1.OperationRequest) any {
	details := map[string]any{"from": request.From, "to": request.To}
	switch {
	case errors.Is(err, move.ErrDestinationExists):
		return failure(contractv1.ErrDestinationExists, err.Error(), details)
	case errors.Is(err, move.ErrConceptNotFound):
		return failure(contractv1.ErrConceptNotFound, err.Error(), details)
	case errors.Is(err, move.ErrPrivacyDirective):
		return failure(contractv1.ErrPrivacyDirective, err.Error(), details)
	case errors.Is(err, move.ErrInvalidRequest):
		return failure(contractv1.ErrInvalidRequest, err.Error(), details)
	default:
		return failure(contractv1.ErrMoveFailed, err.Error(), details)
	}
}
