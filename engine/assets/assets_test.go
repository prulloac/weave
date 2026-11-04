package assets

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

const pngBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="

func TestScanClassifiesAssets(t *testing.T) {
	root := t.TempDir()
	png, _ := base64.StdEncoding.DecodeString(pngBase64)
	if err := os.WriteFile(filepath.Join(root, "photo.png"), png, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "data.bin"), []byte{0, 1, 2, 3}, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git", "ignored.png"), png, 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := Scan(context.Background(), root, Filter{Types: []string{"image/*", "text/plain", "application/octet-stream"}})
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]struct {
		media string
		text  bool
	}{}
	for _, concept := range result.Concepts {
		byID[concept.ID] = struct {
			media string
			text  bool
		}{concept.MediaType, concept.BodyText}
	}
	if len(byID) != 3 {
		t.Fatalf("concepts = %v", byID)
	}
	if got := byID["photo.png"]; got.media != "image/png" || got.text {
		t.Fatalf("photo = %+v", got)
	}
	if got := byID["notes.txt"]; got.media != "text/plain" || !got.text {
		t.Fatalf("notes = %+v", got)
	}
	if got := byID["data.bin"]; got.media != "application/octet-stream" || got.text {
		t.Fatalf("data = %+v", got)
	}
}

func TestScanWarnsOnMediaMismatch(t *testing.T) {
	root := t.TempDir()
	png, _ := base64.StdEncoding.DecodeString(pngBase64)
	if err := os.WriteFile(filepath.Join(root, "photo.jpg"), png, 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Scan(context.Background(), root, Filter{Types: []string{"image/*"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Concepts) != 1 || result.Concepts[0].MediaType != "image/png" {
		t.Fatalf("concepts = %+v", result.Concepts)
	}
	if len(result.Warnings) != 1 {
		t.Fatalf("warnings = %v", result.Warnings)
	}
}

func TestFilterMatchesWildcard(t *testing.T) {
	filter := Filter{Types: []string{"image/*", "application/pdf"}}
	if !filter.Matches("image/webp") || !filter.Matches("application/pdf") {
		t.Fatal("expected matches")
	}
	if filter.Matches("text/plain") {
		t.Fatal("unexpected text match")
	}
}
