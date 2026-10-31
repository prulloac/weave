package sqlite_test

import (
	"context"
	"fmt"
	"path/filepath"
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
