package assets

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestScanPreservesSourceMediaTypes(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"main.go":   "package main\n",
		"app.py":    "print('hello')\n",
		"lib.rs":    "fn main() {}\n",
		"Main.java": "class Main {}\n",
		"index.ts":  "export const x = 1;\n",
		"notes.txt": "plain notes\n",
		"broken.go": string([]byte{0xff, 0xfe, 0x00}),
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	result, err := Scan(context.Background(), root, Filter{Types: []string{"text/x-go"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Concepts) != 1 || result.Concepts[0].ID != "main.go" {
		t.Fatalf("expected only main.go, got %+v", result.Concepts)
	}
	if result.Concepts[0].MediaType != "text/x-go" {
		t.Fatalf("media type = %q, want text/x-go", result.Concepts[0].MediaType)
	}

	result, err = Scan(context.Background(), root, Filter{Types: []string{"text/*", "application/javascript"}})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]string{}
	for _, concept := range result.Concepts {
		seen[concept.ID] = concept.MediaType
	}
	want := map[string]string{
		"main.go":   "text/x-go",
		"app.py":    "text/x-python",
		"lib.rs":    "text/x-rust",
		"Main.java": "text/x-java",
		"index.ts":  "text/x-typescript",
		"notes.txt": "text/plain",
	}
	for name, mediaType := range want {
		if seen[name] != mediaType {
			t.Fatalf("%s media type = %q, want %q", name, seen[name], mediaType)
		}
	}
	if _, ok := seen["broken.go"]; ok {
		t.Fatal("invalid UTF-8 source file must not match a text filter")
	}
}
