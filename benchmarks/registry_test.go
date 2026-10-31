package benchmarks_test

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

type registry struct {
	Version int             `json:"version"`
	Entries []registryEntry `json:"entries"`
}

type registryEntry struct {
	ID        string   `json:"id"`
	Runner    string   `json:"runner"`
	Selector  string   `json:"selector"`
	Tier      string   `json:"tier"`
	Aggregate string   `json:"aggregate"`
	Unit      string   `json:"unit"`
	Budget    *float64 `json:"budget"`
	Blocking  bool     `json:"blocking"`
}

type querySet struct {
	Version int     `json:"version"`
	Seed    int64   `json:"seed"`
	Queries []query `json:"queries"`
}

type query struct {
	Name   string `json:"name"`
	Query  string `json:"query"`
	Mode   string `json:"mode"`
	Expect string `json:"expect"`
}

func TestRegistry(t *testing.T) {
	value := readRegistry(t)
	if value.Version != 1 {
		t.Fatalf("registry version = %d", value.Version)
	}
	if len(value.Entries) < 30 {
		t.Fatalf("expected at least 30 entries, got %d", len(value.Entries))
	}
	seen := map[string]bool{}
	ids := constantIDs(t)
	for _, entry := range value.Entries {
		if !strings.HasPrefix(entry.ID, "WEAVE-B-") {
			t.Errorf("invalid ID %q", entry.ID)
		}
		if seen[entry.ID] {
			t.Errorf("duplicate ID %q", entry.ID)
		}
		seen[entry.ID] = true
		if !ids[entry.ID] {
			t.Errorf("ID %q is missing from ids.go", entry.ID)
		}
		if entry.Runner == "" || entry.Selector == "" || entry.Tier == "" || entry.Aggregate == "" || entry.Unit == "" {
			t.Errorf("incomplete entry %+v", entry)
		}
		if entry.Blocking && entry.Budget == nil {
			t.Errorf("blocking entry %q has no budget", entry.ID)
		}
	}
	for id := range ids {
		if !seen[id] {
			t.Errorf("ids.go constant %q is missing from registry.json", id)
		}
	}
}

func TestQueries(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("queries.json"))
	if err != nil {
		t.Fatal(err)
	}
	var value querySet
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatal(err)
	}
	if value.Version != 1 || value.Seed == 0 {
		t.Fatalf("unexpected query header: %+v", value)
	}
	if len(value.Queries) != 40 {
		t.Fatalf("expected 40 queries, got %d", len(value.Queries))
	}
	names := map[string]bool{}
	for _, item := range value.Queries {
		if item.Name == "" || item.Query == "" {
			t.Errorf("incomplete query %+v", item)
		}
		if names[item.Name] {
			t.Errorf("duplicate query name %q", item.Name)
		}
		names[item.Name] = true
		switch item.Mode {
		case "keyword", "fuzzy", "hybrid", "bm25":
		default:
			t.Errorf("invalid mode %q", item.Mode)
		}
		if item.Expect != "empty" && item.Expect != "nonempty" {
			t.Errorf("invalid expectation %q", item.Expect)
		}
	}
}

func readRegistry(t *testing.T) registry {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("registry.json"))
	if err != nil {
		t.Fatal(err)
	}
	var value registry
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatal(err)
	}
	return value
}

func constantIDs(t *testing.T) map[string]bool {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), filepath.Join("ids.go"), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, declaration := range file.Decls {
		gen, ok := declaration.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			valueSpec, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for _, value := range valueSpec.Values {
				literal, ok := value.(*ast.BasicLit)
				if !ok || literal.Kind != token.STRING {
					continue
				}
				id, err := strconv.Unquote(literal.Value)
				if err != nil {
					t.Fatal(err)
				}
				ids[id] = true
			}
		}
	}
	if len(ids) == 0 {
		t.Fatal("ids.go has no string constants")
	}
	return ids
}
