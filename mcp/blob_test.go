package mcp_test

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine"
	"github.com/prulloac/weave/engine/operations"
	"github.com/prulloac/weave/mcp"
)

const blobPNG = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="

func TestMCPBlobResource(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "note.md"), []byte(`---
type: Note
title: "Note"
keywords:
  - note
---

# Note
`), 0o644); err != nil {
		t.Fatal(err)
	}
	data, err := base64.StdEncoding.DecodeString(blobPNG)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "photo.png"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	db := filepath.Join(t.TempDir(), "index.db")
	executor := operations.New("test", engine.Options{})
	result := executor.Execute(context.Background(), contractv1.OperationRequest{
		Op:         contractv1.OpIngest,
		Path:       root,
		DB:         db,
		Strict:     true,
		Assets:     true,
		AssetTypes: []string{"image/png"},
	})
	if _, ok := result.(contractv1.SuccessEnvelope); !ok {
		t.Fatalf("ingest failed: %+v", result)
	}

	server, err := mcp.New(executor, mcp.Options{Bundle: root, DB: db})
	if err != nil {
		t.Fatal(err)
	}
	content, err := server.ReadResource(context.Background(), "weave://blob/photo.png")
	if err != nil {
		t.Fatal(err)
	}
	if content.MimeType != "image/png" {
		t.Fatalf("mime type = %q", content.MimeType)
	}
	got, err := base64.StdEncoding.DecodeString(content.Blob)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(data) {
		t.Fatal("blob resource bytes did not round-trip")
	}
}
