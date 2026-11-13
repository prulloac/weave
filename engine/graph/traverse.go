package graph

import (
	"sort"

	contractv1 "github.com/prulloac/weave/contract/v1"
)

// DefaultRelations is the default traversal set. Calls and references are opt-in.
var DefaultRelations = []contractv1.RelationKind{
	contractv1.RelationLink,
	contractv1.RelationRelated,
	contractv1.RelationImports,
	contractv1.RelationDeclares,
	contractv1.RelationInherits,
	contractv1.RelationImplements,
	contractv1.RelationContains,
	contractv1.RelationDependsOn,
}

// TraversalOptions restricts traversal to a set of relation kinds.
type TraversalOptions struct {
	Relations []contractv1.RelationKind
}

func (o TraversalOptions) Allows(kind string) bool {
	relations := o.Relations
	if len(relations) == 0 {
		relations = DefaultRelations
	}
	for _, relation := range relations {
		if string(relation) == kind {
			return true
		}
	}
	return false
}

// FindPath returns the shortest directed path using the default relation set.
func FindPath(graph *Graph, from, to string) []string {
	return FindPathWithOptions(graph, from, to, TraversalOptions{})
}

// FindPathWithOptions returns the shortest directed path allowed by options.
func FindPathWithOptions(graph *Graph, from, to string, options TraversalOptions) []string {
	if _, ok := graph.Nodes[from]; !ok {
		return nil
	}
	if _, ok := graph.Nodes[to]; !ok {
		return nil
	}
	if from == to {
		return []string{from}
	}

	parents := make(map[string]string)
	visited := map[string]bool{from: true}
	queue := []string{from}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, edge := range graph.Outgoing[current] {
			if !options.Allows(edge.Source) {
				continue
			}
			if visited[edge.To] {
				continue
			}
			visited[edge.To] = true
			parents[edge.To] = current
			if edge.To == to {
				path := []string{to}
				step := to
				for step != from {
					parent, ok := parents[step]
					if !ok {
						return nil
					}
					path = append([]string{parent}, path...)
					step = parent
				}
				return path
			}
			queue = append(queue, edge.To)
		}
	}
	return nil
}

// Neighbors returns the sorted union of incoming and outgoing neighbors using the default set.
func Neighbors(graph *Graph, id string) []string {
	return NeighborsWithOptions(graph, id, TraversalOptions{})
}

// NeighborsWithOptions returns the sorted neighbor union allowed by options.
func NeighborsWithOptions(graph *Graph, id string, options TraversalOptions) []string {
	set := make(map[string]bool)
	for _, edge := range graph.Outgoing[id] {
		if options.Allows(edge.Source) {
			set[edge.To] = true
		}
	}
	for _, edge := range graph.Incoming[id] {
		if options.Allows(edge.Source) {
			set[edge.From] = true
		}
	}
	result := make([]string, 0, len(set))
	for neighbor := range set {
		result = append(result, neighbor)
	}
	sort.Strings(result)
	return result
}

// FilterEdges returns edges whose source is in relations; empty relations keep all edges.
func FilterEdges(edges []contractv1.GraphEdge, relations []contractv1.RelationKind) []contractv1.GraphEdge {
	if len(relations) == 0 {
		return edges
	}
	allowed := make(map[string]bool, len(relations))
	for _, relation := range relations {
		allowed[string(relation)] = true
	}
	result := make([]contractv1.GraphEdge, 0, len(edges))
	for _, edge := range edges {
		if allowed[edge.Source] {
			result = append(result, edge)
		}
	}
	return result
}

// OrphanNodes returns node ids with no edges, sorted.
func OrphanNodes(graph *Graph) []string {
	result := make([]string, 0)
	for id, node := range graph.Nodes {
		if node.InDegree+node.OutDegree == 0 {
			result = append(result, id)
		}
	}
	sort.Strings(result)
	return result
}

// EdgeCount returns the number of outgoing edges.
func EdgeCount(graph *Graph) int {
	count := 0
	for _, edges := range graph.Outgoing {
		count += len(edges)
	}
	return count
}

// NodeCount returns the number of nodes.
func NodeCount(graph *Graph) int {
	return len(graph.Nodes)
}
