package main

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// These are behavior-level tests for the audit command. They intentionally use
// the same CLI entrypoint as a user: no engine internals, no direct graph calls.
//
// The suite is expected to fail until `weave audit` is implemented. `--permissive`
// is specified with permissive ingestion and is covered by that feature's BDD
// suite; audit must not duplicate parser mode semantics.

type bddAuditReport struct {
	Path    string `json:"path"`
	OK      bool   `json:"ok"`
	FailOn  string `json:"fail_on"`
	Summary struct {
		Concepts            int  `json:"concepts"`
		Valid               bool `json:"valid"`
		Edges               int  `json:"edges"`
		Orphans             int  `json:"orphans"`
		BrokenLinks         int  `json:"broken_links"`
		MissingKeywords     int  `json:"missing_keywords"`
		MissingIndexTargets int  `json:"missing_index_targets"`
		UnindexedConcepts   int  `json:"unindexed_concepts"`
	} `json:"summary"`
	Findings []bddFinding `json:"findings"`
}

type bddFinding struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Concept  string `json:"concept"`
	Source   string `json:"source"`
	Target   string `json:"target"`
	Message  string `json:"message"`
}

func auditFixture(name string) string {
	return filepath.Join("testdata", "audit", name)
}

func runAuditJSON(t *testing.T, args ...string) (int, testEnvelope, bddAuditReport) {
	t.Helper()

	fullArgs := append([]string{"audit"}, args...)
	code, stdout, stderr := invoke(t, fullArgs...)
	if stderr != "" {
		t.Fatalf("expected empty stderr in JSON mode, got %q", stderr)
	}

	envelope := decodeEnvelope(t, stdout)
	if !envelope.OK {
		t.Fatalf("expected completed audit envelope, got failure: %+v", envelope.Error)
	}

	var report bddAuditReport
	if err := json.Unmarshal(envelope.Result, &report); err != nil {
		t.Fatalf("decode audit result: %v\n%s", err, envelope.Result)
	}
	return code, envelope, report
}

func findingCodes(report bddAuditReport) []string {
	codes := make([]string, 0, len(report.Findings))
	for _, finding := range report.Findings {
		codes = append(codes, finding.Code)
	}
	return codes
}

func hasFinding(report bddAuditReport, code string) bool {
	for _, finding := range report.Findings {
		if finding.Code == code {
			return true
		}
	}
	return false
}

func TestAuditBehavior(t *testing.T) {
	t.Run("clean bundle passes with no findings", func(t *testing.T) {
		code, _, report := runAuditJSON(t, auditFixture("clean"), "--json")

		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
		if !report.OK {
			t.Fatal("expected report.ok=true for a clean bundle")
		}
		if !report.Summary.Valid {
			t.Fatal("expected summary.valid=true for a clean bundle")
		}
		if report.Summary.Concepts != 2 {
			t.Fatalf("expected 2 concepts, got %d", report.Summary.Concepts)
		}
		if report.Summary.Edges != 2 {
			t.Fatalf("expected 2 resolved edges, got %d", report.Summary.Edges)
		}
		if len(report.Findings) != 0 {
			t.Fatalf("expected no findings, got %v", findingCodes(report))
		}
	})

	t.Run("invalid documents are reported with stable finding codes", func(t *testing.T) {
		code, _, report := runAuditJSON(t, auditFixture("invalid-docs"), "--json")

		if code != 1 {
			t.Fatalf("expected exit 1 for validation errors, got %d", code)
		}
		if report.Summary.Valid {
			t.Fatal("expected summary.valid=false")
		}
		if report.Summary.MissingKeywords != 1 {
			t.Fatalf("expected 1 missing-keyword concept, got %d", report.Summary.MissingKeywords)
		}

		for _, want := range []string{
			"MISSING_FRONTMATTER",
			"MISSING_TYPE",
			"MISSING_KEYWORDS",
			"UNSUPPORTED_OKF_VERSION",
			"DANGLING_EXTERNAL",
		} {
			if !hasFinding(report, want) {
				t.Fatalf("expected finding %s, got %v", want, findingCodes(report))
			}
		}
	})

	t.Run("graph issues are reported in deterministic severity order", func(t *testing.T) {
		path := auditFixture("graph-issues")
		code, _, report := runAuditJSON(t, path, "--json")

		if code != 1 {
			t.Fatalf("expected exit 1 for graph errors, got %d", code)
		}
		if !report.Summary.Valid {
			t.Fatal("expected summary.valid=true; all graph-issue documents are valid")
		}
		if report.Summary.Concepts != 4 {
			t.Fatalf("expected 4 concepts, got %d", report.Summary.Concepts)
		}
		if report.Summary.Edges != 2 {
			t.Fatalf("expected 2 resolved edges, got %d", report.Summary.Edges)
		}
		if report.Summary.Orphans != 2 {
			t.Fatalf("expected 2 orphan concepts, got %d", report.Summary.Orphans)
		}
		if report.Summary.UnindexedConcepts != 1 {
			t.Fatalf("expected 1 unindexed concept, got %d", report.Summary.UnindexedConcepts)
		}
		if report.Summary.MissingIndexTargets != 1 {
			t.Fatalf("expected 1 missing index target, got %d", report.Summary.MissingIndexTargets)
		}

		wantCodes := []string{
			"BROKEN_LINK",
			"BROKEN_RELATED",
			"MISSING_INDEX_TARGET",
			"ORPHAN_CONCEPT",
			"ORPHAN_CONCEPT",
			"UNINDEXED_CONCEPT",
		}
		if got := findingCodes(report); !slices.Equal(got, wantCodes) {
			t.Fatalf("unexpected finding order:\n got: %v\nwant: %v", got, wantCodes)
		}

		// Repeat the audit and require byte-for-byte stable findings and counts.
		_, _, repeated := runAuditJSON(t, path, "--json")
		if !reflect.DeepEqual(report.Findings, repeated.Findings) {
			t.Fatalf("findings are not deterministic:\nfirst:  %+v\nsecond: %+v", report.Findings, repeated.Findings)
		}
		if !reflect.DeepEqual(report.Summary, repeated.Summary) {
			t.Fatalf("summary is not deterministic:\nfirst:  %+v\nsecond: %+v", report.Summary, repeated.Summary)
		}
	})

	t.Run("fail-on changes only the exit code", func(t *testing.T) {
		path := auditFixture("warnings-only")

		defaultCode, _, defaultReport := runAuditJSON(t, path, "--json")
		warningCode, _, warningReport := runAuditJSON(t, path, "--fail-on", "warning", "--json")
		noneCode, _, noneReport := runAuditJSON(t, path, "--fail-on", "none", "--json")

		if defaultCode != 0 {
			t.Fatalf("expected default fail-on=error to exit 0, got %d", defaultCode)
		}
		if warningCode != 1 {
			t.Fatalf("expected fail-on=warning to exit 1, got %d", warningCode)
		}
		if noneCode != 0 {
			t.Fatalf("expected fail-on=none to exit 0, got %d", noneCode)
		}

		wantCodes := []string{"ORPHAN_CONCEPT"}
		for name, report := range map[string]bddAuditReport{
			"default": defaultReport,
			"warning": warningReport,
			"none":    noneReport,
		} {
			if got := findingCodes(report); !slices.Equal(got, wantCodes) {
				t.Fatalf("%s: unexpected findings: got %v, want %v", name, got, wantCodes)
			}
			if report.Summary.Orphans != 1 {
				t.Fatalf("%s: expected 1 orphan, got %d", name, report.Summary.Orphans)
			}
		}
	})

	t.Run("empty bundle is a warning and does not fail the audit", func(t *testing.T) {
		root := t.TempDir()
		code, _, report := runAuditJSON(t, root, "--json")

		if code != 0 {
			t.Fatalf("expected exit 0 for an empty bundle, got %d", code)
		}
		if report.Summary.Concepts != 0 {
			t.Fatalf("expected 0 concepts, got %d", report.Summary.Concepts)
		}
		if !hasFinding(report, "EMPTY_BUNDLE") {
			t.Fatalf("expected EMPTY_BUNDLE, got %v", findingCodes(report))
		}
	})

	t.Run("invalid root returns a usage failure", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "missing")
		code, stdout, _ := invoke(t, "audit", missing, "--json")

		if code != 2 {
			t.Fatalf("expected exit 2, got %d", code)
		}
		envelope := decodeEnvelope(t, stdout)
		if envelope.OK {
			t.Fatal("expected a failure envelope for an invalid root")
		}
		if envelope.Error == nil || envelope.Error.Code != "INVALID_ROOT" {
			t.Fatalf("expected INVALID_ROOT, got %+v", envelope.Error)
		}
	})

	t.Run("human output summarizes findings", func(t *testing.T) {
		code, stdout, stderr := invoke(t, "audit", auditFixture("graph-issues"))
		if stderr != "" {
			t.Fatalf("expected empty stderr, got %q", stderr)
		}
		if code != 1 {
			t.Fatalf("expected exit 1, got %d", code)
		}
		for _, want := range []string{
			"weave audit:",
			"errors",
			"warnings",
			"BROKEN_LINK",
			"ORPHAN_CONCEPT",
			"audit: failed",
		} {
			if !strings.Contains(stdout, want) {
				t.Fatalf("expected human output to contain %q:\n%s", want, stdout)
			}
		}
	})

	t.Run("audit does not write inside the bundle", func(t *testing.T) {
		root := auditFixture("clean")
		before := snapshotBundle(t, root)

		code, stdout, stderr := invoke(t, "audit", root, "--json")
		if code != 0 || stderr != "" {
			t.Fatalf("exit=%d stderr=%q", code, stderr)
		}
		if stdout == "" {
			t.Fatal("expected an audit envelope")
		}

		after := snapshotBundle(t, root)
		if !reflect.DeepEqual(before, after) {
			t.Fatalf("audit modified the bundle:\nbefore: %v\nafter:  %v", before, after)
		}
	})

	t.Run("verify-keys is a no-op without privacy directives", func(t *testing.T) {
		code, _, report := runAuditJSON(t, auditFixture("clean"), "--verify-keys", "--json")
		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
		if hasFinding(report, "KEY_UNAVAILABLE") {
			t.Fatalf("expected no KEY_UNAVAILABLE, got %v", findingCodes(report))
		}
	})
}

func snapshotBundle(t *testing.T, root string) map[string]string {
	t.Helper()

	files := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(relative)] = string(content)
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot bundle: %v", err)
	}
	return files
}
