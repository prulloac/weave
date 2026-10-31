package search_test

import (
	"context"
	"testing"

	"github.com/prulloac/weave/benchmarks/benchutil"
	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine"
	"github.com/prulloac/weave/engine/nav"
	"github.com/prulloac/weave/engine/okf"
	"github.com/prulloac/weave/engine/relate"
)

func openCorpus(b *testing.B, name string) *engine.Engine {
	b.Helper()
	root := benchutil.RequireCorpus(b, name)
	bundle, err := okf.ParseBundle(context.Background(), root, okf.Options{Cache: false})
	if err != nil {
		b.Fatal(err)
	}
	if len(bundle.Concepts) == 0 {
		b.Fatal("empty bundle")
	}
	eng := engine.FromBundle(bundle)
	eng.SearchIndex()
	return eng
}

func BenchmarkSearchCorpus(b *testing.B) {
	eng := openCorpus(b, "10k")
	manifest := benchutil.ReadManifest(b)
	modes := []struct {
		name string
		mode contractv1.SearchMode
	}{
		{"keyword", contractv1.SearchKeyword},
		{"fuzzy", contractv1.SearchFuzzy},
		{"hybrid", contractv1.SearchHybrid},
	}
	for _, item := range modes {
		item := item
		b.Run("mode="+item.name, func(b *testing.B) {
			query := "term0001 term0002"
			if item.mode == contractv1.SearchKeyword {
				query = "kw000 kw001"
			}
			b.ReportAllocs()
			b.SetBytes(manifest.Profiles["10k"].BodyBytes)
			for index := 0; index < b.N; index++ {
				response, err := eng.Search(query, item.mode, 10)
				if err != nil {
					b.Fatal(err)
				}
				switch item.mode {
				case contractv1.SearchKeyword:
					if len(response.Keyword) == 0 {
						b.Fatal("empty keyword response")
					}
				case contractv1.SearchFuzzy:
					if len(response.Fuzzy) == 0 {
						b.Fatal("empty fuzzy response")
					}
				default:
					if len(response.Hybrid) == 0 {
						b.Fatal("empty hybrid response")
					}
				}
			}
		})
	}
}

func BenchmarkSearchBuild10K(b *testing.B) {
	root := benchutil.RequireCorpus(b, "10k")
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		bundle, err := okf.ParseBundle(context.Background(), root, okf.Options{Cache: false})
		if err != nil {
			b.Fatal(err)
		}
		eng := engine.FromBundle(bundle)
		if eng == nil || eng.SearchIndex() == nil {
			b.Fatal("nil engine or index")
		}
	}
}

func BenchmarkNavigation10K(b *testing.B) {
	eng := openCorpus(b, "10k")
	b.Run("list", func(b *testing.B) {
		b.ReportAllocs()
		for index := 0; index < b.N; index++ {
			if len(eng.List(nav.ListFilters{Limit: 20})) == 0 {
				b.Fatal("empty list")
			}
		}
	})
	b.Run("show", func(b *testing.B) {
		b.ReportAllocs()
		for index := 0; index < b.N; index++ {
			if _, ok := eng.Show("doc-00001"); !ok {
				b.Fatal("missing concept")
			}
		}
	})
	b.Run("backlinks", func(b *testing.B) {
		b.ReportAllocs()
		for index := 0; index < b.N; index++ {
			if _, ok := eng.Backlinks("doc-00001"); !ok {
				b.Fatal("missing concept")
			}
		}
	})
	b.Run("relate", func(b *testing.B) {
		b.ReportAllocs()
		for index := 0; index < b.N; index++ {
			if _, err := relate.Build(eng.Graph(), "doc-00001", contractv1.RelateBoth, 2); err != nil {
				b.Fatal(err)
			}
		}
	})
}
