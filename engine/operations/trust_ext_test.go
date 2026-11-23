package operations_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/prulloac/weave/engine/extensions"
)

func trustExternalExtension(t *testing.T, root, name string) {
	t.Helper()
	if os.Getenv("WEAVE_TRUST_FILE") == "" {
		t.Setenv("WEAVE_TRUST_FILE", filepath.Join(t.TempDir(), "trust.yaml"))
	}
	if _, err := extensions.TrustExtension(root, name); err != nil {
		t.Fatalf("trust %s: %v", name, err)
	}
}
