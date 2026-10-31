package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
)

func aiDoc(t *testing.T, root string, aiSend bool) {
	t.Helper()
	ai := ""
	if aiSend {
		ai = "  ai:\n    send: true\n"
	}
	content := `---
type: Concept
title: "Secret"
keywords:
  - "alpha"
privacy:
  policy: tokenize
  provider: hmac-sha256
  key_id: tenant-salt
` + ai + `---

# Secret

Account internal-client-alpha owns the workspace.
`
	if err := os.WriteFile(filepath.Join(root, "secret.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func aiKeywordHit(t *testing.T, root, db, query string) bool {
	t.Helper()
	code, stdout, stderr := invokeRaw(t, "search", root, query, "--db", db, "--mode", "keyword", "--json")
	if code == 1 && strings.Contains(stderr, "no matches") {
		return false
	}
	if code != 0 {
		t.Fatalf("search exit=%d stderr=%q", code, stderr)
	}
	var response contractv1.SearchResponse
	if err := json.Unmarshal(decodeEnvelope(t, stdout).Result, &response); err != nil {
		t.Fatal(err)
	}
	return len(response.Keyword) > 0
}

func TestAIEnricherReceivesRedactedBody(t *testing.T) {
	t.Setenv("WEAVE_KEY_TENANT_SALT", strings.Repeat("cd", 32))
	root := t.TempDir()
	aiDoc(t, root, false)
	installExtension(t, root, "leak", "leak.sh")
	db := filepath.Join(t.TempDir(), "index.db")

	code, stdout, stderr := invokeRaw(t, "ingest", root, "--db", db, "--strict", "--with-enricher", "leak", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("ingest exit=%d stderr=%q", code, stderr)
	}
	if envelope := decodeEnvelope(t, stdout); !envelope.OK {
		t.Fatalf("ingest failed: %+v", envelope.Error)
	}
	if aiKeywordHit(t, root, db, "leaked") {
		t.Fatal("enricher must not see encrypted plaintext")
	}
	if !aiKeywordHit(t, root, db, "safe") {
		code, stdout, _ := invokeRaw(t, "show", root, "secret", "--db", db, "--json")
		if code != 0 {
			t.Fatalf("show exit=%d", code)
		}
		var view contractv1.ShowResult
		_ = json.Unmarshal(decodeEnvelope(t, stdout).Result, &view)
		t.Fatalf("enricher must receive a redacted body; keywords=%v", view.Keywords)
	}
}

func TestAIEnricherOptInUnredacts(t *testing.T) {
	t.Setenv("WEAVE_KEY_TENANT_SALT", strings.Repeat("cd", 32))
	root := t.TempDir()
	aiDoc(t, root, true)
	installExtension(t, root, "leak", "leak.sh")
	db := filepath.Join(t.TempDir(), "index.db")

	code, stdout, stderr := invokeRaw(t, "ingest", root, "--db", db, "--strict", "--with-enricher", "leak", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("ingest exit=%d stderr=%q", code, stderr)
	}
	if envelope := decodeEnvelope(t, stdout); !envelope.OK {
		t.Fatalf("ingest failed: %+v", envelope.Error)
	}
	if !aiKeywordHit(t, root, db, "leaked") {
		t.Fatal("opt-in document must reach the enricher")
	}
}
