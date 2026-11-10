package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const weaveCodeFixture = `package main

import (
	"encoding/json"
	"os"
)

type file struct {
	ID        string ` + "`json:\"id\"`" + `
	Path      string ` + "`json:\"path\"`" + `
	MediaType string ` + "`json:\"media_type\"`" + `
}

type request struct {
	Files []file ` + "`json:\"files\"`" + `
}

type document struct {
	ID              string   ` + "`json:\"id\"`" + `
	Title           string   ` + "`json:\"title\"`" + `
	Description     string   ` + "`json:\"description,omitempty\"`" + `
	Keywords        []string ` + "`json:\"keywords,omitempty\"`" + `
	ContentMetadata any      ` + "`json:\"content_metadata,omitempty\"`" + `
	SymbolDocuments []document ` + "`json:\"symbol_documents,omitempty\"`" + `
	Body            string   ` + "`json:\"body,omitempty\"`" + `
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--describe" {
		os.Stdout.WriteString(` + "`" + `{"v":"weave.extension/v1","name":"code","source_extensions":[".go"],"analysis_modes":["project"]}` + "`" + `)
		return
	}
	var req request
	_ = json.NewDecoder(os.Stdin).Decode(&req)
	documents := make([]document, 0, len(req.Files))
	relations := []map[string]string{}
	for _, file := range req.Files {
		doc := document{
			ID: file.ID, Title: "go package", Keywords: []string{"go", "main"},
			ContentMetadata: map[string]any{"v": 1, "language": "go"},
		}
		if file.ID == "main.go" {
			doc.SymbolDocuments = []document{{ID: "main.go#main", Title: "main", Body: "func main()"}}
		}
		documents = append(documents, doc)
	}
	if len(req.Files) >= 2 {
		relations = append(relations,
			map[string]string{"from": req.Files[0].ID, "to": req.Files[1].ID, "kind": "imports"},
			map[string]string{"from": req.Files[0].ID + "#main", "to": req.Files[1].ID, "kind": "calls"},
		)
	}
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
		"v": "weave.cli/v1", "documents": documents, "relations": relations,
	})
}
`

func weaveCodeRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	bin := filepath.Join(root, ".weave", "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	program := filepath.Join(t.TempDir(), "main.go")
	if err := os.WriteFile(program, []byte(weaveCodeFixture), 0o644); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(bin, "weave-code")
	build := exec.Command("go", "build", "-o", binary, program)
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build code fixture: %v\n%s", err, output)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "lib.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestWeaveCodeProjectSync(t *testing.T) {
	root := weaveCodeRepo(t)
	db := filepath.Join(t.TempDir(), "index.db")
	if code, _, stderr := Invoke(t, "ingest", root, "--with-enricher", "code", "--db", db, "--strict", "--json"); code != 0 {
		t.Fatalf("ingest exit=%d stderr=%q", code, stderr)
	}
	if err := os.WriteFile(filepath.Join(root, "extra.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := Invoke(t, "sync", root, "--with-enricher", "code", "--db", db, "--strict", "--json")
	if code != 0 {
		t.Fatalf("sync exit=%d stderr=%q stdout=%s", code, stderr, stdout)
	}
	var list struct {
		Result []struct {
			ID string `json:"id"`
		} `json:"result"`
	}
	code, stdout, stderr = Invoke(t, "list", root, "--type", "Code", "--db", db, "--json")
	if code != 0 {
		t.Fatalf("list exit=%d stderr=%q", code, stderr)
	}
	if err := json.Unmarshal([]byte(stdout), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Result) != 3 {
		t.Fatalf("expected 3 code documents after sync, got %+v", list.Result)
	}
}

func TestWeaveCodeProjectSyncEnablesSymbols(t *testing.T) {
	root := weaveCodeRepo(t)
	db := filepath.Join(t.TempDir(), "index.db")
	if code, _, stderr := Invoke(t, "ingest", root, "--with-enricher", "code", "--db", db, "--strict", "--json"); code != 0 {
		t.Fatalf("ingest exit=%d stderr=%q", code, stderr)
	}
	t.Setenv("WEAVE_CODE_SYMBOL_DOCUMENTS", "1")
	if code, _, stderr := Invoke(t, "sync", root, "--with-enricher", "code", "--db", db, "--strict", "--json"); code != 0 {
		t.Fatalf("sync exit=%d stderr=%q", code, stderr)
	}
	code, stdout, stderr := Invoke(t, "list", root, "--type", "CodeSymbol", "--db", db, "--json")
	if code != 0 {
		t.Fatalf("list exit=%d stderr=%q", code, stderr)
	}
	if strings.Contains(stdout, `"result":[]`) {
		t.Fatalf("expected symbols after sync, got %s", stdout)
	}
}

func TestWeaveCodeProjectSyncRemovesSymbols(t *testing.T) {
	root := weaveCodeRepo(t)
	t.Setenv("WEAVE_CODE_SYMBOL_DOCUMENTS", "1")
	db := filepath.Join(t.TempDir(), "index.db")
	if code, _, stderr := Invoke(t, "ingest", root, "--with-enricher", "code", "--db", db, "--strict", "--json"); code != 0 {
		t.Fatalf("ingest exit=%d stderr=%q", code, stderr)
	}
	if err := os.Remove(filepath.Join(root, "main.go")); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := Invoke(t, "sync", root, "--with-enricher", "code", "--db", db, "--strict", "--json"); code != 0 {
		t.Fatalf("sync exit=%d stderr=%q", code, stderr)
	}
	code, stdout, stderr := Invoke(t, "list", root, "--type", "CodeSymbol", "--db", db, "--json")
	if code != 0 {
		t.Fatalf("list exit=%d stderr=%q", code, stderr)
	}
	if string(stdout) == "" || !containsEmptyList(stdout) {
		t.Fatalf("expected no symbols after removal, got %s", stdout)
	}
}

func containsEmptyList(payload string) bool {
	return strings.Contains(payload, `"result":[]`)
}

func TestWeaveCodeProjectIngest(t *testing.T) {
	root := weaveCodeRepo(t)
	db := filepath.Join(t.TempDir(), "index.db")
	code, stdout, stderr := Invoke(t, "ingest", root, "--with-enricher", "code", "--db", db, "--strict", "--json")
	if code != 0 {
		t.Fatalf("ingest exit=%d stderr=%q stdout=%s", code, stderr, stdout)
	}

	code, stdout, stderr = Invoke(t, "list", root, "--type", "Code", "--db", db, "--json")
	if code != 0 {
		t.Fatalf("list exit=%d stderr=%q", code, stderr)
	}
	var list struct {
		Result []struct {
			ID string `json:"id"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(stdout), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Result) != 2 {
		t.Fatalf("expected 2 code documents, got %+v", list.Result)
	}

	code, stdout, stderr = Invoke(t, "list", root, "--type", "CodeSymbol", "--db", db, "--json")
	if code != 0 {
		t.Fatalf("symbol list exit=%d stderr=%q", code, stderr)
	}
	if err := json.Unmarshal([]byte(stdout), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Result) != 1 || list.Result[0].ID != "main.go#main" {
		t.Fatalf("unexpected symbols: %+v", list.Result)
	}

	code, stdout, stderr = Invoke(t, "graph", root, "--relation", "imports", "--db", db, "--json")
	if code != 0 {
		t.Fatalf("graph exit=%d stderr=%q", code, stderr)
	}
	var graph struct {
		Result struct {
			Edges []struct {
				From   string `json:"from"`
				To     string `json:"to"`
				Source string `json:"source"`
			} `json:"edges"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(stdout), &graph); err != nil {
		t.Fatal(err)
	}
	if len(graph.Result.Edges) != 1 || graph.Result.Edges[0].Source != "imports" {
		t.Fatalf("unexpected graph edges: %+v", graph.Result.Edges)
	}
}
