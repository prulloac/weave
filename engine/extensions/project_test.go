package extensions_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/prulloac/weave/engine/extensions"
)

func TestProjectAnalysisCoverage(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]struct {
		body      string
		valid     bool
		relations int
	}{
		"valid": {
			`printf '%s' '{"v":"weave.cli/v1","documents":[{"id":"main.go"},{"id":"app.py"}],"relations":[{"from":"main.go","to":"app.py","kind":"imports"}]}'`,
			true, 1,
		},
		"missing": {
			`printf '%s' '{"v":"weave.cli/v1","documents":[{"id":"main.go"}]}'`,
			true, 0,
		},
		"extra": {
			`printf '%s' '{"v":"weave.cli/v1","documents":[{"id":"main.go"},{"id":"app.py"},{"id":"x.go"}]}'`,
			false, 0,
		},
		"duplicate": {
			`printf '%s' '{"v":"weave.cli/v1","documents":[{"id":"main.go"},{"id":"main.go"}]}'`,
			false, 0,
		},
	}
	for name, testCase := range cases {
		path := filepath.Join(dir, "weave-"+name)
		writeScript(t, path, "cat >/dev/null\n"+testCase.body+"\n")
		response, err := extensions.Runner{}.ProjectAnalysis(context.Background(), extensions.Extension{Name: name, Path: path}, extensions.ProjectAnalysisRequest{
			BundleRoot: "/repo",
			Files: []extensions.ProjectFile{
				{ID: "main.go", Path: "main.go", MediaType: "text/x-go"},
				{ID: "app.py", Path: "app.py", MediaType: "text/x-python"},
			},
		})
		if testCase.valid {
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			if len(response.Relations) != testCase.relations {
				t.Fatalf("%s: unexpected relations: %+v", name, response.Relations)
			}
			continue
		}
		if !errors.Is(err, extensions.ErrProtocol) {
			t.Fatalf("%s: expected protocol error, got %v", name, err)
		}
	}
}
