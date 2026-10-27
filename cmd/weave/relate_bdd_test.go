package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// These are behavior-level tests for the relate command. They use the CLI
// entrypoint and parsed bundles only; no graph internals are called directly.

type bddRelateResult struct {
	Root      string          `json:"root"`
	Direction string          `json:"direction"`
	Depth     int             `json:"depth"`
	Nodes     []bddRelateNode `json:"nodes"`
	Edges     []bddRelateEdge `json:"edges"`
}

type bddRelateNode struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Type      string   `json:"type"`
	Keywords  []string `json:"keywords"`
	InDegree  int      `json:"in_degree"`
	OutDegree int      `json:"out_degree"`
}

type bddRelateEdge struct {
	From   string  `json:"from"`
	To     string  `json:"to"`
	Source string  `json:"source"`
	Raw    *string `json:"raw,omitempty"`
	Depth  int     `json:"depth"`
}

func relateFixture(name string) string {
	return filepath.Join("testdata", "relate", name)
}

func runRelateJSON(t *testing.T, args ...string) (int, testEnvelope, bddRelateResult) {
	t.Helper()

	fullArgs := append([]string{"relate"}, args...)
	code, stdout, stderr := invoke(t, fullArgs...)
	if stderr != "" {
		t.Fatalf("expected empty stderr in JSON mode, got %q", stderr)
	}

	envelope := decodeEnvelope(t, stdout)
	if !envelope.OK {
		t.Fatalf("expected completed relate envelope, got failure: %+v", envelope.Error)
	}

	var result bddRelateResult
	if err := json.Unmarshal(envelope.Result, &result); err != nil {
		t.Fatalf("decode relate result: %v\n%s", err, envelope.Result)
	}
	return code, envelope, result
}

func relateNodeIDs(result bddRelateResult) []string {
	ids := make([]string, len(result.Nodes))
	for index, node := range result.Nodes {
		ids[index] = node.ID
	}
	return ids
}

func relateEdgeKeys(result bddRelateResult) []string {
	keys := make([]string, len(result.Edges))
	for index, edge := range result.Edges {
		keys[index] = fmt.Sprintf("%s->%s:%s:%d", edge.From, edge.To, edge.Source, edge.Depth)
	}
	return keys
}

func TestRelateBehavior(t *testing.T) {
	t.Run("depth 0 returns only the root", func(t *testing.T) {
		code, _, result := runRelateJSON(t,
			relateFixture("chain"), "concepts/x", "--depth", "0", "--direction", "out", "--json")

		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
		if result.Root != "concepts/x" || result.Depth != 0 || result.Direction != "out" {
			t.Fatalf("unexpected header: %+v", result)
		}
		if got, want := relateNodeIDs(result), []string{"concepts/x"}; !slices.Equal(got, want) {
			t.Fatalf("unexpected nodes: got %v, want %v", got, want)
		}
		if len(result.Edges) != 0 {
			t.Fatalf("expected no edges, got %v", relateEdgeKeys(result))
		}
	})

	t.Run("out depth 1 returns direct outbound neighbors", func(t *testing.T) {
		code, _, result := runRelateJSON(t,
			relateFixture("chain"), "concepts/x", "--depth", "1", "--direction", "out", "--json")

		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
		if got, want := relateNodeIDs(result), []string{"concepts/x", "concepts/y"}; !slices.Equal(got, want) {
			t.Fatalf("unexpected nodes: got %v, want %v", got, want)
		}
		if got, want := relateEdgeKeys(result), []string{"concepts/x->concepts/y:link:1"}; !slices.Equal(got, want) {
			t.Fatalf("unexpected edges: got %v, want %v", got, want)
		}
	})

	t.Run("out depth 2 returns the two-hop neighborhood", func(t *testing.T) {
		code, _, result := runRelateJSON(t,
			relateFixture("chain"), "concepts/x", "--depth", "2", "--direction", "out", "--json")

		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
		if got, want := relateNodeIDs(result), []string{"concepts/x", "concepts/y", "concepts/z"}; !slices.Equal(got, want) {
			t.Fatalf("unexpected nodes: got %v, want %v", got, want)
		}
		wantEdges := []string{
			"concepts/x->concepts/y:link:1",
			"concepts/y->concepts/z:link:2",
		}
		if got := relateEdgeKeys(result); !slices.Equal(got, wantEdges) {
			t.Fatalf("unexpected edges: got %v, want %v", got, wantEdges)
		}
	})

	t.Run("in depth 1 follows incoming edges", func(t *testing.T) {
		code, _, result := runRelateJSON(t,
			relateFixture("chain"), "concepts/y", "--depth", "1", "--direction", "in", "--json")

		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
		if got, want := relateNodeIDs(result), []string{"concepts/x", "concepts/y"}; !slices.Equal(got, want) {
			t.Fatalf("unexpected nodes: got %v, want %v", got, want)
		}
		if got, want := relateEdgeKeys(result), []string{"concepts/x->concepts/y:link:1"}; !slices.Equal(got, want) {
			t.Fatalf("unexpected edges: got %v, want %v", got, want)
		}
	})

	t.Run("both depth 1 follows incoming and outgoing edges", func(t *testing.T) {
		code, _, result := runRelateJSON(t,
			relateFixture("chain"), "concepts/y", "--depth", "1", "--direction", "both", "--json")

		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
		if got, want := relateNodeIDs(result), []string{"concepts/x", "concepts/y", "concepts/z"}; !slices.Equal(got, want) {
			t.Fatalf("unexpected nodes: got %v, want %v", got, want)
		}
		wantEdges := []string{
			"concepts/x->concepts/y:link:1",
			"concepts/y->concepts/z:link:1",
		}
		if got := relateEdgeKeys(result); !slices.Equal(got, wantEdges) {
			t.Fatalf("unexpected edges: got %v, want %v", got, wantEdges)
		}
	})

	t.Run("defaults are depth 1 and direction both", func(t *testing.T) {
		code, _, result := runRelateJSON(t, relateFixture("graph"), "concepts/a", "--json")

		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
		if result.Depth != 1 || result.Direction != "both" {
			t.Fatalf("expected depth=1 direction=both, got depth=%d direction=%s", result.Depth, result.Direction)
		}
		if got, want := relateNodeIDs(result), []string{"concepts/a", "concepts/b", "concepts/c", "concepts/d"}; !slices.Equal(got, want) {
			t.Fatalf("unexpected nodes: got %v, want %v", got, want)
		}
	})

	t.Run("related edges are included with their source", func(t *testing.T) {
		code, _, result := runRelateJSON(t,
			relateFixture("graph"), "concepts/a", "--depth", "1", "--direction", "out", "--json")

		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
		if got, want := relateNodeIDs(result), []string{"concepts/a", "concepts/b", "concepts/c", "concepts/d"}; !slices.Equal(got, want) {
			t.Fatalf("unexpected nodes: got %v, want %v", got, want)
		}
		wantEdges := []string{
			"concepts/a->concepts/b:link:1",
			"concepts/a->concepts/c:link:1",
			"concepts/a->concepts/d:related:1",
		}
		if got := relateEdgeKeys(result); !slices.Equal(got, wantEdges) {
			t.Fatalf("unexpected edges: got %v, want %v", got, wantEdges)
		}
	})

	t.Run("self links do not duplicate nodes or edges", func(t *testing.T) {
		code, _, result := runRelateJSON(t,
			relateFixture("graph"), "concepts/self", "--depth", "1", "--direction", "out", "--json")

		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
		if got, want := relateNodeIDs(result), []string{"concepts/self"}; !slices.Equal(got, want) {
			t.Fatalf("unexpected nodes: got %v, want %v", got, want)
		}
		if got, want := relateEdgeKeys(result), []string{"concepts/self->concepts/self:link:1"}; !slices.Equal(got, want) {
			t.Fatalf("unexpected edges: got %v, want %v", got, want)
		}
	})

	t.Run("isolated root returns an empty edge list", func(t *testing.T) {
		code, _, result := runRelateJSON(t,
			relateFixture("graph"), "concepts/isolated", "--depth", "1", "--direction", "both", "--json")

		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
		if got, want := relateNodeIDs(result), []string{"concepts/isolated"}; !slices.Equal(got, want) {
			t.Fatalf("unexpected nodes: got %v, want %v", got, want)
		}
		if len(result.Edges) != 0 {
			t.Fatalf("expected no edges, got %v", relateEdgeKeys(result))
		}
	})

	t.Run("missing root returns CONCEPT_NOT_FOUND", func(t *testing.T) {
		code, stdout, _ := invoke(t, "relate", relateFixture("graph"), "concepts/missing", "--json")

		if code != 1 {
			t.Fatalf("expected exit 1, got %d", code)
		}
		envelope := decodeEnvelope(t, stdout)
		if envelope.OK {
			t.Fatal("expected a failure envelope")
		}
		if envelope.Error == nil || envelope.Error.Code != "CONCEPT_NOT_FOUND" {
			t.Fatalf("expected CONCEPT_NOT_FOUND, got %+v", envelope.Error)
		}
	})

	t.Run("invalid depth is a usage error", func(t *testing.T) {
		code, _, _ := invoke(t, "relate", relateFixture("chain"), "concepts/x", "--depth", "6", "--json")
		if code != 2 {
			t.Fatalf("expected exit 2, got %d", code)
		}
	})

	t.Run("invalid direction is a usage error", func(t *testing.T) {
		code, _, _ := invoke(t, "relate", relateFixture("chain"), "concepts/x", "--direction", "sideways", "--json")
		if code != 2 {
			t.Fatalf("expected exit 2, got %d", code)
		}
	})

	t.Run("ordering is deterministic", func(t *testing.T) {
		path := relateFixture("chain")
		_, _, first := runRelateJSON(t, path, "concepts/x", "--depth", "2", "--direction", "out", "--json")
		_, _, second := runRelateJSON(t, path, "concepts/x", "--depth", "2", "--direction", "out", "--json")

		if !slices.Equal(relateNodeIDs(first), relateNodeIDs(second)) {
			t.Fatalf("nodes are not deterministic: %v vs %v", relateNodeIDs(first), relateNodeIDs(second))
		}
		if !slices.Equal(relateEdgeKeys(first), relateEdgeKeys(second)) {
			t.Fatalf("edges are not deterministic: %v vs %v", relateEdgeKeys(first), relateEdgeKeys(second))
		}
		if !slices.IsSorted(relateNodeIDs(first)) {
			t.Fatalf("nodes are not sorted: %v", relateNodeIDs(first))
		}
		if !slices.IsSorted(relateEdgeKeys(first)) {
			t.Fatalf("edges are not sorted: %v", relateEdgeKeys(first))
		}
	})

	t.Run("human output groups outbound and inbound edges", func(t *testing.T) {
		code, stdout, stderr := invoke(t,
			"relate", relateFixture("graph"), "concepts/a", "--depth", "1", "--direction", "both")
		if stderr != "" {
			t.Fatalf("expected empty stderr, got %q", stderr)
		}
		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
		for _, want := range []string{
			"concepts/a  (1 hop, both)",
			"outbound",
			"inbound",
			"link → concepts/b",
			"related → concepts/d",
			"concepts/b",
		} {
			if !strings.Contains(stdout, want) {
				t.Fatalf("expected human output to contain %q:\n%s", want, stdout)
			}
		}
	})

	t.Run("human output groups deeper hops", func(t *testing.T) {
		code, stdout, stderr := invoke(t,
			"relate", relateFixture("chain"), "concepts/x", "--depth", "2", "--direction", "out")
		if stderr != "" {
			t.Fatalf("expected empty stderr, got %q", stderr)
		}
		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
		for _, want := range []string{"hop 1", "hop 2", "concepts/y", "concepts/z"} {
			if !strings.Contains(stdout, want) {
				t.Fatalf("expected human output to contain %q:\n%s", want, stdout)
			}
		}
	})
}
