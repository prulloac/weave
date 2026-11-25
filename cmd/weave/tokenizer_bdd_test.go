package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
	sqlitestore "github.com/prulloac/weave/storage/sqlite"
)

// Behavior-level tests for the opt-in tokenizer role. They drive the CLI and inspect the persistent
// index through the storage package.

func tokenizerFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	documents := map[string]string{
		"stem.md": "---\ntype: Concept\ntitle: \"Stem\"\nkeywords:\n  - \"mice\"\n---\n\n# Stem\n\nmice content\n",
		"db.md":   "---\ntype: Concept\ntitle: \"Database\"\nkeywords:\n  - \"database\"\n---\n\n# Database\n\ndatabase content\n",
	}
	for name, content := range documents {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return root
}

func tokenizerSearch(t *testing.T, root, db, query string, extra ...string) testEnvelope {
	t.Helper()
	args := append([]string{"search", root, query, "--db", db, "--mode", "keyword"}, extra...)
	args = append(args, "--json")
	code, stdout, stderr := Invoke(t, args...)
	envelope := decodeEnvelope(t, stdout)
	if code != 0 && !strings.Contains(stderr, "no matches") {
		t.Fatalf("search exit=%d stderr=%q stdout=%s", code, stderr, stdout)
	}
	return envelope
}

func tokenizerHitIDs(t *testing.T, envelope testEnvelope) []string {
	t.Helper()
	if !envelope.OK {
		return nil
	}
	var response contractv1.SearchResponse
	if err := json.Unmarshal(envelope.Result, &response); err != nil {
		t.Fatalf("decode search: %v\n%s", err, envelope.Result)
	}
	ids := make([]string, 0, len(response.Keyword))
	for _, hit := range response.Keyword {
		ids = append(ids, hit.ID)
	}
	return ids
}

func tokenizerIdentity(t *testing.T, db, root string) (string, string) {
	t.Helper()
	store, err := sqlitestore.Open(db)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	identity, ok, err := store.TokenizerIdentity(context.Background(), root)
	if err != nil {
		t.Fatalf("tokenizer identity: %v", err)
	}
	if !ok {
		t.Fatal("bundle is not indexed")
	}
	return identity.Name, identity.Version
}

func TestTokenizerDefaultUnchanged(t *testing.T) {
	root := tokenizerFixture(t)
	db := filepath.Join(t.TempDir(), "index.db")
	code, stdout, stderr := Invoke(t, "ingest", root, "--db", db, "--strict", "--json")
	if code != 0 {
		t.Fatalf("ingest exit=%d stderr=%q stdout=%s", code, stderr, stdout)
	}
	if ids := tokenizerHitIDs(t, tokenizerSearch(t, root, db, "mice")); len(ids) != 1 || ids[0] != "stem" {
		t.Fatalf("literal keyword search = %v", ids)
	}
	if ids := tokenizerHitIDs(t, tokenizerSearch(t, root, db, "mouse")); len(ids) != 0 {
		t.Fatalf("built-in tokenizer must not stem, got %v", ids)
	}
	if name, _ := tokenizerIdentity(t, db, root); name != "builtin" {
		t.Fatalf("built-in tokenizer identity = %q, want builtin", name)
	}
}

func TestTokenizerStemming(t *testing.T) {
	root := tokenizerFixture(t)
	db := filepath.Join(t.TempDir(), "index.db")
	installExtension(t, root, "stem", "tokenizer.sh")
	code, stdout, stderr := Invoke(t, "ingest", root, "--db", db, "--strict", "--tokenizer", "stem", "--json")
	if code != 0 {
		t.Fatalf("ingest exit=%d stderr=%q stdout=%s", code, stderr, stdout)
	}
	if ids := tokenizerHitIDs(t, tokenizerSearch(t, root, db, "mouse")); len(ids) != 1 || ids[0] != "stem" {
		t.Fatalf("stemmed keyword search = %v", ids)
	}
	if name, _ := tokenizerIdentity(t, db, root); name != "stem" {
		t.Fatalf("tokenizer identity = %q", name)
	}
}

func TestTokenizerExpansion(t *testing.T) {
	root := tokenizerFixture(t)
	db := filepath.Join(t.TempDir(), "index.db")
	installExtension(t, root, "stem", "tokenizer.sh")
	code, stdout, stderr := Invoke(t, "ingest", root, "--db", db, "--strict", "--tokenizer", "stem", "--json")
	if code != 0 {
		t.Fatalf("ingest exit=%d stderr=%q stdout=%s", code, stderr, stdout)
	}
	if ids := tokenizerHitIDs(t, tokenizerSearch(t, root, db, "db")); len(ids) != 1 || ids[0] != "db" {
		t.Fatalf("expanded keyword search = %v", ids)
	}
	code, stdout, stderr = Invoke(t, "search", root, "db", "--db", db, "--mode", "bm25", "--json")
	if code != 0 {
		t.Fatalf("bm25 search exit=%d stderr=%q stdout=%s", code, stderr, stdout)
	}
	envelope := decodeEnvelope(t, stdout)
	var response contractv1.SearchResponse
	if err := json.Unmarshal(envelope.Result, &response); err != nil {
		t.Fatalf("decode bm25 search: %v", err)
	}
	found := false
	for _, hit := range response.Lexical {
		if hit.ID == "db" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expanded bm25 search missed db: %+v", response.Lexical)
	}
	raw := extensionRaw(t, db)
	defer raw.Close()
	var terms string
	if err := raw.QueryRow("SELECT search_keywords_json FROM documents WHERE bundle_root = ? AND id = ?", root, "db").Scan(&terms); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(terms, `"db"`) {
		t.Fatalf("index terms must not include query expansions: %s", terms)
	}
}

func TestTokenizerChanged(t *testing.T) {
	root := tokenizerFixture(t)
	db := filepath.Join(t.TempDir(), "index.db")
	installExtension(t, root, "stem", "tokenizer.sh")
	installExtension(t, root, "other", "tokenizer.sh")
	code, stdout, stderr := Invoke(t, "ingest", root, "--db", db, "--strict", "--tokenizer", "stem", "--json")
	if code != 0 {
		t.Fatalf("ingest exit=%d stderr=%q stdout=%s", code, stderr, stdout)
	}
	code, stdout, _ = Invoke(t, "search", root, "mouse", "--db", db, "--mode", "keyword", "--tokenizer", "other", "--json")
	if code != 1 {
		t.Fatalf("search exit=%d, want 1\n%s", code, stdout)
	}
	if envelope := decodeEnvelope(t, stdout); envelope.Error == nil || envelope.Error.Code != contractv1.ErrTokenizerChanged {
		t.Fatalf("error = %+v", envelope.Error)
	}
	code, stdout, stderr = Invoke(t, "rebuild", root, "--db", db, "--strict", "--tokenizer", "other", "--json")
	if code != 0 {
		t.Fatalf("rebuild exit=%d stderr=%q stdout=%s", code, stderr, stdout)
	}
	if name, _ := tokenizerIdentity(t, db, root); name != "other" {
		t.Fatalf("tokenizer identity after rebuild = %q", name)
	}
	if ids := tokenizerHitIDs(t, tokenizerSearch(t, root, db, "mouse")); len(ids) != 1 || ids[0] != "stem" {
		t.Fatalf("search after rebuild = %v", ids)
	}
}

func TestTokenizerUsesRecorded(t *testing.T) {
	root := tokenizerFixture(t)
	db := filepath.Join(t.TempDir(), "index.db")
	installExtension(t, root, "stem", "tokenizer.sh")
	code, stdout, stderr := Invoke(t, "ingest", root, "--db", db, "--strict", "--tokenizer", "stem", "--json")
	if code != 0 {
		t.Fatalf("ingest exit=%d stderr=%q stdout=%s", code, stderr, stdout)
	}
	if ids := tokenizerHitIDs(t, tokenizerSearch(t, root, db, "mouse")); len(ids) != 1 || ids[0] != "stem" {
		t.Fatalf("recorded tokenizer search = %v", ids)
	}
}

func TestTokenizerProtocol(t *testing.T) {
	root := tokenizerFixture(t)
	db := filepath.Join(t.TempDir(), "index.db")
	installExtension(t, root, "bad", "tokenizer.sh")
	t.Setenv("TEST_TOKENIZER_INVALID", "1")
	code, stdout, _ := Invoke(t, "ingest", root, "--db", db, "--strict", "--tokenizer", "bad", "--json")
	if code != 1 {
		t.Fatalf("ingest exit=%d, want 1\n%s", code, stdout)
	}
	if envelope := decodeEnvelope(t, stdout); envelope.Error == nil || envelope.Error.Code != contractv1.ErrTokenizerProtocol {
		t.Fatalf("error = %+v", envelope.Error)
	}
	raw := extensionRaw(t, db)
	defer raw.Close()
	var count int
	if err := raw.QueryRow("SELECT COUNT(*) FROM bundles WHERE bundle_root = ?", root).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("partial index committed: %d bundles", count)
	}
}

func TestTokenizerTimeout(t *testing.T) {
	root := tokenizerFixture(t)
	db := filepath.Join(t.TempDir(), "index.db")
	installExtension(t, root, "slow", "tokenizer.sh")
	t.Setenv("TEST_TOKENIZER_SLEEP", "1")
	code, stdout, _ := Invoke(t, "ingest", root, "--db", db, "--strict", "--tokenizer", "slow", "--timeout", "150ms", "--json")
	if code != 1 {
		t.Fatalf("ingest exit=%d, want 1\n%s", code, stdout)
	}
	if envelope := decodeEnvelope(t, stdout); envelope.Error == nil || envelope.Error.Code != contractv1.ErrTokenizerTimeout {
		t.Fatalf("error = %+v", envelope.Error)
	}
}

func TestTokenizerIdentity(t *testing.T) {
	root := tokenizerFixture(t)
	db := filepath.Join(t.TempDir(), "index.db")
	installExtension(t, root, "stem", "tokenizer.sh")
	t.Setenv("TEST_TOKENIZER_VERSION", "0.13.0")
	code, stdout, stderr := Invoke(t, "ingest", root, "--db", db, "--strict", "--tokenizer", "stem", "--json")
	if code != 0 {
		t.Fatalf("ingest exit=%d stderr=%q stdout=%s", code, stderr, stdout)
	}
	if _, version := tokenizerIdentity(t, db, root); version != "0.13.0" {
		t.Fatalf("recorded version = %q", version)
	}
	t.Setenv("TEST_TOKENIZER_VERSION", "0.13.1")
	code, stdout, _ = Invoke(t, "search", root, "mouse", "--db", db, "--mode", "keyword", "--json")
	if code != 1 {
		t.Fatalf("search exit=%d, want 1\n%s", code, stdout)
	}
	if envelope := decodeEnvelope(t, stdout); envelope.Error == nil || envelope.Error.Code != contractv1.ErrTokenizerChanged {
		t.Fatalf("error = %+v", envelope.Error)
	}
}

func TestTokenizerNoDB(t *testing.T) {
	root := tokenizerFixture(t)
	installExtension(t, root, "stem", "tokenizer.sh")
	code, stdout, stderr := Invoke(t, "search", root, "mouse", "--no-db", "--mode", "keyword", "--tokenizer", "stem", "--json")
	if code != 0 {
		t.Fatalf("search exit=%d stderr=%q stdout=%s", code, stderr, stdout)
	}
	if ids := tokenizerHitIDs(t, decodeEnvelope(t, stdout)); len(ids) != 1 || ids[0] != "stem" {
		t.Fatalf("no-db tokenized search = %v", ids)
	}
}

func TestTokenizerNoDBFallback(t *testing.T) {
	root := tokenizerFixture(t)
	installExtension(t, root, "bad", "tokenizer.sh")
	t.Setenv("TEST_TOKENIZER_INVALID", "1")
	code, stdout, stderr := Invoke(t, "search", root, "mice", "--no-db", "--mode", "keyword", "--tokenizer", "bad", "--json")
	if code != 0 {
		t.Fatalf("search exit=%d stderr=%q stdout=%s", code, stderr, stdout)
	}
	if ids := tokenizerHitIDs(t, decodeEnvelope(t, stdout)); len(ids) != 1 || ids[0] != "stem" {
		t.Fatalf("fallback search = %v", ids)
	}
	if !strings.Contains(stderr, "bad") || !strings.Contains(stderr, "built-in tokenizer") {
		t.Fatalf("expected one fallback warning, got %q", stderr)
	}
	if strings.Count(stderr, "built-in tokenizer") != 1 {
		t.Fatalf("expected exactly one fallback warning, got %q", stderr)
	}
}

func TestTokenizerPrivacy(t *testing.T) {
	privacyKeys(t)
	root := privacyFixture(t)
	db := privacyDB(t)
	installExtension(t, root, "stem", "tokenizer.sh")
	marker := filepath.Join(t.TempDir(), "tokenizer-input.log")
	t.Setenv("TEST_TOKENIZER_MARKER", marker)
	code, stdout, stderr := Invoke(t, "ingest", root, "--db", db, "--strict", "--tokenizer", "stem", "--json")
	if code != 0 {
		t.Fatalf("ingest exit=%d stderr=%q stdout=%s", code, stderr, stdout)
	}
	received, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("read tokenizer marker: %v", err)
	}
	if strings.Contains(string(received), "zanzibar-secret") || strings.Contains(string(received), "enc:v1:") {
		t.Fatalf("tokenizer received protected content: %s", received)
	}
}
