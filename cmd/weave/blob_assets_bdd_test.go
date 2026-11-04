package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
)

// 1x1 transparent PNG.
const onePixelPNG = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="

func blobBundle(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "note.md"), []byte(`---
type: Note
title: "Note"
keywords:
  - note
---

# Note

See [photo](photo.png).
`), 0o644); err != nil {
		t.Fatal(err)
	}
	image, err := base64.StdEncoding.DecodeString(onePixelPNG)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "photo.png"), image, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("opening hours are nine to five\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func blobIngest(t *testing.T, root, db string, extra ...string) contractv1.IngestReport {
	t.Helper()
	args := append([]string{"ingest", root, "--db", db}, extra...)
	args = append(args, "--json")
	code, stdout, stderr := Invoke(t, args...)
	if code != 0 || stderr != "" {
		t.Fatalf("ingest exit=%d stderr=%q stdout=%s", code, stderr, stdout)
	}
	envelope := decodeEnvelope(t, stdout)
	if !envelope.OK {
		t.Fatalf("ingest failed: %+v", envelope.Error)
	}
	var report contractv1.IngestReport
	if err := json.Unmarshal(envelope.Result, &report); err != nil {
		t.Fatal(err)
	}
	return report
}

func TestBlobIngestAndShow(t *testing.T) {
	root := blobBundle(t)
	db := filepath.Join(t.TempDir(), "index.db")
	report := blobIngest(t, root, db, "--assets", "--asset-types", "image/png")
	if report.Documents != 2 {
		t.Fatalf("documents = %d, want 2", report.Documents)
	}

	code, stdout, stderr := Invoke(t, "show", root, "photo.png", "--db", db, "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("show exit=%d stderr=%q", code, stderr)
	}
	var view contractv1.ShowResult
	if err := json.Unmarshal(decodeEnvelope(t, stdout).Result, &view); err != nil {
		t.Fatal(err)
	}
	if view.ContentKind != "blob" || view.MediaType != "image/png" {
		t.Fatalf("unexpected blob view: %+v", view)
	}
	if view.BodyForm != "compressed" || view.BodyCodec != "zstd" {
		t.Fatalf("unexpected body form: %+v", view)
	}
	if view.Size == 0 || view.StoredSize == 0 || view.ContentHash == "" {
		t.Fatalf("missing size/hash: %+v", view)
	}
	if view.Body != "" {
		t.Fatalf("blob JSON must not embed bytes, got %d body bytes", len(view.Body))
	}
	if view.BlobURI != "weave://blob/photo.png" {
		t.Fatalf("blob uri = %q", view.BlobURI)
	}

	code, stdout, stderr = Invoke(t, "list", root, "--db", db, "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("list exit=%d stderr=%q", code, stderr)
	}
	var listed contractv1.ListResults
	if err := json.Unmarshal(decodeEnvelope(t, stdout).Result, &listed); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range listed {
		if item.ID == "photo.png" {
			found = true
			if item.Title != "photo.png" || item.ContentKind != "blob" || item.MediaType != "image/png" {
				t.Fatalf("unexpected blob list item: %+v", item)
			}
		}
	}
	if !found {
		t.Fatalf("photo.png missing from list: %+v", listed)
	}

	out := filepath.Join(t.TempDir(), "photo.png")
	code, _, stderr = Invoke(t, "show", root, "photo.png", "--db", db, "--out", out, "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("show --out exit=%d stderr=%q", code, stderr)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := base64.StdEncoding.DecodeString(onePixelPNG)
	if !bytes.Equal(got, want) {
		t.Fatalf("blob bytes did not round-trip")
	}

	code, _, _ = Invoke(t, "show", root, "photo.png", "--db", db, "--out", out, "--json")
	if code != 2 {
		t.Fatalf("existing output without --force exit=%d, want 2", code)
	}
	code, _, stderr = Invoke(t, "show", root, "photo.png", "--db", db, "--out", out, "--force", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("show --force exit=%d stderr=%q", code, stderr)
	}
}

func TestTextBlobIsStoredRawAndSearchable(t *testing.T) {
	root := blobBundle(t)
	db := filepath.Join(t.TempDir(), "index.db")
	blobIngest(t, root, db, "--assets", "--asset-types", "text/plain")

	code, stdout, stderr := Invoke(t, "show", root, "notes.txt", "--db", db, "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("show exit=%d stderr=%q", code, stderr)
	}
	var view contractv1.ShowResult
	if err := json.Unmarshal(decodeEnvelope(t, stdout).Result, &view); err != nil {
		t.Fatal(err)
	}
	if view.BodyForm != "raw" || view.BodyCodec != "" {
		t.Fatalf("unexpected text body form: %+v", view)
	}
	if view.Body != "opening hours are nine to five\n" {
		t.Fatalf("text blob body = %q", view.Body)
	}

	code, stdout, stderr = Invoke(t, "search", root, "opening", "--db", db, "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("search exit=%d stderr=%q", code, stderr)
	}
	var response contractv1.SearchResponse
	if err := json.Unmarshal(decodeEnvelope(t, stdout).Result, &response); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, hit := range response.Hybrid {
		if hit.ID == "notes.txt" {
			found = true
		}
	}
	if !found {
		t.Fatalf("text blob not searchable: %+v", response)
	}
}

func TestAssetFilterPersistsAcrossSync(t *testing.T) {
	root := blobBundle(t)
	db := filepath.Join(t.TempDir(), "index.db")
	blobIngest(t, root, db, "--assets", "--asset-types", "image/png")

	path := filepath.Join(root, "photo.png")
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	updated := append(append([]byte{}, original...), []byte("updated")...)
	if err := os.WriteFile(path, updated, 0o644); err != nil {
		t.Fatal(err)
	}
	second, _ := base64.StdEncoding.DecodeString(onePixelPNG)
	if err := os.WriteFile(filepath.Join(root, "second.png"), second, 0o644); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := Invoke(t, "sync", root, "--db", db, "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("sync exit=%d stderr=%q stdout=%s", code, stderr, stdout)
	}
	out := filepath.Join(t.TempDir(), "photo.png")
	code, _, stderr = Invoke(t, "show", root, "photo.png", "--db", db, "--out", out, "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("show updated exit=%d stderr=%q", code, stderr)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, updated) {
		t.Fatal("sync did not update the blob bytes")
	}
	code, _, _ = Invoke(t, "show", root, "second.png", "--db", db, "--json")
	if code != 0 {
		t.Fatal("sync did not add the new blob")
	}

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	code, _, stderr = Invoke(t, "sync", root, "--db", db, "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("sync delete exit=%d stderr=%q", code, stderr)
	}
	code, _, _ = Invoke(t, "show", root, "photo.png", "--db", db, "--json")
	if code == 0 {
		t.Fatal("sync did not remove the deleted blob")
	}
}

func TestRawExportWritesBlobBytes(t *testing.T) {
	root := blobBundle(t)
	db := filepath.Join(t.TempDir(), "index.db")
	blobIngest(t, root, db, "--assets", "--asset-types", "image/png")
	out := filepath.Join(t.TempDir(), "export")

	code, stdout, stderr := Invoke(t, "export", root, "--out", out, "--format", "raw", "--yes", "--db", db, "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("export exit=%d stderr=%q stdout=%s", code, stderr, stdout)
	}
	got, err := os.ReadFile(filepath.Join(out, "photo.png"))
	if err != nil {
		t.Fatal(err)
	}
	want, _ := base64.StdEncoding.DecodeString(onePixelPNG)
	if !bytes.Equal(got, want) {
		t.Fatal("raw export did not preserve blob bytes")
	}
}

func TestAssetsRequireTypes(t *testing.T) {
	root := blobBundle(t)
	db := filepath.Join(t.TempDir(), "index.db")
	code, stdout, _ := Invoke(t, "ingest", root, "--db", db, "--assets", "--json")
	if code != 2 {
		t.Fatalf("exit=%d, want 2", code)
	}
	envelope := decodeEnvelope(t, stdout)
	if envelope.Error == nil || envelope.Error.Code != "ASSET_TYPES_REQUIRED" {
		t.Fatalf("unexpected error: %+v", envelope.Error)
	}
}

func TestNoAssetsRemovesBlobRows(t *testing.T) {
	root := blobBundle(t)
	db := filepath.Join(t.TempDir(), "index.db")
	blobIngest(t, root, db, "--assets", "--asset-types", "image/png,text/plain")

	code, stdout, stderr := Invoke(t, "rebuild", root, "--db", db, "--no-assets", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("rebuild exit=%d stderr=%q stdout=%s", code, stderr, stdout)
	}
	code, _, _ = Invoke(t, "show", root, "photo.png", "--db", db, "--json")
	if code == 0 {
		t.Fatal("blob must be removed by --no-assets")
	}
}
