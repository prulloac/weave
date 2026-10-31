package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

// Shared CLI test harness. Feature suites should use these helpers instead of
// inventing new invokers or database openers.

func Invoke(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	if os.Getenv("WEAVE_CACHE_DIR") == "" && (len(args) == 0 || args[0] != "config") {
		t.Setenv("WEAVE_CACHE_DIR", t.TempDir())
	}
	var stdout, stderr bytes.Buffer
	code := run(args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func InvokePersistent(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	return Invoke(t, args...)
}

func InvokeRead(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	return Invoke(t, withInMemory(args)...)
}

func InvokeBinary(t *testing.T, binary string, args ...string) (int, string, string) {
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

func OpenDB(t *testing.T, path string) *sql.DB {
	t.Helper()
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = raw.Close() })
	return raw
}

func Decode[T any](t *testing.T, data []byte) T {
	t.Helper()
	var value T
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatalf("decode JSON: %v\n%s", err, data)
	}
	return value
}

func withInMemory(args []string) []string {
	if len(args) == 0 {
		return args
	}
	switch args[0] {
	case "ingest", "rebuild", "sync":
		return args
	}
	for _, arg := range args {
		if arg == "--db" || arg == "--no-db" {
			return args
		}
	}
	return append(args, "--no-db")
}

var (
	testSchemaOnce     sync.Once
	testSchemaCompiler *jsonschema.Compiler
	testSchemaErr      error
)

func testSchemas(t *testing.T) *jsonschema.Compiler {
	t.Helper()
	testSchemaOnce.Do(func() {
		dir := filepath.Join("..", "..", "contract", "v1", "schema")
		entries, err := os.ReadDir(dir)
		if err != nil {
			testSchemaErr = err
			return
		}
		compiler := jsonschema.NewCompiler()
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			file, err := os.Open(filepath.Join(dir, entry.Name()))
			if err != nil {
				testSchemaErr = err
				return
			}
			doc, err := jsonschema.UnmarshalJSON(file)
			_ = file.Close()
			if err != nil {
				testSchemaErr = err
				return
			}
			object, ok := doc.(map[string]any)
			id, _ := object["$id"].(string)
			if !ok || id == "" {
				testSchemaErr = fmt.Errorf("%s: missing $id", entry.Name())
				return
			}
			if err := compiler.AddResource(id, doc); err != nil {
				testSchemaErr = err
				return
			}
		}
		testSchemaCompiler = compiler
	})
	if testSchemaErr != nil {
		t.Fatal(testSchemaErr)
	}
	return testSchemaCompiler
}

func validateTestSchema(t *testing.T, name string, doc any) {
	t.Helper()
	schema, err := testSchemas(t).Compile("https://weave.dev/contract/v1/" + name)
	if err != nil {
		t.Fatalf("compile %s: %v", name, err)
	}
	if err := schema.Validate(doc); err != nil {
		t.Fatalf("%s validation failed: %v", name, err)
	}
}

// validateCLIJSON asserts that stdout is exactly one weave.cli/v1 envelope,
// validates the envelope schema and, for success envelopes, validates the
// result against the named schema.
func validateCLIJSON(t *testing.T, stdout, resultSchema string) testEnvelope {
	t.Helper()
	var doc any
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("stdout is not one JSON value: %v\n%s", err, stdout)
	}
	envelope := decodeEnvelope(t, stdout)
	validateTestSchema(t, "envelope.schema.json", doc)
	if envelope.OK && resultSchema != "" {
		var result any
		if err := json.Unmarshal(envelope.Result, &result); err != nil {
			t.Fatalf("decode result: %v", err)
		}
		validateTestSchema(t, resultSchema, result)
	}
	return envelope
}
