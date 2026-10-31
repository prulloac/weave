package main

import (
	"encoding/json"
	"strings"
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
)

func TestHelpDiscoversCommands(t *testing.T) {
	code, stdout, stderr := invokeRaw(t, "help")
	if code != 0 || stderr != "" {
		t.Fatalf("help exit=%d stderr=%q", code, stderr)
	}
	for _, want := range []string{"local-first", "search", "config", "exit codes", "--config"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("help missing %q:\n%s", want, stdout)
		}
	}
	if strings.Contains(stdout, "mount") {
		t.Fatal("help still advertises the retired mount command")
	}
}

func TestCommandHelp(t *testing.T) {
	for _, args := range [][]string{
		{"help", "search"},
		{"search", "--help"},
	} {
		code, stdout, stderr := invokeRaw(t, args...)
		if code != 0 || stderr != "" {
			t.Fatalf("%v exit=%d stderr=%q", args, code, stderr)
		}
		for _, want := range []string{"weave search", "--mode", "examples"} {
			if !strings.Contains(stdout, want) {
				t.Fatalf("%v missing %q:\n%s", args, want, stdout)
			}
		}
	}

	code, stdout, stderr := invokeRaw(t, "config", "--help")
	if code != 0 || stderr != "" {
		t.Fatalf("config help exit=%d stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, "weave config") || !strings.Contains(stdout, "precedence") {
		t.Fatalf("unexpected config help:\n%s", stdout)
	}
}

func TestFlagEqualsSyntax(t *testing.T) {
	root := copyStorageBundle(t, storageFixture("bundle-a"))
	code, stdout, stderr := invokeRaw(t, "list", root, "--no-db", "--type=Concept", "--limit=1", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("list exit=%d stderr=%q", code, stderr)
	}
	var listed contractv1.ListResults
	if err := json.Unmarshal(decodeEnvelope(t, stdout).Result, &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected one result, got %d", len(listed))
	}
}

func TestConfigEqualsSyntax(t *testing.T) {
	clearConfigEnv(t)
	root := copyStorageBundle(t, storageFixture("bundle-a"))
	config := writeConfig(t, "top_k: 3\n")
	code, _, stderr := invokeRaw(t, "list", root, "--no-db", "--config="+config, "--json")
	if code != 0 {
		t.Fatalf("list exit=%d stderr=%q", code, stderr)
	}
}
