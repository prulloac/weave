package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const configGoldenFixture = `db: /srv/weave/index.db
cache: false
codec: lz4
search_mode: keyword
top_k: 7
ingest_mode: strict
timeout: 2s
enricher: ai
rerank: ai
`

func readConfigGolden(t *testing.T, name string) any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "contract", "v1", "golden", "config", name))
	if err != nil {
		t.Fatal(err)
	}
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatal(err)
	}
	return value
}

func normalizeConfigGolden(value any, configPath string) any {
	switch typed := value.(type) {
	case string:
		return strings.ReplaceAll(typed, configPath, "<config>")
	case []any:
		result := make([]any, len(typed))
		for index, item := range typed {
			result[index] = normalizeConfigGolden(item, configPath)
		}
		return result
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, item := range typed {
			result[key] = normalizeConfigGolden(item, configPath)
		}
		return result
	default:
		return value
	}
}

func TestConfigJSONGoldens(t *testing.T) {
	clearConfigEnv(t)
	config := writeConfig(t, configGoldenFixture)

	cases := []struct {
		name string
		args []string
	}{
		{"show", []string{"config", "show", "--config", config, "--json"}},
		{"get", []string{"config", "get", "top_k", "--config", config, "--json"}},
		{"path", []string{"config", "path", "--config", config, "--json"}},
	}
	for _, testCase := range cases {
		code, stdout, stderr := invokeRaw(t, testCase.args...)
		if code != 0 || stderr != "" {
			t.Fatalf("%s exit=%d stderr=%q", testCase.name, code, stderr)
		}
		envelope := validateCLIJSON(t, stdout, "config.schema.json")
		var actual any
		if err := json.Unmarshal(envelope.Result, &actual); err != nil {
			t.Fatal(err)
		}
		actual = normalizeConfigGolden(actual, config)
		expected := readConfigGolden(t, testCase.name+".json")
		if !reflect.DeepEqual(actual, expected) {
			actualJSON, _ := json.MarshalIndent(actual, "", "  ")
			expectedJSON, _ := json.MarshalIndent(expected, "", "  ")
			t.Fatalf("%s golden mismatch\nactual:\n%s\nexpected:\n%s", testCase.name, actualJSON, expectedJSON)
		}
	}

	path := filepath.Join(t.TempDir(), "nested", "config.yaml")
	code, stdout, stderr := invokeRaw(t, "config", "init", "--config", path, "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("init exit=%d stderr=%q", code, stderr)
	}
	envelope := validateCLIJSON(t, stdout, "config.schema.json")
	var actual any
	if err := json.Unmarshal(envelope.Result, &actual); err != nil {
		t.Fatal(err)
	}
	actual = normalizeConfigGolden(actual, path)
	expected := readConfigGolden(t, "init.json")
	if !reflect.DeepEqual(actual, expected) {
		actualJSON, _ := json.MarshalIndent(actual, "", "  ")
		expectedJSON, _ := json.MarshalIndent(expected, "", "  ")
		t.Fatalf("init golden mismatch\nactual:\n%s\nexpected:\n%s", actualJSON, expectedJSON)
	}
}
