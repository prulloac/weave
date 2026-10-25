package contractv1

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

type goldenManifest struct {
	Contract string       `json:"contract"`
	Cases    []goldenCase `json:"cases"`
}

type goldenCase struct {
	Name   string         `json:"name"`
	Bundle string         `json:"bundle"`
	Search []goldenSearch `json:"search"`
	Nav    []goldenNav    `json:"nav"`
}

type goldenSearch struct {
	Name  string `json:"name"`
	Query string `json:"query"`
}

type goldenNav struct {
	Name    string `json:"name"`
	Command string `json:"command"`
}

type expectedFile struct {
	name   string
	schema string
}

var schemaFiles = map[string]string{
	"bundle":           "bundle.schema.json",
	"graph":            "graph.schema.json",
	"serialized-graph": "serialized-graph.schema.json",
	"search":           "search.schema.json",
	"find":             "find-results.schema.json",
	"list":             "list-results.schema.json",
	"show":             "show.schema.json",
	"backlinks":        "backlinks.schema.json",
	"path":             "path.schema.json",
}

var navSchema = map[string]string{
	"find":      "find",
	"list":      "list",
	"show":      "show",
	"backlinks": "backlinks",
	"path":      "path",
}

func expectedFiles(golden goldenCase) []expectedFile {
	files := []expectedFile{
		{name: golden.Name + ".bundle.json", schema: "bundle"},
		{name: golden.Name + ".graph.json", schema: "graph"},
		{name: golden.Name + ".serialized-graph.json", schema: "serialized-graph"},
	}
	for _, searchCase := range golden.Search {
		files = append(files, expectedFile{
			name:   golden.Name + ".search." + searchCase.Name + ".json",
			schema: "search",
		})
	}
	for _, navCase := range golden.Nav {
		files = append(files, expectedFile{
			name:   golden.Name + ".nav." + navCase.Name + ".json",
			schema: navSchema[navCase.Command],
		})
	}
	return files
}

func readJSON(t *testing.T, path string) any {
	t.Helper()

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer file.Close()

	doc, err := jsonschema.UnmarshalJSON(file)
	if err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return doc
}

func schemaID(t *testing.T, doc any) string {
	t.Helper()

	object, ok := doc.(map[string]any)
	if !ok {
		t.Fatal("schema is not an object")
	}
	id, ok := object["$id"].(string)
	if !ok || id == "" {
		t.Fatal("schema is missing $id")
	}
	return id
}

func newCompiler(t *testing.T) *jsonschema.Compiler {
	t.Helper()

	entries, err := os.ReadDir("schema")
	if err != nil {
		t.Fatalf("read schema dir: %v", err)
	}

	compiler := jsonschema.NewCompiler()
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		doc := readJSON(t, filepath.Join("schema", entry.Name()))
		if err := compiler.AddResource(schemaID(t, doc), doc); err != nil {
			t.Fatalf("add %s: %v", entry.Name(), err)
		}
	}
	return compiler
}

func compileSchema(t *testing.T, compiler *jsonschema.Compiler, name string) *jsonschema.Schema {
	t.Helper()

	schema, err := compiler.Compile("https://weave.dev/contract/v1/" + name)
	if err != nil {
		t.Fatalf("compile %s: %v", name, err)
	}
	return schema
}

func validate(t *testing.T, compiler *jsonschema.Compiler, schemaName string, doc any) {
	t.Helper()

	if err := compileSchema(t, compiler, schemaName).Validate(doc); err != nil {
		t.Fatalf("%s validation failed: %v", schemaName, err)
	}
}

func loadManifest(t *testing.T) goldenManifest {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("golden", "manifest.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}

	var manifest goldenManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	return manifest
}

func TestSchemasCompile(t *testing.T) {
	compiler := newCompiler(t)

	entries, err := os.ReadDir("schema")
	if err != nil {
		t.Fatalf("read schema dir: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		doc := readJSON(t, filepath.Join("schema", entry.Name()))
		if _, err := compiler.Compile(schemaID(t, doc)); err != nil {
			t.Errorf("compile %s: %v", entry.Name(), err)
		}
	}
}

func TestGoldenCorpusValidates(t *testing.T) {
	compiler := newCompiler(t)
	manifest := loadManifest(t)

	if manifest.Contract != "weave.contract/v1" {
		t.Fatalf("unexpected contract version: %s", manifest.Contract)
	}

	for _, golden := range manifest.Cases {
		for _, file := range expectedFiles(golden) {
			file := file
			t.Run(golden.Name+"/"+file.name, func(t *testing.T) {
				doc := readJSON(t, filepath.Join("golden", "expected", file.name))
				validate(t, compiler, schemaFiles[file.schema], doc)
			})
		}
	}
}

func TestGoldenCorpusRoundTripsThroughStructs(t *testing.T) {
	compiler := newCompiler(t)
	manifest := loadManifest(t)

	targets := map[string]func() any{
		"bundle":           func() any { return new(Bundle) },
		"graph":            func() any { return new(Graph) },
		"serialized-graph": func() any { return new(SerializedGraph) },
		"search":           func() any { return new(SearchResults) },
		"find":             func() any { return new(FindResults) },
		"list":             func() any { return new(ListResults) },
		"show":             func() any { return new(ShowResult) },
		"backlinks":        func() any { return new(BacklinksResult) },
		"path":             func() any { return new(PathResult) },
	}

	for _, golden := range manifest.Cases {
		for _, file := range expectedFiles(golden) {
			file := file
			t.Run(golden.Name+"/"+file.name, func(t *testing.T) {
				newTarget, ok := targets[file.schema]
				if !ok {
					t.Fatalf("no struct target for schema %s", file.schema)
				}

				data, err := os.ReadFile(filepath.Join("golden", "expected", file.name))
				if err != nil {
					t.Fatalf("read golden: %v", err)
				}

				target := newTarget()
				if err := json.Unmarshal(data, target); err != nil {
					t.Fatalf("unmarshal %s: %v", file.name, err)
				}

				encoded, err := json.Marshal(target)
				if err != nil {
					t.Fatalf("marshal %s: %v", file.name, err)
				}

				var doc any
				if err := json.Unmarshal(encoded, &doc); err != nil {
					t.Fatalf("decode round-trip: %v", err)
				}
				validate(t, compiler, schemaFiles[file.schema], doc)
			})
		}
	}
}

func TestContractTypesValidate(t *testing.T) {
	compiler := newCompiler(t)

	title := "Example"
	version := OkfVersion02
	status := StatusStable
	score := 1.5
	snippet := "snippet"
	raw := "[Link](b.md)"
	resolved := "b.md"
	indexDescription := "index description"

	bundle := Bundle{
		Root:       "/bundle",
		OkfVersion: &version,
		Concepts: []OkfConcept{{
			ID:          "a",
			Path:        "a.md",
			Frontmatter: map[string]any{"type": "Concept"},
			Type:        "Concept",
			Title:       &title,
			Status:      &status,
			Body:        "# A",
			Links: []OkfLink{{
				Raw:              raw,
				Target:           "b.md",
				Resolved:         &resolved,
				ResolvesInBundle: true,
			}},
			Tags: []string{"tag"},
		}},
		Index: []IndexEntry{{Title: "A", Target: "a.md", Description: &indexDescription}},
		Validation: Validation{
			Valid: true,
			Concepts: []ConceptValidation{{
				ID:             "a",
				HasFrontmatter: true,
				HasType:        true,
				Warnings:       []string{},
			}},
			Warnings: []string{},
		},
	}
	marshalValidate(t, compiler, "bundle.schema.json", bundle)

	graph := Graph{
		Nodes: []GraphNode{{
			ID: "a", Path: "a.md", Type: "Concept", Title: "A",
			Status: &status, Tags: []string{"tag"}, InDegree: 1, OutDegree: 1,
		}},
		Edges:    []GraphEdge{{From: "a", To: "b", Source: "link", Raw: &raw}},
		Dangling: map[string][]OkfLink{"a": {}},
	}
	marshalValidate(t, compiler, "graph.schema.json", graph)

	serialized := SerializedGraph{
		Nodes: []SerializedGraphNode{{
			ID: "a", Title: "A", Type: "Concept", Status: &status,
			Tags: []string{"tag"}, InDegree: 1, OutDegree: 1,
		}},
		Edges: []SerializedGraphEdge{{From: "a", To: "b", Source: "link"}},
	}
	marshalValidate(t, compiler, "serialized-graph.schema.json", serialized)

	searchResults := SearchResults{{
		ID: "a", Title: "A", Path: "a.md", Type: "Concept",
		Score: score, BestField: FieldTitle, Snippet: &snippet,
	}}
	marshalValidate(t, compiler, "search.schema.json", searchResults)

	findResults := FindResults{{
		ID: "a", Title: "A", Path: "a.md", Type: "Concept", Score: score, Snippet: &snippet,
	}}
	marshalValidate(t, compiler, "find-results.schema.json", findResults)

	listResults := ListResults{{
		ID: "a", Title: "A", Path: "a.md", Type: "Concept", Status: &status, Tags: []string{"tag"},
	}}
	marshalValidate(t, compiler, "list-results.schema.json", listResults)

	show := ShowResult{
		ID: "a", Path: "a.md", Type: "Concept", Title: &title, Status: &status,
		Body: "# A", Backlinks: []string{"b"},
		Links: []ShowLink{{Raw: raw, Target: "b.md", ResolvesInBundle: true}},
	}
	marshalValidate(t, compiler, "show.schema.json", show)

	backlinks := BacklinksResult{ID: "a", Backlinks: []Backlink{{From: "b", Title: "B"}}}
	marshalValidate(t, compiler, "backlinks.schema.json", backlinks)

	path := PathResult{From: "a", To: "b", Path: []string{"a", "b"}}
	marshalValidate(t, compiler, "path.schema.json", path)

	disconnected := PathResult{From: "a", To: "b", Path: nil}
	marshalValidate(t, compiler, "path.schema.json", disconnected)

	success := SuccessEnvelope{
		Contract: "weave.contract/v1",
		CLI:      "weave.cli/v1",
		OK:       true,
		Result:   map[string]any{"value": 1},
	}
	marshalValidate(t, compiler, "envelope.schema.json", success)

	failure := FailureEnvelope{
		Contract: "weave.contract/v1",
		CLI:      "weave.cli/v1",
		OK:       false,
		Error: OperationError{
			Code:    ErrConceptNotFound,
			Message: "concept not found: missing",
			Details: map[string]any{"target": "missing"},
		},
	}
	marshalValidate(t, compiler, "envelope.schema.json", failure)

	contractInfo := ContractInfo{
		Contract:      "weave.contract/v1",
		CLI:           "weave.cli/v1",
		BinaryVersion: "0.1.0",
		Operations: []OperationDescriptor{{
			Op: OpFind, Fields: []string{"path", "query"}, Result: "FindResults",
		}},
		Errors: []ErrorCode{ErrInvalidRequest, ErrConceptNotFound},
	}
	marshalValidate(t, compiler, "cli.schema.json", contractInfo)

	request := OperationRequest{
		Op: OpFind, Path: "/bundle", Query: "graph", Limit: intPtr(10),
		Fields: []SearchField{FieldTitle, FieldBody},
	}
	marshalValidate(t, compiler, "cli.schema.json", request)
}

func marshalValidate(t *testing.T, compiler *jsonschema.Compiler, schemaName string, value any) {
	t.Helper()

	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal %T: %v", value, err)
	}
	var doc any
	if err := json.Unmarshal(encoded, &doc); err != nil {
		t.Fatalf("decode %T: %v", value, err)
	}
	validate(t, compiler, schemaName, doc)
}

func intPtr(value int) *int {
	return &value
}
