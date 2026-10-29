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
	"bundle":    "bundle.schema.json",
	"graph":     "graph.schema.json",
	"search":    "search.schema.json",
	"list":      "list-results.schema.json",
	"show":      "show.schema.json",
	"backlinks": "backlinks.schema.json",
	"path":      "path.schema.json",
}

var navSchema = map[string]string{
	"list":      "list",
	"show":      "show",
	"backlinks": "backlinks",
	"path":      "path",
}

func expectedFiles(golden goldenCase) []expectedFile {
	files := []expectedFile{
		{name: golden.Name + ".bundle.json", schema: "bundle"},
		{name: golden.Name + ".graph.json", schema: "graph"},
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
		"bundle":    func() any { return new(Bundle) },
		"graph":     func() any { return new(Graph) },
		"search":    func() any { return new(SearchResponse) },
		"list":      func() any { return new(ListResults) },
		"show":      func() any { return new(ShowResult) },
		"backlinks": func() any { return new(BacklinksResult) },
		"path":      func() any { return new(PathResult) },
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
	score := 0.8
	snippet := "snippet"
	raw := "[Link](b.md)"
	resolved := "b.md"
	indexDescription := "index description"
	keywords := []string{"example", "graph"}
	matchedFields := []SearchField{FieldTitle, FieldDescription}

	bundle := Bundle{
		Root:       "/bundle",
		OkfVersion: &version,
		Concepts: []OkfConcept{{
			ID:          "a",
			Path:        "a.md",
			Frontmatter: map[string]any{"type": "Concept", "keywords": []any{"example", "graph"}},
			Type:        "Concept",
			Keywords:    keywords,
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
				HasKeywords:    true,
				Warnings:       []string{},
			}},
			Warnings: []string{},
		},
	}
	marshalValidate(t, compiler, "bundle.schema.json", bundle)

	graph := Graph{
		Nodes: []GraphNode{{
			ID: "a", Path: "a.md", Type: "Concept", Title: "A", Keywords: keywords,
			Status: &status, Tags: []string{"tag"}, InDegree: 1, OutDegree: 1,
		}},
		Edges:    []GraphEdge{{From: "a", To: "b", Source: "link", Raw: &raw}},
		Dangling: map[string][]OkfLink{"a": {}},
	}
	marshalValidate(t, compiler, "graph.schema.json", graph)

	searchResponse := SearchResponse{
		Query: "graph",
		Mode:  SearchHybrid,
		TopK:  10,
		Keyword: []KeywordHit{{
			ID: "a", Title: "A", Path: "a.md", Type: "Concept",
			Keywords: keywords, MatchedKeywords: keywords, Score: 1,
		}},
		Fuzzy: []FuzzyHit{{
			ID: "a", Title: "A", Path: "a.md", Type: "Concept",
			Score: score, MatchedFields: matchedFields, Snippet: &snippet,
		}},
		Hybrid: []HybridHit{{
			ID: "a", Title: "A", Path: "a.md", Type: "Concept",
			KeywordScore: 1, FuzzyScore: score, TotalScore: 1,
			MatchedKeywords: keywords, MatchedFields: matchedFields, Snippet: &snippet,
		}},
	}
	marshalValidate(t, compiler, "search.schema.json", searchResponse)

	lexicalResponse := SearchResponse{
		Query: "graph", Mode: SearchBM25, TopK: 10,
		Keyword: []KeywordHit{}, Fuzzy: []FuzzyHit{}, Hybrid: []HybridHit{},
		Lexical: []LexicalHit{{
			ID: "a", Title: "A", Path: "a.md", Type: "Concept",
			RawScore: 3.42, Score: 1,
		}},
	}
	marshalValidate(t, compiler, "search.schema.json", lexicalResponse)

	listResults := ListResults{{
		ID: "a", Title: "A", Path: "a.md", Type: "Concept", Keywords: keywords,
		Status: &status, Tags: []string{"tag"},
	}}
	marshalValidate(t, compiler, "list-results.schema.json", listResults)

	show := ShowResult{
		ID: "a", Path: "a.md", Type: "Concept", Keywords: keywords,
		Title: &title, Status: &status, Body: "# A", Backlinks: []string{"b"},
		Links: []ShowLink{{Raw: raw, Target: "b.md", ResolvesInBundle: true}},
	}
	marshalValidate(t, compiler, "show.schema.json", show)

	backlinks := BacklinksResult{ID: "a", Backlinks: []Backlink{{From: "b", Title: "B"}}}
	marshalValidate(t, compiler, "backlinks.schema.json", backlinks)

	path := PathResult{From: "a", To: "b", Path: []string{"a", "b"}}
	marshalValidate(t, compiler, "path.schema.json", path)

	disconnected := PathResult{From: "a", To: "b", Path: nil}
	marshalValidate(t, compiler, "path.schema.json", disconnected)

	auditReport := AuditReport{
		Path:   "/bundle",
		OK:     false,
		FailOn: AuditFailOnError,
		Summary: AuditSummary{
			Concepts: 1, Valid: true, Edges: 1, Orphans: 0,
			BrokenLinks: 1, MissingKeywords: 0, MissingIndexTargets: 0, UnindexedConcepts: 0,
		},
		Findings: []AuditFinding{{
			Code: AuditBrokenLink, Severity: AuditSeverityError,
			Source: "a", Target: "b.md", Message: "link target does not resolve in the bundle",
		}},
	}
	marshalValidate(t, compiler, "audit.schema.json", auditReport)

	relate := RelateResult{
		Root:      "a",
		Direction: RelateBoth,
		Depth:     1,
		Nodes: []GraphNode{{
			ID: "a", Path: "a.md", Type: "Concept", Title: "A", Keywords: keywords,
			Status: &status, Tags: []string{"tag"}, InDegree: 1, OutDegree: 1,
		}},
		Edges: []RelateEdge{{
			From: "a", To: "b", Source: "link", Raw: &raw, Depth: 1,
		}},
	}
	marshalValidate(t, compiler, "relate.schema.json", relate)

	explain := ExplainResult{
		Query: "oauth", Mode: SearchHybrid, TopK: 10, ID: "a", Rank: 1, Found: true,
		Keyword: ExplainKeyword{
			Score: 1, MatchedKeywords: []string{"oauth"},
			Tokens: []ExplainKeywordToken{{
				QueryToken: "oauth", MatchedKeyword: "oauth",
				Match: ExplainMatchExact, Score: 1,
			}},
		},
		Fuzzy: ExplainFuzzy{
			Score: score, MatchedFields: matchedFields,
			Fields: []ExplainFuzzyField{{
				Field: FieldTitle, Score: 1, Weight: 1, Contribution: 0.5263,
			}},
			Tokens: []ExplainFuzzyToken{{
				QueryToken: "tokn", MatchedToken: "token", Field: FieldTitle, Similarity: 0.8,
			}},
			Snippet: &snippet,
		},
		Hybrid: ExplainHybrid{KeywordWeight: 0.5, LexicalWeight: 0.5, FuzzyWeight: 0.5, Total: 0.9},
		Lexical: &ExplainLexical{
			RawScore: 3.42, Score: 1,
			MatchedFields: []SearchField{FieldTitle, FieldBody},
			Tokens: []ExplainLexicalToken{{
				QueryToken: "alpha", Matched: true, Contribution: 2.1,
			}},
		},
	}
	marshalValidate(t, compiler, "explain.schema.json", explain)

	ingestReport := IngestReport{
		DB: "/home/user/.weave/index.db", Bundle: "/bundle", Mode: "strict",
		Documents: 1, Edges: 1, Synthetic: 0, Warnings: 0, Errors: 0,
	}
	marshalValidate(t, compiler, "ingest.schema.json", ingestReport)

	syncReport := SyncReport{
		DB: "/home/user/.weave/index.db", Bundle: "/bundle", Mode: "strict",
		Documents: 1, Added: 0, Updated: 1, Removed: 0, Skipped: 0,
		Warnings: 0, Errors: 0,
	}
	marshalValidate(t, compiler, "sync.schema.json", syncReport)

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
			Op: OpSearch, Fields: []string{"path", "query", "mode", "top_k"}, Result: "SearchResponse",
		}},
		Errors: []ErrorCode{ErrInvalidRequest, ErrConceptNotFound},
	}
	marshalValidate(t, compiler, "cli.schema.json", contractInfo)

	request := OperationRequest{
		Op: OpSearch, Path: "/bundle", Query: "graph",
		Mode: SearchHybrid, TopK: intPtr(10),
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
