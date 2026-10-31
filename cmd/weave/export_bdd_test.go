package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type bddExportReport struct {
	Source    string `json:"source"`
	Out       string `json:"out"`
	Format    string `json:"format"`
	Documents int    `json:"documents"`
	Encrypted int    `json:"encrypted"`
	Tokenized int    `json:"tokenized"`
	Hashed    int    `json:"hashed"`
}

func exportFixture(t *testing.T) (root, db string) {
	t.Helper()
	t.Setenv("WEAVE_KEY_OPS_PROD_KEY", strings.Repeat("ab", 32))
	t.Setenv("WEAVE_KEY_TENANT_SALT", strings.Repeat("cd", 32))

	root = t.TempDir()
	write := func(name, frontmatter, body string) {
		content := "---\ntype: Concept\ntitle: \"" + name + "\"\nkeywords:\n  - \"" + name + "\"\n" + frontmatter + "---\n\n# " + name + "\n\n" + body + "\n"
		if err := os.WriteFile(filepath.Join(root, name+".md"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("plain", "", "public content")
	write("tenant", "privacy:\n  policy: tokenize\n  provider: hmac-sha256\n  key_id: tenant-salt\n", "Account internal-client-alpha owns it.")
	write("secret", "privacy:\n  policy: encrypt\n  provider: aes-256-gcm\n  key_id: ops-prod-key\n", "The launch code is zanzibar-secret.")

	db = filepath.Join(t.TempDir(), "index.db")
	code, _, stderr := invokeRaw(t, "ingest", root, "--db", db, "--strict", "--json")
	if code != 0 {
		t.Fatalf("ingest exit=%d stderr=%q", code, stderr)
	}
	return root, db
}

func runExport(t *testing.T, args ...string) (int, testEnvelope, bddExportReport) {
	t.Helper()
	code, stdout, _ := invokeRaw(t, append([]string{"export"}, args...)...)
	envelope := decodeEnvelope(t, stdout)
	var report bddExportReport
	if envelope.OK {
		if err := json.Unmarshal(envelope.Result, &report); err != nil {
			t.Fatalf("decode export report: %v\n%s", err, envelope.Result)
		}
	}
	return code, envelope, report
}

func readExportFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func TestExportRaw(t *testing.T) {
	root, db := exportFixture(t)
	out := filepath.Join(t.TempDir(), "export")

	code, envelope, report := runExport(t, root, "--out", out, "--format", "raw", "--yes", "--db", db, "--json")
	if code != 0 || !envelope.OK {
		t.Fatalf("export exit=%d error=%+v", code, envelope.Error)
	}
	if report.Documents != 3 || report.Encrypted != 1 || report.Tokenized != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}
	if !strings.Contains(readExportFile(t, filepath.Join(out, "secret.md")), "zanzibar-secret") {
		t.Fatal("raw export must decrypt encrypted bodies")
	}
	tenant := readExportFile(t, filepath.Join(out, "tenant.md"))
	if strings.Contains(tenant, "internal-client-alpha") || !strings.Contains(tenant, "tok") {
		t.Fatalf("tokenized body must stay aliased: %s", tenant)
	}
	if !strings.Contains(readExportFile(t, filepath.Join(out, "weave-export.json")), `"format": "raw"`) {
		t.Fatal("missing raw manifest")
	}

	db2 := filepath.Join(t.TempDir(), "index.db")
	code, _, stderr := invokeRaw(t, "ingest", out, "--db", db2, "--strict", "--json")
	if code != 0 {
		t.Fatalf("re-ingest exit=%d stderr=%q", code, stderr)
	}
	code, stdout, stderr := invokeRaw(t, "show", out, "secret", "--db", db2, "--decrypt", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("show exit=%d stderr=%q", code, stderr)
	}
	if !strings.Contains(string(stdout), "zanzibar-secret") {
		t.Fatal("round-tripped encrypted document must decrypt")
	}
}

func TestExportRawRequiresConfirmation(t *testing.T) {
	root, db := exportFixture(t)
	out := filepath.Join(t.TempDir(), "export")

	code, envelope, _ := runExport(t, root, "--out", out, "--format", "raw", "--db", db, "--json")
	if code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
	if envelope.Error == nil || envelope.Error.Code != "CONFIRMATION_REQUIRED" {
		t.Fatalf("expected CONFIRMATION_REQUIRED, got %+v", envelope.Error)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatalf("target must not be written: %v", err)
	}
}

func TestExportRawMissingKey(t *testing.T) {
	root, db := exportFixture(t)
	out := filepath.Join(t.TempDir(), "export")
	os.Unsetenv("WEAVE_KEY_OPS_PROD_KEY")

	code, envelope, _ := runExport(t, root, "--out", out, "--format", "raw", "--yes", "--db", db, "--json")
	if code != 3 {
		t.Fatalf("expected exit 3, got %d", code)
	}
	if envelope.Error == nil || envelope.Error.Code != "PRIVACY_KEY_UNAVAILABLE" {
		t.Fatalf("expected PRIVACY_KEY_UNAVAILABLE, got %+v", envelope.Error)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatalf("target must not be written: %v", err)
	}
}

func TestExportEncrypted(t *testing.T) {
	root, db := exportFixture(t)
	out := filepath.Join(t.TempDir(), "export")
	t.Setenv("WEAVE_KEY_EXPORT_KEY", strings.Repeat("ef", 32))

	code, envelope, report := runExport(t, root, "--out", out, "--format", "encrypted",
		"--key-id", "export-key", "--db", db, "--json")
	if code != 0 || !envelope.OK {
		t.Fatalf("export exit=%d error=%+v", code, envelope.Error)
	}
	if report.Format != "encrypted" || report.Documents != 3 {
		t.Fatalf("unexpected report: %+v", report)
	}

	secret := readExportFile(t, filepath.Join(out, "secret.md"))
	if !strings.Contains(secret, "enc:v1:") || strings.Contains(secret, "zanzibar-secret") {
		t.Fatalf("encrypted export leaked plaintext: %s", secret)
	}
	if !strings.Contains(secret, "encoded: true") {
		t.Fatalf("missing encoded marker: %s", secret)
	}
	plain := readExportFile(t, filepath.Join(out, "plain.md"))
	if !strings.Contains(plain, "enc:v1:") {
		t.Fatal("plain document must be encrypted in encrypted export")
	}

	db2 := filepath.Join(t.TempDir(), "index.db")
	code, _, stderr := invokeRaw(t, "ingest", out, "--db", db2, "--strict", "--json")
	if code != 0 {
		t.Fatalf("re-ingest exit=%d stderr=%q", code, stderr)
	}
	code, stdout, stderr := invokeRaw(t, "show", out, "plain", "--db", db2, "--decrypt", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("show exit=%d stderr=%q", code, stderr)
	}
	if !strings.Contains(string(stdout), "public content") {
		t.Fatal("encrypted export must decrypt with the export key")
	}
}

func TestExportEncryptedMissingKey(t *testing.T) {
	root, db := exportFixture(t)
	out := filepath.Join(t.TempDir(), "export")
	os.Unsetenv("WEAVE_KEY_EXPORT_KEY")

	code, envelope, _ := runExport(t, root, "--out", out, "--format", "encrypted",
		"--key-id", "export-key", "--db", db, "--json")
	if code != 3 {
		t.Fatalf("expected exit 3, got %d", code)
	}
	if envelope.Error == nil || envelope.Error.Code != "PRIVACY_KEY_UNAVAILABLE" {
		t.Fatalf("expected PRIVACY_KEY_UNAVAILABLE, got %+v", envelope.Error)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatalf("target must not be written: %v", err)
	}
}

func TestExportSafety(t *testing.T) {
	root, db := exportFixture(t)
	out := filepath.Join(t.TempDir(), "export")
	os.MkdirAll(out, 0o755)
	if err := os.WriteFile(filepath.Join(out, "existing.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	code, _, _ := runExport(t, root, "--out", out, "--format", "raw", "--yes", "--db", db, "--json")
	if code != 2 {
		t.Fatalf("non-empty target must need --force, got %d", code)
	}
	code, envelope, _ := runExport(t, root, "--out", out, "--format", "raw", "--yes", "--force", "--db", db, "--json")
	if code != 0 || !envelope.OK {
		t.Fatalf("force export failed: %d %+v", code, envelope.Error)
	}
	if _, err := os.Stat(filepath.Join(out, "existing.txt")); err != nil {
		t.Fatal("force must not delete unrelated files")
	}

	inside := filepath.Join(root, "export")
	code, _, _ = runExport(t, root, "--out", inside, "--format", "raw", "--yes", "--db", db, "--json")
	if code != 2 {
		t.Fatalf("inside-source target must be rejected, got %d", code)
	}

	code, _, _ = runExport(t, root, "--out", out, "--db", db, "--json")
	if code != 2 {
		t.Fatalf("missing format must be rejected, got %d", code)
	}
}
