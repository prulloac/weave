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

type bddConfigShow struct {
	ConfigPath string `json:"config_path"`
	Exists     bool   `json:"exists"`
	Entries    []struct {
		Key    string `json:"key"`
		Value  any    `json:"value"`
		Source string `json:"source"`
		Origin string `json:"origin"`
	} `json:"entries"`
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func clearConfigEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"WEAVE_CONFIG", "WEAVE_DB", "WEAVE_NO_DB", "WEAVE_CACHE", "WEAVE_CACHE_DIR",
		"WEAVE_CODEC", "WEAVE_SEARCH_MODE", "WEAVE_TOP_K", "WEAVE_INGEST_MODE",
		"WEAVE_TIMEOUT", "WEAVE_ENRICHER", "WEAVE_RERANK", "WEAVE_STRICT_CONFIG",
	} {
		os.Unsetenv(key)
	}
}

func configShow(t *testing.T, args ...string) (int, bddConfigShow, string) {
	t.Helper()
	code, stdout, stderr := invokeRaw(t, append([]string{"config", "show"}, args...)...)
	var show bddConfigShow
	if stdout != "" {
		envelope := decodeEnvelope(t, stdout)
		if envelope.OK {
			if err := json.Unmarshal(envelope.Result, &show); err != nil {
				t.Fatalf("decode config: %v", err)
			}
		}
	}
	return code, show, stderr
}

func TestConfigPrecedence(t *testing.T) {
	clearConfigEnv(t)
	root := copyStorageBundle(t, storageFixture("bundle-a"))
	db := filepath.Join(t.TempDir(), "config.db")
	config := writeConfig(t, "db: "+db+"\nsearch_mode: keyword\ntop_k: 2\n")

	code, _, stderr := invokeRaw(t, "ingest", root, "--config", config, "--strict", "--json")
	if code != 0 {
		t.Fatalf("ingest exit=%d stderr=%q", code, stderr)
	}
	code, stdout, stderr := invokeRaw(t, "search", root, "alpha", "--config", config, "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("search exit=%d stderr=%q", code, stderr)
	}
	var response contractv1.SearchResponse
	if err := json.Unmarshal(decodeEnvelope(t, stdout).Result, &response); err != nil {
		t.Fatal(err)
	}
	if response.Mode != contractv1.SearchKeyword || response.TopK != 2 {
		t.Fatalf("file values not applied: mode=%s top_k=%d", response.Mode, response.TopK)
	}

	t.Setenv("WEAVE_SEARCH_MODE", "hybrid")
	t.Setenv("WEAVE_TOP_K", "1")
	code, stdout, stderr = invokeRaw(t, "search", root, "alpha", "--config", config, "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("env search exit=%d stderr=%q", code, stderr)
	}
	if err := json.Unmarshal(decodeEnvelope(t, stdout).Result, &response); err != nil {
		t.Fatal(err)
	}
	if response.Mode != contractv1.SearchHybrid || response.TopK != 1 {
		t.Fatalf("env did not override file: mode=%s top_k=%d", response.Mode, response.TopK)
	}

	code, stdout, stderr = invokeRaw(t, "search", root, "alpha", "--config", config,
		"--mode", "keyword", "--top-k", "3", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("flag search exit=%d stderr=%q", code, stderr)
	}
	if err := json.Unmarshal(decodeEnvelope(t, stdout).Result, &response); err != nil {
		t.Fatal(err)
	}
	if response.Mode != contractv1.SearchKeyword || response.TopK != 3 {
		t.Fatalf("flags did not override env: mode=%s top_k=%d", response.Mode, response.TopK)
	}
}

func TestConfigDatabaseAndEnv(t *testing.T) {
	clearConfigEnv(t)
	root := copyStorageBundle(t, storageFixture("bundle-a"))
	fileDB := filepath.Join(t.TempDir(), "file.db")
	envDB := filepath.Join(t.TempDir(), "env.db")
	config := writeConfig(t, "db: "+fileDB+"\n")

	code, stdout, stderr := invokeRaw(t, "ingest", root, "--config", config, "--strict", "--json")
	if code != 0 {
		t.Fatalf("ingest exit=%d stderr=%q", code, stderr)
	}
	var report contractv1.IngestReport
	if err := json.Unmarshal(decodeEnvelope(t, stdout).Result, &report); err != nil {
		t.Fatal(err)
	}
	if report.DB != fileDB {
		t.Fatalf("expected file db %s, got %s", fileDB, report.DB)
	}

	t.Setenv("WEAVE_DB", envDB)
	code, stdout, stderr = invokeRaw(t, "ingest", root, "--config", config, "--strict", "--json")
	if code != 0 {
		t.Fatalf("env ingest exit=%d stderr=%q", code, stderr)
	}
	if err := json.Unmarshal(decodeEnvelope(t, stdout).Result, &report); err != nil {
		t.Fatal(err)
	}
	if report.DB != envDB {
		t.Fatalf("expected env db %s, got %s", envDB, report.DB)
	}
}

func TestConfigValidation(t *testing.T) {
	clearConfigEnv(t)
	root := copyStorageBundle(t, storageFixture("bundle-a"))

	badTopK := writeConfig(t, "top_k: 101\n")
	code, _, stderr := invokeRaw(t, "list", root, "--config", badTopK, "--json")
	if code != 2 || !strings.Contains(stderr, "top_k") {
		t.Fatalf("expected top_k error, code=%d stderr=%q", code, stderr)
	}

	reserved := writeConfig(t, "keys:\n  secret: value\n")
	code, _, stderr = invokeRaw(t, "list", root, "--config", reserved, "--json")
	if code != 2 || !strings.Contains(stderr, "reserved") {
		t.Fatalf("expected reserved key error, code=%d stderr=%q", code, stderr)
	}

	good := writeConfig(t, "top_k: 5\n")
	code, _, _ = invokeRaw(t, "list", root, "--config", good, "--no-config", "--json")
	if code != 2 {
		t.Fatalf("expected --config/--no-config conflict, got %d", code)
	}
}

func TestConfigUnknownKeys(t *testing.T) {
	clearConfigEnv(t)
	root := copyStorageBundle(t, storageFixture("bundle-a"))
	config := writeConfig(t, "top_k: 5\nunknown_key: true\n")

	code, _, stderr := invokeRaw(t, "list", root, "--config", config, "--no-db", "--json")
	if code != 0 {
		t.Fatalf("unknown key must warn, code=%d stderr=%q", code, stderr)
	}
	if !strings.Contains(stderr, "CONFIG_UNKNOWN_KEY") {
		t.Fatalf("missing warning: %q", stderr)
	}

	code, _, _ = invokeRaw(t, "list", root, "--config", config, "--strict-config", "--no-db", "--json")
	if code != 2 {
		t.Fatalf("strict config must fail, got %d", code)
	}
}

func TestConfigShowAndPath(t *testing.T) {
	clearConfigEnv(t)
	config := writeConfig(t, "top_k: 7\ndb: /tmp/config.db\n")
	t.Setenv("WEAVE_TOP_K", "9")

	code, show, stderr := configShow(t, "--config", config, "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("config show exit=%d stderr=%q", code, stderr)
	}
	if show.ConfigPath != config {
		t.Fatalf("config_path = %q, want %q", show.ConfigPath, config)
	}
	sources := map[string]string{}
	values := map[string]any{}
	for _, entry := range show.Entries {
		sources[entry.Key] = entry.Source
		values[entry.Key] = entry.Value
	}
	if sources["top_k"] != "env" || values["top_k"].(float64) != 9 {
		t.Fatalf("unexpected top_k entry: %+v", show.Entries)
	}
	if sources["db"] != "file" {
		t.Fatalf("expected file source for db: %+v", show.Entries)
	}
	if sources["codec"] != "default" {
		t.Fatalf("expected default source for codec: %+v", show.Entries)
	}

	code, stdout, stderr := invokeRaw(t, "config", "path", "--config", config, "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("config path exit=%d stderr=%q", code, stderr)
	}
	var pathResult struct {
		Path   string `json:"path"`
		Exists bool   `json:"exists"`
	}
	if err := json.Unmarshal(decodeEnvelope(t, stdout).Result, &pathResult); err != nil {
		t.Fatal(err)
	}
	if pathResult.Path != config || !pathResult.Exists {
		t.Fatalf("unexpected path result: %+v", pathResult)
	}
}

func TestConfigInit(t *testing.T) {
	clearConfigEnv(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	if runtime.GOOS == "windows" {
		t.Setenv("AppData", filepath.Join(home, "appdata"))
	}
	path := defaultConfigPath()

	code, _, stderr := invokeRaw(t, "config", "init", "--json")
	if code != 0 {
		t.Fatalf("init exit=%d stderr=%q", code, stderr)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("config not created: %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("config mode = %o, want 600", info.Mode().Perm())
	}

	code, _, _ = invokeRaw(t, "config", "init", "--json")
	if code != 2 {
		t.Fatalf("overwrite must fail, got %d", code)
	}
	code, _, _ = invokeRaw(t, "config", "init", "--force", "--json")
	if code != 0 {
		t.Fatalf("force init failed: %d", code)
	}
}

func TestConfigNoConfigIgnoresFile(t *testing.T) {
	clearConfigEnv(t)
	root := copyStorageBundle(t, storageFixture("bundle-a"))
	fileDB := filepath.Join(t.TempDir(), "file.db")
	envConfig := writeConfig(t, "db: "+fileDB+"\n")
	t.Setenv("WEAVE_CONFIG", envConfig)
	t.Setenv("WEAVE_TOP_K", "4")

	code, stdout, stderr := invokeRaw(t, "ingest", root, "--no-config", "--strict", "--json")
	if code != 0 {
		t.Fatalf("ingest exit=%d stderr=%q", code, stderr)
	}
	var report contractv1.IngestReport
	if err := json.Unmarshal(decodeEnvelope(t, stdout).Result, &report); err != nil {
		t.Fatal(err)
	}
	if report.DB == fileDB {
		t.Fatal("--no-config must ignore the file database")
	}

	code, _, _ = invokeRaw(t, "search", root, "alpha", "--no-config", "--db", report.DB, "--json")
	if code != 0 {
		t.Fatalf("search exit=%d", code)
	}
}
