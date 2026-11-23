package main

import (
	"encoding/json"
	"strings"
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
)

// Behavior-level tests for `weave search --where`. See docs/bdd/search-field-filters.md.

func searchFilterResponse(t *testing.T, args ...string) (int, contractv1.SearchResponse, string) {
	t.Helper()
	code, stdout, stderr := InvokeRead(t, args...)
	if code != 0 {
		return code, contractv1.SearchResponse{}, stderr
	}
	envelope := decodeEnvelope(t, stdout)
	if !envelope.OK {
		t.Fatalf("unexpected failure: %+v", envelope.Error)
	}
	var response contractv1.SearchResponse
	if err := json.Unmarshal(envelope.Result, &response); err != nil {
		t.Fatalf("decode search response: %v\n%s", err, stdout)
	}
	return code, response, stderr
}

func allHitTypes(response contractv1.SearchResponse) []string {
	types := make([]string, 0)
	for _, hit := range response.Keyword {
		types = append(types, hit.Type)
	}
	for _, hit := range response.Fuzzy {
		types = append(types, hit.Type)
	}
	for _, hit := range response.Hybrid {
		types = append(types, hit.Type)
	}
	return types
}

func TestSearchFiltersByType(t *testing.T) {
	code, response, stderr := searchFilterResponse(t, "search", fixturePath(), "graph",
		"--where", "type:Concept", "--json")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d stderr=%s", code, stderr)
	}
	if len(response.Hybrid) == 0 {
		t.Fatal("expected hybrid hits")
	}
	for _, hitType := range allHitTypes(response) {
		if hitType != "Concept" {
			t.Fatalf("hit type %q leaked through the filter", hitType)
		}
	}
}

func TestSearchFiltersKeepScoresAndOrder(t *testing.T) {
	_, unfiltered, _ := searchFilterResponse(t, "search", fixturePath(), "graph", "--json")
	_, filtered, _ := searchFilterResponse(t, "search", fixturePath(), "graph",
		"--where", "type:Concept", "--json")

	scores := make(map[string]float64, len(unfiltered.Hybrid))
	order := make([]string, 0, len(unfiltered.Hybrid))
	for _, hit := range unfiltered.Hybrid {
		scores[hit.ID] = hit.TotalScore
		order = append(order, hit.ID)
	}
	kept := make(map[string]bool, len(filtered.Hybrid))
	for _, hit := range filtered.Hybrid {
		if hit.TotalScore != scores[hit.ID] {
			t.Fatalf("score changed for %s: %v != %v", hit.ID, hit.TotalScore, scores[hit.ID])
		}
		kept[hit.ID] = true
	}
	last := -1
	for position, id := range order {
		if !kept[id] {
			continue
		}
		if position < last {
			t.Fatalf("filtered order changed at %s", id)
		}
		last = position
	}
}

func TestSearchFiltersCombineWithAnd(t *testing.T) {
	_, response, _ := searchFilterResponse(t, "search", fixturePath(), "graph",
		"--where", "type:Concept", "--where", "status:draft", "--json")
	if len(response.Hybrid) != 1 || response.Hybrid[0].ID != "concepts/node-graph-engine" {
		t.Fatalf("unexpected hits: %+v", response.Hybrid)
	}
}

func TestSearchFilterKeywordIsCaseInsensitive(t *testing.T) {
	code, response, stderr := searchFilterResponse(t, "search", fixturePath(), "graph",
		"--where", "keyword:GRAPH", "--json")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d stderr=%s", code, stderr)
	}
	if len(response.Hybrid) == 0 {
		t.Fatal("expected a keyword match")
	}
}

func TestSearchFilterUnknownField(t *testing.T) {
	code, _, stderr := InvokeRead(t, "search", fixturePath(), "graph",
		"--where", "colour:red", "--json")
	if code != 2 || !strings.Contains(stderr, "type, status, tag or keyword") {
		t.Fatalf("expected a usage error, got %d %q", code, stderr)
	}
}

func TestSearchFilterEmptyValue(t *testing.T) {
	code, _, stderr := InvokeRead(t, "search", fixturePath(), "graph",
		"--where", "type:", "--json")
	if code != 2 || !strings.Contains(stderr, "empty") {
		t.Fatalf("expected a usage error, got %d %q", code, stderr)
	}
}

func TestSearchFiltersBeforeTopK(t *testing.T) {
	code, response, stderr := searchFilterResponse(t, "search", fixturePath(), "graph",
		"--where", "type:Concept", "--top-k", "1", "--json")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d stderr=%s", code, stderr)
	}
	if len(response.Hybrid) != 1 || response.Hybrid[0].Type != "Concept" {
		t.Fatalf("top-k applied before filtering: %+v", response.Hybrid)
	}
}

func TestSearchFiltersNoDatabase(t *testing.T) {
	code, response, stderr := searchFilterResponse(t, "search", fixturePath(), "graph",
		"--where", "type:Concept", "--no-db", "--json")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d stderr=%s", code, stderr)
	}
	for _, hitType := range allHitTypes(response) {
		if hitType != "Concept" {
			t.Fatalf("hit type %q leaked through the filter", hitType)
		}
	}
}

func TestSearchFiltersOutputField(t *testing.T) {
	_, response, _ := searchFilterResponse(t, "search", fixturePath(), "graph",
		"--where", "type:Concept", "--where", "keyword:graph", "--json")
	if len(response.Filters) != 2 {
		t.Fatalf("filters = %+v, want two entries", response.Filters)
	}
	if response.Filters[0].Field != "type" || response.Filters[1].Field != "keyword" {
		t.Fatalf("filters lost their order: %+v", response.Filters)
	}
}

func TestSearchWithoutFiltersUnchanged(t *testing.T) {
	_, response, _ := searchFilterResponse(t, "search", fixturePath(), "graph", "--json")
	if response.Filters != nil {
		t.Fatalf("filters must be omitted without --where: %+v", response.Filters)
	}
	if len(response.Hybrid) == 0 {
		t.Fatal("expected hits")
	}
}

func TestSearchFiltersPersistent(t *testing.T) {
	storageHome(t)
	if code, stdout, stderr := InvokePersistent(t, "ingest", fixturePath(), "--permissive", "--json"); code != 0 {
		t.Fatalf("ingest failed: %d %s %s", code, stdout, stderr)
	}
	code, stdout, stderr := InvokePersistent(t, "search", fixturePath(), "graph",
		"--where", "type:Concept", "--json")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d stderr=%s", code, stderr)
	}
	envelope := decodeEnvelope(t, stdout)
	var response contractv1.SearchResponse
	if err := json.Unmarshal(envelope.Result, &response); err != nil {
		t.Fatal(err)
	}
	for _, hit := range response.Hybrid {
		if hit.Type != "Concept" {
			t.Fatalf("persistent filter leaked %q", hit.Type)
		}
	}
}
