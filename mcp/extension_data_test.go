package mcp_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine"
	"github.com/prulloac/weave/engine/operations"
	"github.com/prulloac/weave/mcp"
)

const mcpExtensionDataProgram = `package main

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
	if err := os.WriteFile(filepath.Join(files, "sample.bin"), []byte("mcp-bytes"), 0o644); err != nil {
		os.Exit(1)
	}
	key := "assets/" + value.DocID + "/sample.bin"
	manifest := fmt.Sprintf(` + "`" + `{"v":"weave.extension-data/v1","items":[{"key":%q,"path":"files/sample.bin","media_type":"application/octet-stream"}]}` + "`" + `, key)
	if err := os.WriteFile(filepath.Join(value.Workspace, "manifest.json"), []byte(manifest), 0o644); err != nil {
		os.Exit(1)
	}
	os.Stdout.WriteString(` + "`" + `{"v":"weave.cli/v1"}` + "`" + `)
}
`

func TestMCPExtensionDataResource(t *testing.T) {
	root := filepath.Join(t.TempDir(), "bundle")
	if err := os.CopyFS(root, os.DirFS(filepath.Join("..", "cmd/weave/testdata/audit/clean"))); err != nil {
		t.Fatal(err)
	}
	extensionDir := filepath.Join(root, ".weave", "bin")
	if err := os.MkdirAll(extensionDir, 0o755); err != nil {
		t.Fatal(err)
	}
	program := filepath.Join(t.TempDir(), "main.go")
	if err := os.WriteFile(program, []byte(mcpExtensionDataProgram), 0o644); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("go", "build", "-o", filepath.Join(extensionDir, "weave-data"), program)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build fixture extension: %v\n%s", err, output)
	}

	db := filepath.Join(t.TempDir(), "index.db")
	executor := operations.New("test", engine.Options{})
	result := executor.Execute(context.Background(), contractv1.OperationRequest{
		Op:        contractv1.OpIngest,
		Path:      root,
		DB:        db,
		Strict:    true,
		Enricher:  "data",
		TimeoutMS: 5000,
	})
	if _, ok := result.(contractv1.SuccessEnvelope); !ok {
		t.Fatalf("ingest failed: %+v", result)
	}

	server, err := mcp.New(executor, mcp.Options{Bundle: root, DB: db})
	if err != nil {
		t.Fatal(err)
	}
	uri := "weave://extension/data/assets/concepts/alpha/sample.bin"
	found := false
	for _, resource := range server.Resources() {
		if resource.URI == uri {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("extension data resource %s not listed", uri)
	}
	templateFound := false
	for _, template := range server.ResourceTemplates() {
		if template.URITemplate == "weave://extension/{name}/{key}" {
			templateFound = true
			break
		}
	}
	if !templateFound {
		t.Fatal("extension data resource template not listed")
	}

	content, err := server.ReadResource(context.Background(), uri)
	if err != nil {
		t.Fatal(err)
	}
	if content.MimeType != "application/octet-stream" {
		t.Fatalf("unexpected mime type: %s", content.MimeType)
	}
	if content.Text != "" {
		t.Fatalf("expected a blob resource, got text %q", content.Text)
	}
	data, err := base64.StdEncoding.DecodeString(content.Blob)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "mcp-bytes" {
		t.Fatalf("unexpected bytes: %q", data)
	}

	var raw map[string]any
	encoded, _ := json.Marshal(content)
	if err := json.Unmarshal(encoded, &raw); err != nil {
		t.Fatal(err)
	}
	if _, ok := raw["blob"]; !ok {
		t.Fatalf("resource content missing blob: %s", encoded)
	}
}
