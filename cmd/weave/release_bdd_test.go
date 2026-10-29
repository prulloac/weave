package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Behavior-level tests for release version injection. The packaging and
// workflow contract live in release/release_test.go.

func TestReleaseVersionInjection(t *testing.T) {
	goBinary, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go binary not available")
	}
	root := filepath.Join("..", "..")
	binary := filepath.Join(t.TempDir(), "weave")

	build := exec.Command(goBinary, "build", "-trimpath",
		"-ldflags", "-s -w -X main.version=v9.9.9",
		"-o", binary, "./cmd/weave")
	build.Dir = root
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, output)
	}

	code, stdout, stderr := runBinary(t, binary, "version", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("version exit=%d stderr=%q", code, stderr)
	}
	var envelope testEnvelope
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("decode version: %v\n%s", err, stdout)
	}
	var info struct {
		BinaryVersion string `json:"binary_version"`
	}
	if err := json.Unmarshal(envelope.Result, &info); err != nil {
		t.Fatal(err)
	}
	if info.BinaryVersion != "v9.9.9" {
		t.Fatalf("binary_version = %q, want v9.9.9", info.BinaryVersion)
	}

	code, stdout, stderr = runBinary(t, binary, "--version")
	if code != 0 || stderr != "" {
		t.Fatalf("--version exit=%d stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, "v9.9.9") {
		t.Fatalf("--version output missing injected version: %q", stdout)
	}
}

func runBinary(t *testing.T, binary string, args ...string) (int, string, string) {
	t.Helper()
	command := exec.Command(binary, args...)
	command.Env = append(os.Environ(), "WEAVE_CACHE_DIR="+t.TempDir())
	var stdout, stderr strings.Builder
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	code := 0
	if exit, ok := err.(*exec.ExitError); ok {
		code = exit.ExitCode()
	} else if err != nil {
		t.Fatalf("run %s: %v", binary, err)
	}
	return code, stdout.String(), stderr.String()
}
