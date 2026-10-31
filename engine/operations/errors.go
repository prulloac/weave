package operations

import (
	"errors"
	"os"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine/extensions"
)

type errorDomain int

const (
	domainStorage errorDomain = iota
	domainOpen
	domainExtension
)

type errorMapping struct {
	domain   errorDomain
	sentinel error
	code     contractv1.ErrorCode
	prefix   string
}

// operationErrorMappings is the single operations-boundary table for sentinel
// errors. Storage, bundle-open and extension errors share it so transport
// surfaces (CLI and MCP) receive one code per failure class.
var operationErrorMappings = []errorMapping{
	{domainStorage, os.ErrPermission, contractv1.ErrPermissionDenied, "permission denied: "},
	{domainStorage, os.ErrNotExist, contractv1.ErrInvalidRequest, "database does not exist: "},
	{domainOpen, os.ErrPermission, contractv1.ErrPermissionDenied, "permission denied: "},
	{domainOpen, os.ErrNotExist, contractv1.ErrInvalidRoot, "path does not exist: "},
	{domainExtension, extensions.ErrNotFound, contractv1.ErrExtensionMissing, ""},
	{domainExtension, extensions.ErrTimeout, contractv1.ErrExtensionTimeout, ""},
	{domainExtension, extensions.ErrFailed, contractv1.ErrExtensionFailed, ""},
	{domainExtension, extensions.ErrProtocol, contractv1.ErrExtensionProto, ""},
}

var errorFallbacks = map[errorDomain]errorMapping{
	domainStorage:   {domain: domainStorage, code: contractv1.ErrInvalidRequest, prefix: "cannot open database: "},
	domainOpen:      {domain: domainOpen, code: contractv1.ErrInvalidRoot, prefix: "cannot open bundle: "},
	domainExtension: {domain: domainExtension, code: contractv1.ErrExtensionFailed},
}

func mapDomainError(domain errorDomain, err error) (contractv1.ErrorCode, string) {
	for _, mapping := range operationErrorMappings {
		if mapping.domain == domain && errors.Is(err, mapping.sentinel) {
			return mapping.code, mapping.prefix + err.Error()
		}
	}
	fallback := errorFallbacks[domain]
	return fallback.code, fallback.prefix + err.Error()
}

func mapStorageError(err error) (contractv1.ErrorCode, string) {
	return mapDomainError(domainStorage, err)
}

func mapOpenError(err error) (contractv1.ErrorCode, string) {
	return mapDomainError(domainOpen, err)
}

func extensionErrorCode(err error) contractv1.ErrorCode {
	code, _ := mapDomainError(domainExtension, err)
	return code
}
