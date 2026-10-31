package sqlite_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/prulloac/weave/benchmarks/benchutil"
	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine/okf"
	sqlitestore "github.com/prulloac/weave/storage/sqlite"
)

func corpusBundle(b *testing.B, name string) *contractv1.Bundle {
	b.Helper()
	root := benchutil.RequireCorpus(b, name)
	bundle, err := okf.ParseBundle(context.Background(), root, okf.Options{Cache: false})
	if err != nil {
		b.Fatal(err)
	}
	if len(bundle.Concepts) == 0 {
		b.Fatal("empty bundle")
	}
	return bundle
}

func BenchmarkIngestCorpus(b *testing.B) {
	bundle := corpusBundle(b, "10k")
	b.ReportAllocs()
	for index := 0; index < b.N; index++ {
		db := filepath.Join(b.TempDir(), fmt.Sprintf("index-%d.db", index))
		store, err := sqlitestore.Open(db)
		if err != nil {
			b.Fatal(err)
		}
		if _, err := store.Ingest(context.Background(), bundle, sqlitestore.IngestOptions{
			Mode: "strict", Codec: "zstd",
		}); err != nil {
			b.Fatal(err)
		}
		if err := store.Close(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLoadCorpus(b *testing.B) {
	bundle := corpusBundle(b, "10k")
	db := filepath.Join(b.TempDir(), "index.db")
	store, err := sqlitestore.Open(db)
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()
	if _, err := store.Ingest(context.Background(), bundle, sqlitestore.IngestOptions{
		Mode: "strict", Codec: "zstd",
	}); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		if _, err := store.LoadBundle(context.Background(), bundle.Root); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBM25Corpus(b *testing.B) {
	bundle := corpusBundle(b, "10k")
	db := filepath.Join(b.TempDir(), "index.db")
	store, err := sqlitestore.Open(db)
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()
	if _, err := store.Ingest(context.Background(), bundle, sqlitestore.IngestOptions{
		Mode: "strict", Codec: "zstd",
	}); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		if _, err := store.SearchBM25(context.Background(), bundle.Root, "term0001 term0002", 10); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFuzzyHitsCorpus(b *testing.B) {
	bundle := corpusBundle(b, "10k")
	db := filepath.Join(b.TempDir(), "index.db")
	store, err := sqlitestore.Open(db)
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()
	if _, err := store.Ingest(context.Background(), bundle, sqlitestore.IngestOptions{
		Mode: "strict", Codec: "zstd",
	}); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		if _, err := store.FuzzyHits(context.Background(), bundle.Root, "term0001x", 10); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSync1PctCorpus(b *testing.B) {
	source := benchutil.RequireCorpus(b, "10k")
	root := filepath.Join(b.TempDir(), "corpus")
	if err := copyBenchTree(source, root); err != nil {
		b.Fatal(err)
	}
	bundle, err := okf.ParseBundle(context.Background(), root, okf.Options{Cache: false})
	if err != nil {
		b.Fatal(err)
	}
	db := filepath.Join(b.TempDir(), "index.db")
	store, err := sqlitestore.Open(db)
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()
	if _, err := store.Ingest(context.Background(), bundle, sqlitestore.IngestOptions{
		Mode: "strict", Codec: "zstd",
	}); err != nil {
		b.Fatal(err)
	}
	if err := modifyBenchPercent(root, 1); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		if _, err := store.Sync(context.Background(), root, sqlitestore.SyncOptions{}); err != nil {
			b.Fatal(err)
		}
	}
}

func copyBenchTree(source, target string) error {
	return filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(target, relative)
		if info.IsDir() {
			return os.MkdirAll(destination, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(destination, data, 0o644)
	})
}

func modifyBenchPercent(root string, percent int) error {
	files := make([]string, 0)
	if err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".md") {
			files = append(files, path)
		}
		return nil
	}); err != nil {
		return err
	}
	count := len(files) * percent / 100
	for index := 0; index < count; index++ {
		file, err := os.OpenFile(files[index], os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		if _, err := file.WriteString("\n<!-- bench change -->\n"); err != nil {
			_ = file.Close()
			return err
		}
		if err := file.Close(); err != nil {
			return err
		}
	}
	return nil
}

func BenchmarkSyncNoChangeCorpus(b *testing.B) {
	bundle := corpusBundle(b, "10k")
	db := filepath.Join(b.TempDir(), "index.db")
	store, err := sqlitestore.Open(db)
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()
	if _, err := store.Ingest(context.Background(), bundle, sqlitestore.IngestOptions{
		Mode: "strict", Codec: "zstd",
	}); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		if _, err := store.Sync(context.Background(), bundle.Root, sqlitestore.SyncOptions{}); err != nil {
			b.Fatal(err)
		}
	}
}
