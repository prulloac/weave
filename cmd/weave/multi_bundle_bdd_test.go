package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
)

// Behavior-level tests for multiple bundle sources.

type bddSourceHit struct {
	ID     string `json:"id"`
	Source string `json:"source"`
}

type bddMultiSearch struct {
	Keyword []bddSourceHit `json:"keyword"`
	Lexical []bddSourceHit `json:"lexical"`
}

func writeMultiDoc(t *testing.T, root, name, id, keyword, body string) {
	t.Helper()
	content := "---\ntype: Concept\ntitle: \"" + id + "\"\nkeywords:\n  - \"" + keyword + "\"\n---\n\n# " + id + "\n\n" + body + "\n"
	if err := os.WriteFile(filepath.Join(root, name+".md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func multiBundleFixture(t *testing.T) (string, string) {
	t.Helper()
	base := t.TempDir()
	rootA := filepath.Join(base, "team-a")
	rootB := filepath.Join(base, "team-b")
	for _, root := range []string{rootA, rootB} {
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeMultiDoc(t, rootA, "common", "common", "common", "shared search term")
	writeMultiDoc(t, rootA, "unique-a", "unique-a", "common", "only in team a")
	writeMultiDoc(t, rootA, "shared", "shared", "shared", "duplicate id")
	writeMultiDoc(t, rootA, "linker", "linker", "linker", "[only b](only-b.md)")
	writeMultiDoc(t, rootB, "shared", "shared", "shared", "duplicate id b")
	writeMultiDoc(t, rootB, "only-b", "only-b", "common", "only in team b")
	return rootA, rootB
}

func runMultiSearch(t *testing.T, rootA, rootB, query, mode string, extra ...string) bddMultiSearch {
	t.Helper()
	args := []string{"search", rootA, query, "--source", "b=" + rootB, "--mode", mode, "--no-db"}
	args = append(args, extra...)
	args = append(args, "--json")
	code, stdout, stderr := invokeRaw(t, args...)
	if code != 0 {
		t.Fatalf("search exit=%d stderr=%q", code, stderr)
	}
	var response bddMultiSearch
	if err := json.Unmarshal(decodeEnvelope(t, stdout).Result, &response); err != nil {
		t.Fatalf("decode search: %v", err)
	}
	return response
}

func TestMultiBundleSearchMerge(t *testing.T) {
	rootA, rootB := multiBundleFixture(t)
	response := runMultiSearch(t, rootA, rootB, "common", "keyword")
	if len(response.Keyword) != 3 {
		t.Fatalf("expected 3 keyword hits, got %+v", response.Keyword)
	}
	seen := map[string]string{}
	for _, hit := range response.Keyword {
		seen[hit.ID] = hit.Source
	}
	if seen["common"] != "team-a" || seen["unique-a"] != "team-a" || seen["only-b"] != "b" {
		t.Fatalf("unexpected source attribution: %+v", seen)
	}
	for index := 1; index < len(response.Keyword); index++ {
		previous := response.Keyword[index-1]
		current := response.Keyword[index]
		if previous.Source > current.Source {
			t.Fatalf("hits are not ordered by source tie-break: %+v", response.Keyword)
		}
	}
}

func TestMultiBundleAmbiguousShow(t *testing.T) {
	rootA, rootB := multiBundleFixture(t)
	code, stdout, stderr := invokeRaw(t, "show", rootA, "shared", "--source", "b="+rootB, "--no-db", "--json")
	if code != 2 {
		t.Fatalf("expected ambiguous target exit 2, got %d stderr=%q stdout=%s", code, stderr, stdout)
	}
	envelope := decodeEnvelope(t, stdout)
	if envelope.Error == nil || envelope.Error.Code != "INVALID_REQUEST" {
		t.Fatalf("expected INVALID_REQUEST, got %+v", envelope.Error)
	}

	code, stdout, stderr = invokeRaw(t, "show", rootA, "b:shared", "--source", "b="+rootB, "--no-db", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("qualified show exit=%d stderr=%q", code, stderr)
	}
	var view contractv1.ShowResult
	if err := json.Unmarshal(decodeEnvelope(t, stdout).Result, &view); err != nil {
		t.Fatal(err)
	}
	if view.ID != "shared" || view.Source != "b" {
		t.Fatalf("unexpected qualified show: %+v", view)
	}
}

func TestMultiBundleListAndAudit(t *testing.T) {
	rootA, rootB := multiBundleFixture(t)
	code, stdout, stderr := invokeRaw(t, "list", rootA, "--source", "b="+rootB, "--no-db", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("list exit=%d stderr=%q", code, stderr)
	}
	var listed contractv1.ListResults
	if err := json.Unmarshal(decodeEnvelope(t, stdout).Result, &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed) != 6 {
		t.Fatalf("expected 6 merged entries, got %d", len(listed))
	}
	for index := 1; index < len(listed); index++ {
		if listed[index-1].Source > listed[index].Source {
			t.Fatalf("list is not source-ordered: %+v", listed)
		}
	}

	code, stdout, stderr = invokeRaw(t, "audit", rootA, "--source", "b="+rootB, "--no-db", "--fail-on", "none", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("audit exit=%d stderr=%q", code, stderr)
	}
	var report contractv1.AuditReport
	if err := json.Unmarshal(decodeEnvelope(t, stdout).Result, &report); err != nil {
		t.Fatal(err)
	}
	if report.Summary.Concepts != 6 {
		t.Fatalf("audit summary concepts = %d, want 6", report.Summary.Concepts)
	}
	found := false
	for _, finding := range report.Findings {
		if finding.Source == "b" {
			found = true
		}
	}
	if !found {
		t.Fatal("audit findings must carry a source alias")
	}
}

func TestMultiBundleWrites(t *testing.T) {
	rootA, rootB := multiBundleFixture(t)
	db := filepath.Join(t.TempDir(), "index.db")

	code, stdout, stderr := invokeRaw(t, "ingest", rootA, "--source", "b="+rootB, "--db", db, "--strict", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("ingest exit=%d stderr=%q", code, stderr)
	}
	var report contractv1.IngestReport
	if err := json.Unmarshal(decodeEnvelope(t, stdout).Result, &report); err != nil {
		t.Fatal(err)
	}
	if report.Documents != 6 || len(report.Sources) != 2 {
		t.Fatalf("unexpected aggregate report: %+v", report)
	}
	if report.Sources[0].Source != "team-a" || report.Sources[1].Source != "b" {
		t.Fatalf("unexpected source reports: %+v", report.Sources)
	}

	writeMultiDoc(t, rootA, "common", "common", "common", "updated shared search term")
	code, stdout, stderr = invokeRaw(t, "sync", rootA, "--source", "b="+rootB, "--db", db, "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("sync exit=%d stderr=%q", code, stderr)
	}
	var synced contractv1.SyncReport
	if err := json.Unmarshal(decodeEnvelope(t, stdout).Result, &synced); err != nil {
		t.Fatal(err)
	}
	if len(synced.Sources) != 2 || synced.Updated < 1 {
		t.Fatalf("unexpected sync report: %+v", synced)
	}

	code, stdout, stderr = invokeRaw(t, "search", rootA, "common", "--source", "b="+rootB, "--db", db, "--mode", "bm25", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("bm25 exit=%d stderr=%q", code, stderr)
	}
	var response bddMultiSearch
	if err := json.Unmarshal(decodeEnvelope(t, stdout).Result, &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Lexical) < 2 {
		t.Fatalf("expected lexical hits from both sources: %+v", response.Lexical)
	}
}

func TestMultiBundleCrossSourcePathFails(t *testing.T) {
	rootA, rootB := multiBundleFixture(t)
	code, stdout, _ := invokeRaw(t, "path", rootA, "linker", "only-b", "--source", "b="+rootB, "--no-db", "--json")
	if code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
	envelope := decodeEnvelope(t, stdout)
	if envelope.Error == nil || envelope.Error.Code != "INVALID_REQUEST" {
		t.Fatalf("expected INVALID_REQUEST, got %+v", envelope.Error)
	}
}

func TestMultiBundleParseShape(t *testing.T) {
	rootA, rootB := multiBundleFixture(t)
	code, stdout, stderr := invokeRaw(t, "parse", rootA, "--source", "b="+rootB, "--no-db", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("parse exit=%d stderr=%q", code, stderr)
	}
	var result contractv1.SourceBundles
	if err := json.Unmarshal(decodeEnvelope(t, stdout).Result, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Sources) != 2 || result.Sources[0].Source != "team-a" || result.Sources[1].Source != "b" {
		t.Fatalf("unexpected parse result: %+v", result.Sources)
	}
	if len(result.Sources[0].Bundle.Concepts) != 4 || len(result.Sources[1].Bundle.Concepts) != 2 {
		t.Fatalf("unexpected concept counts: %+v", result.Sources)
	}
}

func TestMultiBundleAliasValidation(t *testing.T) {
	rootA, rootB := multiBundleFixture(t)
	cases := [][]string{
		{"list", rootA, "--source", "team-a=" + rootB, "--no-db", "--json"},
		{"list", rootA, "--source", "b=" + rootA, "--no-db", "--json"},
		{"list", rootA, "--source", "Bad=" + rootB, "--no-db", "--json"},
	}
	for _, args := range cases {
		code, _, _ := invokeRaw(t, args...)
		if code != 2 {
			t.Fatalf("%v: expected exit 2, got %d", args, code)
		}
	}
}

func TestMultiBundleSingleSourceUnchanged(t *testing.T) {
	rootA, _ := multiBundleFixture(t)
	code, stdout, stderr := invokeRaw(t, "search", rootA, "common", "--mode", "keyword", "--no-db", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("search exit=%d stderr=%q", code, stderr)
	}
	if strings.Contains(stdout, `"source"`) {
		t.Fatalf("single-source output must not add source fields: %s", stdout)
	}
}
