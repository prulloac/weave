package engine_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine"
	"github.com/prulloac/weave/engine/graph"
	"github.com/prulloac/weave/engine/nav"
)

const goldenDir = "../contract/v1/golden"

type manifest struct {
	Contract string         `json:"contract"`
	Cases    []manifestCase `json:"cases"`
}

type manifestCase struct {
	Name   string           `json:"name"`
	Bundle string           `json:"bundle"`
	Search []manifestSearch `json:"search"`
	Nav    []manifestNav    `json:"nav"`
}

type manifestSearch struct {
	Name  string                `json:"name"`
	Query string                `json:"query"`
	Mode  contractv1.SearchMode `json:"mode"`
	TopK  int                   `json:"top_k"`
}

type manifestNav struct {
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

func loadManifest(t *testing.T) manifest {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(goldenDir, "manifest.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var result manifest
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	return result
}

func TestConformance(t *testing.T) {
	manifest := loadManifest(t)
	if manifest.Contract != "weave.contract/v1" {
		t.Fatalf("unexpected contract: %s", manifest.Contract)
	}

	for _, testCase := range manifest.Cases {
		testCase := testCase
		t.Run(testCase.Name, func(t *testing.T) {
			eng, err := engine.Open(context.Background(), filepath.Join(goldenDir, testCase.Bundle), engine.Options{})
			if err != nil {
				t.Fatalf("open bundle: %v", err)
			}
			defer eng.Close()

			bundle := *eng.Bundle()
			bundle.Root = "<root>"
			assertGolden(t, bundle, testCase.Name+".bundle.json")

			graphView := *eng.Graph()
			assertGolden(t, contractGraph(&graphView), testCase.Name+".graph.json")

			for _, searchCase := range testCase.Search {
				topK := searchCase.TopK
				if topK == 0 {
					topK = 10
				}
				mode := searchCase.Mode
				if mode == "" {
					mode = contractv1.SearchHybrid
				}
				response, err := eng.Search(searchCase.Query, mode, topK)
				if err != nil {
					t.Fatalf("search %s: %v", searchCase.Name, err)
				}
				assertGolden(t, response, testCase.Name+".search."+searchCase.Name+".json")
			}

			for _, navCase := range testCase.Nav {
				assertGolden(t, runNav(t, eng, navCase), testCase.Name+".nav."+navCase.Name+".json")
			}
		})
	}
}

func contractGraph(view *graph.Graph) contractv1.Graph {
	return contractv1.Graph{
		Nodes:    graph.SortedNodes(view),
		Edges:    graph.SortedEdges(view),
		Dangling: view.Dangling,
	}
}

func runNav(t *testing.T, eng *engine.Engine, navCase manifestNav) any {
	t.Helper()

	switch navCase.Command {
	case "list":
		return eng.List(nav.ListFilters{
			Types:  navCase.Types,
			Tags:   navCase.Tags,
			Status: navCase.Status,
			Limit:  navCase.Limit,
		})
	case "show":
		result, ok := eng.Show(navCase.Target)
		if !ok {
			t.Fatalf("show %q returned not found", navCase.Target)
		}
		return result
	case "backlinks":
		result, ok := eng.Backlinks(navCase.Target)
		if !ok {
			t.Fatalf("backlinks %q returned not found", navCase.Target)
		}
		return result
	case "path":
		result, ok := eng.Path(navCase.From, navCase.To)
		if !ok {
			t.Fatalf("path %q -> %q returned not found", navCase.From, navCase.To)
		}
		return result
	default:
		t.Fatalf("unknown nav command %q", navCase.Command)
		return nil
	}
}

func assertGolden(t *testing.T, actual any, file string) {
	t.Helper()

	actualJSON, err := json.Marshal(actual)
	if err != nil {
		t.Fatalf("marshal actual: %v", err)
	}
	var actualDoc any
	if err := json.Unmarshal(actualJSON, &actualDoc); err != nil {
		t.Fatalf("decode actual: %v", err)
	}

	expectedData, err := os.ReadFile(filepath.Join(goldenDir, "expected", file))
	if err != nil {
		t.Fatalf("read golden %s: %v", file, err)
	}
	var expectedDoc any
	if err := json.Unmarshal(expectedData, &expectedDoc); err != nil {
		t.Fatalf("decode golden %s: %v", file, err)
	}

	if reflect.DeepEqual(actualDoc, expectedDoc) {
		return
	}

	actualPretty, _ := json.MarshalIndent(actualDoc, "", "  ")
	expectedPretty, _ := json.MarshalIndent(expectedDoc, "", "  ")
	t.Errorf("golden mismatch: %s\n--- actual ---\n%s\n--- expected ---\n%s", file, actualPretty, expectedPretty)
}
