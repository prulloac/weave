package audit

import (
	"encoding/json"
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
)

func renderFixture(t *testing.T) (sarifLog, []byte) {
	t.Helper()
	report := contractv1.AuditReport{
		Path: "/notes",
		Findings: []contractv1.AuditFinding{
			{Code: contractv1.AuditBrokenLink, Severity: contractv1.AuditSeverityError, Source: "docs/b.md", Message: "broken link"},
			{Code: contractv1.AuditMissingKeywords, Severity: contractv1.AuditSeverityWarning, Source: "docs/a.md", Message: "missing keywords"},
			{Code: contractv1.AuditBrokenLink, Severity: contractv1.AuditSeverityError, Source: "docs/a.md", Message: "another broken link"},
		},
	}
	data, err := RenderSARIF(report, "v0.13.0")
	if err != nil {
		t.Fatal(err)
	}
	var log sarifLog
	if err := json.Unmarshal(data, &log); err != nil {
		t.Fatalf("decode SARIF: %v\n%s", err, data)
	}
	return log, data
}

func TestSARIFShape(t *testing.T) {
	log, _ := renderFixture(t)
	if log.Version != "2.1.0" || log.Schema == "" {
		t.Fatalf("unexpected header: %+v", log)
	}
	if len(log.Runs) != 1 || log.Runs[0].Tool.Driver.Name != "weave" || log.Runs[0].Tool.Driver.Version != "v0.13.0" {
		t.Fatalf("unexpected run: %+v", log.Runs)
	}
	if len(log.Runs[0].Tool.Driver.Rules) != 2 {
		t.Fatalf("rules = %+v", log.Runs[0].Tool.Driver.Rules)
	}
}

func TestSARIFLevelsAndOrder(t *testing.T) {
	log, _ := renderFixture(t)
	results := log.Runs[0].Results
	if len(results) != 3 {
		t.Fatalf("results = %+v", results)
	}
	if results[0].RuleID != "BROKEN_LINK" || results[0].Level != "error" || results[1].RuleID != "MISSING_KEYWORDS" {
		t.Fatalf("unexpected order: %+v", results)
	}
	if results[1].Level != "warning" {
		t.Fatalf("warning level missing: %+v", results[1])
	}
	if results[0].Locations[0].PhysicalLocation.ArtifactLocation.URI != "docs/a.md" {
		t.Fatalf("location = %+v", results[0].Locations)
	}
}

func TestSARIFDeterministic(t *testing.T) {
	_, first := renderFixture(t)
	_, second := renderFixture(t)
	if string(first) != string(second) {
		t.Fatal("SARIF output is not deterministic")
	}
}
