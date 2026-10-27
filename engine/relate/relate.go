// Package relate builds a bounded graph neighborhood for `weave relate`.
package relate

import (
	"errors"
	"sort"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine/graph"
)

var (
	// ErrRootNotFound is returned when the requested root is not in the graph.
	ErrRootNotFound = errors.New("root concept not found")
	// ErrInvalidDepth is returned when depth is outside 0..5.
	ErrInvalidDepth = errors.New("depth must be between 0 and 5")
	// ErrInvalidDirection is returned for an unknown traversal direction.
	ErrInvalidDirection = errors.New("direction must be out, in or both")
)

type queueItem struct {
	id    string
	depth int
}

// Build returns the bounded neighborhood of root.
func Build(
	view *graph.Graph,
	root string,
	direction contractv1.RelateDirection,
	depth int,
) (contractv1.RelateResult, error) {
	if depth < 0 || depth > 5 {
		return contractv1.RelateResult{}, ErrInvalidDepth
	}
	switch direction {
	case "":
		direction = contractv1.RelateBoth
	case contractv1.RelateOut, contractv1.RelateIn, contractv1.RelateBoth:
	default:
		return contractv1.RelateResult{}, ErrInvalidDirection
	}
	if view == nil {
		return contractv1.RelateResult{}, ErrRootNotFound
	}
	if _, ok := view.Nodes[root]; !ok {
		return contractv1.RelateResult{}, ErrRootNotFound
	}

	result := contractv1.RelateResult{
		Root:      root,
		Direction: direction,
		Depth:     depth,
		Nodes:     []contractv1.GraphNode{},
		Edges:     []contractv1.RelateEdge{},
	}
	visited := map[string]bool{root: true}
	edgeSeen := make(map[string]bool)
	queue := []queueItem{{id: root, depth: 0}}

	addEdge := func(edge contractv1.GraphEdge, next string, nextDepth int) {
		key := edge.From + "\x00" + edge.To + "\x00" + edge.Source
		if !edgeSeen[key] {
			edgeSeen[key] = true
			result.Edges = append(result.Edges, contractv1.RelateEdge{
				From:   edge.From,
				To:     edge.To,
				Source: edge.Source,
				Raw:    edge.Raw,
				Depth:  nextDepth,
			})
		}
		if !visited[next] {
			visited[next] = true
			queue = append(queue, queueItem{id: next, depth: nextDepth})
		}
	}

	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]
		if item.depth >= depth {
			continue
		}
		nextDepth := item.depth + 1

		if direction == contractv1.RelateOut || direction == contractv1.RelateBoth {
			for _, edge := range view.Outgoing[item.id] {
				addEdge(edge, edge.To, nextDepth)
			}
		}
		if direction == contractv1.RelateIn || direction == contractv1.RelateBoth {
			for _, edge := range view.Incoming[item.id] {
				addEdge(edge, edge.From, nextDepth)
			}
		}
	}

	ids := make([]string, 0, len(visited))
	for id := range visited {
		if _, ok := view.Nodes[id]; ok {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	for _, id := range ids {
		result.Nodes = append(result.Nodes, view.Nodes[id])
	}

	sort.Slice(result.Edges, func(i, j int) bool {
		left, right := result.Edges[i], result.Edges[j]
		if left.From != right.From {
			return left.From < right.From
		}
		if left.To != right.To {
			return left.To < right.To
		}
		if left.Source != right.Source {
			return left.Source < right.Source
		}
		return left.Depth < right.Depth
	})

	return result, nil
}
