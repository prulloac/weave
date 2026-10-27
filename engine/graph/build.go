package graph

import (
	"sort"
	"strings"

	contractv1 "github.com/prulloac/weave/contract/v1"
)

// Graph is the bidirectional concept graph.
type Graph struct {
	Nodes    map[string]contractv1.GraphNode
	Outgoing map[string][]contractv1.GraphEdge
	Incoming map[string][]contractv1.GraphEdge
	Dangling map[string][]contractv1.OkfLink
}

// Build creates a graph from a parsed bundle.
func Build(bundle *contractv1.Bundle) *Graph {
	graph := &Graph{
		Nodes:    make(map[string]contractv1.GraphNode, len(bundle.Concepts)),
		Outgoing: make(map[string][]contractv1.GraphEdge),
		Incoming: make(map[string][]contractv1.GraphEdge),
		Dangling: make(map[string][]contractv1.OkfLink),
	}

	for _, concept := range bundle.Concepts {
		title := concept.ID
		if concept.Title != nil {
			title = *concept.Title
		}
		graph.Nodes[concept.ID] = contractv1.GraphNode{
			ID:        concept.ID,
			Path:      concept.Path,
			Type:      concept.Type,
			Title:     title,
			Keywords:  concept.Keywords,
			Status:    concept.Status,
			Tags:      concept.Tags,
			InDegree:  0,
			OutDegree: 0,
		}
	}

	for _, concept := range bundle.Concepts {
		for _, link := range concept.Links {
			if link.ResolvesInBundle && link.Resolved != nil {
				target := strings.TrimSuffix(*link.Resolved, ".md")
				if _, ok := graph.Nodes[target]; ok {
					raw := link.Raw
					addEdge(graph, concept.ID, target, "link", &raw)
				} else {
					addDangling(graph, concept.ID, link)
				}
			} else {
				addDangling(graph, concept.ID, link)
			}
		}

		related, ok := concept.Frontmatter["related"]
		if !ok {
			continue
		}
		for _, entry := range RelatedEntries(related) {
			if entry == "" {
				continue
			}
			target, found := relatedTarget(entry, concept.ID, graph.Nodes)
			if found {
				addEdge(graph, concept.ID, target, "related", nil)
			} else {
				addDangling(graph, concept.ID, contractv1.OkfLink{
					Raw:              entry,
					Target:           entry,
					ResolvesInBundle: false,
				})
			}
		}
	}

	for _, concept := range bundle.Concepts {
		from := concept.ID
		for _, edge := range graph.Outgoing[from] {
			graph.Incoming[edge.To] = append(graph.Incoming[edge.To], edge)
			node := graph.Nodes[from]
			node.OutDegree++
			graph.Nodes[from] = node
			target := graph.Nodes[edge.To]
			target.InDegree++
			graph.Nodes[edge.To] = target
		}
	}

	return graph
}

func addEdge(graph *Graph, from, to, source string, raw *string) {
	for _, existing := range graph.Outgoing[from] {
		if existing.To == to && existing.Source == source {
			return
		}
	}
	graph.Outgoing[from] = append(graph.Outgoing[from], contractv1.GraphEdge{
		From:   from,
		To:     to,
		Source: source,
		Raw:    raw,
	})
}

func addDangling(graph *Graph, id string, link contractv1.OkfLink) {
	graph.Dangling[id] = append(graph.Dangling[id], link)
}

// RelatedEntries returns the non-empty `related` frontmatter entries.
func RelatedEntries(value any) []string {
	return relatedStrings(value)
}

// ResolveRelated resolves a `related` entry against the graph nodes.
func (g *Graph) ResolveRelated(entry, fromID string) (string, bool) {
	return relatedTarget(entry, fromID, g.Nodes)
}

func relatedStrings(value any) []string {
	switch typed := value.(type) {
	case []any:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			if text, ok := item.(string); ok && text != "" {
				result = append(result, text)
			}
		}
		return result
	case []string:
		return typed
	default:
		return nil
	}
}

func relatedTarget(entry, fromID string, nodes map[string]contractv1.GraphNode) (string, bool) {
	cleaned := strings.TrimSuffix(entry, ".md")
	cleaned = strings.TrimLeft(cleaned, "/")
	cleaned = strings.TrimPrefix(cleaned, "./")

	candidates := []string{cleaned}
	if !strings.Contains(cleaned, "/") && strings.Contains(fromID, "/") {
		dir := fromID[:strings.LastIndex(fromID, "/")]
		candidates = append([]string{dir + "/" + cleaned}, cleaned)
	}
	for _, candidate := range candidates {
		if _, ok := nodes[candidate]; ok {
			return candidate, true
		}
	}
	return "", false
}

// SortedNodes returns nodes ordered by id.
func SortedNodes(graph *Graph) []contractv1.GraphNode {
	nodes := make([]contractv1.GraphNode, 0, len(graph.Nodes))
	for _, node := range graph.Nodes {
		nodes = append(nodes, node)
	}
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].ID < nodes[j].ID
	})
	return nodes
}

// SortedEdges returns outgoing edges ordered by from, to and source.
func SortedEdges(graph *Graph) []contractv1.GraphEdge {
	edges := make([]contractv1.GraphEdge, 0)
	for _, outgoing := range graph.Outgoing {
		edges = append(edges, outgoing...)
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From != edges[j].From {
			return edges[i].From < edges[j].From
		}
		if edges[i].To != edges[j].To {
			return edges[i].To < edges[j].To
		}
		return edges[i].Source < edges[j].Source
	})
	return edges
}
