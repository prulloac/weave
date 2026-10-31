package main

import (
	"encoding/json"
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
