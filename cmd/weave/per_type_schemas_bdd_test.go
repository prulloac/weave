package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
	sqlitestore "github.com/prulloac/weave/storage/sqlite"
)

// Behavior-level tests for .weave/types schemas. See docs/bdd/per-type-schemas.md.

func writeTypeSchema(t *testing.T, root, name, body string) {
	t.Helper()
	dir := filepath.Join(root, ".weave", "types")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeTypeDoc(t *testing.T, root, name, frontmatter string) {
	t.Helper()
	content := "---\n" + frontmatter + "---\n\n# Doc\n"
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func typeIngest(t *testing.T, root string, extra ...string) (int, string, string, string) {
	t.Helper()
	db := filepath.Join(t.TempDir(), "index.db")
	args := append([]string{"ingest", root, "--db", db, "--json"}, extra...)
	code, stdout, stderr := Invoke(t, args...)
	return code, stdout, stderr, db
}

func TestNoTypeSchemasUnchanged(t *testing.T) {
	root := t.TempDir()
	writeTypeDoc(t, root, "a.md", "type: Concept\nkeywords:\n  - alpha\n")
	code, _, stderr, _ := typeIngest(t, root, "--strict")
	if code != 0 {
		t.Fatalf("ingest failed: %s", stderr)
	}
	if strings.Contains(stderr, "type schema") {
		t.Fatalf("unexpected schema warning: %s", stderr)
	}
}

func TestTypeSchemaValidDocument(t *testing.T) {
	root := t.TempDir()
	writeTypeSchema(t, root, "adr.schema.json", `{"type":"object","required":["status"]}`)
	writeTypeDoc(t, root, "a.md", "type: adr\nkeywords:\n  - adr\nstatus: accepted\n")
	code, _, stderr, _ := typeIngest(t, root, "--strict")
	if code != 0 {
		t.Fatalf("ingest failed: %s", stderr)
	}
}

func TestTypeSchemaStrictFailure(t *testing.T) {
	root := t.TempDir()
	writeTypeSchema(t, root, "adr.schema.json", `{"type":"object","required":["status"]}`)
	writeTypeDoc(t, root, "a.md", "type: adr\nkeywords:\n  - adr\n")
	code, stdout, _, _ := typeIngest(t, root, "--strict")
	if code != 1 {
		t.Fatalf("expected exit 1, got %d stdout=%s", code, stdout)
	}
	envelope := decodeEnvelope(t, stdout)
	if envelope.Error == nil || envelope.Error.Code != contractv1.ErrTypeSchemaInvalid {
		t.Fatalf("expected TYPE_SCHEMA_INVALID, got %+v", envelope.Error)
	}
	if !strings.Contains(envelope.Error.Message, "a.md") {
		t.Fatalf("violation does not name the document: %s", envelope.Error.Message)
	}
}

func TestTypeSchemaNoPartialWrite(t *testing.T) {
	root := t.TempDir()
	writeTypeSchema(t, root, "adr.schema.json", `{"type":"object","required":["status"]}`)
	writeTypeDoc(t, root, "a.md", "type: adr\nkeywords:\n  - adr\n")
	_, _, _, db := typeIngest(t, root, "--strict")
	store, err := sqlitestore.OpenExisting(db)
	if err != nil {
		return // no database was created
	}
	defer store.Close()
	if has, err := store.HasBundle(context.Background(), root); err != nil || has {
		t.Fatalf("partial index written: has=%v err=%v", has, err)
	}
}

func TestTypeSchemaPermissive(t *testing.T) {
	root := t.TempDir()
	writeTypeSchema(t, root, "adr.schema.json", `{"type":"object","required":["status"]}`)
	writeTypeDoc(t, root, "a.md", "type: adr\nkeywords:\n  - adr\n")
	code, _, stderr, db := typeIngest(t, root, "--permissive")
	if code != 0 {
		t.Fatalf("ingest failed: %s", stderr)
	}
	if !strings.Contains(stderr, "type schema") || !strings.Contains(stderr, "a.md") {
		t.Fatalf("missing schema warning: %s", stderr)
	}
	store, err := sqlitestore.OpenExisting(db)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if has, err := store.HasBundle(context.Background(), root); err != nil || !has {
		t.Fatalf("document was not indexed: has=%v err=%v", has, err)
	}
}

func TestTypeSchemaTypeIsolation(t *testing.T) {
	root := t.TempDir()
	writeTypeSchema(t, root, "adr.schema.json", `{"type":"object","required":["status"]}`)
	writeTypeDoc(t, root, "a.md", "type: adr\nkeywords:\n  - adr\nstatus: accepted\n")
	writeTypeDoc(t, root, "r.md", "type: runbook\nkeywords:\n  - ops\n")
	code, _, stderr, _ := typeIngest(t, root, "--strict")
	if code != 0 {
		t.Fatalf("runbook was validated: %s", stderr)
	}
}

func TestTypeSchemaAdditionalProperties(t *testing.T) {
	root := t.TempDir()
	writeTypeSchema(t, root, "adr.schema.json", `{"type":"object","properties":{"status":{"type":"string"}},"additionalProperties":false}`)
	writeTypeDoc(t, root, "a.md", "type: adr\nkeywords:\n  - adr\nstatus: accepted\nowner: team\n")
	code, _, _, _ := typeIngest(t, root, "--strict")
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
}

func TestTypeSchemaLocalRef(t *testing.T) {
	root := t.TempDir()
	writeTypeSchema(t, root, "common.schema.json", `{"type":"object","required":["status"]}`)
	writeTypeSchema(t, root, "adr.schema.json", `{"$ref":"common.schema.json"}`)
	writeTypeDoc(t, root, "a.md", "type: adr\nkeywords:\n  - adr\nstatus: accepted\n")
	code, _, stderr, _ := typeIngest(t, root, "--strict")
	if code != 0 {
		t.Fatalf("local ref failed: %s", stderr)
	}
}

func TestTypeSchemaRemoteRef(t *testing.T) {
	root := t.TempDir()
	writeTypeSchema(t, root, "adr.schema.json", `{"$ref":"https://example.com/schema.json"}`)
	writeTypeDoc(t, root, "a.md", "type: adr\nkeywords:\n  - adr\n")
	code, stdout, _, _ := typeIngest(t, root, "--strict")
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	envelope := decodeEnvelope(t, stdout)
	if envelope.Error == nil || envelope.Error.Code != contractv1.ErrTypeSchemaUnsupported {
		t.Fatalf("expected TYPE_SCHEMA_UNSUPPORTED, got %+v", envelope.Error)
	}
}

func TestTypeSchemaInvalidSchema(t *testing.T) {
	root := t.TempDir()
	writeTypeSchema(t, root, "adr.schema.json", `{not json`)
	writeTypeDoc(t, root, "a.md", "type: adr\nkeywords:\n  - adr\n")
	code, stdout, _, _ := typeIngest(t, root, "--permissive")
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	envelope := decodeEnvelope(t, stdout)
	if envelope.Error == nil || envelope.Error.Code != contractv1.ErrTypeSchemaInvalid {
		t.Fatalf("expected TYPE_SCHEMA_INVALID, got %+v", envelope.Error)
	}
}

func TestTypeSchemaAuditParity(t *testing.T) {
	root := t.TempDir()
	writeTypeSchema(t, root, "adr.schema.json", `{"type":"object","required":["status"]}`)
	writeTypeDoc(t, root, "a.md", "type: adr\nkeywords:\n  - adr\n")
	code, _, report := runAuditJSON(t, root, "--json")
	if code != 1 {
		t.Fatalf("expected audit gate to fail, got %d", code)
	}
	found := false
	for _, finding := range report.Findings {
		if finding.Code == "TYPE_SCHEMA_INVALID" && finding.Concept == "a" {
			found = true
		}
	}
	if !found {
		t.Fatalf("TYPE_SCHEMA_INVALID finding missing: %+v", report.Findings)
	}
}

func TestTypeSchemaAuditGate(t *testing.T) {
	root := t.TempDir()
	writeTypeSchema(t, root, "adr.schema.json", `{"type":"object","required":["status"]}`)
	writeTypeDoc(t, root, "a.md", "type: adr\nkeywords:\n  - adr\n")
	code, _, _ := runAuditJSON(t, root, "--fail-on", "none", "--json")
	if code != 0 {
		t.Fatalf("fail-on none returned %d", code)
	}
}

func TestTypeSchemaBaseProfile(t *testing.T) {
	root := t.TempDir()
	writeTypeSchema(t, root, "adr.schema.json", `{"type":"object"}`)
	writeTypeDoc(t, root, "a.md", "type: adr\n")
	_, _, report := runAuditJSON(t, root, "--json")
	found := false
	for _, finding := range report.Findings {
		if finding.Code == "MISSING_KEYWORDS" {
			found = true
		}
	}
	if !found {
		t.Fatalf("base profile was relaxed: %+v", report.Findings)
	}
}
