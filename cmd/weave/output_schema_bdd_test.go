package main

import (
	"path/filepath"
	"testing"
)

func TestJSONOutputValidatesSchemas(t *testing.T) {
	root := fixturePath()

	readCases := []struct {
		name   string
		schema string
		args   []string
	}{
		{"version", "cli.schema.json", []string{"version", "--json"}},
		{"contract", "cli.schema.json", []string{"contract", "--json"}},
		{"parse", "bundle.schema.json", []string{"parse", root, "--json"}},
		{"graph", "graph.schema.json", []string{"graph", root, "--json"}},
		{"search", "search.schema.json", []string{"search", root, "search", "--json"}},
		{"list", "list-results.schema.json", []string{"list", root, "--json"}},
		{"show", "show.schema.json", []string{"show", root, "concepts/okf-bundle", "--json"}},
		{"backlinks", "backlinks.schema.json", []string{"backlinks", root, "concepts/search-index", "--json"}},
		{"path", "path.schema.json", []string{"path", root, "concepts/okf-bundle", "concepts/search-index", "--json"}},
		{"audit", "audit.schema.json", []string{"audit", root, "--json"}},
		{"relate", "relate.schema.json", []string{"relate", root, "concepts/okf-bundle", "--json"}},
		{"explain", "explain.schema.json", []string{
			"explain", root, "search", "--id", "concepts/search-index", "--json",
		}},
	}
	for _, testCase := range readCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			code, stdout, stderr := invoke(t, testCase.args...)
			if stdout == "" {
				t.Fatalf("exit=%d stderr=%q: no JSON output", code, stderr)
			}
			validateCLIJSON(t, stdout, testCase.schema)
		})
	}

	rootA, rootB := multiBundleFixture(t)
	for _, command := range []string{"parse", "graph"} {
		code, stdout, stderr := invoke(t, command, rootA, "--source", "b="+rootB, "--json")
		if stdout == "" {
			t.Fatalf("%s exit=%d stderr=%q: no JSON output", command, code, stderr)
		}
		validateCLIJSON(t, stdout, "multi.schema.json")
	}

	storageRoot := copyStorageBundle(t, storageFixture("bundle-a"))
	db := filepath.Join(t.TempDir(), "index.db")
	code, stdout, stderr := invokeRaw(t, "ingest", storageRoot, "--db", db, "--strict", "--json")
	if code != 0 {
		t.Fatalf("ingest exit=%d stderr=%q", code, stderr)
	}
	validateCLIJSON(t, stdout, "ingest.schema.json")

	code, stdout, stderr = invokeRaw(t, "rebuild", storageRoot, "--db", db, "--json")
	if code != 0 {
		t.Fatalf("rebuild exit=%d stderr=%q", code, stderr)
	}
	validateCLIJSON(t, stdout, "ingest.schema.json")

	code, stdout, stderr = invokeRaw(t, "sync", storageRoot, "--db", db, "--json")
	if code != 0 {
		t.Fatalf("sync exit=%d stderr=%q", code, stderr)
	}
	validateCLIJSON(t, stdout, "sync.schema.json")

	extensionRoot := extensionFixture(t)
	code, stdout, stderr = invoke(t, "extensions", "list", extensionRoot, "--json")
	if code != 0 {
		t.Fatalf("extensions exit=%d stderr=%q", code, stderr)
	}
	validateCLIJSON(t, stdout, "extensions.schema.json")

	exportRoot, exportDB := exportFixture(t)
	code, stdout, stderr = invokeRaw(t, "export", exportRoot,
		"--out", filepath.Join(t.TempDir(), "export"),
		"--format", "raw", "--dry-run", "--db", exportDB, "--json")
	if code != 0 {
		t.Fatalf("export exit=%d stderr=%q", code, stderr)
	}
	validateCLIJSON(t, stdout, "export.schema.json")
}

func TestConfigJSONValidatesSchemas(t *testing.T) {
	clearConfigEnv(t)
	config := writeConfig(t, "top_k: 7\ndb: /srv/weave/index.db\n")

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
		if code != 0 {
			t.Fatalf("%s exit=%d stderr=%q", testCase.name, code, stderr)
		}
		validateCLIJSON(t, stdout, "config.schema.json")
	}

	path := filepath.Join(t.TempDir(), "weave", "config.yaml")
	code, stdout, stderr := invokeRaw(t, "config", "init", "--config", path, "--json")
	if code != 0 {
		t.Fatalf("init exit=%d stderr=%q", code, stderr)
	}
	validateCLIJSON(t, stdout, "config.schema.json")
}

func TestErrorEnvelopesValidate(t *testing.T) {
	root := fixturePath()
	exportRoot, exportDB := exportFixture(t)

	commands := [][]string{
		{"show", root, "missing", "--no-db", "--json"},
		{"search", root, "alpha", "--mode", "bogus", "--no-db", "--json"},
		{"config", "get", "nope", "--json"},
		{"export", exportRoot, "--out", filepath.Join(t.TempDir(), "export"),
			"--format", "raw", "--no-input", "--db", exportDB, "--json"},
	}
	for _, args := range commands {
		code, stdout, stderr := invokeRaw(t, args...)
		if code == 0 {
			t.Fatalf("%v: expected a failure, stderr=%q", args, stderr)
		}
		if stdout == "" {
			t.Fatalf("%v: expected a JSON error envelope", args)
		}
		validateCLIJSON(t, stdout, "")
	}
}
