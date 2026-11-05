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

func TestEmbeddingMaintenanceCommands(t *testing.T) {
	root := embeddingFixture(t)
	db := filepath.Join(t.TempDir(), "index.db")
	code, _, stderr := Invoke(t, "ingest", root, "--db", db, "--with-embedder", "embed", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("ingest exit=%d stderr=%q", code, stderr)
	}
	code, stdout, stderr := Invoke(t, "embeddings", "list", root, "--db", db, "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("list exit=%d stderr=%q", code, stderr)
	}
	var list struct {
		Spaces []struct {
			Model string `json:"model"`
			Dim   int    `json:"dim"`
			Rows  int    `json:"rows"`
		} `json:"spaces"`
	}
	if err := json.Unmarshal(decodeEnvelope(t, stdout).Result, &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Spaces) != 1 || list.Spaces[0].Model != "test-model" || list.Spaces[0].Rows != 3 {
		t.Fatalf("spaces = %+v", list.Spaces)
	}

	code, _, stderr = Invoke(t, "embeddings", "rebuild", root, "--model", "test-model", "--db", db, "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("rebuild exit=%d stderr=%q", code, stderr)
	}
	code, _, stderr = Invoke(t, "embeddings", "drop", root, "--model", "test-model", "--keep-source", "--db", db, "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("drop exit=%d stderr=%q", code, stderr)
	}
	code, stdout, _ = Invoke(t, "embeddings", "list", root, "--db", db, "--json")
	if code != 0 {
		t.Fatal("list after drop failed")
	}
	if err := json.Unmarshal(decodeEnvelope(t, stdout).Result, &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Spaces) != 1 || list.Spaces[0].Rows != 3 {
		t.Fatalf("keep-source spaces = %+v", list.Spaces)
	}
	code, _, stderr = Invoke(t, "embeddings", "drop", root, "--model", "test-model", "--db", db, "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("drop source exit=%d stderr=%q", code, stderr)
	}
	code, stdout, _ = Invoke(t, "embeddings", "list", root, "--db", db, "--json")
	if code != 0 {
		t.Fatal("list after source drop failed")
	}
	if err := json.Unmarshal(decodeEnvelope(t, stdout).Result, &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Spaces) != 0 {
		t.Fatalf("spaces after drop = %+v", list.Spaces)
	}
}

func TestEmbeddingAuditFindingsCLI(t *testing.T) {
	root := embeddingFixture(t)
	db := filepath.Join(t.TempDir(), "index.db")
	code, _, stderr := Invoke(t, "ingest", root, "--db", db, "--with-embedder", "embed", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("ingest exit=%d stderr=%q", code, stderr)
	}
	content := "---\ntype: Concept\ntitle: \"alpha\"\nkeywords:\n  - alpha\n---\n\n# alpha updated\n"
	if err := os.WriteFile(filepath.Join(root, "alpha.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, stderr = Invoke(t, "sync", root, "--db", db, "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("sync exit=%d stderr=%q", code, stderr)
	}
	code, stdout, stderr := Invoke(t, "audit", root, "--db", db, "--fail-on", "warning", "--json")
	if code != 1 || stderr != "" {
		t.Fatalf("audit exit=%d stderr=%q", code, stderr)
	}
	var report contractv1.AuditReport
	if err := json.Unmarshal(decodeEnvelope(t, stdout).Result, &report); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, finding := range report.Findings {
		if finding.Code == contractv1.AuditEmbeddingStale && finding.Concept == "alpha" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing EMBEDDING_STALE: %+v", report.Findings)
	}

	code, _, stderr = Invoke(t, "embeddings", "drop", root, "--model", "test-model",
		"--keep-source", "--db", db, "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("drop exit=%d stderr=%q", code, stderr)
	}
	code, stdout, stderr = Invoke(t, "audit", root, "--db", db, "--fail-on", "warning", "--json")
	if code != 1 || stderr != "" {
		t.Fatalf("audit after drop exit=%d stderr=%q", code, stderr)
	}
	if err := json.Unmarshal(decodeEnvelope(t, stdout).Result, &report); err != nil {
		t.Fatal(err)
	}
	found = false
	for _, finding := range report.Findings {
		if finding.Code == contractv1.AuditEmbeddingIndexMismatch {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing EMBEDDING_INDEX_MISMATCH: %+v", report.Findings)
	}
}

func TestRebuildNewModelWithEmbedder(t *testing.T) {
	root := embeddingFixture(t)
	db := filepath.Join(t.TempDir(), "index.db")
	code, _, stderr := Invoke(t, "ingest", root, "--db", db, "--with-embedder", "embed", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("ingest exit=%d stderr=%q", code, stderr)
	}
	code, stdout, stderr := Invoke(t, "embeddings", "rebuild", root, "--model", "test-model-v2",
		"--with-embedder", "embed", "--db", db, "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("rebuild exit=%d stderr=%q stdout=%s", code, stderr, stdout)
	}
	code, stdout, stderr = Invoke(t, "embeddings", "list", root, "--db", db, "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("list exit=%d stderr=%q", code, stderr)
	}
	var list struct {
		Spaces []struct {
			Model string `json:"model"`
			Rows  int    `json:"rows"`
		} `json:"spaces"`
	}
	if err := json.Unmarshal(decodeEnvelope(t, stdout).Result, &list); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, space := range list.Spaces {
		if space.Model == "test-model-v2" && space.Rows == 3 {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing new model space: %+v", list.Spaces)
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
