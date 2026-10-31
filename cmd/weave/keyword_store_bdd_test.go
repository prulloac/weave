package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
)

func TestKeywordStoreMatchesInMemory(t *testing.T) {
	root := copyStorageBundle(t, storageFixture("bundle-a"))
	db := filepath.Join(t.TempDir(), "index.db")
	if code, _, stderr := Invoke(t, "ingest", root, "--db", db, "--strict", "--json"); code != 0 {
		t.Fatalf("ingest exit=%d stderr=%q", code, stderr)
	}

	unindexed := copyStorageBundle(t, storageFixture("bundle-b"))
	code, stdout, _ := Invoke(t, "search", unindexed, "alpha", "--db", db, "--mode", "keyword", "--json")
	if code != 1 {
		t.Fatalf("unindexed keyword search exit=%d", code)
	}
	if envelope := decodeEnvelope(t, stdout); envelope.Error == nil || envelope.Error.Code != contractv1.ErrBundleNotIndexed {
		t.Fatalf("expected BUNDLE_NOT_INDEXED, got %+v", envelope.Error)
	}

	for _, query := range []string{"alpha", "alp", "alpha beta", "beta", "missing"} {
		code, persistentOut, stderr := Invoke(t, "search", root, query, "--db", db, "--mode", "keyword", "--json")
		if code != 0 && code != 1 {
			t.Fatalf("%s persistent exit=%d stderr=%q", query, code, stderr)
		}
		code, memoryOut, stderr := Invoke(t, "search", root, query, "--no-db", "--mode", "keyword", "--json")
		if code != 0 && code != 1 {
			t.Fatalf("%s memory exit=%d stderr=%q", query, code, stderr)
		}
		var persistent, memory contractv1.SearchResponse
		if err := json.Unmarshal(decodeEnvelope(t, persistentOut).Result, &persistent); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(decodeEnvelope(t, memoryOut).Result, &memory); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(persistent.Keyword, memory.Keyword) {
			t.Fatalf("%s keyword mismatch\npersistent: %+v\nmemory: %+v", query, persistent.Keyword, memory.Keyword)
		}
		if !reflect.DeepEqual(persistent.Hybrid, memory.Hybrid) {
			t.Fatalf("%s hybrid mismatch\npersistent: %+v\nmemory: %+v", query, persistent.Hybrid, memory.Hybrid)
		}
	}
}

func TestIncrementalSyncKeepsSearchParity(t *testing.T) {
	root := copyStorageBundle(t, storageFixture("bundle-a"))
	incrementalDB := filepath.Join(t.TempDir(), "incremental.db")
	if code, _, stderr := Invoke(t, "ingest", root, "--db", incrementalDB, "--strict", "--json"); code != 0 {
		t.Fatalf("ingest exit=%d stderr=%q", code, stderr)
	}

	content := "---\ntype: Concept\ntitle: \"B Updated\"\nkeywords:\n  - \"beta\"\n  - \"delta\"\n---\n\n# B Updated\n\nbeta updated search body\n"
	if err := os.WriteFile(filepath.Join(root, "b.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := Invoke(t, "sync", root, "--db", incrementalDB, "--json"); code != 0 {
		t.Fatalf("sync exit=%d stderr=%q", code, stderr)
	}

	fullDB := filepath.Join(t.TempDir(), "full.db")
	if code, _, stderr := Invoke(t, "ingest", root, "--db", fullDB, "--strict", "--json"); code != 0 {
		t.Fatalf("full ingest exit=%d stderr=%q", code, stderr)
	}

	for _, mode := range []string{"keyword", "fuzzy", "bm25"} {
		code, incrementalOut, stderr := Invoke(t, "search", root, "beta", "--db", incrementalDB, "--mode", mode, "--json")
		if code != 0 {
			t.Fatalf("%s incremental exit=%d stderr=%q", mode, code, stderr)
		}
		code, fullOut, stderr := Invoke(t, "search", root, "beta", "--db", fullDB, "--mode", mode, "--json")
		if code != 0 {
			t.Fatalf("%s full exit=%d stderr=%q", mode, code, stderr)
		}
		var incremental, full contractv1.SearchResponse
		if err := json.Unmarshal(decodeEnvelope(t, incrementalOut).Result, &incremental); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(decodeEnvelope(t, fullOut).Result, &full); err != nil {
			t.Fatal(err)
		}
		for index := range incremental.Fuzzy {
			incremental.Fuzzy[index].Snippet = nil
		}
		for index := range full.Fuzzy {
			full.Fuzzy[index].Snippet = nil
		}
		switch mode {
		case "keyword":
			if !reflect.DeepEqual(incremental.Keyword, full.Keyword) {
				t.Fatalf("keyword parity failed\nincremental: %+v\nfull: %+v", incremental.Keyword, full.Keyword)
			}
		case "fuzzy":
			if !reflect.DeepEqual(incremental.Fuzzy, full.Fuzzy) {
				t.Fatalf("fuzzy parity failed\nincremental: %+v\nfull: %+v", incremental.Fuzzy, full.Fuzzy)
			}
		case "bm25":
			if !reflect.DeepEqual(incremental.Lexical, full.Lexical) {
				t.Fatalf("bm25 parity failed\nincremental: %+v\nfull: %+v", incremental.Lexical, full.Lexical)
			}
		}
	}
}

func TestPersistentDefaultMatchesInMemory(t *testing.T) {
	root := copyStorageBundle(t, storageFixture("bundle-a"))
	db := filepath.Join(t.TempDir(), "index.db")
	if code, _, stderr := Invoke(t, "ingest", root, "--db", db, "--strict", "--json"); code != 0 {
		t.Fatalf("ingest exit=%d stderr=%q", code, stderr)
	}
	for _, query := range []string{"alpha", "alpa", "alpha beta"} {
		code, persistentOut, stderr := Invoke(t, "search", root, query, "--db", db, "--json")
		if code != 0 && code != 1 {
			t.Fatalf("%s persistent exit=%d stderr=%q", query, code, stderr)
		}
		code, memoryOut, stderr := Invoke(t, "search", root, query, "--no-db", "--json")
		if code != 0 && code != 1 {
			t.Fatalf("%s memory exit=%d stderr=%q", query, code, stderr)
		}
		var persistent, memory contractv1.SearchResponse
		if err := json.Unmarshal(decodeEnvelope(t, persistentOut).Result, &persistent); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(decodeEnvelope(t, memoryOut).Result, &memory); err != nil {
			t.Fatal(err)
		}
		for index := range persistent.Hybrid {
			persistent.Hybrid[index].Snippet = nil
		}
		for index := range memory.Hybrid {
			memory.Hybrid[index].Snippet = nil
		}
		if !reflect.DeepEqual(persistent.Keyword, memory.Keyword) ||
			!reflect.DeepEqual(persistent.Hybrid, memory.Hybrid) {
			t.Fatalf("%s default mismatch\npersistent: %+v\nmemory: %+v", query, persistent, memory)
		}
	}
}

func TestPersistentFuzzyMatchesInMemory(t *testing.T) {
	root := copyStorageBundle(t, storageFixture("bundle-a"))
	db := filepath.Join(t.TempDir(), "index.db")
	if code, _, stderr := Invoke(t, "ingest", root, "--db", db, "--strict", "--json"); code != 0 {
		t.Fatalf("ingest exit=%d stderr=%q", code, stderr)
	}

	for _, query := range []string{"alpha", "alpa", "beta"} {
		code, persistentOut, stderr := Invoke(t, "search", root, query, "--db", db, "--mode", "fuzzy", "--json")
		if code != 0 && code != 1 {
			t.Fatalf("%s persistent exit=%d stderr=%q", query, code, stderr)
		}
		code, memoryOut, stderr := Invoke(t, "search", root, query, "--no-db", "--mode", "fuzzy", "--json")
		if code != 0 && code != 1 {
			t.Fatalf("%s memory exit=%d stderr=%q", query, code, stderr)
		}
		var persistent, memory contractv1.SearchResponse
		if err := json.Unmarshal(decodeEnvelope(t, persistentOut).Result, &persistent); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(decodeEnvelope(t, memoryOut).Result, &memory); err != nil {
			t.Fatal(err)
		}
		for index := range persistent.Fuzzy {
			persistent.Fuzzy[index].Snippet = nil
		}
		for index := range memory.Fuzzy {
			memory.Fuzzy[index].Snippet = nil
		}
		if !reflect.DeepEqual(persistent.Fuzzy, memory.Fuzzy) {
			t.Fatalf("%s fuzzy mismatch\npersistent: %+v\nmemory: %+v", query, persistent.Fuzzy, memory.Fuzzy)
		}
	}
}
