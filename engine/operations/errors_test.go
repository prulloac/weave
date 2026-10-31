package operations

import (
	"errors"
	"os"
	"strings"
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine/extensions"
)

func TestOperationErrorMappings(t *testing.T) {
	cases := []struct {
		name   string
		domain errorDomain
		err    error
		code   contractv1.ErrorCode
		prefix string
	}{
		{"storage permission", domainStorage, os.ErrPermission, contractv1.ErrPermissionDenied, "permission denied: "},
		{"storage missing", domainStorage, os.ErrNotExist, contractv1.ErrInvalidRequest, "database does not exist: "},
		{"storage fallback", domainStorage, errors.New("boom"), contractv1.ErrInvalidRequest, "cannot open database: "},
		{"open permission", domainOpen, os.ErrPermission, contractv1.ErrPermissionDenied, "permission denied: "},
		{"open missing", domainOpen, os.ErrNotExist, contractv1.ErrInvalidRoot, "path does not exist: "},
		{"open fallback", domainOpen, errors.New("boom"), contractv1.ErrInvalidRoot, "cannot open bundle: "},
		{"extension missing", domainExtension, extensions.ErrNotFound, contractv1.ErrExtensionMissing, ""},
		{"extension timeout", domainExtension, extensions.ErrTimeout, contractv1.ErrExtensionTimeout, ""},
		{"extension failed", domainExtension, extensions.ErrFailed, contractv1.ErrExtensionFailed, ""},
		{"extension protocol", domainExtension, extensions.ErrProtocol, contractv1.ErrExtensionProto, ""},
		{"extension fallback", domainExtension, errors.New("boom"), contractv1.ErrExtensionFailed, ""},
	}

	for _, test := range cases {
		test := test
		t.Run(test.name, func(t *testing.T) {
			code, message := mapDomainError(test.domain, test.err)
			if code != test.code {
				t.Fatalf("code = %s, want %s", code, test.code)
			}
			if !strings.HasPrefix(message, test.prefix) {
				t.Fatalf("message = %q, want prefix %q", message, test.prefix)
			}
		})
	}
}

func TestExtensionErrorCode(t *testing.T) {
	cases := map[error]contractv1.ErrorCode{
		extensions.ErrNotFound: contractv1.ErrExtensionMissing,
		extensions.ErrTimeout:  contractv1.ErrExtensionTimeout,
		extensions.ErrFailed:   contractv1.ErrExtensionFailed,
		extensions.ErrProtocol: contractv1.ErrExtensionProto,
	}
	for err, want := range cases {
		if got := extensionErrorCode(err); got != want {
			t.Fatalf("extensionErrorCode(%v) = %s, want %s", err, got, want)
		}
	}
}
