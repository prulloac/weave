package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"os"
	"testing"
)

// Shared CLI test harness. Feature suites should use these helpers instead of
// inventing new invokers or database openers.

func HarnessInvoke(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	if os.Getenv("WEAVE_CACHE_DIR") == "" {
		t.Setenv("WEAVE_CACHE_DIR", t.TempDir())
	}
	var stdout, stderr bytes.Buffer
	code := run(args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func HarnessInvokePersistent(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	return HarnessInvoke(t, args...)
}

func HarnessOpenDB(t *testing.T, path string) *sql.DB {
	t.Helper()
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = raw.Close() })
	return raw
}

func HarnessDecode[T any](t *testing.T, data []byte) T {
	t.Helper()
	var value T
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatalf("decode JSON: %v\n%s", err, data)
	}
	return value
}
