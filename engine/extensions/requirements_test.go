package extensions_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/prulloac/weave/engine/extensions"
)

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		left, right string
		want        int
		ok          bool
	}{
		{"0.10.1", "0.8.0", 1, true},
		{"0.8.0", "0.8.0", 0, true},
		{"0.8.0", "0.10.1", -1, true},
		{"v0.9.0", "0.9.0", 0, true},
		{"1.0.0-rc.1", "1.0.0", -1, true},
		{"1.0.0", "1.0.0-rc.1", 1, true},
		{"1.0.0+build.7", "1.0.0", 0, true},
		{"x", "0.1.0", 0, false},
		{"0.1", "0.1.0", 0, false},
	}
	for _, c := range cases {
		got, ok := extensions.CompareVersions(c.left, c.right)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("CompareVersions(%q, %q) = %d, %v; want %d, %v", c.left, c.right, got, ok, c.want, c.ok)
		}
	}
}

func installTestExtension(t *testing.T, root, name, manifest string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell-script extensions are POSIX only")
	}
	dir := filepath.Join(root, ".weave", "bin")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "#!/bin/sh\nif [ \"$1\" = \"--describe\" ]; then\nprintf '%s' '" + manifest + "'\nexit 0\nfi\ncat >/dev/null\nprintf '%s' '{\"v\":\"weave.cli/v1\",\"inferred_keywords\":[\"gate\"]}'\n"
	if err := os.WriteFile(filepath.Join(dir, "weave-"+name), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("WEAVE_TRUST_FILE") == "" {
		t.Setenv("WEAVE_TRUST_FILE", filepath.Join(t.TempDir(), "trust.yaml"))
	}
	if _, err := extensions.TrustExtension(root, name); err != nil {
		t.Fatalf("trust %s: %v", name, err)
	}
}

func useCoreVersion(t *testing.T, release string) {
	t.Helper()
	previous := extensions.CoreVersion
	extensions.CoreVersion = release
	t.Cleanup(func() { extensions.CoreVersion = previous })
}

func TestCheckDescriptorVersion(t *testing.T) {
	cases := []struct {
		name        string
		core        string
		version     string
		wantRefusal bool
		wantWarning bool
	}{
		{"missing version on the 0.11 line", "0.11.0", "", true, false},
		{"version below the line", "0.11.0", "0.9.0", true, false},
		{"version at the line", "0.11.0", "0.11.0", false, false},
		{"version above the line", "0.12.0", "0.11.1", false, false},
		{"invalid version", "0.11.0", "main", true, false},
		{"pre-descriptor core does not enforce", "0.10.1", "", false, false},
		{"development build warns", "dev", "", false, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			useCoreVersion(t, c.core)
			warning, err := extensions.CheckDescriptorVersion("git", c.version)
			if got := err != nil; got != c.wantRefusal {
				t.Fatalf("refusal = %v, want %v (err=%v)", got, c.wantRefusal, err)
			}
			if c.wantRefusal && !errors.Is(err, extensions.ErrRequirementUnmet) {
				t.Fatalf("expected ErrRequirementUnmet, got %v", err)
			}
			if got := warning != ""; got != c.wantWarning {
				t.Fatalf("warning = %q, want %v", warning, c.wantWarning)
			}
		})
	}
}

func TestRequireRefusesUnmetCoreRequirement(t *testing.T) {
	useCoreVersion(t, "0.10.1")
	root := t.TempDir()
	installTestExtension(t, root, "newer", `{"v":"weave.extension/v1","name":"newer","version":"0.1.0","requires":[{"name":"weave","min_version":"0.12.0"}]}`)

	_, warnings, err := extensions.Require(context.Background(), root, "newer", time.Second)
	if !errors.Is(err, extensions.ErrRequirementUnmet) {
		t.Fatalf("expected ErrRequirementUnmet, got %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("no warnings expected for a comparable core, got %v", warnings)
	}
}

func TestRequireAcceptsSatisfiedRequirement(t *testing.T) {
	useCoreVersion(t, "0.10.1")
	root := t.TempDir()
	installTestExtension(t, root, "satisfied", `{"v":"weave.extension/v1","name":"satisfied","version":"0.1.0","requires":[{"name":"weave","min_version":"0.8.0"}]}`)

	extension, _, err := extensions.Require(context.Background(), root, "satisfied", time.Second)
	if err != nil {
		t.Fatalf("Require: %v", err)
	}
	if extension.Name != "satisfied" {
		t.Fatalf("unexpected extension %+v", extension)
	}
}

func TestRequireWarnsOnDevelopmentBuild(t *testing.T) {
	useCoreVersion(t, "dev")
	root := t.TempDir()
	installTestExtension(t, root, "newer", `{"v":"weave.extension/v1","name":"newer","requires":[{"name":"weave","min_version":"0.12.0"}]}`)

	_, warnings, err := extensions.Require(context.Background(), root, "newer", time.Second)
	if err != nil {
		t.Fatalf("development builds must not refuse: %v", err)
	}
	if len(warnings) != 1 {
		t.Fatalf("expected one warning, got %v", warnings)
	}
}

func TestRequireRefusesMissingCompanion(t *testing.T) {
	useCoreVersion(t, "0.10.1")
	root := t.TempDir()
	installTestExtension(t, root, "needshelper", `{"v":"weave.extension/v1","name":"needshelper","requires":[{"name":"helper","min_version":"0.1.0"}]}`)

	_, _, err := extensions.Require(context.Background(), root, "needshelper", time.Second)
	if !errors.Is(err, extensions.ErrRequirementUnmet) {
		t.Fatalf("expected ErrRequirementUnmet for a missing companion, got %v", err)
	}
}

func TestRequireAllowsExtensionWithoutReadableManifest(t *testing.T) {
	useCoreVersion(t, "0.10.1")
	root := t.TempDir()
	dir := filepath.Join(root, ".weave", "bin")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "weave-plain"), []byte("#!/bin/sh\ncat >/dev/null\nprintf '%s' '{}'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("WEAVE_TRUST_FILE") == "" {
		t.Setenv("WEAVE_TRUST_FILE", filepath.Join(t.TempDir(), "trust.yaml"))
	}
	if _, err := extensions.TrustExtension(root, "plain"); err != nil {
		t.Fatal(err)
	}

	if _, _, err := extensions.Require(context.Background(), root, "plain", time.Second); err != nil {
		t.Fatalf("an extension without a readable manifest declares no requirements: %v", err)
	}
}
