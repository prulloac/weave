package extensions_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/prulloac/weave/engine/extensions"
)

func writeScript(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestValidName(t *testing.T) {
	for _, name := range []string{"echo", "a1", "my-ext"} {
		if !extensions.ValidName(name) {
			t.Fatalf("%q must be valid", name)
		}
	}
	for _, name := range []string{"", "-bad", "Bad", "a/b", "a..b", "a b"} {
		if extensions.ValidName(name) {
			t.Fatalf("%q must be invalid", name)
		}
	}
}

func TestDiscoverPrecedenceAndOrdering(t *testing.T) {
	root := t.TempDir()
	writeScript(t, filepath.Join(root, ".weave", "bin", "weave-bundle"), "cat >/dev/null\n")
	pathDir := t.TempDir()
	writeScript(t, filepath.Join(pathDir, "weave-bundle"), "cat >/dev/null\n")
	writeScript(t, filepath.Join(pathDir, "weave-alpha"), "cat >/dev/null\n")
	writeScript(t, filepath.Join(pathDir, "weave-skip"), "cat >/dev/null\n")
	if err := os.Chmod(filepath.Join(pathDir, "weave-skip"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", pathDir)

	discovered := extensions.Discover(root)
	if len(discovered) != 2 {
		t.Fatalf("expected 2 extensions, got %+v", discovered)
	}
	if discovered[0].Name != "alpha" || discovered[0].Source != "path" {
		t.Fatalf("unexpected first extension: %+v", discovered[0])
	}
	if discovered[1].Name != "bundle" || discovered[1].Source != "bundle" {
		t.Fatalf("bundle extension must win: %+v", discovered[1])
	}

	resolved, err := extensions.Resolve(root, "bundle")
	if err != nil || resolved.Source != "bundle" {
		t.Fatalf("resolve bundle: %+v %v", resolved, err)
	}
	if _, err := extensions.Resolve(root, "../escape"); !errors.Is(err, extensions.ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestRunnerEnrichment(t *testing.T) {
	path := filepath.Join(t.TempDir(), "weave-echo")
	writeScript(t, path, `cat >/dev/null
printf '%s' '{"v":"weave.cli/v1","inferred_keywords":[" one ","ONE","two","", "`+strings.Repeat("x", 200)+`"],"custom_metadata":{"a":1}}'
`)
	extension := extensions.Extension{Name: "echo", Path: path}
	response, err := extensions.Runner{}.Enrich(context.Background(), extension, extensions.EnrichmentRequest{
		DocID: "a",
	})
	if err != nil {
		t.Fatalf("enrich: %v", err)
	}
	if len(response.InferredKeywords) != 2 || response.InferredKeywords[0] != "one" {
		t.Fatalf("unexpected keywords: %v", response.InferredKeywords)
	}
}

func TestRunnerFailures(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]struct {
		body string
		want error
	}{
		"exit":    {"exit 3\n", extensions.ErrFailed},
		"json":    {"cat >/dev/null\nprintf 'nope'\n", extensions.ErrProtocol},
		"version": {"cat >/dev/null\nprintf '%s' '{\"v\":\"v2\"}'\n", extensions.ErrProtocol},
		"timeout": {"cat >/dev/null\nsleep 1\nprintf '%s' '{\"v\":\"weave.cli/v1\"}'\n", extensions.ErrTimeout},
	}
	for name, testCase := range cases {
		path := filepath.Join(dir, "weave-"+name)
		writeScript(t, path, testCase.body)
		runner := extensions.Runner{Timeout: 100 * time.Millisecond}
		_, err := runner.Enrich(context.Background(), extensions.Extension{Name: name, Path: path}, extensions.EnrichmentRequest{})
		if !errors.Is(err, testCase.want) {
			t.Fatalf("%s: expected %v, got %v", name, testCase.want, err)
		}
	}
}

func TestRunnerRerankValidation(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]struct {
		body  string
		valid bool
	}{
		"valid":   {`printf '%s' '{"v":"weave.cli/v1","ranked_candidates":[{"id":"a","rerank_score":0.4},{"id":"b","rerank_score":1.4}]}'`, true},
		"missing": {`printf '%s' '{"v":"weave.cli/v1","ranked_candidates":[{"id":"a","rerank_score":0.4}]}'`, false},
		"extra":   {`printf '%s' '{"v":"weave.cli/v1","ranked_candidates":[{"id":"a","rerank_score":0.4},{"id":"b","rerank_score":0.2},{"id":"c","rerank_score":0.1}]}'`, false},
		"dup":     {`printf '%s' '{"v":"weave.cli/v1","ranked_candidates":[{"id":"a","rerank_score":0.4},{"id":"a","rerank_score":0.2}]}'`, false},
	}
	for name, testCase := range cases {
		path := filepath.Join(dir, "weave-"+name)
		writeScript(t, path, "cat >/dev/null\n"+testCase.body+"\n")
		scores, err := extensions.Runner{}.Rerank(context.Background(), extensions.Extension{Name: name, Path: path}, extensions.RerankRequest{
			Candidates: []extensions.Candidate{{ID: "a"}, {ID: "b"}},
		})
		if testCase.valid {
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			if scores["b"] != 1 {
				t.Fatalf("%s: score must clamp to 1, got %v", name, scores["b"])
			}
			continue
		}
		if !errors.Is(err, extensions.ErrProtocol) {
			t.Fatalf("%s: expected protocol error, got %v", name, err)
		}
	}
}
