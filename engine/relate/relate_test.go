package relate

import (
	"errors"
	"slices"
	"strconv"
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine/graph"
)

func TestBuildOutAndDepth(t *testing.T) {
	view := chainGraph()

	depthZero, err := Build(view, "x", contractv1.RelateOut, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := nodeIDs(depthZero), []string{"x"}; !slices.Equal(got, want) {
		t.Fatalf("depth 0 nodes: got %v, want %v", got, want)
	}
	if len(depthZero.Edges) != 0 {
		t.Fatalf("depth 0 edges: got %v", edgeKeys(depthZero))
	}

	depthTwo, err := Build(view, "x", contractv1.RelateOut, 2)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := nodeIDs(depthTwo), []string{"x", "y", "z"}; !slices.Equal(got, want) {
		t.Fatalf("depth 2 nodes: got %v, want %v", got, want)
	}
	wantEdges := []string{"x->y:link:1", "y->z:link:2"}
	if got := edgeKeys(depthTwo); !slices.Equal(got, wantEdges) {
		t.Fatalf("depth 2 edges: got %v, want %v", got, wantEdges)
	}
}

func TestBuildInAndBoth(t *testing.T) {
	view := chainGraph()

	incoming, err := Build(view, "y", contractv1.RelateIn, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := nodeIDs(incoming), []string{"x", "y"}; !slices.Equal(got, want) {
		t.Fatalf("in nodes: got %v, want %v", got, want)
	}
	if got, want := edgeKeys(incoming), []string{"x->y:link:1"}; !slices.Equal(got, want) {
		t.Fatalf("in edges: got %v, want %v", got, want)
	}

	both, err := Build(view, "y", contractv1.RelateBoth, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := nodeIDs(both), []string{"x", "y", "z"}; !slices.Equal(got, want) {
		t.Fatalf("both nodes: got %v, want %v", got, want)
	}
	wantEdges := []string{"x->y:link:1", "y->z:link:1"}
	if got := edgeKeys(both); !slices.Equal(got, wantEdges) {
		t.Fatalf("both edges: got %v, want %v", got, wantEdges)
	}
}

func TestBuildDeduplicatesSelfLinks(t *testing.T) {
	self := contractv1.GraphEdge{From: "self", To: "self", Source: "link"}
	view := &graph.Graph{
		Nodes: map[string]contractv1.GraphNode{
			"self": {ID: "self", InDegree: 1, OutDegree: 1},
		},
		Outgoing: map[string][]contractv1.GraphEdge{"self": {self}},
		Incoming: map[string][]contractv1.GraphEdge{"self": {self}},
	}

	result, err := Build(view, "self", contractv1.RelateBoth, 2)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := nodeIDs(result), []string{"self"}; !slices.Equal(got, want) {
		t.Fatalf("self nodes: got %v, want %v", got, want)
	}
	if got, want := edgeKeys(result), []string{"self->self:link:1"}; !slices.Equal(got, want) {
		t.Fatalf("self edges: got %v, want %v", got, want)
	}
}

func TestBuildRejectsInvalidInput(t *testing.T) {
	view := chainGraph()

	if _, err := Build(view, "x", contractv1.RelateOut, 6); !errors.Is(err, ErrInvalidDepth) {
		t.Fatalf("expected ErrInvalidDepth, got %v", err)
	}
	if _, err := Build(view, "x", "sideways", 1); !errors.Is(err, ErrInvalidDirection) {
		t.Fatalf("expected ErrInvalidDirection, got %v", err)
	}
	if _, err := Build(view, "missing", contractv1.RelateOut, 1); !errors.Is(err, ErrRootNotFound) {
		t.Fatalf("expected ErrRootNotFound, got %v", err)
	}
}

func TestBuildReturnsRequestedHeader(t *testing.T) {
	result, err := Build(chainGraph(), "y", contractv1.RelateBoth, 3)
	if err != nil {
		t.Fatal(err)
	}
	if result.Root != "y" || result.Direction != contractv1.RelateBoth || result.Depth != 3 {
		t.Fatalf("unexpected header: %+v", result)
	}
}

func chainGraph() *graph.Graph {
	edges := []contractv1.GraphEdge{
		{From: "x", To: "y", Source: "link"},
		{From: "y", To: "z", Source: "link"},
		{From: "z", To: "w", Source: "link"},
	}
	nodes := map[string]contractv1.GraphNode{
		"x": {ID: "x", OutDegree: 1},
		"y": {ID: "y", InDegree: 1, OutDegree: 1},
		"z": {ID: "z", InDegree: 1, OutDegree: 1},
		"w": {ID: "w", InDegree: 1},
	}
	outgoing := map[string][]contractv1.GraphEdge{}
	incoming := map[string][]contractv1.GraphEdge{}
	for _, edge := range edges {
		outgoing[edge.From] = append(outgoing[edge.From], edge)
		incoming[edge.To] = append(incoming[edge.To], edge)
	}
	return &graph.Graph{Nodes: nodes, Outgoing: outgoing, Incoming: incoming}
}

func nodeIDs(result contractv1.RelateResult) []string {
	ids := make([]string, len(result.Nodes))
	for index, node := range result.Nodes {
		ids[index] = node.ID
	}
	return ids
}

func edgeKeys(result contractv1.RelateResult) []string {
	keys := make([]string, len(result.Edges))
	for index, edge := range result.Edges {
		keys[index] = edge.From + "->" + edge.To + ":" + edge.Source + ":" + strconv.Itoa(edge.Depth)
	}
	return keys
}
