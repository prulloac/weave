// Package audit evaluates a parsed bundle as a knowledge base. It reports
// conformance, graph and index-quality findings without mutating the bundle.
package audit

import (
	"path"
	"sort"
	"strings"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine/graph"
	"github.com/prulloac/weave/engine/privacy"
)

// Options controls audit report construction.
type Options struct {
	FailOn     contractv1.AuditFailOn
	VerifyKeys bool
	Permissive bool
}

// Build evaluates a parsed bundle and its graph.
func Build(bundle *contractv1.Bundle, view *graph.Graph, options Options) contractv1.AuditReport {
	failOn := normalizeFailOn(options.FailOn)
	if view == nil {
		view = graph.Build(bundle)
	}

	report := contractv1.AuditReport{
		Path:     bundle.Root,
		FailOn:   failOn,
		Findings: []contractv1.AuditFinding{},
		Summary: contractv1.AuditSummary{
			Concepts: len(bundle.Concepts),
			Valid:    bundle.Validation.Valid,
			Edges:    len(graph.SortedEdges(view)),
		},
	}

	addValidationFindings(bundle, &report, options.Permissive || bundle.Validation.Permissive)
	addGraphFindings(bundle, view, &report)
	addIndexFindings(bundle, &report)
	addBundleWarnings(bundle, &report)
	if options.VerifyKeys {
		addPrivacyFindings(bundle, &report)
	}

	sortFindings(report.Findings)
	report.OK = !hasError(report.Findings)
	return report
}

// Failed reports whether the report meets its configured failure threshold.
func Failed(report contractv1.AuditReport) bool {
	switch normalizeFailOn(report.FailOn) {
	case contractv1.AuditFailOnNone:
		return false
	case contractv1.AuditFailOnWarning:
		for _, finding := range report.Findings {
			if finding.Severity == contractv1.AuditSeverityError ||
				finding.Severity == contractv1.AuditSeverityWarning {
				return true
			}
		}
		return false
	default:
		return hasError(report.Findings)
	}
}

func normalizeFailOn(value contractv1.AuditFailOn) contractv1.AuditFailOn {
	switch value {
	case contractv1.AuditFailOnWarning, contractv1.AuditFailOnNone:
		return value
	default:
		return contractv1.AuditFailOnError
	}
}

func addValidationFindings(bundle *contractv1.Bundle, report *contractv1.AuditReport, permissive bool) {
	severity := contractv1.AuditSeverityError
	if permissive {
		severity = contractv1.AuditSeverityWarning
	}
	for _, validation := range bundle.Validation.Concepts {
		if !validation.HasFrontmatter {
			report.Findings = append(report.Findings, finding(
				contractv1.AuditMissingFrontmatter,
				severity,
				validation.ID,
				"",
				"",
				"file has no frontmatter",
			))
			continue
		}
		if !validation.HasType {
			report.Findings = append(report.Findings, finding(
				contractv1.AuditMissingType,
				severity,
				validation.ID,
				"",
				"",
				"concept has no type",
			))
		}
		if !validation.HasKeywords {
			report.Summary.MissingKeywords++
			report.Findings = append(report.Findings, finding(
				contractv1.AuditMissingKeywords,
				severity,
				validation.ID,
				"",
				"",
				"concept has no keywords",
			))
		}
	}
}

func addGraphFindings(bundle *contractv1.Bundle, view *graph.Graph, report *contractv1.AuditReport) {
	for _, node := range view.Nodes {
		if node.InDegree == 0 && node.OutDegree == 0 {
			report.Summary.Orphans++
			report.Findings = append(report.Findings, finding(
				contractv1.AuditOrphanConcept,
				contractv1.AuditSeverityWarning,
				node.ID,
				"",
				"",
				"concept has no incoming or outgoing edges",
			))
		}
	}

	for _, concept := range bundle.Concepts {
		for _, link := range concept.Links {
			if link.Resolved == nil {
				report.Findings = append(report.Findings, finding(
					contractv1.AuditDanglingExternal,
					contractv1.AuditSeverityInfo,
					"",
					concept.ID,
					link.Target,
					"external or non-bundle link",
				))
				continue
			}
			if link.ResolvesInBundle {
				continue
			}
			report.Summary.BrokenLinks++
			report.Findings = append(report.Findings, finding(
				contractv1.AuditBrokenLink,
				contractv1.AuditSeverityError,
				"",
				concept.ID,
				*link.Resolved,
				"link target does not resolve in the bundle",
			))
		}

		related, ok := concept.Frontmatter["related"]
		if !ok {
			continue
		}
		for _, entry := range graph.RelatedEntries(related) {
			if entry == "" {
				continue
			}
			if _, found := view.ResolveRelated(entry, concept.ID); found {
				continue
			}
			report.Findings = append(report.Findings, finding(
				contractv1.AuditBrokenRelated,
				contractv1.AuditSeverityError,
				"",
				concept.ID,
				entry,
				"related target does not resolve in the bundle",
			))
		}
	}
}

func addIndexFindings(bundle *contractv1.Bundle, report *contractv1.AuditReport) {
	conceptIDs := make(map[string]struct{}, len(bundle.Concepts))
	for _, concept := range bundle.Concepts {
		conceptIDs[concept.ID] = struct{}{}
	}

	indexed := make(map[string]bool, len(bundle.Index))
	for _, entry := range bundle.Index {
		target := indexTargetID(entry.Target)
		if target == "" {
			continue
		}
		if _, ok := conceptIDs[target]; !ok {
			report.Summary.MissingIndexTargets++
			report.Findings = append(report.Findings, finding(
				contractv1.AuditMissingIndexTarget,
				contractv1.AuditSeverityError,
				"",
				"index.md",
				entry.Target,
				"index entry points to an unknown concept",
			))
			continue
		}
		indexed[target] = true
	}

	if len(bundle.Index) == 0 {
		return
	}
	for _, concept := range bundle.Concepts {
		if indexed[concept.ID] {
			continue
		}
		report.Summary.UnindexedConcepts++
		report.Findings = append(report.Findings, finding(
			contractv1.AuditUnindexedConcept,
			contractv1.AuditSeverityWarning,
			concept.ID,
			"",
			"",
			"concept is absent from the root index",
		))
	}
}

func addBundleWarnings(bundle *contractv1.Bundle, report *contractv1.AuditReport) {
	for _, warning := range bundle.Validation.Warnings {
		switch {
		case strings.HasPrefix(warning, "unsupported okf_version:"):
			report.Findings = append(report.Findings, finding(
				contractv1.AuditUnsupportedOKFVersion,
				contractv1.AuditSeverityWarning,
				"",
				"index.md",
				"",
				warning,
			))
		case strings.HasPrefix(warning, "empty bundle:"):
			report.Findings = append(report.Findings, finding(
				contractv1.AuditEmptyBundle,
				contractv1.AuditSeverityWarning,
				"",
				"",
				"",
				warning,
			))
		}
	}
}

func addPrivacyFindings(bundle *contractv1.Bundle, report *contractv1.AuditReport) {
	for _, concept := range bundle.Concepts {
		seen := make(map[string]bool)
		check := func(keyID string) {
			if keyID == "" || seen[keyID] {
				return
			}
			seen[keyID] = true
			if privacy.HasKey(bundle.Root, keyID) {
				return
			}
			report.Findings = append(report.Findings, finding(
				contractv1.AuditKeyUnavailable,
				contractv1.AuditSeverityWarning,
				concept.ID,
				concept.Path,
				"",
				"privacy key unavailable: "+keyID,
			))
		}
		if concept.Privacy != nil {
			check(concept.Privacy.KeyID)
		}
		spans, _ := privacy.ScanInline(concept.Body)
		for _, span := range spans {
			check(span.KeyID)
		}
	}
}

func finding(
	code contractv1.AuditFindingCode,
	severity contractv1.AuditSeverity,
	concept, source, target, message string,
) contractv1.AuditFinding {
	return contractv1.AuditFinding{
		Code:     code,
		Severity: severity,
		Concept:  concept,
		Source:   source,
		Target:   target,
		Message:  message,
	}
}

func indexTargetID(target string) string {
	if index := strings.IndexAny(target, "#?"); index >= 0 {
		target = target[:index]
	}
	target = strings.TrimSpace(target)
	target = strings.TrimPrefix(target, "/")
	target = strings.TrimSuffix(target, ".md")
	if target == "" {
		return ""
	}
	cleaned := path.Clean(target)
	if cleaned == "." {
		return ""
	}
	return cleaned
}

func sortFindings(findings []contractv1.AuditFinding) {
	sort.SliceStable(findings, func(i, j int) bool {
		left, right := findings[i], findings[j]
		if rank(left.Severity) != rank(right.Severity) {
			return rank(left.Severity) < rank(right.Severity)
		}
		if left.Code != right.Code {
			return left.Code < right.Code
		}
		if findingKey(left) != findingKey(right) {
			return findingKey(left) < findingKey(right)
		}
		if left.Target != right.Target {
			return left.Target < right.Target
		}
		return left.Message < right.Message
	})
}

func rank(severity contractv1.AuditSeverity) int {
	switch severity {
	case contractv1.AuditSeverityError:
		return 0
	case contractv1.AuditSeverityWarning:
		return 1
	default:
		return 2
	}
}

func findingKey(finding contractv1.AuditFinding) string {
	if finding.Concept != "" {
		return finding.Concept
	}
	return finding.Source
}

func hasError(findings []contractv1.AuditFinding) bool {
	for _, finding := range findings {
		if finding.Severity == contractv1.AuditSeverityError {
			return true
		}
	}
	return false
}
