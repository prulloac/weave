package privacy

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/prulloac/weave/engine/extensions"
)

func TestCryptoProviderRequirementUnmetRefuses(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-script providers are POSIX only")
	}
	previous := extensions.CoreVersion
	extensions.CoreVersion = "0.10.1"
	t.Cleanup(func() { extensions.CoreVersion = previous })

	root := t.TempDir()
	dir := filepath.Join(root, ".weave", "bin")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{"v":"weave.extension/v1","name":"crypto-newer","version":"0.1.0","requires":[{"name":"weave","min_version":"0.12.0"}]}`
	body := "#!/bin/sh\nprintf '%s' '" + manifest + "'\n"
	if err := os.WriteFile(filepath.Join(dir, "weave-crypto-newer"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}

	provider, err := ResolveProvider(root, "newer")
	if !errors.Is(err, extensions.ErrRequirementUnmet) {
		t.Fatalf("expected ErrRequirementUnmet, got provider=%v err=%v", provider, err)
	}
	if provider != nil {
		t.Fatalf("an unmet requirement must not return a provider")
	}
}
