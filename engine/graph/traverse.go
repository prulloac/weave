package graph

import "sort"

// FindPath returns the shortest directed path, or nil when disconnected.
func FindPath(graph *Graph, from, to string) []string {
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

// Neighbors returns the sorted union of incoming and outgoing neighbors.
func Neighbors(graph *Graph, id string) []string {
	set := make(map[string]bool)
	for _, edge := range graph.Outgoing[id] {
		set[edge.To] = true
	}
	for _, edge := range graph.Incoming[id] {
		set[edge.From] = true
	}
	result := make([]string, 0, len(set))
	for neighbor := range set {
		result = append(result, neighbor)
	}
	sort.Strings(result)
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
