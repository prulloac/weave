// Command contract-export regenerates the contract golden corpus.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine"
	"github.com/prulloac/weave/engine/graph"
	"github.com/prulloac/weave/engine/nav"
)

type manifest struct {
	Contract string      `json:"contract"`
	Cases    []caseEntry `json:"cases"`
}

type caseEntry struct {
	Name   string       `json:"name"`
	Bundle string       `json:"bundle"`
	Search []searchCase `json:"search"`
	Nav    []navCase    `json:"nav"`
}

type searchCase struct {
	Name  string                `json:"name"`
	Query string                `json:"query"`
	Mode  contractv1.SearchMode `json:"mode"`
	TopK  int                   `json:"top_k"`
}

type navCase struct {
	Name    string   `json:"name"`
	Command string   `json:"command"`
	Query   string   `json:"query"`
	Target  string   `json:"target"`
	From    string   `json:"from"`
	To      string   `json:"to"`
	Types   []string `json:"types"`
	Tags    []string `json:"tags"`
	Status  string   `json:"status"`
	Limit   int      `json:"limit"`
}

func main() {
	goldenDir := filepath.Join("contract", "v1", "golden")
	expectedDir := filepath.Join(goldenDir, "expected")

	data, err := os.ReadFile(filepath.Join(goldenDir, "manifest.json"))
	if err != nil {
		fatal(err)
	}
	var spec manifest
	if err := json.Unmarshal(data, &spec); err != nil {
		fatal(err)
	}

	if err := os.RemoveAll(expectedDir); err != nil {
		fatal(err)
	}
	if err := os.MkdirAll(expectedDir, 0o755); err != nil {
		fatal(err)
	}

	total := 0
	for _, entry := range spec.Cases {
		count := exportCase(goldenDir, expectedDir, entry)
		total += count
		fmt.Printf("contract: %s -> %d files\n", entry.Name, count)
	}
	fmt.Printf("contract: wrote %d golden files to %s\n", total, expectedDir)
}

func exportCase(goldenDir, expectedDir string, entry caseEntry) int {
	eng, err := engine.Open(context.Background(), filepath.Join(goldenDir, entry.Bundle), engine.Options{})
	if err != nil {
		fatal(fmt.Errorf("%s: %w", entry.Name, err))
	}
	defer eng.Close()

	bundle := *eng.Bundle()
	bundle.Root = "<root>"
	writeJSON(expectedDir, entry.Name+".bundle.json", bundle)

	view := eng.Graph()
	writeJSON(expectedDir, entry.Name+".graph.json", contractv1.Graph{
		Nodes:    graph.SortedNodes(view),
		Edges:    graph.SortedEdges(view),
		Dangling: view.Dangling,
	})

	count := 2
	for _, searchEntry := range entry.Search {
		topK := searchEntry.TopK
		if topK == 0 {
			topK = 10
		}
		mode := searchEntry.Mode
		if mode == "" {
			mode = contractv1.SearchHybrid
		}
		response, err := eng.Search(searchEntry.Query, mode, topK)
		if err != nil {
			fatal(fmt.Errorf("%s search %s: %w", entry.Name, searchEntry.Name, err))
		}
		writeJSON(expectedDir, entry.Name+".search."+searchEntry.Name+".json", response)
		count++
	}

	for _, navEntry := range entry.Nav {
		writeJSON(expectedDir, entry.Name+".nav."+navEntry.Name+".json", runNav(eng, navEntry))
		count++
	}
	return count
}

func runNav(eng *engine.Engine, entry navCase) any {
	switch entry.Command {
	case "list":
		return eng.List(nav.ListFilters{
			Types:  entry.Types,
			Tags:   entry.Tags,
			Status: entry.Status,
			Limit:  entry.Limit,
		})
	case "show":
		result, ok := eng.Show(entry.Target)
		if !ok {
			fmt.Fprintf(os.Stderr, "show not found: %s\n", entry.Target)
			os.Exit(1)
		}
		return result
	case "backlinks":
		result, ok := eng.Backlinks(entry.Target)
		if !ok {
			fmt.Fprintf(os.Stderr, "backlinks not found: %s\n", entry.Target)
			os.Exit(1)
		}
		return result
	case "path":
		result, ok := eng.Path(entry.From, entry.To)
		if !ok {
			fmt.Fprintf(os.Stderr, "path not found: %s -> %s\n", entry.From, entry.To)
			os.Exit(1)
		}
		return result
	default:
		fmt.Fprintf(os.Stderr, "unknown nav command: %s\n", entry.Command)
		os.Exit(1)
		return nil
	}
}

func writeJSON(dir, name string, value any) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		fatal(err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "contract-export:", err)
	os.Exit(1)
}
