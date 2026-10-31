package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
)

func configShowEntries(show contractv1.ConfigShow) map[string]contractv1.ConfigEntry {
	entries := make(map[string]contractv1.ConfigEntry, len(show.Entries))
	for _, entry := range show.Entries {
		entries[entry.Key] = entry
	}
	return entries
}

func TestConfigGetTypedResult(t *testing.T) {
	clearConfigEnv(t)
	config := writeConfig(t, "top_k: 7\ndb: /srv/weave/index.db\n")
	t.Setenv("WEAVE_TOP_K", "9")

	code, stdout, stderr := invokeRaw(t, "config", "get", "top_k", "--config", config, "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("config get exit=%d stderr=%q", code, stderr)
	}
	envelope := validateCLIJSON(t, stdout, "config.schema.json")
	var result contractv1.ConfigGet
	if err := json.Unmarshal(envelope.Result, &result); err != nil {
		t.Fatal(err)
	}
	if result.Key != "top_k" || result.Value != float64(9) {
		t.Fatalf("unexpected get result: %+v", result)
	}
	if result.Source != "env" || result.Origin != "env WEAVE_TOP_K" {
		t.Fatalf("unexpected top_k provenance: %+v", result)
	}

	code, stdout, stderr = invokeRaw(t, "config", "get", "db", "--config", config, "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("config get db exit=%d stderr=%q", code, stderr)
	}
	envelope = validateCLIJSON(t, stdout, "config.schema.json")
	if err := json.Unmarshal(envelope.Result, &result); err != nil {
		t.Fatal(err)
	}
	if result.Value != "/srv/weave/index.db" || result.Source != "file" {
		t.Fatalf("unexpected db result: %+v", result)
	}
	if result.Origin != "file "+config {
		t.Fatalf("unexpected db origin: %q", result.Origin)
	}

	code, stdout, stderr = invokeRaw(t, "config", "get", "codec", "--config", config, "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("config get codec exit=%d stderr=%q", code, stderr)
	}
	envelope = validateCLIJSON(t, stdout, "config.schema.json")
	if err := json.Unmarshal(envelope.Result, &result); err != nil {
		t.Fatal(err)
	}
	if result.Value != "zstd" || result.Source != "default" || result.Origin != "default" {
		t.Fatalf("unexpected codec result: %+v", result)
	}

	code, stdout, stderr = invokeRaw(t, "config", "get", "top_k", "--config", config)
	if code != 0 || stderr != "" {
		t.Fatalf("human config get exit=%d stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, "top_k: 9 (env WEAVE_TOP_K)") {
		t.Fatalf("human output missing provenance: %q", stdout)
	}
}

func TestConfigGetRejectsUnknownKey(t *testing.T) {
	clearConfigEnv(t)
	code, stdout, _ := invokeRaw(t, "config", "get", "nope", "--json")
	if code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
	envelope := validateCLIJSON(t, stdout, "")
	if envelope.OK || envelope.Error == nil || envelope.Error.Code != contractv1.ErrInvalidRequest {
		t.Fatalf("unexpected error envelope: %+v", envelope)
	}
}

func TestConfigShowOrigins(t *testing.T) {
	clearConfigEnv(t)
	config := writeConfig(t, "top_k: 7\ndb: /srv/weave/index.db\n")
	code, stdout, stderr := invokeRaw(t, "config", "show", "--config", config, "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("config show exit=%d stderr=%q", code, stderr)
	}
	envelope := validateCLIJSON(t, stdout, "config.schema.json")
	var show contractv1.ConfigShow
	if err := json.Unmarshal(envelope.Result, &show); err != nil {
		t.Fatal(err)
	}
	if !show.Exists || show.ConfigPath != config {
		t.Fatalf("unexpected show header: %+v", show)
	}
	entries := configShowEntries(show)
	if entries["top_k"].Origin != "file "+config {
		t.Fatalf("unexpected top_k origin: %+v", entries["top_k"])
	}
	if entries["codec"].Source != "default" || entries["codec"].Origin != "default" {
		t.Fatalf("unexpected codec origin: %+v", entries["codec"])
	}
}

func TestConfigInitPrintsNextCommand(t *testing.T) {
	clearConfigEnv(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	if runtime.GOOS == "windows" {
		t.Setenv("AppData", filepath.Join(home, "appdata"))
	}

	code, stdout, stderr := invokeRaw(t, "config", "init", "--json")
	if code != 0 {
		t.Fatalf("init exit=%d stderr=%q", code, stderr)
	}
	envelope := validateCLIJSON(t, stdout, "config.schema.json")
	var result contractv1.ConfigInit
	if err := json.Unmarshal(envelope.Result, &result); err != nil {
		t.Fatal(err)
	}
	if !result.Created || result.NextCommand != "weave config show" {
		t.Fatalf("unexpected init result: %+v", result)
	}

	code, stdout, stderr = invokeRaw(t, "config", "init", "--force")
	if code != 0 || stderr != "" {
		t.Fatalf("human init exit=%d stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, "next: weave config show") {
		t.Fatalf("human init missing next command: %q", stdout)
	}
}

func TestDefaultConfigPathUsesUserConfigDir(t *testing.T) {
	clearConfigEnv(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")

	dir, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "weave", "config.yaml")
	if got := defaultConfigPath(); got != want {
		t.Fatalf("default config path = %q, want %q", got, want)
	}
}

func TestDefaultConfigPathUsesAbsoluteXDGOnLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("XDG semantics are Linux-specific")
	}
	clearConfigEnv(t)
	home := t.TempDir()
	xdg := filepath.Join(home, "xdg")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", xdg)

	want := filepath.Join(xdg, "weave", "config.yaml")
	if got := defaultConfigPath(); got != want {
		t.Fatalf("default config path = %q, want %q", got, want)
	}
}

func TestDefaultConfigPathIgnoresRelativeXDGOnLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("XDG semantics are Linux-specific")
	}
	clearConfigEnv(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "relative-config")

	want := filepath.Join(home, ".config", "weave", "config.yaml")
	if got := defaultConfigPath(); got != want {
		t.Fatalf("default config path = %q, want %q", got, want)
	}
}

func TestDefaultConfigPathIgnoresAppDataOnUnix(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("XDG semantics are Linux-specific")
	}
	clearConfigEnv(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("AppData", filepath.Join(home, "appdata"))

	want := filepath.Join(home, ".config", "weave", "config.yaml")
	if got := defaultConfigPath(); got != want {
		t.Fatalf("default config path = %q, want %q (AppData must be ignored)", got, want)
	}
}

func TestConfigSubcommandHelp(t *testing.T) {
	code, stdout, stderr := invokeRaw(t, "config", "get", "--help")
	if code != 0 || stderr != "" {
		t.Fatalf("config get --help exit=%d stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, "weave config") || !strings.Contains(stdout, "config get") {
		t.Fatalf("unexpected config help:\n%s", stdout)
	}
}

func TestConfigShowWarningsStayOnStderr(t *testing.T) {
	clearConfigEnv(t)
	config := writeConfig(t, "unknown_key: true\n")
	code, stdout, stderr := invokeRaw(t, "config", "show", "--config", config, "--json")
	if code != 0 {
		t.Fatalf("config show exit=%d", code)
	}
	if !strings.Contains(stderr, "CONFIG_UNKNOWN_KEY") {
		t.Fatalf("expected warning on stderr, got %q", stderr)
	}
	validateCLIJSON(t, stdout, "config.schema.json")
}

func TestConfigPathAndInitIgnoreInvalidFile(t *testing.T) {
	clearConfigEnv(t)
	config := writeConfig(t, "top_k: 101\n")

	code, stdout, stderr := invokeRaw(t, "config", "path", "--config", config, "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("config path exit=%d stderr=%q", code, stderr)
	}
	validateCLIJSON(t, stdout, "config.schema.json")

	code, stdout, stderr = invokeRaw(t, "config", "init", "--config", config, "--force", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("config init --force exit=%d stderr=%q", code, stderr)
	}
	validateCLIJSON(t, stdout, "config.schema.json")
}

func TestConfigPathUsesWeaveConfig(t *testing.T) {
	clearConfigEnv(t)
	config := writeConfig(t, "top_k: 5\n")
	t.Setenv("WEAVE_CONFIG", config)

	code, stdout, stderr := invokeRaw(t, "config", "path", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("config path exit=%d stderr=%q", code, stderr)
	}
	envelope := validateCLIJSON(t, stdout, "config.schema.json")
	var result contractv1.ConfigPath
	if err := json.Unmarshal(envelope.Result, &result); err != nil {
		t.Fatal(err)
	}
	if result.Path != config || !result.Exists {
		t.Fatalf("unexpected config path: %+v", result)
	}

	code, stdout, stderr = invokeRaw(t, "config", "path", "--config="+config, "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("config path --config= exit=%d stderr=%q", code, stderr)
	}
	validateCLIJSON(t, stdout, "config.schema.json")
}
