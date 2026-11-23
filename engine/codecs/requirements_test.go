package codecs

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/prulloac/weave/engine/extensions"
)

func installCodec(t *testing.T, root, name, manifest string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell-script codecs are POSIX only")
	}
	dir := filepath.Join(root, ".weave", "bin")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "#!/bin/sh\nif [ \"$1\" = \"--describe\" ]; then\nprintf '%s' '" + manifest + "'\nexit 0\nfi\ncat\n"
	if err := os.WriteFile(filepath.Join(dir, "weave-codec-"+name), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	trustInternalCodec(t, root, name)
}

func trustInternalCodec(t *testing.T, root, name string) {
	t.Helper()
	if os.Getenv("WEAVE_TRUST_FILE") == "" {
		t.Setenv("WEAVE_TRUST_FILE", filepath.Join(t.TempDir(), "trust.yaml"))
	}
	if _, err := extensions.TrustExtension(root, "codec-"+name); err != nil {
		t.Fatalf("trust codec %s: %v", name, err)
	}
}

func TestCodecRequirementUnmetRefusesCompress(t *testing.T) {
	previous := extensions.CoreVersion
	extensions.CoreVersion = "0.10.1"
	t.Cleanup(func() { extensions.CoreVersion = previous })

	root := t.TempDir()
	installCodec(t, root, "newer", `{"v":"weave.extension/v1","name":"newer","kind":"codec","version":"0.1.0","directions":["compress","decompress"],"requires":[{"name":"weave","min_version":"0.12.0"}]}`)

	_, err := New(root, time.Second).Compress(context.Background(), "newer", []byte("payload"))
	codecErr, ok := err.(*Error)
	if !ok || codecErr.Code != "EXTENSION_REQUIREMENT_UNMET" {
		t.Fatalf("expected EXTENSION_REQUIREMENT_UNMET, got %v", err)
	}
}

func TestCodecDevBuildWarnsAndRuns(t *testing.T) {
	previous := extensions.CoreVersion
	extensions.CoreVersion = "dev"
	t.Cleanup(func() { extensions.CoreVersion = previous })

	root := t.TempDir()
	installCodec(t, root, "newer", `{"v":"weave.extension/v1","name":"newer","kind":"codec","version":"0.1.0","directions":["compress","decompress"],"requires":[{"name":"weave","min_version":"0.12.0"}]}`)

	var warnings []string
	registry := New(root, time.Second)
	registry.Warn = func(message string) { warnings = append(warnings, message) }
	out, err := registry.Compress(context.Background(), "newer", []byte("payload"))
	if err != nil {
		t.Fatalf("development build must not refuse: %v", err)
	}
	if string(out) != "payload" {
		t.Fatalf("unexpected output %q", out)
	}
	if len(warnings) != 1 {
		t.Fatalf("expected one warning, got %v", warnings)
	}
}
