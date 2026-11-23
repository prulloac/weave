package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
)

// Behavior-level tests for `weave extensions check`. See docs/bdd/extension-conformance.md.

func decodeCheckReport(t *testing.T, output string) contractv1.ExtensionCheckReport {
	t.Helper()
	envelope := decodeEnvelope(t, output)
	if envelope.Error != nil {
		t.Fatalf("unexpected error envelope: %+v", envelope.Error)
	}
	var report contractv1.ExtensionCheckReport
	if err := json.Unmarshal(envelope.Result, &report); err != nil {
		t.Fatalf("decode check report: %v\n%s", err, output)
	}
	return report
}

func checkBy(t *testing.T, report contractv1.ExtensionCheckReport, name string) contractv1.ExtensionCheckResult {
	t.Helper()
	for _, check := range report.Checks {
		if check.Check == name {
			return check
		}
	}
	t.Fatalf("check %q is missing from %+v", name, report.Checks)
	return contractv1.ExtensionCheckResult{}
}

func TestCheckCommandPasses(t *testing.T) {
	withCoreVersion(t, "0.11.0")
	root := extensionFixture(t)
	installExtension(t, root, "ok", "check-ok.sh")

	code, stdout, stderr := Invoke(t, "extensions", "check", "ok", root, "--timeout", "5s", "--json")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d stdout=%s stderr=%s", code, stdout, stderr)
	}
	report := decodeCheckReport(t, stdout)
	if !report.OK || report.V != "weave.cli/v1" || report.Extension != "ok" {
		t.Fatalf("unexpected report: %+v", report)
	}
	if got := checkBy(t, report, "manifest").Status; got != contractv1.ExtensionCheckPass {
		t.Fatalf("manifest status = %s", got)
	}
	if got := checkBy(t, report, "enrichment").Status; got != contractv1.ExtensionCheckPass {
		t.Fatalf("enrichment status = %s", got)
	}
}

func TestCheckCommandFailsMalformedRole(t *testing.T) {
	withCoreVersion(t, "0.11.0")
	root := extensionFixture(t)
	installExtension(t, root, "bad", "check-bad.sh")

	code, stdout, _ := Invoke(t, "extensions", "check", "bad", root, "--json")
	if code != 1 {
		t.Fatalf("expected exit 1, got %d: %s", code, stdout)
	}
	report := decodeCheckReport(t, stdout)
	if report.OK {
		t.Fatal("expected a failing report")
	}
	check := checkBy(t, report, "enrichment")
	if check.Status != contractv1.ExtensionCheckFail || check.Code != contractv1.ErrExtensionProto {
		t.Fatalf("unexpected enrichment check: %+v", check)
	}
	if got := checkBy(t, report, "manifest").Status; got != contractv1.ExtensionCheckPass {
		t.Fatalf("manifest status = %s", got)
	}
}

func TestCheckCommandDescriptorMinimum(t *testing.T) {
	withCoreVersion(t, "0.11.0")
	root := extensionFixture(t)
	installExtension(t, root, "code", "check-project-unversioned.sh")

	code, stdout, _ := Invoke(t, "extensions", "check", "code", root, "--json")
	if code != 1 {
		t.Fatalf("expected exit 1, got %d: %s", code, stdout)
	}
	report := decodeCheckReport(t, stdout)
	check := checkBy(t, report, "requirements")
	if check.Status != contractv1.ExtensionCheckFail || check.Code != contractv1.ErrExtensionRequirement {
		t.Fatalf("unexpected requirements check: %+v", check)
	}
	for _, other := range report.Checks {
		if other.Check == "project" {
			t.Fatal("project check must not run when the descriptor minimum is unmet")
		}
	}
}

func TestCheckCommandVersionedProject(t *testing.T) {
	withCoreVersion(t, "0.11.0")
	root := extensionFixture(t)
	installExtension(t, root, "code", "check-project.sh")

	code, stdout, stderr := Invoke(t, "extensions", "check", "code", root, "--json")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d stdout=%s stderr=%s", code, stdout, stderr)
	}
	report := decodeCheckReport(t, stdout)
	if got := checkBy(t, report, "project").Status; got != contractv1.ExtensionCheckPass {
		t.Fatalf("project status = %s", got)
	}
}

func TestCheckCommandUnknownExtension(t *testing.T) {
	withCoreVersion(t, "0.11.0")
	root := extensionFixture(t)

	code, stdout, _ := Invoke(t, "extensions", "check", "missing", root, "--json")
	if code != 2 {
		t.Fatalf("expected exit 2, got %d: %s", code, stdout)
	}
	envelope := decodeEnvelope(t, stdout)
	if envelope.Error == nil || envelope.Error.Code != contractv1.ErrExtensionMissing {
		t.Fatalf("expected EXTENSION_NOT_FOUND, got %+v", envelope.Error)
	}
}

func TestCheckCommandCodec(t *testing.T) {
	withCoreVersion(t, "0.11.0")
	root := extensionFixture(t)
	installExtension(t, root, "codec-ident", "check-codec.sh")

	code, stdout, stderr := Invoke(t, "extensions", "check", "codec-ident", root, "--json")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d stdout=%s stderr=%s", code, stdout, stderr)
	}
	report := decodeCheckReport(t, stdout)
	if !report.OK || report.Extension != "codec-ident" {
		t.Fatalf("unexpected report: %+v", report)
	}
	for _, name := range []string{"manifest", "compress", "decompress", "determinism"} {
		if got := checkBy(t, report, name).Status; got != contractv1.ExtensionCheckPass {
			t.Fatalf("%s status = %s", name, got)
		}
	}
}

func TestCheckCommandUsesDeclaredRoles(t *testing.T) {
	withCoreVersion(t, "0.11.0")
	root := extensionFixture(t)
	installExtension(t, root, "rerank", "check-rerank.sh")

	code, stdout, stderr := Invoke(t, "extensions", "check", "rerank", root, "--json")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d stdout=%s stderr=%s", code, stdout, stderr)
	}
	report := decodeCheckReport(t, stdout)
	if !report.OK {
		t.Fatalf("expected a passing report: %+v", report.Checks)
	}
	if got := checkBy(t, report, "rerank").Status; got != contractv1.ExtensionCheckPass {
		t.Fatalf("rerank status = %s", got)
	}
	for _, other := range report.Checks {
		if other.Check == "enrichment" {
			t.Fatal("roles must suppress the legacy enrichment inference")
		}
	}
}

func TestCheckCommandRerankRole(t *testing.T) {
	withCoreVersion(t, "0.11.0")
	root := extensionFixture(t)
	installExtension(t, root, "rerank", "check-rerank.sh")

	code, stdout, stderr := Invoke(t, "extensions", "check", "rerank", root, "--json")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d stdout=%s stderr=%s", code, stdout, stderr)
	}
	report := decodeCheckReport(t, stdout)
	names := make([]string, 0, len(report.Checks))
	for _, check := range report.Checks {
		names = append(names, check.Check)
	}
	want := []string{"manifest", "protocols", "rerank"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("checks = %v, want %v", names, want)
	}
}

func TestDeclaredRolesDoNotAutoSelect(t *testing.T) {
	withCoreVersion(t, "0.11.0")
	marker := filepath.Join(t.TempDir(), "invoked")
	t.Setenv("TEST_MARKER", marker)
	root := extensionFixture(t)
	installExtension(t, root, "rerank", "check-rerank-marker.sh")

	code, stdout, stderr := Invoke(t, "search", root, "alpha", "--no-db", "--json")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d stdout=%s stderr=%s", code, stdout, stderr)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("a declared role must not be selected implicitly, stat err = %v", err)
	}
}

func TestCheckCommandHumanOutput(t *testing.T) {
	withCoreVersion(t, "0.11.0")
	root := extensionFixture(t)
	installExtension(t, root, "ok", "check-ok.sh")

	code, stdout, stderr := Invoke(t, "extensions", "check", "ok", root)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d stdout=%s stderr=%s", code, stdout, stderr)
	}
	for _, want := range []string{"manifest", "enrichment", "ok: ok (2 checks)"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("stdout missing %q:\n%s", want, stdout)
		}
	}
}

func TestCheckCommandDoesNotWrite(t *testing.T) {
	withCoreVersion(t, "0.11.0")
	root := extensionFixture(t)
	installExtension(t, root, "ok", "check-ok.sh")
	before := listBundleFiles(t, root)

	if code, stdout, stderr := Invoke(t, "extensions", "check", "ok", root, "--json"); code != 0 {
		t.Fatalf("expected exit 0, got %d stdout=%s stderr=%s", code, stdout, stderr)
	}
	after := listBundleFiles(t, root)
	if strings.Join(before, "\n") != strings.Join(after, "\n") {
		t.Fatalf("bundle changed:\nbefore %v\nafter  %v", before, after)
	}
	if _, err := os.Stat(filepath.Join(root, ".weave", "index.db")); !os.IsNotExist(err) {
		t.Fatalf("check must not create a database, stat err = %v", err)
	}
}

func listBundleFiles(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(relative))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}
