package graph

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"sort"
	"strings"

	contractv1 "github.com/prulloac/weave/contract/v1"
)

// ExportFormats lists the graph export formats.
var ExportFormats = []string{"dot", "mermaid", "graphml"}

// ValidExportFormat reports whether format names a supported export.
func ValidExportFormat(format string) bool {
	for _, candidate := range ExportFormats {
		if format == candidate {
			return true
		}
	}
	return false
}

// RenderExport renders one graph in the requested format. Nodes sort by identifier and edges by
// source, target and kind, so the same graph always produces byte-identical output.
func RenderExport(g contractv1.Graph, format string) ([]byte, error) {
	nodes := append([]contractv1.GraphNode(nil), g.Nodes...)
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
	edges := append([]contractv1.GraphEdge(nil), g.Edges...)
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From != edges[j].From {
			return edges[i].From < edges[j].From
		}
		if edges[i].To != edges[j].To {
			return edges[i].To < edges[j].To
		}
		return edges[i].Source < edges[j].Source
	})
	present := make(map[string]bool, len(nodes))
	for _, node := range nodes {
		present[node.ID] = true
	}
	kept := edges[:0]
	for _, edge := range edges {
		if present[edge.From] && present[edge.To] {
			kept = append(kept, edge)
		}
	}
	edges = kept

	switch format {
	case "dot":
		return renderDOT(nodes, edges), nil
	case "mermaid":
		return renderMermaid(nodes, edges), nil
	case "graphml":
		return renderGraphML(nodes, edges), nil
	default:
		return nil, fmt.Errorf("unknown graph export format %q", format)
	}
}

func nodeLabel(node contractv1.GraphNode) string {
	if strings.TrimSpace(node.Title) != "" {
		return node.Title
	}
	return node.ID
}

func renderDOT(nodes []contractv1.GraphNode, edges []contractv1.GraphEdge) []byte {
	var out bytes.Buffer
	out.WriteString("digraph weave {\n")
	out.WriteString("  rankdir=LR;\n")
	for _, node := range nodes {
		fmt.Fprintf(&out, "  \"%s\" [label=\"%s\"];\n", dotEscape(node.ID), dotEscape(nodeLabel(node)))
	}
	for _, edge := range edges {
		fmt.Fprintf(&out, "  \"%s\" -> \"%s\" [label=\"%s\"];\n",
			dotEscape(edge.From), dotEscape(edge.To), dotEscape(edge.Source))
	}
	out.WriteString("}\n")
	return out.Bytes()
}

func dotEscape(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	value = strings.ReplaceAll(value, "\n", `\n`)
	return value
}

func renderMermaid(nodes []contractv1.GraphNode, edges []contractv1.GraphEdge) []byte {
	var out bytes.Buffer
	out.WriteString("flowchart LR\n")
	ids := make(map[string]string, len(nodes))
	for index, node := range nodes {
		id := fmt.Sprintf("n%d", index)
		ids[node.ID] = id
		fmt.Fprintf(&out, "  %s[\"%s\"]\n", id, mermaidLabel(nodeLabel(node)))
	}
	for _, edge := range edges {
		from, fromOK := ids[edge.From]
		to, toOK := ids[edge.To]
		if !fromOK || !toOK {
			continue
		}
		fmt.Fprintf(&out, "  %s -->|%s| %s\n", from, mermaidLabel(edge.Source), to)
	}
	return out.Bytes()
}

func mermaidLabel(value string) string {
	value = strings.ReplaceAll(value, `"`, "#quot;")
	value = strings.ReplaceAll(value, "\r", "")
	value = strings.ReplaceAll(value, "\n", " ")
	return value
}

func renderGraphML(nodes []contractv1.GraphNode, edges []contractv1.GraphEdge) []byte {
	var out bytes.Buffer
	out.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	out.WriteString("\n")
	out.WriteString(`<graphml xmlns="http://graphml.graphdrawing.org/xmlns">`)
	out.WriteString("\n")
	out.WriteString(`  <key id="label" for="node" attr.name="label" attr.type="string"/>`)
	out.WriteString("\n")
	out.WriteString(`  <key id="kind" for="edge" attr.name="kind" attr.type="string"/>`)
	out.WriteString("\n")
	out.WriteString(`  <graph id="weave" edgedefault="directed">`)
	out.WriteString("\n")
	for _, node := range nodes {
		fmt.Fprintf(&out, `    <node id="%s"><data key="label">%s</data></node>`, xmlEscape(node.ID), xmlEscape(nodeLabel(node)))
		out.WriteString("\n")
	}
	for _, edge := range edges {
		fmt.Fprintf(&out, `    <edge source="%s" target="%s"><data key="kind">%s</data></edge>`,
			xmlEscape(edge.From), xmlEscape(edge.To), xmlEscape(edge.Source))
		out.WriteString("\n")
	}
	out.WriteString("  </graph>\n")
	out.WriteString("</graphml>\n")
	return out.Bytes()
}

func xmlEscape(value string) string {
	var out bytes.Buffer
	if err := xml.EscapeText(&out, []byte(value)); err != nil {
		return ""
	}
	return out.String()
}
