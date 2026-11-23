package extensions

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func batchExtension(t *testing.T, response string) Extension {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell-script extensions are POSIX only")
	}
	path := filepath.Join(t.TempDir(), "weave-batch")
	body := "#!/bin/sh\ncat >/dev/null\nprintf '%s' '" + response + "'\n"
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return Extension{Name: "batch", Path: path}
}

func TestEnrichBatchValidatesItems(t *testing.T) {
	extension := batchExtension(t, `{"v":"weave.cli/v1","type":"batch_response","results":[{"v":"weave.cli/v1","inferred_keywords":["one"]},{"ok":false,"error":{"code":"EXTENSION_FAILED","message":"second failed"}},{"v":"weave.cli/v1","inferred_keywords":["three"]}]}`)
	requests := []EnrichmentRequest{
		{DocID: "a", Title: "A", Frontmatter: map[string]any{}, Body: "a"},
		{DocID: "b", Title: "B", Frontmatter: map[string]any{}, Body: "b"},
		{DocID: "c", Title: "C", Frontmatter: map[string]any{}, Body: "c"},
	}
	responses, itemErrors, err := (Runner{Timeout: time.Second}).EnrichBatch(context.Background(), extension, requests)
	if err != nil {
		t.Fatal(err)
	}
	if len(responses) != 3 {
		t.Fatalf("responses = %+v", responses)
	}
	if responses[0].InferredKeywords[0] != "one" || responses[2].InferredKeywords[0] != "three" {
		t.Fatalf("unexpected responses: %+v", responses)
	}
	if itemErrors[0] != nil || itemErrors[2] != nil {
		t.Fatalf("unexpected item errors: %+v", itemErrors)
	}
	if itemErrors[1] == nil {
		t.Fatal("second item error missing")
	}
}

func TestEnrichBatchCount(t *testing.T) {
	extension := batchExtension(t, `{"v":"weave.cli/v1","type":"batch_response","results":[{"v":"weave.cli/v1","inferred_keywords":["one"]}]}`)
	_, _, err := (Runner{Timeout: time.Second}).EnrichBatch(context.Background(), extension, []EnrichmentRequest{
		{DocID: "a", Title: "A", Frontmatter: map[string]any{}},
		{DocID: "b", Title: "B", Frontmatter: map[string]any{}},
	})
	if !errors.Is(err, ErrProtocol) {
		t.Fatalf("expected ErrProtocol, got %v", err)
	}
}

func TestEnrichBatchLimit(t *testing.T) {
	requests := make([]EnrichmentRequest, MaxBatchRequests+1)
	for index := range requests {
		requests[index] = EnrichmentRequest{DocID: "doc", Title: "T", Frontmatter: map[string]any{}}
	}
	_, _, err := (Runner{Timeout: time.Second}).EnrichBatch(context.Background(), Extension{Name: "batch"}, requests)
	if !errors.Is(err, ErrProtocol) {
		t.Fatalf("expected ErrProtocol, got %v", err)
	}
}

func TestEnrichBatchMalformed(t *testing.T) {
	extension := batchExtension(t, `{"v":"weave.cli/v1","type":"batch_response","results":[{"v":"weave.cli/v1","inferred_keywords":"not-a-list"}]}`)
	_, _, err := (Runner{Timeout: time.Second}).EnrichBatch(context.Background(), extension, []EnrichmentRequest{
		{DocID: "a", Title: "A", Frontmatter: map[string]any{}},
	})
	if !errors.Is(err, ErrProtocol) {
		t.Fatalf("expected ErrProtocol, got %v", err)
	}
}
