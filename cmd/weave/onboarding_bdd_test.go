package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
)

func TestNoArgsStartsHere(t *testing.T) {
	code, stdout, stderr := Invoke(t)
	if code != 0 || stderr != "" {
		t.Fatalf("no-args exit=%d stderr=%q", code, stderr)
	}
	for _, want := range []string{"start here", "weave ingest", "weave search", "weave --help"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("start output missing %q:\n%s", want, stdout)
		}
	}
}

func TestEmptyListSuggestsIngest(t *testing.T) {
	root := fixturePath()
	code, _, stderr := InvokeRead(t, "list", root, "--type", "Nope")
	if code != 0 {
		t.Fatalf("empty list exit=%d", code)
	}
	if !strings.Contains(stderr, "weave ingest") {
		t.Fatalf("empty list hint missing ingest: %q", stderr)
	}
}

func TestEmptyExtensionsSuggestsInstall(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	root := fixturePath()
	code, _, stderr := InvokeRead(t, "extensions", "list", root)
	if code != 0 {
		t.Fatalf("extensions exit=%d", code)
	}
	if !strings.Contains(stderr, "no extensions found") || !strings.Contains(stderr, "hint:") {
		t.Fatalf("empty extensions hint missing: %q", stderr)
	}
}

func TestLocalFirstStatement(t *testing.T) {
	for _, args := range [][]string{{"--version"}, {"version"}, {"help"}} {
		code, stdout, stderr := Invoke(t, args...)
		if code != 0 || stderr != "" {
			t.Fatalf("%v exit=%d stderr=%q", args, code, stderr)
		}
		if !strings.Contains(stdout, "no telemetry, no network by default") {
			t.Fatalf("%v missing local-first statement:\n%s", args, stdout)
		}
	}
}

func TestExtensionsSearchPaths(t *testing.T) {
	root := fixturePath()
	code, stdout, stderr := InvokeRead(t, "extensions", "list", root, "--json")
	if code != 0 {
		t.Fatalf("extensions exit=%d stderr=%q", code, stderr)
	}
	var listed contractv1.ExtensionList
	if err := json.Unmarshal(decodeEnvelope(t, stdout).Result, &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.SearchPaths) == 0 || listed.SearchPaths[0] != filepath.Join(root, ".weave", "bin") {
		t.Fatalf("unexpected extension search paths: %+v", listed.SearchPaths)
	}

	code, stdout, stderr = InvokeRead(t, "extensions", "list", root)
	if code != 0 {
		t.Fatalf("human extensions exit=%d", code)
	}
	if !strings.Contains(stdout+stderr, "searched:") {
		t.Fatalf("extensions output missing search paths:\nstdout=%s\nstderr=%s", stdout, stderr)
	}
}

func TestCompletionScripts(t *testing.T) {
	cases := map[string]string{
		"bash": "complete -F _weave_completion weave",
		"zsh":  "#compdef weave",
		"fish": "complete -c weave",
	}
	for shell, marker := range cases {
		code, stdout, stderr := Invoke(t, "completion", shell)
		if code != 0 || stderr != "" {
			t.Fatalf("%s completion exit=%d stderr=%q", shell, code, stderr)
		}
		if !strings.Contains(stdout, marker) || !strings.Contains(stdout, "search") {
			t.Fatalf("%s completion missing marker or commands:\n%s", shell, stdout)
		}
	}
}

func TestCompletionRejectsUnknownShell(t *testing.T) {
	code, _, _ := Invoke(t, "completion", "powershell")
	if code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
}

func TestCompletionHelp(t *testing.T) {
	code, stdout, stderr := Invoke(t, "completion", "--help")
	if code != 0 || stderr != "" {
		t.Fatalf("completion help exit=%d stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, "weave completion bash|zsh|fish") {
		t.Fatalf("completion help missing usage:\n%s", stdout)
	}

	code, stdout, stderr = Invoke(t, "help", "completion")
	if code != 0 || stderr != "" {
		t.Fatalf("help completion exit=%d stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, "weave completion") {
		t.Fatalf("command help missing completion:\n%s", stdout)
	}
}
