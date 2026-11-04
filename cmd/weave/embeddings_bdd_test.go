package main

import (
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
)

const embedderSource = `package main

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"math"
	"os"
	"strings"
)

type request struct {
	V    string ` + "`json:\"v\"`" + `
	Type string ` + "`json:\"type\"`" + `
	Text string ` + "`json:\"text\"`" + `
	Model string ` + "`json:\"model\"`" + `
}

func main() {
	var req request
	json.NewDecoder(os.Stdin).Decode(&req)
	values := []float32{0, 0, 1}
	switch {
	case strings.Contains(req.Text, "alpha"):
		values = []float32{1, 0, 0}
	case strings.Contains(req.Text, "beta"):
		values = []float32{0, 1, 0}
	}
	raw := make([]byte, 12)
	for i, value := range values {
		binary.LittleEndian.PutUint32(raw[i*4:], math.Float32bits(value))
	}
	model := req.Model
	if model == "" {
		model = "test-model"
	}
	json.NewEncoder(os.Stdout).Encode(map[string]any{
		"v": "weave.cli/v1", "model": model, "dim": 3,
		"vector": base64.StdEncoding.EncodeToString(raw),
	})
}
`

func buildEmbedder(t *testing.T, root string) {
	t.Helper()
	source := filepath.Join(t.TempDir(), "main.go")
	if err := os.WriteFile(source, []byte(embedderSource), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, ".weave", "bin")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("go", "build", "-o", filepath.Join(dir, "weave-embed"), source)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build embedder: %v\n%s", err, output)
	}
}

func embeddingFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, name := range []string{"alpha", "beta", "gamma"} {
		content := "---\ntype: Concept\ntitle: \"" + name + "\"\nkeywords:\n  - " + name + "\n---\n\n# " + name + "\n"
		if err := os.WriteFile(filepath.Join(root, name+".md"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	buildEmbedder(t, root)
	return root
}

func TestEmbeddingsAndSemanticSearch(t *testing.T) {
	root := embeddingFixture(t)
	db := filepath.Join(t.TempDir(), "index.db")
	code, stdout, stderr := Invoke(t, "ingest", root, "--db", db, "--with-embedder", "embed", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("ingest exit=%d stderr=%q stdout=%s", code, stderr, stdout)
	}

	code, stdout, stderr = Invoke(t, "search", root, "alpha", "--mode", "semantic", "--embedder", "embed", "--db", db, "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("search exit=%d stderr=%q", code, stderr)
	}
	var response contractv1.SearchResponse
	if err := json.Unmarshal(decodeEnvelope(t, stdout).Result, &response); err != nil {
		t.Fatal(err)
	}
	if response.Mode != contractv1.SearchSemantic || len(response.Semantic) != 3 {
		t.Fatalf("unexpected response: %+v", response)
	}
	first := response.Semantic[0]
	if first.ID != "alpha" || first.EmbeddingModel != "test-model" || first.EmbeddingDim != 3 {
		t.Fatalf("unexpected first hit: %+v", first)
	}
	if math.Abs(first.SemanticScore-1) > 0.001 {
		t.Fatalf("score = %f", first.SemanticScore)
	}
}

func TestSemanticSearchRequiresEmbedder(t *testing.T) {
	root := embeddingFixture(t)
	db := filepath.Join(t.TempDir(), "index.db")
	code, _, _ := Invoke(t, "ingest", root, "--db", db, "--json")
	if code != 0 {
		t.Fatal("ingest failed")
	}
	code, stdout, _ := Invoke(t, "search", root, "alpha", "--mode", "semantic", "--db", db, "--json")
	if code != 2 {
		t.Fatalf("exit=%d, want 2", code)
	}
	envelope := decodeEnvelope(t, stdout)
	if envelope.Error == nil || !strings.Contains(envelope.Error.Message, "--embedder") {
		t.Fatalf("unexpected error: %+v", envelope.Error)
	}
}
