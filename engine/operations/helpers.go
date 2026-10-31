package operations

import (
	"errors"
	"os"

	contractv1 "github.com/prulloac/weave/contract/v1"
	sqlitestore "github.com/prulloac/weave/storage/sqlite"
)

func resolveDBPath(path string) (string, error) {
	if path == "" {
		return sqlitestore.DefaultPath()
	}
	return sqlitestore.ExpandPath(path)
}

func mapStorageError(err error) (contractv1.ErrorCode, string) {
	if errors.Is(err, os.ErrPermission) {
		return contractv1.ErrPermissionDenied, "permission denied: " + err.Error()
	}
	if errors.Is(err, os.ErrNotExist) {
		return contractv1.ErrInvalidRequest, "database does not exist: " + err.Error()
	}
	return contractv1.ErrInvalidRequest, "cannot open database: " + err.Error()
}

func success(result any) contractv1.SuccessEnvelope {
	return contractv1.SuccessEnvelope{
		Contract: ContractVersion,
		CLI:      CLIVersion,
		OK:       true,
		Result:   result,
	}
}

func failure(code contractv1.ErrorCode, message string, details map[string]any) contractv1.FailureEnvelope {
	return contractv1.FailureEnvelope{
		Contract: ContractVersion,
		CLI:      CLIVersion,
		OK:       false,
		Error: contractv1.OperationError{
			Code:    code,
			Message: message,
			Details: details,
		},
	}
}

func mapOpenError(err error) (contractv1.ErrorCode, string) {
	if errors.Is(err, os.ErrPermission) {
		return contractv1.ErrPermissionDenied, "permission denied: " + err.Error()
	}
	if errors.Is(err, os.ErrNotExist) {
		return contractv1.ErrInvalidRoot, "path does not exist: " + err.Error()
	}
	return contractv1.ErrInvalidRoot, "cannot open bundle: " + err.Error()
}
