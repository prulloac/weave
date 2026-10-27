package audit

import (
	"slices"
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine/graph"
)

func TestBuildReportsValidationAndBundleFindings(t *testing.T) {
	bundle := &contractv1.Bundle{
		Root: "/bundle",
		Concepts: []contractv1.OkfConcept{{
			ID: "valid", Path: "valid.md", Type: "Concept", Keywords: []string{"valid"},
		}},
		Index: []contractv1.IndexEntry{{Title: "Valid", Target: "valid.md"}},
		Validation: contractv1.Validation{
			Valid: false,
			Concepts: []contractv1.ConceptValidation{
				{ID: "loose", HasFrontmatter: false},
				{ID: "no-type", HasFrontmatter: true, HasType: false, HasKeywords: true},
				{ID: "no-keywords", HasFrontmatter: true, HasType: true, HasKeywords: false},
			},
			Warnings: []string{
				"unsupported okf_version: 1.0",
				"empty bundle: no markdown files found",
			},
		},
	}

	report := Build(bundle, graph.Build(bundle), Options{})

	for _, want := range []contractv1.AuditFindingCode{
		contractv1.AuditMissingFrontmatter,
		contractv1.AuditMissingType,
		contractv1.AuditMissingKeywords,
		contractv1.AuditOrphanConcept,
		contractv1.AuditUnsupportedOKFVersion,
		contractv1.AuditEmptyBundle,
	} {
		if !hasCode(report, want) {
			t.Fatalf("expected %s, got %v", want, codes(report))
		}
	}
	if report.FailOn != contractv1.AuditFailOnError {
		t.Fatalf("expected default fail_on=error, got %s", report.FailOn)
	}
	if report.Summary.MissingKeywords != 1 {
		t.Fatalf("expected 1 missing-keyword concept, got %d", report.Summary.MissingKeywords)
	}
	if report.Summary.Concepts != 1 {
		t.Fatalf("expected 1 concept, got %d", report.Summary.Concepts)
	}
	if report.OK {
		t.Fatal("expected report.ok=false")
	}
}

func TestBuildReportsGraphIndexAndLinkFindingsInOrder(t *testing.T) {
	broken := "concepts/missing.md"
	bundle := &contractv1.Bundle{
		Root: "/bundle",
		Concepts: []contractv1.OkfConcept{
			{
				ID: "concepts/linked", Path: "concepts/linked.md", Type: "Concept",
				Keywords: []string{"linked"},
				Frontmatter: map[string]any{
					"related": []any{"concepts/missing-related.md"},
				},
				Links: []contractv1.OkfLink{
					{Raw: "[Target](target.md)", Target: "target.md", Resolved: stringPtr("concepts/target.md"), ResolvesInBundle: true},
					{Raw: "[Missing](missing.md)", Target: "missing.md", Resolved: &broken, ResolvesInBundle: false},
					{Raw: "[External](https://example.com)", Target: "https://example.com", ResolvesInBundle: false},
				},
			},
			{ID: "concepts/target", Path: "concepts/target.md", Type: "Concept", Keywords: []string{"target"}},
			{ID: "concepts/unindexed", Path: "concepts/unindexed.md", Type: "Concept", Keywords: []string{"unindexed"}},
		},
		Index: []contractv1.IndexEntry{
			{Title: "Linked", Target: "concepts/linked.md"},
			{Title: "Target", Target: "concepts/target.md"},
			{Title: "Missing", Target: "concepts/missing.md"},
		},
		Validation: contractv1.Validation{Valid: true},
	}

	report := Build(bundle, graph.Build(bundle), Options{})

	wantCodes := []contractv1.AuditFindingCode{
		contractv1.AuditBrokenLink,
		contractv1.AuditBrokenRelated,
		contractv1.AuditMissingIndexTarget,
		contractv1.AuditOrphanConcept,
		contractv1.AuditUnindexedConcept,
		contractv1.AuditDanglingExternal,
	}
	if got := codes(report); !slices.Equal(got, wantCodes) {
		t.Fatalf("unexpected finding order:\n got: %v\nwant: %v", got, wantCodes)
	}
	if report.Summary.Concepts != 3 {
		t.Fatalf("expected 3 concepts, got %d", report.Summary.Concepts)
	}
	if report.Summary.Edges != 1 {
		t.Fatalf("expected 1 resolved edge, got %d", report.Summary.Edges)
	}
	if report.Summary.Orphans != 1 {
		t.Fatalf("expected 1 orphan, got %d", report.Summary.Orphans)
	}
	if report.Summary.BrokenLinks != 1 {
		t.Fatalf("expected 1 broken link, got %d", report.Summary.BrokenLinks)
	}
	if report.Summary.MissingIndexTargets != 1 {
		t.Fatalf("expected 1 missing index target, got %d", report.Summary.MissingIndexTargets)
	}
	if report.Summary.UnindexedConcepts != 1 {
		t.Fatalf("expected 1 unindexed concept, got %d", report.Summary.UnindexedConcepts)
	}
}

func TestFailedUsesConfiguredThreshold(t *testing.T) {
	errorReport := contractv1.AuditReport{
		Findings: []contractv1.AuditFinding{{
			Code: contractv1.AuditBrokenLink, Severity: contractv1.AuditSeverityError, Message: "broken",
		}},
	}
	warningReport := contractv1.AuditReport{
		Findings: []contractv1.AuditFinding{{
			Code: contractv1.AuditOrphanConcept, Severity: contractv1.AuditSeverityWarning, Message: "orphan",
		}},
	}

	tests := []struct {
		name   string
		report contractv1.AuditReport
		failOn contractv1.AuditFailOn
		want   bool
	}{
		{name: "error default", report: errorReport, failOn: "", want: true},
		{name: "error threshold", report: errorReport, failOn: contractv1.AuditFailOnError, want: true},
		{name: "warning threshold with error", report: errorReport, failOn: contractv1.AuditFailOnWarning, want: true},
		{name: "none threshold with error", report: errorReport, failOn: contractv1.AuditFailOnNone, want: false},
		{name: "error threshold with warning", report: warningReport, failOn: contractv1.AuditFailOnError, want: false},
		{name: "warning threshold with warning", report: warningReport, failOn: contractv1.AuditFailOnWarning, want: true},
		{name: "none threshold with warning", report: warningReport, failOn: contractv1.AuditFailOnNone, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.report.FailOn = test.failOn
			if got := Failed(test.report); got != test.want {
				t.Fatalf("Failed()=%t, want %t", got, test.want)
			}
		})
	}
}

func codes(report contractv1.AuditReport) []contractv1.AuditFindingCode {
	result := make([]contractv1.AuditFindingCode, len(report.Findings))
	for index, finding := range report.Findings {
		result[index] = finding.Code
	}
	return result
}

func hasCode(report contractv1.AuditReport, code contractv1.AuditFindingCode) bool {
	return slices.Contains(codes(report), code)
}

func stringPtr(value string) *string {
	return &value
}
