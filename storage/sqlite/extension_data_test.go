package sqlite_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/prulloac/weave/engine/extensions"
	sqlitestore "github.com/prulloac/weave/storage/sqlite"
)

func extensionDataSource(t *testing.T, content string) (string, string, int64) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "source.bin")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(content))
	return path, hex.EncodeToString(sum[:]), int64(len(content))
}

func countExtensionBlobs(t *testing.T, dbPath string) int {
	t.Helper()
	root := filepath.Join(filepath.Dir(dbPath), "weave-extension-data")
	count := 0
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if !entry.IsDir() {
			count++
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return count
}

func TestExtensionDataRoundTrip(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "index.db")
	store, err := sqlitestore.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	root := t.TempDir()
	source, hash, size := extensionDataSource(t, "image-bytes")
	file := extensions.DataFile{
		Key:        "images/a.bin",
		MediaType:  "application/octet-stream",
		Metadata:   json.RawMessage(`{"caption":"A"}`),
		SHA256:     hash,
		Size:       size,
		SourcePath: source,
	}

	if err := store.ReplaceExtensionData(context.Background(), root, "images", "doc-a", []extensions.DataFile{file}); err != nil {
		t.Fatal(err)
	}
	records, err := store.ListExtensionData(context.Background(), root, sqlitestore.ExtensionDataFilter{Extension: "images", DocID: "doc-a"})
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("expected one record, got %+v", records)
	}
	record := records[0]
	if record.Key != file.Key || record.MediaType != file.MediaType || record.SHA256 != hash || record.Size != size {
		t.Fatalf("unexpected record: %+v", record)
	}
	if string(record.Metadata) != `{"caption":"A"}` {
		t.Fatalf("unexpected metadata: %s", record.Metadata)
	}

	data, _, err := store.ReadExtensionData(context.Background(), root, "images", "doc-a", file.Key)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "image-bytes" {
		t.Fatalf("unexpected bytes: %q", data)
	}
	if blobs := countExtensionBlobs(t, dbPath); blobs != 1 {
		t.Fatalf("expected one blob, got %d", blobs)
	}

	if err := store.ReplaceExtensionData(context.Background(), root, "images", "doc-a", nil); err != nil {
		t.Fatal(err)
	}
	records, err = store.ListExtensionData(context.Background(), root, sqlitestore.ExtensionDataFilter{Extension: "images", DocID: "doc-a"})
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 0 {
		t.Fatalf("expected empty data set, got %+v", records)
	}
	removed, err := store.GCExtensionData(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 1 {
		t.Fatalf("expected one removed blob, got %d", removed)
	}
}

func TestExtensionDataDeduplicatesBlobs(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "index.db")
	store, err := sqlitestore.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	root := t.TempDir()
	source, hash, size := extensionDataSource(t, "same-bytes")
	file := extensions.DataFile{Key: "a", SHA256: hash, Size: size, SourcePath: source}

	if err := store.ReplaceExtensionData(context.Background(), root, "images", "doc-a", []extensions.DataFile{file}); err != nil {
		t.Fatal(err)
	}
	if err := store.ReplaceExtensionData(context.Background(), root, "images", "doc-b", []extensions.DataFile{file}); err != nil {
		t.Fatal(err)
	}
	if blobs := countExtensionBlobs(t, dbPath); blobs != 1 {
		t.Fatalf("expected one deduplicated blob, got %d", blobs)
	}
	records, err := store.ListExtensionData(context.Background(), root, sqlitestore.ExtensionDataFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 {
		t.Fatalf("expected two records, got %+v", records)
	}
}

func TestExtensionDataRejectsHashMismatch(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "index.db")
	store, err := sqlitestore.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	root := t.TempDir()
	source, _, size := extensionDataSource(t, "bytes")
	file := extensions.DataFile{Key: "a", SHA256: "deadbeef", Size: size, SourcePath: source}
	if err := store.ReplaceExtensionData(context.Background(), root, "images", "doc-a", []extensions.DataFile{file}); err == nil {
		t.Fatal("expected a hash mismatch")
	}
	records, err := store.ListExtensionData(context.Background(), root, sqlitestore.ExtensionDataFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 0 {
		t.Fatalf("expected no records, got %+v", records)
	}
}

func TestExtensionDataDeleteBundle(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "index.db")
	store, err := sqlitestore.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	root := t.TempDir()
	source, hash, size := extensionDataSource(t, "delete-me")
	file := extensions.DataFile{Key: "a", SHA256: hash, Size: size, SourcePath: source}
	if err := store.ReplaceExtensionData(context.Background(), root, "images", "doc-a", []extensions.DataFile{file}); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteExtensionDataForBundle(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	records, err := store.ListExtensionData(context.Background(), root, sqlitestore.ExtensionDataFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 0 {
		t.Fatalf("expected no records, got %+v", records)
	}
	if removed, err := store.GCExtensionData(context.Background(), root); err != nil || removed != 1 {
		t.Fatalf("gc = %d, %v", removed, err)
	}
}
