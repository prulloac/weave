package mcp_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine"
	"github.com/prulloac/weave/engine/okf"
	"github.com/prulloac/weave/engine/operations"
	"github.com/prulloac/weave/mcp"
	sqlitestore "github.com/prulloac/weave/storage/sqlite"
)

func sensitiveFixture(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	write := func(name, frontmatter, body string) {
		content := "---\ntype: Concept\ntitle: \"" + name + "\"\nkeywords:\n  - \"" + name + "\"\n" + frontmatter + "---\n\n" + body + "\n"
		if err := os.WriteFile(filepath.Join(root, name+".md"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("secret", "privacy:\n  policy: encrypt\n  provider: aes-256-gcm\n  key_id: ops-prod-key\n", "zanzibar-secret")
	write("tenant", "privacy:\n  policy: tokenize\n  provider: hmac-sha256\n  key_id: tenant-salt\n", "internal-client-alpha")
	write("plain", "", "nothing protected")
	sensitiveKeys(t)
	db := filepath.Join(t.TempDir(), "index.db")
	store, err := sqlitestore.Open(db)
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := okf.ParseBundle(context.Background(), root, okf.Options{})
	if err != nil {
		store.Close()
		t.Fatal(err)
	}
	if _, err := store.Ingest(context.Background(), bundle, sqlitestore.IngestOptions{Mode: "strict"}); err != nil {
		store.Close()
		t.Fatal(err)
	}
	store.Close()
	return root, db
}

func sensitiveKeys(t *testing.T) {
	t.Helper()
	t.Setenv("WEAVE_KEY_OPS_PROD_KEY", strings.Repeat("ab", 32))
	t.Setenv("WEAVE_KEY_TENANT_SALT", strings.Repeat("cd", 32))
}

func sensitiveServer(t *testing.T, root, db string, allow bool) *mcp.Server {
	t.Helper()
	server, err := mcp.New(operations.New("test", engine.Options{}), mcp.Options{
		Bundle:         root,
		DB:             db,
		AllowSensitive: allow,
	})
	if err != nil {
		t.Fatalf("server: %v", err)
	}
	return server
}

func sensitiveShow(t *testing.T, server *mcp.Server, id string) *contractv1.ShowResult {
	t.Helper()
	value, err := server.Call(context.Background(), "weave_show", map[string]any{"id": id})
	if err != nil {
		t.Fatalf("show %s: %v", id, err)
	}
	view, ok := value.(*contractv1.ShowResult)
	if !ok {
		t.Fatalf("unexpected payload: %T", value)
	}
	return view
}

func TestMCPRedactedDefault(t *testing.T) {
	sensitiveKeys(t)
	root, db := sensitiveFixture(t)
	server := sensitiveServer(t, root, db, false)

	view := sensitiveShow(t, server, "secret")
	if view.Body != "" {
		t.Fatalf("encrypted body must be redacted, got %q", view.Body)
	}
	if view.Privacy == nil || !view.Privacy.Redacted {
		t.Fatalf("unexpected privacy state: %+v", view.Privacy)
	}

	content, err := server.ReadResource(context.Background(), "weave://concept/secret")
	if err != nil {
		t.Fatalf("resource: %v", err)
	}
	if strings.Contains(content.Text, "zanzibar-secret") {
		t.Fatalf("redacted resource leaked plaintext: %s", content.Text)
	}

	value, err := server.Call(context.Background(), "weave_list", map[string]any{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	list := value.([]contractv1.ListResult)
	if len(list) != 3 {
		t.Fatalf("expected metadata for all documents, got %+v", list)
	}
}

func TestMCPAllowSensitiveDecryptsToolsAndResources(t *testing.T) {
	sensitiveKeys(t)
	root, db := sensitiveFixture(t)
	server := sensitiveServer(t, root, db, true)

	view := sensitiveShow(t, server, "secret")
	if !strings.Contains(view.Body, "zanzibar-secret") {
		t.Fatalf("expected plaintext, got %q", view.Body)
	}
	if view.Privacy == nil || !view.Privacy.Decrypted {
		t.Fatalf("expected decrypted state: %+v", view.Privacy)
	}

	content, err := server.ReadResource(context.Background(), "weave://concept/secret")
	if err != nil {
		t.Fatalf("resource: %v", err)
	}
	if !strings.Contains(content.Text, "zanzibar-secret") {
		t.Fatalf("expected decrypted resource: %s", content.Text)
	}
}

func TestMCPAllowSensitiveKeepsSearchExclusion(t *testing.T) {
	sensitiveKeys(t)
	root, db := sensitiveFixture(t)
	for _, allow := range []bool{false, true} {
		server := sensitiveServer(t, root, db, allow)
		value, err := server.Call(context.Background(), "weave_search", map[string]any{
			"query": "secret", "mode": "keyword",
		})
		if err != nil {
			t.Fatalf("search: %v", err)
		}
		response := value.(contractv1.SearchResponse)
		for _, hit := range response.Keyword {
			if hit.ID == "secret" {
				t.Fatalf("encrypted document leaked into search (allow=%t)", allow)
			}
		}
	}
}

func TestMCPTokenizedStaysIrreversible(t *testing.T) {
	sensitiveKeys(t)
	root, db := sensitiveFixture(t)
	for _, allow := range []bool{false, true} {
		server := sensitiveServer(t, root, db, allow)
		view := sensitiveShow(t, server, "tenant")
		if strings.Contains(view.Body, "internal-client-alpha") {
			t.Fatalf("tokenized body leaked plaintext (allow=%t): %q", allow, view.Body)
		}
		if !strings.Contains(view.Body, "tok") {
			t.Fatalf("expected aliases (allow=%t): %q", allow, view.Body)
		}
	}
}

func TestMCPInMemoryTokenizedBodiesAreRedacted(t *testing.T) {
	sensitiveKeys(t)
	root, _ := sensitiveFixture(t)
	for _, allow := range []bool{false, true} {
		server, err := mcp.New(operations.New("test", engine.Options{}), mcp.Options{
			Bundle:         root,
			AllowSensitive: allow,
		})
		if err != nil {
			t.Fatal(err)
		}
		view := sensitiveShow(t, server, "tenant")
		if view.Body != "" {
			t.Fatalf("in-memory tokenized body must be redacted (allow=%t): %q", allow, view.Body)
		}
		if view.Privacy == nil || !view.Privacy.Redacted {
			t.Fatalf("expected redacted state (allow=%t): %+v", allow, view.Privacy)
		}
	}
}

func TestMCPMissingKeyFailsClosed(t *testing.T) {
	root, db := sensitiveFixture(t)
	t.Setenv("WEAVE_KEY_TENANT_SALT", strings.Repeat("cd", 32))
	os.Unsetenv("WEAVE_KEY_OPS_PROD_KEY")
	server := sensitiveServer(t, root, db, true)

	_, err := server.Call(context.Background(), "weave_show", map[string]any{"id": "secret"})
	if err == nil {
		t.Fatal("expected missing key error")
	}
	mcpErr, ok := err.(*mcp.Error)
	if !ok || mcpErr.Code != contractv1.ErrPrivacyKey {
		t.Fatalf("expected PRIVACY_KEY_UNAVAILABLE, got %v", err)
	}
	if strings.Contains(err.Error(), "zanzibar") {
		t.Fatalf("error leaked content: %v", err)
	}

	if _, err := server.ReadResource(context.Background(), "weave://concept/secret"); err == nil {
		t.Fatal("expected resource error for missing key")
	}
}

func TestMCPAuditVerifyKeys(t *testing.T) {
	root, db := sensitiveFixture(t)
	t.Setenv("WEAVE_KEY_TENANT_SALT", strings.Repeat("cd", 32))
	os.Unsetenv("WEAVE_KEY_OPS_PROD_KEY")
	server := sensitiveServer(t, root, db, false)

	value, err := server.Call(context.Background(), "weave_audit", map[string]any{"fail_on": "none"})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	report := value.(contractv1.AuditReport)
	for _, finding := range report.Findings {
		if finding.Code == "KEY_UNAVAILABLE" {
			t.Fatal("key findings must be opt-in")
		}
	}

	value, err = server.Call(context.Background(), "weave_audit", map[string]any{
		"fail_on": "none", "verify_keys": true,
	})
	if err != nil {
		t.Fatalf("audit verify: %v", err)
	}
	report = value.(contractv1.AuditReport)
	found := false
	for _, finding := range report.Findings {
		if finding.Code == "KEY_UNAVAILABLE" && finding.Concept == "secret" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected KEY_UNAVAILABLE for secret, got %+v", report.Findings)
	}
}
