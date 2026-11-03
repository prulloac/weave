package extensions_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/prulloac/weave/engine/extensions"
	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

func compileExtensionIPC(t *testing.T) *jsonschema.Schema {
	t.Helper()

	compiler := jsonschema.NewCompiler()
	for _, name := range []string{"defs.schema.json", "extension-ipc.schema.json"} {
		data, err := os.ReadFile(filepath.Join("..", "..", "contract", "v1", "schema", name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		var root map[string]any
		if err := json.Unmarshal(data, &root); err != nil {
			t.Fatalf("decode %s: %v", name, err)
		}
		id, _ := root["$id"].(string)
		if id == "" {
			t.Fatalf("%s: missing $id", name)
		}
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		if err := compiler.AddResource(id, doc); err != nil {
			t.Fatalf("add %s: %v", name, err)
		}
	}
	schema, err := compiler.Compile("https://weave.dev/contract/v1/extension-ipc.schema.json")
	if err != nil {
		t.Fatalf("compile extension-ipc: %v", err)
	}
	return schema
}

func validateExtensionIPC(t *testing.T, schema *jsonschema.Schema, value any) {
	t.Helper()

	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal %T: %v", value, err)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode %T: %v", value, err)
	}
	if err := schema.Validate(doc); err != nil {
		t.Fatalf("%T does not match extension-ipc schema: %v", value, err)
	}
}

func TestExtensionIPCRequestShapes(t *testing.T) {
	schema := compileExtensionIPC(t)

	validateExtensionIPC(t, schema, extensions.EnrichmentRequest{
		V:           extensions.ProtocolVersion,
		Type:        "enrichment_request",
		DocID:       "docs/adr-004.md",
		DocPath:     "docs/adr-004.md",
		BundleRoot:  "/home/user/notes",
		Workspace:   "/tmp/weave-extension-12345",
		Title:       "Database Storage Selection",
		Frontmatter: map[string]any{"type": "adr", "keywords": []any{"database", "storage"}},
		Body:        "We decided to adopt pure-Go SQLite...",
	})
	validateExtensionIPC(t, schema, extensions.RerankRequest{
		V:     extensions.ProtocolVersion,
		Type:  "rerank_request",
		Query: "zero cgo database",
		Candidates: []extensions.Candidate{
			{ID: "docs/adr-004.md", KeywordScore: 0.82, LexicalScore: 0.82, FuzzyScore: 0.45},
			{ID: "docs/adr-001.md", KeywordScore: 0.31, LexicalScore: 0.31, FuzzyScore: 0.1},
		},
	})
}
