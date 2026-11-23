package extensions

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func writeTrustFixture(t *testing.T, root, name, body string) {
	t.Helper()
	dir := filepath.Join(root, ".weave", "bin")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "weave-"+name), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestTrustLifecycle(t *testing.T) {
	t.Setenv("WEAVE_TRUST_FILE", filepath.Join(t.TempDir(), "trust.yaml"))
	root := t.TempDir()
	writeTrustFixture(t, root, "demo", "#!/bin/sh\ncat >/dev/null\n")
	extension, err := Resolve(root, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckTrusted(root, extension); !errors.Is(err, ErrUntrusted) {
		t.Fatalf("expected ErrUntrusted, got %v", err)
	}
	record, err := TrustExtension(root, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if record.SHA256 == "" || record.Path == "" {
		t.Fatalf("incomplete record: %+v", record)
	}
	if err := CheckTrusted(root, extension); err != nil {
		t.Fatalf("trusted extension refused: %v", err)
	}
	states, err := TrustStates()
	if err != nil || len(states) != 1 || !states[0].Matches {
		t.Fatalf("states = %+v, err = %v", states, err)
	}

	writeTrustFixture(t, root, "demo", "#!/bin/sh\ncat >/dev/null\necho changed\n")
	if err := CheckTrusted(root, extension); !errors.Is(err, ErrUntrusted) {
		t.Fatalf("expected ErrUntrusted after a hash change, got %v", err)
	}
	states, _ = TrustStates()
	if len(states) != 1 || states[0].Matches {
		t.Fatalf("hash change not reported: %+v", states)
	}

	if err := UntrustExtension("demo"); err != nil {
		t.Fatal(err)
	}
	if err := CheckTrusted(root, extension); !errors.Is(err, ErrUntrusted) {
		t.Fatalf("expected ErrUntrusted after untrust, got %v", err)
	}
}

func TestPathExtensionNotGated(t *testing.T) {
	t.Setenv("WEAVE_TRUST_FILE", filepath.Join(t.TempDir(), "trust.yaml"))
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "weave-demo"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	extension, err := Resolve("", "demo")
	if err != nil {
		t.Fatal(err)
	}
	if extension.Source != "path" {
		t.Fatalf("source = %s", extension.Source)
	}
	if err := CheckTrusted("", extension); err != nil {
		t.Fatalf("PATH extension was gated: %v", err)
	}
}

func TestBundleTrustFileIgnored(t *testing.T) {
	t.Setenv("WEAVE_TRUST_FILE", filepath.Join(t.TempDir(), "trust.yaml"))
	root := t.TempDir()
	writeTrustFixture(t, root, "demo", "#!/bin/sh\ncat >/dev/null\n")
	extension, err := Resolve(root, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".weave", "trust.yaml"), []byte("version: 1\nextensions:\n  demo:\n    path: "+extension.Path+"\n    sha256: sha256:0000\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CheckTrusted(root, extension); !errors.Is(err, ErrUntrusted) {
		t.Fatalf("bundle trust file granted trust: %v", err)
	}
}
