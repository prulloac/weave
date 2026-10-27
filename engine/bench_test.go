package engine_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine"
)

func makeBenchBundle(b *testing.B, count int) string {
	b.Helper()

	root := b.TempDir()
	for index := 0; index < count; index++ {
		next := (index + 1) % count
		content := fmt.Sprintf(`---
type: Concept
title: "Concept %d"
tags:
  - bench
---

# Concept %d

Graph search benchmark. See [next](concept-%d.md).
`, index, index, next)
		if err := os.WriteFile(filepath.Join(root, fmt.Sprintf("concept-%d.md", index)), []byte(content), 0o644); err != nil {
			b.Fatal(err)
		}
	}
	return root
}

func BenchmarkParseBundle(b *testing.B) {
	root := makeBenchBundle(b, 1000)
	b.ResetTimer()

	for index := 0; index < b.N; index++ {
		if _, err := engine.Open(context.Background(), root, engine.Options{}); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSearch(b *testing.B) {
	root := makeBenchBundle(b, 1000)
	eng, err := engine.Open(context.Background(), root, engine.Options{})
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()

	for index := 0; index < b.N; index++ {
		response, err := eng.Search("graph", contractv1.SearchHybrid, 20)
		if err != nil {
			b.Fatal(err)
		}
		if len(response.Hybrid) == 0 {
			b.Fatal("expected results")
		}
	}
}
