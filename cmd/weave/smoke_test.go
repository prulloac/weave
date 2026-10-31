package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

type smokeResult struct {
	code   int
	stdout string
	stderr string
}

func TestSmoke(t *testing.T) {
	binary := smokeBinary(t)
	root := t.TempDir()
	env := smokeEnvironment(t, root)

	version := smokeJSON(t, binary, env, "version", "--json")
	expectedVersion := os.Getenv("WEAVE_SMOKE_VERSION")
	if expectedVersion == "" {
		expectedVersion = "smoke"
	}
	if version["binary_version"] != expectedVersion {
		t.Fatalf("binary_version = %v, want %s", version["binary_version"], expectedVersion)
	}
	smokeJSON(t, binary, env, "contract", "--json")
	if help := smokeText(t, binary, env, "help"); !strings.Contains(help, "search") {
		t.Fatalf("help does not mention search:\n%s", help)
	}
	if completion := smokeText(t, binary, env, "completion", "bash"); !strings.Contains(completion, "weave") {
		t.Fatalf("completion does not mention weave:\n%s", completion)
	}

	smokeOK(t, binary, env, "config", "init", "--json")
	smokeJSON(t, binary, env, "config", "show", "--json")
	smokeJSON(t, binary, env, "config", "paths", "--json")

	bundle := filepath.Join(root, "bundle")
	smokeCopy(t, filepath.Join("testdata", "storage", "bundle-a"), bundle)
	db := filepath.Join(root, "index.db")

	ingest := smokeJSON(t, binary, env, "ingest", bundle, "--strict", "--db", db, "--json")
	if ingest["documents"].(float64) != 3 {
		t.Fatalf("ingest documents = %v, want 3", ingest["documents"])
	}

	for _, testCase := range []struct {
		mode string
		key  string
	}{
		{mode: "keyword", key: "keyword"},
		{mode: "fuzzy", key: "fuzzy"},
		{mode: "hybrid", key: "hybrid"},
		{mode: "bm25", key: "lexical"},
	} {
		result := smokeJSON(t, binary, env, "search", bundle, "alpha", "--mode", testCase.mode, "--db", db, "--json")
		if len(smokeArray(t, result, testCase.key)) == 0 {
			t.Fatalf("%s search returned no hits: %+v", testCase.mode, result)
		}
	}

	smokeJSON(t, binary, env, "explain", bundle, "alpha", "--db", db, "--json")
	smokeJSON(t, binary, env, "list", bundle, "--db", db, "--json")
	smokeJSON(t, binary, env, "show", bundle, "a", "--db", db, "--json")
	smokeJSON(t, binary, env, "backlinks", bundle, "b", "--db", db, "--json")
	smokeJSON(t, binary, env, "path", bundle, "a", "c", "--db", db, "--json")
	smokeJSON(t, binary, env, "relate", bundle, "a", "--db", db, "--json")
	smokeJSON(t, binary, env, "audit", bundle, "--db", db, "--fail-on", "none", "--json")

	noop := smokeJSON(t, binary, env, "sync", bundle, "--db", db, "--json")
	if noop["updated"].(float64) != 0 || noop["added"].(float64) != 0 || noop["removed"].(float64) != 0 {
		t.Fatalf("no-op sync changed documents: %+v", noop)
	}

	smokeWrite(t, filepath.Join(bundle, "b.md"), `---
type: Concept
title: "B Updated"
keywords:
  - "beta"
---

# B Updated

See [A](a.md).
`)
	smokeWrite(t, filepath.Join(bundle, "d.md"), `---
type: Concept
title: "D"
keywords:
  - "delta"
---

# D

See [A](a.md).
`)
	if err := os.Remove(filepath.Join(bundle, "c.md")); err != nil {
		t.Fatal(err)
	}
	syncReport := smokeJSON(t, binary, env, "sync", bundle, "--db", db, "--json")
	if syncReport["added"].(float64) != 1 || syncReport["updated"].(float64) != 1 || syncReport["removed"].(float64) != 1 {
		t.Fatalf("unexpected sync report: %+v", syncReport)
	}
	delta := smokeJSON(t, binary, env, "search", bundle, "delta", "--mode", "keyword", "--db", db, "--json")
	if len(smokeArray(t, delta, "keyword")) == 0 {
		t.Fatalf("sync did not index the new document: %+v", delta)
	}

	exportDir := filepath.Join(root, "export")
	smokeJSON(t, binary, env, "export", bundle, "--out", exportDir, "--format", "raw", "--yes", "--db", db, "--json")
	if _, err := os.Stat(filepath.Join(exportDir, "index.md")); err != nil {
		t.Fatalf("raw export missing index.md: %v", err)
	}

	secondary := filepath.Join(root, "secondary")
	smokeCopy(t, filepath.Join("testdata", "storage", "bundle-b"), secondary)
	multiDB := filepath.Join(root, "multi.db")
	smokeJSON(t, binary, env, "ingest", bundle, "--source", "b="+secondary, "--strict", "--db", multiDB, "--json")
	multi := smokeJSON(t, binary, env, "search", bundle, "quartz", "--source", "b="+secondary, "--mode", "keyword", "--db", multiDB, "--json")
	if len(smokeArray(t, multi, "keyword")) == 0 {
		t.Fatalf("multi-source search returned no hits: %+v", multi)
	}

	privacy := filepath.Join(root, "privacy")
	smokeCopy(t, filepath.Join("testdata", "privacy", "bundle-a"), privacy)
	privacyEnv := append(append([]string{}, env...),
		"WEAVE_KEY_OPS_PROD_KEY="+strings.Repeat("ab", 32),
		"WEAVE_KEY_TENANT_SALT="+strings.Repeat("cd", 32),
		"WEAVE_KEY_AUDIT_DIGEST="+strings.Repeat("ef", 32),
	)
	smokeJSON(t, binary, privacyEnv, "ingest", privacy, "--strict", "--db", filepath.Join(root, "privacy.db"), "--json")
	decrypted := smokeJSON(t, binary, privacyEnv, "show", privacy, "secret", "--decrypt", "--db", filepath.Join(root, "privacy.db"), "--json")
	if !strings.Contains(fmt.Sprint(decrypted["body"]), "zanzibar-secret") {
		t.Fatalf("decrypted body missing plaintext: %+v", decrypted)
	}
	smokeJSON(t, binary, privacyEnv, "audit", privacy, "--verify-keys", "--fail-on", "none", "--db", filepath.Join(root, "privacy.db"), "--json")

	smokeMCP(t, binary, env, bundle)

	smokeFailure(t, binary, env, "unknown")
	smokeFailure(t, binary, env, "search", bundle, "alpha", "--mode", "nope", "--db", db, "--json")
	smokeFailure(t, binary, env, "show", bundle, "missing", "--db", db, "--json")
}

func smokeBinary(t *testing.T) string {
	t.Helper()
	if path := os.Getenv("WEAVE_SMOKE_BINARY"); path != "" {
		absolute, err := filepath.Abs(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(absolute); err != nil {
			t.Fatalf("WEAVE_SMOKE_BINARY: %v", err)
		}
		return absolute
	}
	binary := filepath.Join(t.TempDir(), "weave")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	command := exec.Command("go", "build", "-trimpath", "-ldflags", "-s -w -X main.version=smoke", "-o", binary, ".")
	command.Dir = "."
	command.Env = append(os.Environ(), "CGO_ENABLED=0")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("build smoke binary: %v\n%s", err, output)
	}
	return binary
}

func smokeEnvironment(t *testing.T, root string) []string {
	t.Helper()
	home := filepath.Join(root, "home")
	config := filepath.Join(home, ".config")
	cache := filepath.Join(home, ".cache")
	for _, path := range []string{home, config, cache, filepath.Join(cache, "weave")} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	overrides := []string{
		"HOME=" + home,
		"USERPROFILE=" + home,
		"XDG_CONFIG_HOME=" + config,
		"XDG_CACHE_HOME=" + cache,
		"WEAVE_CACHE_DIR=" + filepath.Join(cache, "weave"),
		"AppData=" + filepath.Join(home, "appdata"),
		"NO_COLOR=1",
	}
	return append(cleanSmokeEnv(), overrides...)
}

func cleanSmokeEnv() []string {
	var env []string
	for _, entry := range os.Environ() {
		key := strings.SplitN(entry, "=", 2)[0]
		upper := strings.ToUpper(key)
		switch {
		case upper == "HOME", upper == "USERPROFILE", upper == "APPDATA", upper == "LOCALAPPDATA",
			strings.HasPrefix(upper, "XDG_"), strings.HasPrefix(upper, "WEAVE_"), upper == "NO_COLOR":
			continue
		}
		env = append(env, entry)
	}
	return env
}

func smokeRun(t *testing.T, binary string, env []string, args ...string) smokeResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, binary, args...)
	command.Env = env
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	if ctx.Err() != nil {
		t.Fatalf("command %v timed out\nstdout:\n%s\nstderr:\n%s", args, stdout.String(), stderr.String())
	}
	code := 0
	if exit, ok := err.(*exec.ExitError); ok {
		code = exit.ExitCode()
	} else if err != nil {
		t.Fatalf("run %v: %v\nstderr:\n%s", args, err, stderr.String())
	}
	return smokeResult{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

func smokeOK(t *testing.T, binary string, env []string, args ...string) map[string]any {
	t.Helper()
	result := smokeRun(t, binary, env, args...)
	if result.code != 0 {
		t.Fatalf("command %v exit=%d\nstdout:\n%s\nstderr:\n%s", args, result.code, result.stdout, result.stderr)
	}
	envelope := smokeEnvelope(t, args, result.stdout)
	if envelope["ok"] != true {
		t.Fatalf("command %v returned a failure envelope: %+v", args, envelope)
	}
	value, _ := envelope["result"].(map[string]any)
	if value == nil {
		return map[string]any{}
	}
	return value
}

func smokeText(t *testing.T, binary string, env []string, args ...string) string {
	t.Helper()
	result := smokeRun(t, binary, env, args...)
	if result.code != 0 {
		t.Fatalf("command %v exit=%d\nstdout:\n%s\nstderr:\n%s", args, result.code, result.stdout, result.stderr)
	}
	return result.stdout
}

func smokeJSON(t *testing.T, binary string, env []string, args ...string) map[string]any {
	t.Helper()
	return smokeOK(t, binary, env, args...)
}

func smokeEnvelope(t *testing.T, args []string, stdout string) map[string]any {
	t.Helper()
	var envelope map[string]any
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("command %v returned invalid JSON: %v\n%s", args, err, stdout)
	}
	return envelope
}

func smokeFailure(t *testing.T, binary string, env []string, args ...string) {
	t.Helper()
	result := smokeRun(t, binary, env, args...)
	if result.code == 0 {
		t.Fatalf("command %v unexpectedly succeeded\nstdout:\n%s", args, result.stdout)
	}
	stdout := strings.TrimSpace(result.stdout)
	if !strings.HasPrefix(stdout, "{") {
		if strings.TrimSpace(result.stderr) == "" {
			t.Fatalf("command %v failed without a diagnostic\nstdout:\n%s", args, result.stdout)
		}
		return
	}
	envelope := smokeEnvelope(t, args, result.stdout)
	if envelope["ok"] != false {
		t.Fatalf("command %v returned success envelope for failure: %+v", args, envelope)
	}
}

func smokeArray(t *testing.T, result map[string]any, key string) []any {
	t.Helper()
	value, ok := result[key].([]any)
	if !ok {
		t.Fatalf("result %q is not an array: %+v", key, result)
	}
	return value
}

func smokeWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func smokeCopy(t *testing.T, source, destination string) {
	t.Helper()
	if err := os.CopyFS(destination, os.DirFS(source)); err != nil {
		t.Fatalf("copy %s: %v", source, err)
	}
}

func smokeMCP(t *testing.T, binary string, env []string, bundle string) {
	t.Helper()
	lines := []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"smoke","version":"1"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"weave_search","arguments":{"query":"alpha","mode":"keyword","top_k":5}}}`,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, binary, "mcp", "--bundle", bundle, "--stdio")
	command.Env = env
	command.Stdin = strings.NewReader(strings.Join(lines, "\n") + "\n")
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		t.Fatalf("mcp stdio: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}
	responses := make(map[float64]map[string]any)
	for _, line := range strings.Split(strings.TrimSpace(stdout.String()), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var response map[string]any
		if err := json.Unmarshal([]byte(line), &response); err != nil {
			t.Fatalf("decode MCP response %q: %v", line, err)
		}
		if id, ok := response["id"].(float64); ok {
			responses[id] = response
		}
	}
	for _, id := range []float64{1, 2, 3} {
		if responses[id] == nil {
			t.Fatalf("missing MCP response id %v: %s", id, stdout.String())
		}
	}
	if responses[1]["result"].(map[string]any)["protocolVersion"] != "2025-06-18" {
		t.Fatalf("unexpected MCP initialize response: %+v", responses[1])
	}
	tools := responses[2]["result"].(map[string]any)["tools"].([]any)
	if len(tools) == 0 {
		t.Fatalf("MCP tools/list returned no tools: %+v", responses[2])
	}
	if _, ok := responses[3]["result"].(map[string]any)["content"]; !ok {
		t.Fatalf("MCP search returned no content: %+v", responses[3])
	}
}
