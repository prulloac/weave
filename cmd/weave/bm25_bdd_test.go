package main

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
)

// Behavior tests for BM25 lexical ranking. Hybrid rebase and BM25 explain are
// covered by the calibration slice.

type bddBM25Response struct {
	Query   string          `json:"query"`
	Mode    string          `json:"mode"`
	TopK    int             `json:"top_k"`
	Lexical []bddLexicalHit `json:"lexical"`
}

type bddLexicalHit struct {
	ID       string  `json:"id"`
	Title    string  `json:"title"`
	Path     string  `json:"path"`
	Type     string  `json:"type"`
	RawScore float64 `json:"raw_score"`
	Score    float64 `json:"score"`
}

func bm25Fixture() string {
	return filepath.Join("testdata", "bm25", "bundle")
}

func ingestBM25(t *testing.T, bundle string) {
	t.Helper()
	storageHome(t)
	runStorageJSON(t, "ingest", bundle, "--permissive", "--json")
}

func runBM25(t *testing.T, bundle, query string, extra ...string) (int, bddBM25Response) {
	t.Helper()

	args := []string{"search", bundle, query, "--mode", "bm25", "--json"}
	args = append(args, extra...)
	code, stdout, stderr := invokePersistent(t, args...)
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	envelope := decodeEnvelope(t, stdout)
	if !envelope.OK {
		t.Fatalf("expected BM25 results, got failure: %+v", envelope.Error)
	}
	var response bddBM25Response
	if err := json.Unmarshal(envelope.Result, &response); err != nil {
		t.Fatalf("decode BM25 response: %v\n%s", err, envelope.Result)
	}
	return code, response
}

func TestBM25Behavior(t *testing.T) {
	t.Run("ingest indexes valid and synthetic documents", func(t *testing.T) {
		bundle := bm25Fixture()
		ingestBM25(t, bundle)

		code, response := runBM25(t, bundle, "alpha")
		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
		ids := lexicalIDs(response)
		for _, want := range []string{"title-alpha", "synthetic"} {
			if !slices.Contains(ids, want) {
				t.Fatalf("expected %s, got %v", want, ids)
			}
		}
		for _, hit := range response.Lexical {
			if hit.RawScore <= 0 {
				t.Fatalf("expected positive raw score: %+v", hit)
			}
			if hit.Score < 0 || hit.Score > 1 {
				t.Fatalf("score out of range: %+v", hit)
			}
		}
	})

	t.Run("sync updates the FTS index", func(t *testing.T) {
		bundle := copyStorageBundle(t, bm25Fixture())
		ingestBM25(t, bundle)

		writeSyncFile(t, filepath.Join(bundle, "beta-only.md"), `---
type: Concept
title: "Beta Only"
keywords:
  - "beta-only"
---

# Beta Only

Alpha content after sync.
`)
		runSyncJSON(t, bundle, "--permissive", "--json")

		_, response := runBM25(t, bundle, "alpha")
		if !slices.Contains(lexicalIDs(response), "beta-only") {
			t.Fatalf("beta-only missing after sync: %v", lexicalIDs(response))
		}
	})

	t.Run("documents matching a subset of terms are included", func(t *testing.T) {
		bundle := bm25Fixture()
		ingestBM25(t, bundle)

		_, response := runBM25(t, bundle, "alpha beta")
		ids := lexicalIDs(response)
		for _, want := range []string{"alpha-only", "beta-only", "both"} {
			if !slices.Contains(ids, want) {
				t.Fatalf("expected %s, got %v", want, ids)
			}
		}
	})

	t.Run("title matches outweigh body matches", func(t *testing.T) {
		bundle := bm25Fixture()
		ingestBM25(t, bundle)

		_, response := runBM25(t, bundle, "alpha")
		title := slices.Index(lexicalIDs(response), "title-alpha")
		body := slices.Index(lexicalIDs(response), "body-alpha")
		if title < 0 || body < 0 {
			t.Fatalf("missing title/body docs: %v", lexicalIDs(response))
		}
		if title >= body {
			t.Fatalf("title-alpha rank=%d, body-alpha rank=%d", title, body)
		}
	})

	t.Run("phrase queries are exact", func(t *testing.T) {
		bundle := bm25Fixture()
		ingestBM25(t, bundle)

		_, response := runBM25(t, bundle, `"red green"`)
		ids := lexicalIDs(response)
		if !slices.Contains(ids, "phrase") {
			t.Fatalf("expected phrase, got %v", ids)
		}
		if slices.Contains(ids, "phrase-reverse") {
			t.Fatalf("phrase-reverse should not match: %v", ids)
		}
	})

	t.Run("negative terms are excluded", func(t *testing.T) {
		bundle := bm25Fixture()
		ingestBM25(t, bundle)

		_, response := runBM25(t, bundle, "alpha -beta")
		ids := lexicalIDs(response)
		for _, want := range []string{"alpha-only", "title-alpha"} {
			if !slices.Contains(ids, want) {
				t.Fatalf("expected %s, got %v", want, ids)
			}
		}
		for _, unwanted := range []string{"both", "beta-only"} {
			if slices.Contains(ids, unwanted) {
				t.Fatalf("%s should be excluded: %v", unwanted, ids)
			}
		}
	})

	t.Run("top-k caps lexical results", func(t *testing.T) {
		bundle := bm25Fixture()
		ingestBM25(t, bundle)

		_, response := runBM25(t, bundle, "alpha", "--top-k", "1")
		if len(response.Lexical) != 1 {
			t.Fatalf("expected 1 lexical hit, got %d", len(response.Lexical))
		}
	})

	t.Run("repeated BM25 searches are deterministic", func(t *testing.T) {
		bundle := bm25Fixture()
		ingestBM25(t, bundle)

		_, first := runBM25(t, bundle, "alpha")
		_, second := runBM25(t, bundle, "alpha")
		if !reflect.DeepEqual(first.Lexical, second.Lexical) {
			t.Fatalf("BM25 results are not deterministic:\nfirst:  %+v\nsecond: %+v", first.Lexical, second.Lexical)
		}
	})

	t.Run("BM25 requires persistent storage", func(t *testing.T) {
		storageHome(t)
		code, stdout, _ := invokePersistent(t, "search", bm25Fixture(), "alpha",
			"--mode", "bm25", "--no-db", "--json")
		if code != 2 {
			t.Fatalf("expected exit 2, got %d", code)
		}
		if failure := storageFailure(t, stdout); failure.Code != "INVALID_REQUEST" {
			t.Fatalf("expected INVALID_REQUEST, got %s", failure.Code)
		}
	})

	t.Run("empty query has no lexical results", func(t *testing.T) {
		bundle := bm25Fixture()
		ingestBM25(t, bundle)

		code, _, _ := invokePersistent(t, "search", bundle, "", "--mode", "bm25", "--json")
		if code != 1 {
			t.Fatalf("expected exit 1, got %d", code)
		}
	})
}

func lexicalIDs(response bddBM25Response) []string {
	ids := make([]string, len(response.Lexical))
	for index, hit := range response.Lexical {
		ids[index] = hit.ID
	}
	return ids
}
