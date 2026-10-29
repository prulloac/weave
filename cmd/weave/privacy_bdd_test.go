package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine"
	"github.com/prulloac/weave/engine/operations"
	"github.com/prulloac/weave/mcp"
	sqlitestore "github.com/prulloac/weave/storage/sqlite"
)

// Behavior-level tests for declared privacy directives. They drive the same
// CLI entrypoint as a user and inspect the persistent index through the
// storage package, never through privacy internals.

type bddPrivacyState struct {
	Policy    string `json:"policy"`
	Provider  string `json:"provider"`
	KeyID     string `json:"key_id"`
	Redacted  bool   `json:"redacted"`
	Decrypted bool   `json:"decrypted"`
}

type bddShowResult struct {
	ID       string           `json:"id"`
	Title    *string          `json:"title,omitempty"`
	Keywords []string         `json:"keywords"`
	Body     string           `json:"body"`
	Privacy  *bddPrivacyState `json:"privacy,omitempty"`
}

type bddParsedConcept struct {
	ID      string `json:"id"`
	Body    string `json:"body"`
	Title   string `json:"title,omitempty"`
	Privacy *struct {
		Policy   string `json:"policy"`
		Provider string `json:"provider"`
		KeyID    string `json:"key_id"`
	} `json:"privacy,omitempty"`
}

type bddParsedBundle struct {
	Concepts   []bddParsedConcept `json:"concepts"`
	Validation struct {
		Warnings []string `json:"warnings"`
	} `json:"validation"`
}

const (
	privacyOpsKey    = "WEAVE_KEY_OPS_PROD_KEY"
	privacyTenantKey = "WEAVE_KEY_TENANT_SALT"
	privacyAuditKey  = "WEAVE_KEY_AUDIT_DIGEST"
)

func privacyFixture(t *testing.T) string {
	t.Helper()
	source := filepath.Join("testdata", "privacy", "bundle-a")
	destination := t.TempDir()
	if err := os.CopyFS(destination, os.DirFS(source)); err != nil {
		t.Fatalf("copy privacy bundle: %v", err)
	}
	return destination
}

func privacyKeys(t *testing.T) {
	t.Helper()
	t.Setenv(privacyOpsKey, strings.Repeat("ab", 32))
	t.Setenv(privacyTenantKey, strings.Repeat("cd", 32))
	t.Setenv(privacyAuditKey, strings.Repeat("ef", 32))
}

func privacyDB(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "index.db")
}

func privacyIngest(t *testing.T, root, db string) testEnvelope {
	t.Helper()
	code, stdout, stderr := invokeRaw(t, "ingest", root, "--db", db, "--strict", "--json")
	if code != 0 {
		t.Fatalf("ingest exit=%d stderr=%q stdout=%s", code, stderr, stdout)
	}
	envelope := decodeEnvelope(t, stdout)
	if !envelope.OK {
		t.Fatalf("ingest failed: %+v", envelope.Error)
	}
	return envelope
}

func privacyLoadBundle(t *testing.T, db, root string) *contractv1.Bundle {
	t.Helper()
	store, err := sqlitestore.Open(db)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	bundle, err := store.LoadBundle(context.Background(), root)
	if err != nil {
		t.Fatalf("load bundle: %v", err)
	}
	return bundle
}

func privacyBody(bundle *contractv1.Bundle, id string) string {
	for _, concept := range bundle.Concepts {
		if concept.ID == id {
			return concept.Body
		}
	}
	return ""
}

func privacyShow(t *testing.T, root, db, id string, extra ...string) (int, testEnvelope, bddShowResult) {
	t.Helper()
	args := append([]string{"show", root, id, "--db", db}, extra...)
	args = append(args, "--json")
	code, stdout, stderr := invokeRaw(t, args...)
	envelope := decodeEnvelope(t, stdout)
	var result bddShowResult
	if envelope.OK {
		if err := json.Unmarshal(envelope.Result, &result); err != nil {
			t.Fatalf("decode show result: %v\n%s", err, envelope.Result)
		}
	} else if stderr != "" && code == 0 {
		t.Fatalf("unexpected stderr: %q", stderr)
	}
	return code, envelope, result
}

func privacySearch(t *testing.T, root, db, query, mode string) contractv1.SearchResponse {
	t.Helper()
	code, stdout, stderr := invokeRaw(t, "search", root, query, "--db", db, "--mode", mode, "--json")
	if code == 1 && strings.TrimSpace(stdout) == "" {
		return contractv1.SearchResponse{}
	}
	if code != 0 {
		t.Fatalf("search exit=%d stderr=%q", code, stderr)
	}
	envelope := decodeEnvelope(t, stdout)
	if !envelope.OK {
		t.Fatalf("search failed: %+v", envelope.Error)
	}
	var response contractv1.SearchResponse
	if err := json.Unmarshal(envelope.Result, &response); err != nil {
		t.Fatalf("decode search result: %v\n%s", err, envelope.Result)
	}
	return response
}

func hasKeywordHit(response contractv1.SearchResponse, id string) bool {
	for _, hit := range response.Keyword {
		if hit.ID == id {
			return true
		}
	}
	return false
}

func hasFuzzyHit(response contractv1.SearchResponse, id string) bool {
	for _, hit := range response.Fuzzy {
		if hit.ID == id {
			return true
		}
	}
	return false
}

func hasLexicalHit(response contractv1.SearchResponse, id string) bool {
	for _, hit := range response.Lexical {
		if hit.ID == id {
			return true
		}
	}
	return false
}

func hasHybridHit(response contractv1.SearchResponse, id string) bool {
	for _, hit := range response.Hybrid {
		if hit.ID == id {
			return true
		}
	}
	return false
}

func hasListedTitle(bundle contractv1.ListResults, title string) bool {
	for _, entry := range bundle {
		if entry.Title == title {
			return true
		}
	}
	return false
}

func TestPrivacyParseDeclarations(t *testing.T) {
	root := privacyFixture(t)

	code, stdout, stderr := invokeRaw(t, "parse", root, "--no-db", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("parse exit=%d stderr=%q", code, stderr)
	}
	envelope := decodeEnvelope(t, stdout)
	if !envelope.OK {
		t.Fatalf("parse failed: %+v", envelope.Error)
	}
	var bundle bddParsedBundle
	if err := json.Unmarshal(envelope.Result, &bundle); err != nil {
		t.Fatalf("decode bundle: %v", err)
	}

	var secret *bddParsedConcept
	for index := range bundle.Concepts {
		if bundle.Concepts[index].ID == "secret" {
			secret = &bundle.Concepts[index]
		}
	}
	if secret == nil {
		t.Fatal("secret concept missing from parse output")
	}
	if secret.Privacy == nil {
		t.Fatal("expected parsed privacy declaration")
	}
	if secret.Privacy.Policy != "encrypt" ||
		secret.Privacy.Provider != "aes-256-gcm" ||
		secret.Privacy.KeyID != "ops-prod-key" {
		t.Fatalf("unexpected declaration: %+v", secret.Privacy)
	}
	if !strings.Contains(secret.Body, "zanzibar-secret") {
		t.Fatalf("parse must not transform content: %q", secret.Body)
	}
}

func TestPrivacyInvalidDirectiveWarns(t *testing.T) {
	root := t.TempDir()
	writePrivacyFile(t, filepath.Join(root, "bad.md"), `---
type: Concept
title: "Bad Directive"
keywords:
  - "bad"
privacy:
  policy: scramble
---

# Bad Directive

Untouched content.
`)

	code, stdout, stderr := invokeRaw(t, "parse", root, "--no-db", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("parse exit=%d stderr=%q", code, stderr)
	}
	envelope := decodeEnvelope(t, stdout)
	var bundle bddParsedBundle
	if err := json.Unmarshal(envelope.Result, &bundle); err != nil {
		t.Fatalf("decode bundle: %v", err)
	}
	if !slicesContains(bundle.Validation.Warnings, "PRIVACY_DIRECTIVE_INVALID") {
		t.Fatalf("expected PRIVACY_DIRECTIVE_INVALID warning, got %v", bundle.Validation.Warnings)
	}
	if len(bundle.Concepts) != 1 || bundle.Concepts[0].Privacy != nil {
		t.Fatalf("invalid directive must not be exposed as a declaration: %+v", bundle.Concepts)
	}
}

func TestPrivacyEncryptedDocument(t *testing.T) {
	privacyKeys(t)
	root := privacyFixture(t)
	db := privacyDB(t)
	privacyIngest(t, root, db)

	stored := privacyLoadBundle(t, db, root)
	if body := privacyBody(stored, "secret"); !strings.HasPrefix(body, "enc:v1:") {
		t.Fatalf("expected ciphertext body, got %q", body)
	}
	if body := privacyBody(stored, "secret"); strings.Contains(body, "zanzibar-secret") {
		t.Fatalf("stored body leaks plaintext: %q", body)
	}

	if response := privacySearch(t, root, db, "zanzibar", "fuzzy"); hasFuzzyHit(response, "secret") {
		t.Fatal("fuzzy search leaked encrypted document")
	}
	if response := privacySearch(t, root, db, "zanzibar", "hybrid"); hasHybridHit(response, "secret") {
		t.Fatal("hybrid search leaked encrypted document")
	}
	if response := privacySearch(t, root, db, "zanzibar", "bm25"); hasLexicalHit(response, "secret") {
		t.Fatal("bm25 search leaked encrypted document")
	}
	if response := privacySearch(t, root, db, "secret", "keyword"); hasKeywordHit(response, "secret") {
		t.Fatal("metadata keyword search must not return encrypted document")
	}

	code, envelope, view := privacyShow(t, root, db, "secret")
	if code != 0 || !envelope.OK {
		t.Fatalf("show failed: %+v", envelope.Error)
	}
	if view.Body != "" {
		t.Fatalf("expected redacted body, got %q", view.Body)
	}
	if view.Privacy == nil || !view.Privacy.Redacted || view.Privacy.Policy != "encrypt" {
		t.Fatalf("unexpected privacy state: %+v", view.Privacy)
	}

	code, envelope, decrypted := privacyShow(t, root, db, "secret", "--decrypt")
	if code != 0 || !envelope.OK {
		t.Fatalf("decrypt failed: %+v", envelope.Error)
	}
	if !strings.Contains(decrypted.Body, "zanzibar-secret") {
		t.Fatalf("expected plaintext body, got %q", decrypted.Body)
	}
	if decrypted.Privacy == nil || !decrypted.Privacy.Decrypted {
		t.Fatalf("expected decrypted state, got %+v", decrypted.Privacy)
	}

	code, stdout, stderr := invokeRaw(t, "list", root, "--db", db, "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("list exit=%d stderr=%q", code, stderr)
	}
	listEnvelope := decodeEnvelope(t, stdout)
	var listed contractv1.ListResults
	if err := json.Unmarshal(listEnvelope.Result, &listed); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if !hasListedTitle(listed, "Operations Secret") {
		t.Fatal("encrypted document metadata must stay listable")
	}
}

func TestPrivacyTokenizedDocument(t *testing.T) {
	privacyKeys(t)
	root := privacyFixture(t)
	db := privacyDB(t)
	privacyIngest(t, root, db)

	stored := privacyLoadBundle(t, db, root)
	body := privacyBody(stored, "tenant")
	if strings.Contains(body, "internal-client-alpha") {
		t.Fatalf("tokenized body leaks plaintext: %q", body)
	}
	if !strings.Contains(body, "tok") {
		t.Fatalf("expected token aliases in stored body: %q", body)
	}

	response := privacySearch(t, root, db, "internal-client-alpha", "bm25")
	if !hasLexicalHit(response, "tenant") {
		t.Fatal("tokenized content must remain searchable by the original term")
	}
	partial := privacySearch(t, root, db, "interna", "bm25")
	if hasLexicalHit(partial, "tenant") {
		t.Fatal("tokenized content must not match partial terms")
	}
}

func TestPrivacyHashedSpan(t *testing.T) {
	privacyKeys(t)
	root := privacyFixture(t)
	db := privacyDB(t)
	privacyIngest(t, root, db)

	stored := privacyLoadBundle(t, db, root)
	body := privacyBody(stored, "digest")
	if strings.Contains(body, "CR-2026-0042") {
		t.Fatalf("hashed body leaks plaintext: %q", body)
	}
	if !strings.Contains(body, "hash") {
		t.Fatalf("expected hash alias in stored body: %q", body)
	}

	exact := privacySearch(t, root, db, "CR-2026-0042", "bm25")
	if !hasLexicalHit(exact, "digest") {
		t.Fatal("hashed span must match the exact protected phrase")
	}
	truncated := privacySearch(t, root, db, "CR-2026", "bm25")
	if hasLexicalHit(truncated, "digest") {
		t.Fatal("hashed span must not match a truncated phrase")
	}
}

func TestPrivacyMissingKeyFails(t *testing.T) {
	root := privacyFixture(t)
	db := privacyDB(t)
	t.Setenv(privacyTenantKey, strings.Repeat("cd", 32))
	t.Setenv(privacyAuditKey, strings.Repeat("ef", 32))
	os.Unsetenv(privacyOpsKey)

	code, stdout, stderr := invokeRaw(t, "ingest", root, "--db", db, "--strict", "--json")
	if code != 3 {
		t.Fatalf("expected exit 3, got %d (stderr=%q stdout=%s)", code, stderr, stdout)
	}
	envelope := decodeEnvelope(t, stdout)
	if envelope.Error == nil || envelope.Error.Code != "PRIVACY_KEY_UNAVAILABLE" {
		t.Fatalf("expected PRIVACY_KEY_UNAVAILABLE, got %+v", envelope.Error)
	}
}

func TestPrivacyMissingKeyShowFails(t *testing.T) {
	privacyKeys(t)
	root := privacyFixture(t)
	db := privacyDB(t)
	privacyIngest(t, root, db)

	os.Unsetenv(privacyOpsKey)
	code, _, _ := privacyShow(t, root, db, "secret", "--decrypt")
	if code != 3 {
		t.Fatalf("expected exit 3 for missing key, got %d", code)
	}
}

func TestPrivacyAuditVerifyKeys(t *testing.T) {
	privacyKeys(t)
	root := privacyFixture(t)
	db := privacyDB(t)
	privacyIngest(t, root, db)

	os.Unsetenv(privacyOpsKey)
	code, stdout, stderr := invokeRaw(t, "audit", root, "--db", db, "--verify-keys", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("audit exit=%d stderr=%q", code, stderr)
	}
	envelope := decodeEnvelope(t, stdout)
	if !envelope.OK {
		t.Fatalf("audit failed: %+v", envelope.Error)
	}
	var report bddAuditReport
	if err := json.Unmarshal(envelope.Result, &report); err != nil {
		t.Fatalf("decode audit: %v", err)
	}
	if !hasFinding(report, "KEY_UNAVAILABLE") {
		t.Fatalf("expected KEY_UNAVAILABLE finding, got %v", findingCodes(report))
	}
	found := false
	for _, finding := range report.Findings {
		if finding.Code == "KEY_UNAVAILABLE" && finding.Concept == "secret" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected KEY_UNAVAILABLE for the secret concept")
	}
}

func TestPrivacySyncRemovalRestoresPlaintext(t *testing.T) {
	privacyKeys(t)
	root := privacyFixture(t)
	db := privacyDB(t)
	privacyIngest(t, root, db)

	stored := privacyLoadBundle(t, db, root)
	if body := privacyBody(stored, "secret"); !strings.HasPrefix(body, "enc:v1:") {
		t.Fatalf("expected ciphertext before removal, got %q", body)
	}

	writePrivacyFile(t, filepath.Join(root, "secret.md"), `---
type: Concept
title: "Operations Secret"
keywords:
  - "secret"
  - "runbook"
---

# Operations Secret

The launch code is zanzibar-secret.
`)

	code, stdout, stderr := invokeRaw(t, "sync", root, "--db", db, "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("sync exit=%d stderr=%q", code, stderr)
	}
	if envelope := decodeEnvelope(t, stdout); !envelope.OK {
		t.Fatalf("sync failed: %+v", envelope.Error)
	}

	stored = privacyLoadBundle(t, db, root)
	body := privacyBody(stored, "secret")
	if strings.HasPrefix(body, "enc:v1:") || !strings.Contains(body, "zanzibar-secret") {
		t.Fatalf("expected plaintext after directive removal, got %q", body)
	}
	response := privacySearch(t, root, db, "zanzibar", "fuzzy")
	if !hasFuzzyHit(response, "secret") {
		t.Fatal("plaintext document must be searchable after directive removal")
	}
}

func TestPrivacyMCPRedactsUnlessSensitive(t *testing.T) {
	privacyKeys(t)
	root := privacyFixture(t)
	db := privacyDB(t)
	privacyIngest(t, root, db)

	executor := operations.New("test", engine.Options{})
	server, err := mcp.New(executor, mcp.Options{Bundle: root, DB: db})
	if err != nil {
		t.Fatalf("mcp server: %v", err)
	}
	value, err := server.Call(context.Background(), "weave_show", map[string]any{"id": "secret"})
	if err != nil {
		t.Fatalf("mcp show: %v", err)
	}
	redacted, ok := value.(*contractv1.ShowResult)
	if !ok {
		t.Fatalf("unexpected show payload: %T", value)
	}
	if redacted.Body != "" {
		t.Fatalf("expected redacted MCP body, got %q", redacted.Body)
	}

	sensitive, err := mcp.New(executor, mcp.Options{Bundle: root, DB: db, AllowSensitive: true})
	if err != nil {
		t.Fatalf("mcp sensitive server: %v", err)
	}
	value, err = sensitive.Call(context.Background(), "weave_show", map[string]any{"id": "secret"})
	if err != nil {
		t.Fatalf("mcp sensitive show: %v", err)
	}
	decrypted, ok := value.(*contractv1.ShowResult)
	if !ok {
		t.Fatalf("unexpected show payload: %T", value)
	}
	if !strings.Contains(decrypted.Body, "zanzibar-secret") {
		t.Fatalf("expected decrypted MCP body, got %q", decrypted.Body)
	}
}

func writePrivacyFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func slicesContains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
