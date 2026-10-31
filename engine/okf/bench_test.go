package okf_test

import (
	"context"
	"testing"

	"github.com/prulloac/weave/benchmarks/benchutil"
	"github.com/prulloac/weave/engine/okf"
)

func BenchmarkParseCorpus(b *testing.B) {
	for _, name := range []string{"1k", "10k"} {
		name := name
		b.Run("corpus="+name, func(b *testing.B) {
			root := benchutil.RequireCorpus(b, name)
			manifest := benchutil.ReadManifest(b)
			b.ReportAllocs()
			b.SetBytes(manifest.Profiles[name].BodyBytes)

			b.Run("cache=off", func(b *testing.B) {
				for index := 0; index < b.N; index++ {
					bundle, err := okf.ParseBundle(context.Background(), root, okf.Options{Cache: false})
					if err != nil {
						b.Fatal(err)
					}
					if len(bundle.Concepts) == 0 {
						b.Fatal("empty bundle")
					}
				}
			})

			b.Run("cache=on", func(b *testing.B) {
				cacheDir := b.TempDir()
				if _, err := okf.ParseBundle(context.Background(), root, okf.Options{
					Cache: true, CacheDir: cacheDir,
				}); err != nil {
					b.Fatal(err)
				}
				b.ResetTimer()
				for index := 0; index < b.N; index++ {
					bundle, err := okf.ParseBundle(context.Background(), root, okf.Options{
						Cache: true, CacheDir: cacheDir,
					})
					if err != nil {
						b.Fatal(err)
					}
					if len(bundle.Concepts) == 0 {
						b.Fatal("empty bundle")
					}
				}
			})
		})
	}
}
