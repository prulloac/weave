package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const cliExtensionDataProgram = `package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type request struct {
	Workspace string ` + "`json:\"workspace\"`" + `
	DocID     string ` + "`json:\"doc_id\"`" + `
}

func main() {
	var value request
	if err := json.NewDecoder(os.Stdin).Decode(&value); err != nil {
		os.Exit(1)
	}
	files := filepath.Join(value.Workspace, "files")
	if err := os.MkdirAll(files, 0o755); err != nil {
		os.Exit(1)
	}
	if err := os.WriteFile(filepath.Join(files, "sample.bin"), []byte("asset-bytes"), 0o644); err != nil {
		os.Exit(1)
	}
	key := "assets/" + value.DocID + "/sample.bin"
	manifest := fmt.Sprintf(` + "`" + `{"v":"weave.extension-data/v1","items":[{"key":%q,"path":"files/sample.bin","media_type":"application/octet-stream","metadata":{"source":"test"}}]}` + "`" + `, key)
	if err := os.WriteFile(filepath.Join(value.Workspace, "manifest.json"), []byte(manifest), 0o644); err != nil {
		os.Exit(1)
	}
	os.Stdout.WriteString(` + "`" + `{"v":"weave.cli/v1","inferred_keywords":["asset"]}` + "`" + `)
}
`

type extensionDataListEnvelope struct {
	OK     bool `json:"ok"`
	Result struct {
		Data []struct {
			Extension string `json:"extension"`
			DocID     string `json:"doc_id"`
			Key       string `json:"key"`
			MediaType string `json:"media_type"`
			Size      int64  `json:"size"`
			SHA256    string `json:"sha256"`
			Metadata  any    `json:"metadata"`
		} `json:"data"`
	} `json:"result"`
}

type extensionDataGCEnvelope struct {
	OK     bool `json:"ok"`
	Result struct {
		Removed int  `json:"removed"`
		DryRun  bool `json:"dry_run"`
	} `json:"result"`
}

func extensionDataCLIFixture(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "bundle")
	if err := os.CopyFS(root, os.DirFS(filepath.Join("testdata", "audit", "clean"))); err != nil {
		t.Fatal(err)
	}
	extensionDir := filepath.Join(root, ".weave", "bin")
	if err := os.MkdirAll(extensionDir, 0o755); err != nil {
		t.Fatal(err)
	}
	program := filepath.Join(t.TempDir(), "main.go")
	if err := os.WriteFile(program, []byte(cliExtensionDataProgram), 0o644); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("go", "build", "-o", filepath.Join(extensionDir, "weave-data"), program)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build fixture extension: %v\n%s", err, output)
	}
	return root
}

func TestExtensionDataCLIListAndGet(t *testing.T) {
	root := extensionDataCLIFixture(t)
	db := filepath.Join(t.TempDir(), "index.db")
	code, _, stderr := Invoke(t, "ingest", root, "--db", db, "--strict", "--with-enricher", "data", "--json")
	if code != 0 {
		t.Fatalf("ingest exit=%d stderr=%q", code, stderr)
	}

	code, stdout, stderr := Invoke(t, "extensions", "data", "list", root, "--db", db, "--json")
	if code != 0 {
		t.Fatalf("list exit=%d stderr=%q", code, stderr)
	}
	var list extensionDataListEnvelope
	if err := json.Unmarshal([]byte(stdout), &list); err != nil {
		t.Fatal(err)
	}
	if !list.OK || len(list.Result.Data) != 2 {
		t.Fatalf("unexpected list: %s", stdout)
	}
	item := list.Result.Data[0]
	if item.Extension != "data" || !strings.HasPrefix(item.Key, "assets/") || item.MediaType != "application/octet-stream" || item.Size != 11 {
		t.Fatalf("unexpected item: %+v", item)
	}

	output := filepath.Join(t.TempDir(), "sample.bin")
	code, _, stderr = Invoke(t, "extensions", "data", "get", root, "data", item.Key, "--out", output, "--db", db, "--json")
	if code != 0 {
		t.Fatalf("get exit=%d stderr=%q", code, stderr)
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "asset-bytes" {
		t.Fatalf("unexpected data: %q", data)
	}

	if code, _, _ := Invoke(t, "extensions", "data", "get", root, "data", item.Key, "--out", output, "--db", db, "--json"); code != 2 {
		t.Fatalf("expected OUTPUT_EXISTS exit 2, got %d", code)
	}
	if code, _, stderr := Invoke(t, "extensions", "data", "get", root, "data", item.Key, "--out", output, "--force", "--db", db, "--json"); code != 0 {
		t.Fatalf("force get exit=%d stderr=%q", code, stderr)
	}

	code, stdout, stderr = Invoke(t, "show", root, "concepts/alpha", "--db", db, "--json")
	if code != 0 {
		t.Fatalf("show exit=%d stderr=%q", code, stderr)
	}
	validateCLIJSON(t, stdout, "show.schema.json")
	var shown struct {
		OK     bool `json:"ok"`
		Result struct {
			ExtensionData []struct {
				Extension string `json:"extension"`
				Key       string `json:"key"`
			} `json:"extension_data"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(stdout), &shown); err != nil {
		t.Fatal(err)
	}
	if !shown.OK || len(shown.Result.ExtensionData) != 1 {
		t.Fatalf("unexpected show extension data: %s", stdout)
	}
}

func TestExtensionDataCLIGC(t *testing.T) {
	root := extensionDataCLIFixture(t)
	db := filepath.Join(t.TempDir(), "index.db")
	if code, _, stderr := Invoke(t, "ingest", root, "--db", db, "--strict", "--with-enricher", "data", "--json"); code != 0 {
		t.Fatalf("ingest exit=%d stderr=%q", code, stderr)
	}

	code, stdout, stderr := Invoke(t, "extensions", "data", "gc", root, "--db", db, "--dry-run", "--json")
	if code != 0 {
		t.Fatalf("gc exit=%d stderr=%q", code, stderr)
	}
	var result extensionDataGCEnvelope
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatal(err)
	}
	if !result.OK || !result.Result.DryRun || result.Result.Removed != 0 {
		t.Fatalf("unexpected gc result: %s", stdout)
	}
}

func TestExtensionDataCLIRequiresOut(t *testing.T) {
	root := extensionDataCLIFixture(t)
	code, stdout, stderr := Invoke(t, "extensions", "data", "get", root, "data", "assets/sample.bin", "--json")
	if code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
	if !strings.Contains(stdout+stderr, "OUTPUT_REQUIRED") {
		t.Fatalf("missing OUTPUT_REQUIRED: stdout=%s stderr=%s", stdout, stderr)
	}
}
