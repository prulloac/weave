package operations_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
	sqlitestore "github.com/prulloac/weave/storage/sqlite"
)

const extensionDataProgram = `package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type request struct {
	Workspace string ` + "`json:\"workspace\"`" + `
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
	manifest := ` + "`" + `{"v":"weave.extension-data/v1","items":[{"key":"assets/sample.bin","path":"files/sample.bin","media_type":"application/octet-stream","metadata":{"source":"test"}}]}` + "`" + `
	if err := os.WriteFile(filepath.Join(value.Workspace, "manifest.json"), []byte(manifest), 0o644); err != nil {
		os.Exit(1)
	}
	os.Stdout.WriteString(` + "`" + `{"v":"weave.cli/v1","inferred_keywords":["asset"]}` + "`" + `)
}
`

func TestExtensionDataIngest(t *testing.T) {
	root := filepath.Join(t.TempDir(), "bundle")
	if err := os.CopyFS(root, os.DirFS(filepath.Join("..", "..", "cmd/weave/testdata/audit/clean"))); err != nil {
		t.Fatal(err)
	}
	extensionDir := filepath.Join(root, ".weave", "bin")
	if err := os.MkdirAll(extensionDir, 0o755); err != nil {
		t.Fatal(err)
	}
	program := filepath.Join(t.TempDir(), "main.go")
	if err := os.WriteFile(program, []byte(extensionDataProgram), 0o644); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(extensionDir, "weave-data")
	command := exec.Command("go", "build", "-o", binary, program)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build fixture extension: %v\n%s", err, output)
	}

	dbPath := filepath.Join(t.TempDir(), "index.db")
	successResult(t, executor().Execute(context.Background(), contractv1.OperationRequest{
		Op:        contractv1.OpIngest,
		Path:      root,
		DB:        dbPath,
		Strict:    true,
		Enricher:  "data",
		TimeoutMS: 5000,
	}))

	store, err := sqlitestore.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	records, err := store.ListExtensionData(context.Background(), root, sqlitestore.ExtensionDataFilter{Extension: "data"})
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 {
		t.Fatalf("expected two extension data rows, got %+v", records)
	}
	data, record, err := store.ReadExtensionData(context.Background(), root, records[0].Extension, records[0].DocID, records[0].Key)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "asset-bytes" {
		t.Fatalf("unexpected data: %q", data)
	}
	if string(record.Metadata) != `{"source":"test"}` {
		t.Fatalf("unexpected metadata: %s", record.Metadata)
	}

	parsed := successResult(t, executor().Execute(context.Background(), contractv1.OperationRequest{
		Op:   contractv1.OpParse,
		Path: root,
		DB:   dbPath,
	}))
	bundle, ok := parsed.(*contractv1.Bundle)
	if !ok {
		t.Fatalf("expected a bundle, got %T", parsed)
	}
	for _, concept := range bundle.Concepts {
		if !containsKeyword(concept.Keywords, "asset") {
			t.Fatalf("inferred keyword missing from %s: %v", concept.ID, concept.Keywords)
		}
	}

	show := successResult(t, executor().Execute(context.Background(), contractv1.OperationRequest{
		Op:     contractv1.OpShow,
		Path:   root,
		DB:     dbPath,
		Target: "concepts/alpha",
	}))
	shown, ok := show.(*contractv1.ShowResult)
	if !ok {
		t.Fatalf("expected a show result, got %T", show)
	}
	if len(shown.ExtensionData) != 1 {
		t.Fatalf("expected one extension data reference, got %+v", shown.ExtensionData)
	}
	if shown.ExtensionData[0].Extension != "data" || !strings.HasPrefix(shown.ExtensionData[0].Key, "assets/") {
		t.Fatalf("unexpected extension data reference: %+v", shown.ExtensionData[0])
	}
}

func containsKeyword(keywords []string, target string) bool {
	for _, keyword := range keywords {
		if keyword == target {
			return true
		}
	}
	return false
}
