package benchutil_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/prulloac/weave/benchmarks/benchutil"
)

func TestRootAndQueries(t *testing.T) {
	root := benchutil.Root()
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("root %q has no go.mod: %v", root, err)
	}
	queries := benchutil.ReadQueries(t)
	if len(queries.Queries) != 40 {
		t.Fatalf("expected 40 queries, got %d", len(queries.Queries))
	}
	manifest := benchutil.ReadManifest(t)
	if manifest.Profiles["10k"].Documents != 10000 {
		t.Fatalf("unexpected manifest: %+v", manifest.Profiles["10k"])
	}
}
