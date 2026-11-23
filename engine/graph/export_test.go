package graph

import (
	"strings"
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
)

func exportFixture() contractv1.Graph {
	return contractv1.Graph{
		Nodes: []contractv1.GraphNode{
			{ID: "concepts/z.md", Title: "Zeta", Path: "concepts/z.md"},
			{ID: "concepts/a.md", Title: `Alpha "quoted" <tag>`, Path: "concepts/a.md"},
			{ID: "concepts/no-title.md", Path: "concepts/no-title.md"},
		},
		Edges: []contractv1.GraphEdge{
			{From: "concepts/z.md", To: "concepts/a.md", Source: "related"},
			{From: "concepts/a.md", To: "concepts/z.md", Source: "link"},
			{From: "concepts/a.md", To: "concepts/missing.md", Source: "related"},
		},
	}
}

func TestRenderExportDOT(t *testing.T) {
	data, err := RenderExport(exportFixture(), "dot")
	if err != nil {
		t.Fatal(err)
	}
	want := `digraph weave {
  rankdir=LR;
  "concepts/a.md" [label="Alpha \"quoted\" <tag>"];
  "concepts/no-title.md" [label="concepts/no-title.md"];
  "concepts/z.md" [label="Zeta"];
  "concepts/a.md" -> "concepts/z.md" [label="link"];
  "concepts/z.md" -> "concepts/a.md" [label="related"];
}
`
	if string(data) != want {
		t.Fatalf("DOT output:\n%s\nwant:\n%s", data, want)
	}
}

func TestRenderExportMermaid(t *testing.T) {
	data, err := RenderExport(exportFixture(), "mermaid")
	if err != nil {
		t.Fatal(err)
	}
	out := string(data)
	for _, want := range []string{
		"flowchart LR",
		`n0["Alpha #quot;quoted#quot; <tag>"]`,
		`n1["concepts/no-title.md"]`,
		`n2["Zeta"]`,
		"n0 -->|link| n2",
		"n2 -->|related| n0",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("Mermaid output missing %q:\n%s", want, out)
		}
	}
}

func TestRenderExportGraphML(t *testing.T) {
	data, err := RenderExport(exportFixture(), "graphml")
	if err != nil {
		t.Fatal(err)
	}
	out := string(data)
	for _, want := range []string{
		`<?xml version="1.0" encoding="UTF-8"?>`,
		`<graphml xmlns="http://graphml.graphdrawing.org/xmlns">`,
		`<key id="label" for="node"`,
		`<key id="kind" for="edge"`,
		`<data key="label">Alpha &#34;quoted&#34; &lt;tag&gt;</data>`,
		`<data key="kind">related</data>`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("GraphML output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "concepts/missing.md") {
		t.Fatalf("dangling edge must be omitted:\n%s", out)
	}
}

func TestRenderExportDeterministic(t *testing.T) {
	for _, format := range ExportFormats {
		first, err := RenderExport(exportFixture(), format)
		if err != nil {
			t.Fatal(err)
		}
		second, err := RenderExport(exportFixture(), format)
		if err != nil {
			t.Fatal(err)
		}
		if string(first) != string(second) {
			t.Fatalf("%s output is not deterministic", format)
		}
	}
}

func TestRenderExportUnknownFormat(t *testing.T) {
	if _, err := RenderExport(exportFixture(), "svg"); err == nil {
		t.Fatal("expected an error for an unknown format")
	}
}
