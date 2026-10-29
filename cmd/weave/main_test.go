package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
)

type testEnvelope struct {
	Contract string                     `json:"contract"`
	CLI      string                     `json:"cli"`
	OK       bool                       `json:"ok"`
	Result   json.RawMessage            `json:"result"`
	Error    *contractv1.OperationError `json:"error"`
}

func fixturePath() string {
	return filepath.Join("..", "..", "contract/v1/golden/bundles/okf-bundle")
}

func invoke(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	return invokeRaw(t, withInMemory(args)...)
}

func invokeRaw(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	if os.Getenv("WEAVE_CACHE_DIR") == "" {
		t.Setenv("WEAVE_CACHE_DIR", t.TempDir())
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run(args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
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

func decodeEnvelope(t *testing.T, output string) testEnvelope {
	t.Helper()

	var envelope testEnvelope
	if err := json.Unmarshal([]byte(output), &envelope); err != nil {
		t.Fatalf("decode envelope: %v\n%s", err, output)
	}
	if envelope.Contract != operationsContract {
		t.Fatalf("unexpected contract: %s", envelope.Contract)
	}
	if envelope.CLI != operationsCLI {
		t.Fatalf("unexpected cli: %s", envelope.CLI)
	}
	return envelope
}

const (
	operationsContract = "weave.contract/v1"
	operationsCLI      = "weave.cli/v1"
)

func TestVersionJSON(t *testing.T) {
	code, stdout, stderr := invoke(t, "version", "--json")
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	envelope := decodeEnvelope(t, stdout)
	if !envelope.OK {
		t.Fatal("expected ok envelope")
	}
	var version contractv1.VersionInfo
	if err := json.Unmarshal(envelope.Result, &version); err != nil {
		t.Fatal(err)
	}
	if version.Contract != operationsContract {
		t.Fatalf("unexpected version: %+v", version)
	}
}

func TestContractJSON(t *testing.T) {
	code, stdout, stderr := invoke(t, "contract", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
	envelope := decodeEnvelope(t, stdout)
	var info contractv1.ContractInfo
	if err := json.Unmarshal(envelope.Result, &info); err != nil {
		t.Fatal(err)
	}
	if len(info.Operations) != 15 {
		t.Fatalf("expected 15 operations, got %d", len(info.Operations))
	}
}

func TestParseJSON(t *testing.T) {
	code, stdout, stderr := invoke(t, "parse", fixturePath(), "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
	envelope := decodeEnvelope(t, stdout)
	var bundle contractv1.Bundle
	if err := json.Unmarshal(envelope.Result, &bundle); err != nil {
		t.Fatal(err)
	}
	if len(bundle.Concepts) != 5 {
		t.Fatalf("expected 5 concepts, got %d", len(bundle.Concepts))
	}
	for _, concept := range bundle.Concepts {
		if concept.ID == "concepts/okf-bundle" && concept.Body == "" {
			t.Fatal("expected parse to include body")
		}
	}
}

func TestGraphJSON(t *testing.T) {
	code, stdout, stderr := invoke(t, "graph", fixturePath(), "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
	envelope := decodeEnvelope(t, stdout)
	var graph contractv1.Graph
	if err := json.Unmarshal(envelope.Result, &graph); err != nil {
		t.Fatal(err)
	}
	if len(graph.Nodes) != 5 || len(graph.Edges) != 5 {
		t.Fatalf("unexpected graph size: %d nodes, %d edges", len(graph.Nodes), len(graph.Edges))
	}
}

func TestSearchJSONAndHuman(t *testing.T) {
	code, stdout, stderr := invoke(t, "search", fixturePath(), "graph", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
	envelope := decodeEnvelope(t, stdout)
	var response contractv1.SearchResponse
	if err := json.Unmarshal(envelope.Result, &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Hybrid) == 0 || response.Hybrid[0].ID != "concepts/node-graph-engine" {
		t.Fatalf("unexpected results: %+v", response.Hybrid)
	}

	code, stdout, stderr = invoke(t, "search", fixturePath(), "graph")
	if code != 0 || stderr != "" {
		t.Fatalf("human exit=%d stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, "Node Graph Engine") || strings.Contains(stdout, `"contract"`) {
		t.Fatalf("unexpected human output: %q", stdout)
	}
}

func TestSearchModesAndTopK(t *testing.T) {
	code, stdout, stderr := invoke(t, "search", fixturePath(), "graph", "--mode", "keyword", "--top-k", "1", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
	envelope := decodeEnvelope(t, stdout)
	var response contractv1.SearchResponse
	if err := json.Unmarshal(envelope.Result, &response); err != nil {
		t.Fatal(err)
	}
	if response.Mode != contractv1.SearchKeyword || len(response.Keyword) > 1 {
		t.Fatalf("unexpected keyword response: %+v", response)
	}

	code, stdout, stderr = invoke(t, "search", fixturePath(), "graph", "--mode", "fuzzy", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
	envelope = decodeEnvelope(t, stdout)
	if err := json.Unmarshal(envelope.Result, &response); err != nil {
		t.Fatal(err)
	}
	if response.Mode != contractv1.SearchFuzzy {
		t.Fatalf("unexpected fuzzy response: %+v", response)
	}

	code, _, stderr = invoke(t, "search", fixturePath(), "graph", "--top-k", "0", "--json")
	if code != 2 || !strings.Contains(stderr, "top-k") {
		t.Fatalf("expected top-k usage error, got %d %q", code, stderr)
	}

	code, _, stderr = invoke(t, "search", fixturePath(), "graph", "--mode", "nope", "--json")
	if code != 2 || !strings.Contains(stderr, "mode") {
		t.Fatalf("expected mode usage error, got %d %q", code, stderr)
	}
}

func TestSearchNoMatchesJSON(t *testing.T) {
	code, stdout, stderr := invoke(t, "search", fixturePath(), "zzzz", "--json")
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr, "no matches") {
		t.Fatalf("unexpected stderr: %q", stderr)
	}
	envelope := decodeEnvelope(t, stdout)
	if !envelope.OK {
		t.Fatal("empty result should still be a success envelope")
	}
}

func TestListJSON(t *testing.T) {
	code, stdout, stderr := invoke(t, "list", fixturePath(), "--type", "Concept", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
	envelope := decodeEnvelope(t, stdout)
	var results []contractv1.ListResult
	if err := json.Unmarshal(envelope.Result, &results); err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 concepts, got %d", len(results))
	}
}

func TestShowAndBacklinksJSON(t *testing.T) {
	code, stdout, stderr := invoke(t, "show", fixturePath(), "concepts/okf-bundle", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
	envelope := decodeEnvelope(t, stdout)
	var show contractv1.ShowResult
	if err := json.Unmarshal(envelope.Result, &show); err != nil {
		t.Fatal(err)
	}
	if show.ID != "concepts/okf-bundle" || show.Body == "" {
		t.Fatalf("unexpected show: %+v", show)
	}

	code, stdout, stderr = invoke(t, "backlinks", fixturePath(), "concepts/search-index", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("backlinks exit=%d stderr=%q", code, stderr)
	}
	envelope = decodeEnvelope(t, stdout)
	var backlinks contractv1.BacklinksResult
	if err := json.Unmarshal(envelope.Result, &backlinks); err != nil {
		t.Fatal(err)
	}
	if len(backlinks.Backlinks) != 1 {
		t.Fatalf("expected one backlink, got %+v", backlinks)
	}
}

func TestPathJSONAndNoRoute(t *testing.T) {
	code, stdout, stderr := invoke(t, "path", fixturePath(), "concepts/okf-bundle", "tables/users", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
	envelope := decodeEnvelope(t, stdout)
	var route contractv1.PathResult
	if err := json.Unmarshal(envelope.Result, &route); err != nil {
		t.Fatal(err)
	}
	if len(route.Path) != 2 {
		t.Fatalf("unexpected route: %+v", route)
	}

	code, stdout, stderr = invoke(t, "path", fixturePath(), "concepts/okf-bundle", "concepts/missing-type", "--json")
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr, "no route") {
		t.Fatalf("unexpected stderr: %q", stderr)
	}
	envelope = decodeEnvelope(t, stdout)
	if envelope.OK || envelope.Error == nil || envelope.Error.Code != contractv1.ErrPathNotFound {
		t.Fatalf("unexpected error envelope: %+v", envelope)
	}
}

func TestShowMissingJSON(t *testing.T) {
	code, stdout, stderr := invoke(t, "show", fixturePath(), "does-not-exist", "--json")
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr, "concept not found") {
		t.Fatalf("unexpected stderr: %q", stderr)
	}
	envelope := decodeEnvelope(t, stdout)
	if envelope.OK || envelope.Error == nil || envelope.Error.Code != contractv1.ErrConceptNotFound {
		t.Fatalf("unexpected error envelope: %+v", envelope)
	}
}

func TestUsageErrors(t *testing.T) {
	code, stdout, stderr := invoke(t, "parse", "--json")
	if code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
	if !strings.Contains(stderr, "requires 1 positional") {
		t.Fatalf("unexpected stderr: %q", stderr)
	}
	envelope := decodeEnvelope(t, stdout)
	if envelope.OK || envelope.Error == nil || envelope.Error.Code != contractv1.ErrInvalidRequest {
		t.Fatalf("unexpected usage envelope: %+v", envelope)
	}

	code, _, stderr = invoke(t, "nope")
	if code != 2 || !strings.Contains(stderr, "unknown command") {
		t.Fatalf("unexpected unknown command result: %d %q", code, stderr)
	}
}

func TestCacheDefaultAndNoCache(t *testing.T) {
	cacheDir := t.TempDir()
	t.Setenv("WEAVE_CACHE_DIR", cacheDir)

	code, _, stderr := invoke(t, "search", fixturePath(), "graph", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("expected default cache to write a manifest")
	}

	noCacheDir := t.TempDir()
	t.Setenv("WEAVE_CACHE_DIR", noCacheDir)
	code, _, stderr = invoke(t, "search", fixturePath(), "graph", "--json", "--no-cache")
	if code != 0 || stderr != "" {
		t.Fatalf("no-cache exit=%d stderr=%q", code, stderr)
	}
	entries, _ = os.ReadDir(noCacheDir)
	if len(entries) != 0 {
		t.Fatalf("expected no cache files, got %d", len(entries))
	}
}

func TestHumanCommands(t *testing.T) {
	for _, args := range [][]string{
		{"contract"},
		{"parse", fixturePath()},
		{"graph", fixturePath()},
		{"list", fixturePath()},
		{"show", fixturePath(), "concepts/okf-bundle"},
		{"path", fixturePath(), "concepts/okf-bundle", "tables/users"},
	} {
		code, stdout, stderr := invoke(t, args...)
		if code != 0 {
			t.Fatalf("%v: exit=%d stderr=%q", args, code, stderr)
		}
		if stdout == "" {
			t.Fatalf("%v: expected human output", args)
		}
	}
}
