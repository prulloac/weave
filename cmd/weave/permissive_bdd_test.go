package main

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
)

// Behavior tests for permissive ingestion. Incremental sync is covered by a
// later suite once sync lands.

func permissiveFixture() string {
	return filepath.Join("testdata", "permissive", "bundle")
}

func TestPermissiveIngestionBehavior(t *testing.T) {
	t.Run("ingest defaults to permissive", func(t *testing.T) {
		storageHome(t)
		code, _, report := runStorageJSON(t, "ingest", permissiveFixture(), "--json")

		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
		if report.Mode != "permissive" {
			t.Fatalf("mode=%q, want permissive", report.Mode)
		}
		if report.Documents != 6 || report.Synthetic != 5 {
			t.Fatalf("unexpected counts: %+v", report)
		}
		if report.Warnings != 5 || report.Errors != 0 {
			t.Fatalf("unexpected findings: %+v", report)
		}
	})

	t.Run("explicit permissive matches the default", func(t *testing.T) {
		storageHome(t)
		_, _, defaultReport := runStorageJSON(t, "ingest", permissiveFixture(), "--json")
		_, _, explicitReport := runStorageJSON(t, "ingest", permissiveFixture(), "--permissive", "--json")

		if defaultReport.Mode != explicitReport.Mode ||
			defaultReport.Documents != explicitReport.Documents ||
			defaultReport.Synthetic != explicitReport.Synthetic ||
			defaultReport.Warnings != explicitReport.Warnings ||
			defaultReport.Errors != explicitReport.Errors {
			t.Fatalf("default and explicit permissive differ:\ndefault:  %+v\nexplicit: %+v", defaultReport, explicitReport)
		}
	})

	t.Run("strict ingest rejects invalid documents", func(t *testing.T) {
		storageHome(t)
		code, _, report := runStorageJSON(t, "ingest", permissiveFixture(), "--strict", "--json")

		if code != 1 {
			t.Fatalf("expected exit 1, got %d", code)
		}
		if report.Mode != "strict" || report.Synthetic != 0 || report.Errors == 0 {
			t.Fatalf("unexpected strict report: %+v", report)
		}
	})

	t.Run("permissive and strict are mutually exclusive", func(t *testing.T) {
		storageHome(t)
		code, stdout, _ := invokePersistent(t, "ingest", permissiveFixture(),
			"--permissive", "--strict", "--json")
		if code != 2 {
			t.Fatalf("expected exit 2, got %d", code)
		}
		if failure := storageFailure(t, stdout); failure.Code != "INVALID_REQUEST" {
			t.Fatalf("expected INVALID_REQUEST, got %s", failure.Code)
		}
	})

	t.Run("synthetic fallbacks are deterministic", func(t *testing.T) {
		storageHome(t)
		runStorageJSON(t, "ingest", permissiveFixture(), "--json")
		bundle := runParseBundle(t, permissiveFixture(), "--json")

		plain := findConcept(t, bundle, "plain")
		if plain.Type != "Document" || len(plain.Keywords) != 0 {
			t.Fatalf("unexpected plain type/keywords: %+v", plain)
		}
		if plain.Title == nil || *plain.Title != "Plain Notes" {
			t.Fatalf("unexpected plain title: %+v", plain.Title)
		}
		if plain.Description == nil ||
			*plain.Description != "A plain Markdown document used for permissive ingestion." {
			t.Fatalf("unexpected plain description: %+v", plain.Description)
		}
		if !strings.Contains(plain.Body, "# Plain Notes") {
			t.Fatalf("plain body was not preserved: %q", plain.Body)
		}

		noType := findConcept(t, bundle, "no-type")
		if noType.Type != "Document" || !slices.Equal(noType.Keywords, []string{"no-type"}) {
			t.Fatalf("unexpected no-type concept: %+v", noType)
		}
		noKeywords := findConcept(t, bundle, "no-keywords")
		if noKeywords.Type != "Concept" || len(noKeywords.Keywords) != 0 {
			t.Fatalf("unexpected no-keywords concept: %+v", noKeywords)
		}
		empty := findConcept(t, bundle, "empty")
		if empty.Type != "Document" || empty.Title == nil || *empty.Title != "empty" || empty.Body != "" {
			t.Fatalf("unexpected empty concept: %+v", empty)
		}

		for _, reserved := range []string{"index", "log"} {
			for _, concept := range bundle.Concepts {
				if concept.ID == reserved {
					t.Fatalf("reserved file %q became a concept", reserved)
				}
			}
		}
	})

	t.Run("re-ingest is deterministic", func(t *testing.T) {
		storageHome(t)
		_, _, first := runStorageJSON(t, "ingest", permissiveFixture(), "--json")
		_, _, second := runStorageJSON(t, "ingest", permissiveFixture(), "--json")

		if first.Documents != second.Documents ||
			first.Synthetic != second.Synthetic ||
			first.Warnings != second.Warnings ||
			first.Errors != second.Errors {
			t.Fatalf("re-ingest is not deterministic: %+v vs %+v", first, second)
		}
	})

	t.Run("exact keyword search excludes synthetic documents", func(t *testing.T) {
		storageHome(t)
		runStorageJSON(t, "ingest", permissiveFixture(), "--json")

		code, _, stderr := invokePersistent(t, "search", permissiveFixture(),
			"plain", "--mode", "keyword", "--json")
		if code != 1 {
			t.Fatalf("expected exit 1, got %d", code)
		}
		if !strings.Contains(stderr, "no matches") {
			t.Fatalf("expected no-match diagnostic, got %q", stderr)
		}
	})

	t.Run("fuzzy and hybrid search include synthetic documents", func(t *testing.T) {
		storageHome(t)
		runStorageJSON(t, "ingest", permissiveFixture(), "--json")

		fuzzy := permissiveSearch(t, permissiveFixture(), "plain", "--mode", "fuzzy")
		if !hasHybridOrFuzzyID(fuzzy, "plain") {
			t.Fatalf("plain missing from fuzzy results: %+v", fuzzy)
		}
		hybrid := permissiveSearch(t, permissiveFixture(), "plain")
		if !hasHybridOrFuzzyID(hybrid, "plain") {
			t.Fatalf("plain missing from hybrid results: %+v", hybrid)
		}
	})

	t.Run("synthetic documents participate in the graph", func(t *testing.T) {
		storageHome(t)
		runStorageJSON(t, "ingest", permissiveFixture(), "--json")

		code, stdout, stderr := invokePersistent(t, "graph", permissiveFixture(), "--json")
		if code != 0 || stderr != "" {
			t.Fatalf("exit=%d stderr=%q", code, stderr)
		}
		envelope := decodeEnvelope(t, stdout)
		var graph contractv1.Graph
		if err := json.Unmarshal(envelope.Result, &graph); err != nil {
			t.Fatal(err)
		}
		ids := make([]string, len(graph.Nodes))
		for index, node := range graph.Nodes {
			ids[index] = node.ID
		}
		if !slices.Contains(ids, "links") || !slices.Contains(ids, "valid") {
			t.Fatalf("expected links and valid nodes, got %v", ids)
		}
		found := false
		for _, edge := range graph.Edges {
			if edge.From == "links" && edge.To == "valid" {
				found = true
			}
		}
		if !found {
			t.Fatalf("expected links -> valid edge, got %+v", graph.Edges)
		}
	})

	t.Run("permissive audit downgrades synthetic findings", func(t *testing.T) {
		storageHome(t)
		code, report := permissiveAudit(t, "--permissive", "--no-db", "--json")

		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
		if report.Summary.Valid {
			t.Fatal("expected summary.valid=false")
		}
		if hasSeverity(report, "error") {
			t.Fatalf("expected no error findings, got %+v", report.Findings)
		}
		for _, code := range []string{"MISSING_FRONTMATTER", "MISSING_TYPE", "MISSING_KEYWORDS"} {
			if !hasAuditCode(report, code, "warning") {
				t.Fatalf("expected warning %s, got %+v", code, report.Findings)
			}
		}
	})

	t.Run("strict audit reports synthetic findings as errors", func(t *testing.T) {
		storageHome(t)
		code, report := permissiveAudit(t, "--no-db", "--json")

		if code != 1 {
			t.Fatalf("expected exit 1, got %d", code)
		}
		for _, code := range []string{"MISSING_FRONTMATTER", "MISSING_TYPE", "MISSING_KEYWORDS"} {
			if !hasAuditCode(report, code, "error") {
				t.Fatalf("expected error %s, got %+v", code, report.Findings)
			}
		}
	})

	t.Run("permissive audit can fail on warnings", func(t *testing.T) {
		storageHome(t)
		code, report := permissiveAudit(t,
			"--permissive", "--fail-on", "warning", "--no-db", "--json")

		if code != 1 {
			t.Fatalf("expected exit 1, got %d", code)
		}
		if hasSeverity(report, "error") {
			t.Fatalf("expected only warnings, got %+v", report.Findings)
		}
	})
}

func permissiveSearch(t *testing.T, path, query string, extra ...string) contractv1.SearchResponse {
	t.Helper()

	args := []string{"search", path, query, "--json"}
	args = append(args, extra...)
	code, stdout, stderr := invokePersistent(t, args...)
	if code != 0 || stderr != "" {
		t.Fatalf("search exit=%d stderr=%q", code, stderr)
	}
	envelope := decodeEnvelope(t, stdout)
	var response contractv1.SearchResponse
	if err := json.Unmarshal(envelope.Result, &response); err != nil {
		t.Fatal(err)
	}
	return response
}

func permissiveAudit(t *testing.T, extra ...string) (int, bddAuditReport) {
	t.Helper()

	args := []string{"audit", permissiveFixture()}
	args = append(args, extra...)
	code, stdout, stderr := invokePersistent(t, args...)
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	envelope := decodeEnvelope(t, stdout)
	if !envelope.OK {
		t.Fatalf("expected audit report, got failure: %+v", envelope.Error)
	}
	var report bddAuditReport
	if err := json.Unmarshal(envelope.Result, &report); err != nil {
		t.Fatal(err)
	}
	return code, report
}

func findConcept(t *testing.T, bundle contractv1.Bundle, id string) contractv1.OkfConcept {
	t.Helper()
	for _, concept := range bundle.Concepts {
		if concept.ID == id {
			return concept
		}
	}
	t.Fatalf("concept %q not found", id)
	return contractv1.OkfConcept{}
}

func hasHybridOrFuzzyID(response contractv1.SearchResponse, id string) bool {
	for _, hit := range response.Fuzzy {
		if hit.ID == id {
			return true
		}
	}
	for _, hit := range response.Hybrid {
		if hit.ID == id {
			return true
		}
	}
	return false
}

func hasSeverity(report bddAuditReport, severity string) bool {
	for _, finding := range report.Findings {
		if finding.Severity == severity {
			return true
		}
	}
	return false
}

func hasAuditCode(report bddAuditReport, code, severity string) bool {
	for _, finding := range report.Findings {
		if finding.Code == code && finding.Severity == severity {
			return true
		}
	}
	return false
}
