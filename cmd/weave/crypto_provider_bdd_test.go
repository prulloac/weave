package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
)

func installCryptoMock(t *testing.T, root, script string, config string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "crypto", script))
	if err != nil {
		t.Fatal(err)
	}
	binDir := filepath.Join(root, ".weave", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(binDir, "weave-crypto-mock")
	if err := os.WriteFile(path, data, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if config == "" {
		config = "default_provider: mock\n"
	}
	if err := os.WriteFile(filepath.Join(root, ".weave", "crypto.yaml"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
}

func cryptoFixture(t *testing.T, policy, keyID, body string) string {
	t.Helper()
	root := t.TempDir()
	frontmatter := "privacy:\n  policy: " + policy + "\n  provider: aes-256-gcm\n  key_id: " + keyID + "\n"
	if policy != "encrypt" {
		frontmatter = "privacy:\n  policy: " + policy + "\n  provider: hmac-sha256\n  key_id: " + keyID + "\n"
	}
	content := "---\ntype: Concept\ntitle: \"Secret\"\nkeywords:\n  - \"secret\"\n" + frontmatter + "---\n\n# Secret\n\n" + body + "\n"
	if err := os.WriteFile(filepath.Join(root, "secret.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestExternalCryptoEncryptDecrypt(t *testing.T) {
	root := cryptoFixture(t, "encrypt", "ops-prod-key", "zanzibar-secret")
	installCryptoMock(t, root, "mock.py", "")
	db := filepath.Join(t.TempDir(), "index.db")

	code, stdout, stderr := invokeRaw(t, "ingest", root, "--db", db, "--strict", "--json")
	if code != 0 {
		t.Fatalf("ingest exit=%d stderr=%q stdout=%s", code, stderr, stdout)
	}
	raw := compressionRaw(t, db)
	stored := string(storedBody(t, raw, root, "secret"))
	if !strings.HasPrefix(stored, "mock:v1:") || strings.Contains(stored, "zanzibar-secret") {
		t.Fatalf("expected provider ciphertext, got %q", stored)
	}

	code, stdout, stderr = invokeRaw(t, "show", root, "secret", "--db", db, "--decrypt", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("show exit=%d stderr=%q", code, stderr)
	}
	var view contractv1.ShowResult
	if err := json.Unmarshal(decodeEnvelope(t, stdout).Result, &view); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(view.Body, "zanzibar-secret") {
		t.Fatalf("expected decrypted body, got %q", view.Body)
	}
}

func TestExternalCryptoTokenizeAndHash(t *testing.T) {
	for _, policy := range []string{"tokenize", "hash"} {
		root := cryptoFixture(t, policy, "tenant-salt", "internal-client-alpha")
		installCryptoMock(t, root, "mock.py", "")
		db := filepath.Join(t.TempDir(), "index.db")
		code, _, stderr := invokeRaw(t, "ingest", root, "--db", db, "--strict", "--json")
		if code != 0 {
			t.Fatalf("%s ingest exit=%d stderr=%q", policy, code, stderr)
		}
		raw := compressionRaw(t, db)
		stored := string(storedBody(t, raw, root, "secret"))
		prefix := "tok"
		if policy == "hash" {
			prefix = "hash"
		}
		if !strings.Contains(stored, prefix) || strings.Contains(stored, "internal-client-alpha") {
			t.Fatalf("%s: unexpected stored body %q", policy, stored)
		}
	}
}

func TestExternalCryptoMissingProviderFails(t *testing.T) {
	root := cryptoFixture(t, "encrypt", "ops-prod-key", "zanzibar-secret")
	os.MkdirAll(filepath.Join(root, ".weave"), 0o755)
	if err := os.WriteFile(filepath.Join(root, ".weave", "crypto.yaml"), []byte("default_provider: mock\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	db := filepath.Join(t.TempDir(), "index.db")

	code, stdout, _ := invokeRaw(t, "ingest", root, "--db", db, "--strict", "--json")
	if code != 3 {
		t.Fatalf("expected exit 3, got %d stdout=%s", code, stdout)
	}
	envelope := decodeEnvelope(t, stdout)
	if envelope.Error == nil || envelope.Error.Code != "PRIVACY_KEY_UNAVAILABLE" {
		t.Fatalf("expected PRIVACY_KEY_UNAVAILABLE, got %+v", envelope.Error)
	}
}

func TestExternalCryptoInvalidResponseFails(t *testing.T) {
	root := cryptoFixture(t, "encrypt", "ops-prod-key", "zanzibar-secret")
	installCryptoMock(t, root, "mock.py", "")
	t.Setenv("MOCK_BROKEN", "1")
	db := filepath.Join(t.TempDir(), "index.db")

	code, _, _ := invokeRaw(t, "ingest", root, "--db", db, "--strict", "--json")
	if code != 3 {
		t.Fatalf("expected exit 3 for invalid provider response, got %d", code)
	}
	if _, err := os.Stat(db); err == nil {
		raw := compressionRaw(t, db)
		var count int
		if err := raw.QueryRow("SELECT COUNT(*) FROM documents").Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("failed encryption must not store documents, got %d", count)
		}
	}
}

func TestExternalCryptoLocalOverride(t *testing.T) {
	t.Setenv("WEAVE_KEY_OPS_PROD_KEY", strings.Repeat("ab", 32))
	root := cryptoFixture(t, "encrypt", "ops-prod-key", "zanzibar-secret")
	installCryptoMock(t, root, "mock.py", "")
	db := filepath.Join(t.TempDir(), "index.db")

	code, _, stderr := invokeRaw(t, "ingest", root, "--db", db, "--strict", "--provider", "local", "--json")
	if code != 0 {
		t.Fatalf("ingest exit=%d stderr=%q", code, stderr)
	}
	raw := compressionRaw(t, db)
	stored := string(storedBody(t, raw, root, "secret"))
	if !strings.HasPrefix(stored, "enc:v1:") {
		t.Fatalf("expected local ciphertext, got %q", stored)
	}
}

func TestExternalCryptoAudit(t *testing.T) {
	root := cryptoFixture(t, "encrypt", "ops-prod-key", "zanzibar-secret")
	installCryptoMock(t, root, "mock.py", "")
	db := filepath.Join(t.TempDir(), "index.db")
	if code, _, stderr := invokeRaw(t, "ingest", root, "--db", db, "--strict", "--json"); code != 0 {
		t.Fatalf("ingest exit=%d stderr=%q", code, stderr)
	}

	code, stdout, stderr := invokeRaw(t, "audit", root, "--db", db, "--verify-keys", "--fail-on", "none", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("audit exit=%d stderr=%q", code, stderr)
	}
	var report contractv1.AuditReport
	if err := json.Unmarshal(decodeEnvelope(t, stdout).Result, &report); err != nil {
		t.Fatal(err)
	}
	for _, finding := range report.Findings {
		if finding.Code == "KEY_UNAVAILABLE" {
			t.Fatalf("provider key should be available: %+v", finding)
		}
	}

	t.Setenv("MOCK_KEY_IDS", "other-key")
	code, stdout, _ = invokeRaw(t, "audit", root, "--db", db, "--verify-keys", "--fail-on", "none", "--json")
	if code != 0 {
		t.Fatalf("audit exit=%d", code)
	}
	if err := json.Unmarshal(decodeEnvelope(t, stdout).Result, &report); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, finding := range report.Findings {
		if finding.Code == "KEY_UNAVAILABLE" && finding.Concept == "secret" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected KEY_UNAVAILABLE when provider lacks the key")
	}
}
