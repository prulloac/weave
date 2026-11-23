package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
)

// Behavior-level tests for the bundle-local trust gate. See docs/bdd/extension-trust.md.

const trustEnricherScript = `#!/bin/sh
if [ "$1" = "--describe" ]; then
  printf '%s' '{"v":"weave.extension/v1","name":"ai","version":"0.11.0"}'
  exit 0
fi
cat >/dev/null
printf '%s' '{"v":"weave.cli/v1","inferred_keywords":["trusted"]}'
`

func installUntrusted(t *testing.T, root, name, body string) string {
	t.Helper()
	dir := filepath.Join(root, ".weave", "bin")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "weave-"+name)
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestUntrustedExplicitSelectionRefused(t *testing.T) {
	withCoreVersion(t, "0.11.0")
	root := extensionFixture(t)
	installUntrusted(t, root, "ai", trustEnricherScript)

	code, stdout, stderr := Invoke(t, "ingest", root, "--with-enricher", "ai", "--db", filepath.Join(t.TempDir(), "index.db"), "--json")
	if code != 1 {
		t.Fatalf("expected exit 1, got %d stdout=%s", code, stdout)
	}
	envelope := decodeEnvelope(t, stdout)
	if envelope.Error == nil || envelope.Error.Code != contractv1.ErrExtensionUntrusted {
		t.Fatalf("expected EXTENSION_UNTRUSTED, got %+v", envelope.Error)
	}
	if !strings.Contains(stderr, "extensions trust ai") {
		t.Fatalf("missing trust hint: %q", stderr)
	}
}

func TestTrustedExtensionRuns(t *testing.T) {
	withCoreVersion(t, "0.11.0")
	root := extensionFixture(t)
	installUntrusted(t, root, "ai", trustEnricherScript)

	if code, stdout, stderr := Invoke(t, "extensions", "trust", "ai", root); code != 0 {
		t.Fatalf("trust failed: %d %s %s", code, stdout, stderr)
	}
	code, stdout, stderr := Invoke(t, "ingest", root, "--with-enricher", "ai", "--db", filepath.Join(t.TempDir(), "index.db"), "--json")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d stdout=%s stderr=%s", code, stdout, stderr)
	}
}

func TestTrustHashChange(t *testing.T) {
	withCoreVersion(t, "0.11.0")
	root := extensionFixture(t)
	path := installUntrusted(t, root, "ai", trustEnricherScript)
	if code, _, stderr := Invoke(t, "extensions", "trust", "ai", root); code != 0 {
		t.Fatalf("trust failed: %s", stderr)
	}
	if err := os.WriteFile(path, []byte(trustEnricherScript+"\n# changed\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	code, stdout, _ := Invoke(t, "extensions", "trusted", "--json")
	if code != 0 {
		t.Fatalf("trusted failed: %d", code)
	}
	envelope := decodeEnvelope(t, stdout)
	var report struct {
		Extensions []struct {
			Name    string `json:"name"`
			Matches bool   `json:"matches"`
		} `json:"extensions"`
	}
	if err := json.Unmarshal(envelope.Result, &report); err != nil {
		t.Fatal(err)
	}
	if len(report.Extensions) != 1 || report.Extensions[0].Matches {
		t.Fatalf("hash change not reported: %+v", report.Extensions)
	}
	code, _, _ = Invoke(t, "ingest", root, "--with-enricher", "ai", "--db", filepath.Join(t.TempDir(), "index.db"), "--json")
	if code != 1 {
		t.Fatalf("expected exit 1 after a hash change, got %d", code)
	}
}

func TestUntrustRemovesGrant(t *testing.T) {
	withCoreVersion(t, "0.11.0")
	root := extensionFixture(t)
	installUntrusted(t, root, "ai", trustEnricherScript)
	Invoke(t, "extensions", "trust", "ai", root)
	if code, _, stderr := Invoke(t, "extensions", "untrust", "ai"); code != 0 {
		t.Fatalf("untrust failed: %s", stderr)
	}
	code, _, _ := Invoke(t, "ingest", root, "--with-enricher", "ai", "--db", filepath.Join(t.TempDir(), "index.db"), "--json")
	if code != 1 {
		t.Fatalf("expected exit 1 after untrust, got %d", code)
	}
}

func TestBundleTrustFileCannotGrant(t *testing.T) {
	withCoreVersion(t, "0.11.0")
	root := extensionFixture(t)
	path := installUntrusted(t, root, "ai", trustEnricherScript)
	content := "version: 1\nextensions:\n  ai:\n    path: " + path + "\n    sha256: sha256:0000\n"
	if err := os.WriteFile(filepath.Join(root, ".weave", "trust.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, _ := Invoke(t, "ingest", root, "--with-enricher", "ai", "--db", filepath.Join(t.TempDir(), "index.db"), "--json")
	if code != 1 {
		t.Fatalf("bundle trust file granted trust, exit=%d", code)
	}
}

func TestPathExtensionNotGated(t *testing.T) {
	withCoreVersion(t, "0.11.0")
	root := extensionFixture(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "weave-ai"), []byte(trustEnricherScript), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	code, stdout, stderr := Invoke(t, "ingest", root, "--with-enricher", "ai", "--db", filepath.Join(t.TempDir(), "index.db"), "--json")
	if code != 0 {
		t.Fatalf("PATH extension was gated: %d stdout=%s stderr=%s", code, stdout, stderr)
	}
}

func TestListHasNoTrustField(t *testing.T) {
	root := extensionFixture(t)
	installUntrusted(t, root, "ai", trustEnricherScript)
	code, stdout, stderr := Invoke(t, "extensions", "list", root, "--json")
	if code != 0 {
		t.Fatalf("list failed: %d %s", code, stderr)
	}
	if strings.Contains(stdout, `"trust"`) || strings.Contains(stdout, `"trusted"`) {
		t.Fatalf("extensions list leaked trust state: %s", stdout)
	}
}

func TestTrustedReport(t *testing.T) {
	root := extensionFixture(t)
	path := installUntrusted(t, root, "ai", trustEnricherScript)
	if code, _, stderr := Invoke(t, "extensions", "trust", "ai", root); code != 0 {
		t.Fatalf("trust failed: %s", stderr)
	}
	code, stdout, stderr := Invoke(t, "extensions", "trusted", "--json")
	if code != 0 {
		t.Fatalf("trusted failed: %d %s", code, stderr)
	}
	envelope := decodeEnvelope(t, stdout)
	var report struct {
		Extensions []struct {
			Name    string `json:"name"`
			Path    string `json:"path"`
			SHA256  string `json:"sha256"`
			Matches bool   `json:"matches"`
		} `json:"extensions"`
	}
	if err := json.Unmarshal(envelope.Result, &report); err != nil {
		t.Fatal(err)
	}
	if len(report.Extensions) != 1 {
		t.Fatalf("extensions = %+v", report.Extensions)
	}
	entry := report.Extensions[0]
	absolute, _ := filepath.Abs(path)
	if entry.Name != "ai" || entry.Path != absolute || !strings.HasPrefix(entry.SHA256, "sha256:") || !entry.Matches {
		t.Fatalf("unexpected trust entry: %+v", entry)
	}
}

func TestCheckUntrusted(t *testing.T) {
	withCoreVersion(t, "0.11.0")
	root := extensionFixture(t)
	installUntrusted(t, root, "ai", trustEnricherScript)
	code, stdout, _ := Invoke(t, "extensions", "check", "ai", root, "--json")
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	report := decodeCheckReport(t, stdout)
	check := checkBy(t, report, "requirements")
	if check.Status != contractv1.ExtensionCheckFail || check.Code != contractv1.ErrExtensionUntrusted {
		t.Fatalf("unexpected requirements check: %+v", check)
	}
	for _, other := range report.Checks {
		if other.Check == "enrichment" {
			t.Fatal("no role check may run for an untrusted extension")
		}
	}
}

func TestAutoDiscoveredUntrustedSkipped(t *testing.T) {
	withCoreVersion(t, "0.11.0")
	workspaceDir := t.TempDir()
	notes := filepath.Join(workspaceDir, "notes")
	if err := os.MkdirAll(notes, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\ntype: Concept\ntitle: Note\nkeywords:\n  - note\n---\n\n# Note\n"
	if err := os.WriteFile(filepath.Join(notes, "note.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(workspaceDir, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := "version: 1\nsources:\n  notes: " + notes + "\n  core:\n    git: " + repo + "\n"
	if err := os.WriteFile(filepath.Join(workspaceDir, workspaceManifestName), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	installUntrusted(t, workspaceDir, "git", `#!/bin/sh
if [ "$1" = "--describe" ]; then
  printf '%s' '{"v":"weave.extension/v1","name":"git","version":"0.11.0","source_providers":["git"]}'
  exit 0
fi
echo "must not run" >&2
exit 1
`)

	code, stdout, stderr := Invoke(t, "ingest", "--workspace", workspaceDir, "--permissive", "--json")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d stdout=%s stderr=%s", code, stdout, stderr)
	}
	if !strings.Contains(stderr, "untrusted") || !strings.Contains(stderr, "extensions trust") {
		t.Fatalf("missing auto-skip warning: %q", stderr)
	}
	envelope := decodeEnvelope(t, stdout)
	var report contractv1.IngestReport
	if err := json.Unmarshal(envelope.Result, &report); err != nil {
		t.Fatal(err)
	}
	for _, source := range report.Sources {
		if source.Source == "core" {
			t.Fatalf("untrusted source was indexed: %+v", report.Sources)
		}
	}
	if len(report.Sources) != 1 || report.Sources[0].Source != "notes" {
		t.Fatalf("unexpected sources: %+v", report.Sources)
	}
}
