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
		body  string
		valid bool
	}{
		"valid": {
			`printf '%s' '{"v":"weave.cli/v1","documents":[{"id":"main.go"},{"id":"app.py"}],"relations":[{"from":"main.go","to":"app.py","kind":"imports"}]}'`,
			true,
		},
		"missing": {
			`printf '%s' '{"v":"weave.cli/v1","documents":[{"id":"main.go"}]}'`,
			false,
		},
		"extra": {
			`printf '%s' '{"v":"weave.cli/v1","documents":[{"id":"main.go"},{"id":"app.py"},{"id":"x.go"}]}'`,
			false,
		},
		"duplicate": {
			`printf '%s' '{"v":"weave.cli/v1","documents":[{"id":"main.go"},{"id":"main.go"}]}'`,
			false,
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
			if len(response.Relations) != 1 || response.Relations[0].Kind != "imports" {
				t.Fatalf("%s: unexpected relations: %+v", name, response.Relations)
			}
			continue
		}
		if !errors.Is(err, extensions.ErrProtocol) {
			t.Fatalf("%s: expected protocol error, got %v", name, err)
		}
	}
}
