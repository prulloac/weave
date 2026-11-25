package typeschema_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine/typeschema"
)

func writeSchema(t *testing.T, root, name, content string) {
	t.Helper()
	dir := filepath.Join(root, ".weave", "types")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func adrSchema(extra string) string {
	return `{"type":"object","required":["status"],"properties":{"status":{"type":"string"}}` + extra + `}`
}

func TestNoSchemas(t *testing.T) {
	set, err := typeschema.Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !set.Empty() {
		t.Fatal("expected an empty set")
	}
	if violations := set.Validate(contractv1.OkfConcept{Type: "adr"}); len(violations) != 0 {
		t.Fatalf("violations = %+v", violations)
	}
}

func TestValidate(t *testing.T) {
	root := t.TempDir()
	writeSchema(t, root, "adr.schema.json", adrSchema(""))
	set, err := typeschema.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	valid := contractv1.OkfConcept{Type: "adr", Path: "a.md", Frontmatter: map[string]any{"status": "accepted"}}
	if violations := set.Validate(valid); len(violations) != 0 {
		t.Fatalf("valid document failed: %+v", violations)
	}
	invalid := contractv1.OkfConcept{Type: "adr", Path: "b.md", Frontmatter: map[string]any{}}
	if violations := set.Validate(invalid); len(violations) != 1 || violations[0].Path != "b.md" {
		t.Fatalf("violations = %+v", violations)
	}
}

func TestTypeIsolation(t *testing.T) {
	root := t.TempDir()
	writeSchema(t, root, "adr.schema.json", adrSchema(""))
	set, err := typeschema.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	runbook := contractv1.OkfConcept{Type: "runbook", Path: "r.md", Frontmatter: map[string]any{"owner": "ops"}}
	if violations := set.Validate(runbook); len(violations) != 0 {
		t.Fatalf("unrelated type was validated: %+v", violations)
	}
}

func TestAdditionalProperties(t *testing.T) {
	root := t.TempDir()
	writeSchema(t, root, "adr.schema.json", adrSchema(`,"additionalProperties":false`))
	set, err := typeschema.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	concept := contractv1.OkfConcept{Type: "adr", Path: "a.md", Frontmatter: map[string]any{"status": "accepted", "extra": true}}
	if violations := set.Validate(concept); len(violations) == 0 {
		t.Fatal("extra property was accepted")
	}
}

func TestLocalRef(t *testing.T) {
	root := t.TempDir()
	writeSchema(t, root, "common.schema.json", `{"type":"object","required":["status"],"properties":{"status":{"type":"string"}}}`)
	writeSchema(t, root, "adr.schema.json", `{"$ref":"common.schema.json"}`)
	set, err := typeschema.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	valid := contractv1.OkfConcept{Type: "adr", Path: "a.md", Frontmatter: map[string]any{"status": "accepted"}}
	if violations := set.Validate(valid); len(violations) != 0 {
		t.Fatalf("local ref failed: %+v", violations)
	}
}

func TestRemoteRef(t *testing.T) {
	root := t.TempDir()
	writeSchema(t, root, "adr.schema.json", `{"$ref":"https://example.com/schema.json"}`)
	if _, err := typeschema.Load(root); !errors.Is(err, typeschema.ErrUnsupported) {
		t.Fatalf("expected ErrUnsupported, got %v", err)
	}
}

func TestInvalidSchema(t *testing.T) {
	root := t.TempDir()
	writeSchema(t, root, "adr.schema.json", `{not json`)
	if _, err := typeschema.Load(root); !errors.Is(err, typeschema.ErrInvalid) {
		t.Fatalf("expected ErrInvalid, got %v", err)
	}
}

func TestInvalidTypeName(t *testing.T) {
	root := t.TempDir()
	writeSchema(t, root, "Bad Name.schema.json", adrSchema(""))
	if _, err := typeschema.Load(root); !errors.Is(err, typeschema.ErrInvalid) {
		t.Fatalf("expected ErrInvalid, got %v", err)
	}
}
