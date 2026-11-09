package extensions_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/prulloac/weave/engine/extensions"
)

func TestRunnerScorerValidation(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]struct {
		body  string
		valid bool
	}{
		"valid":   {`printf '%s' '{"v":"weave.cli/v1","ranked_candidates":[{"id":"a","score":0.4},{"id":"b","score":1.4}]}'`, true},
		"missing": {`printf '%s' '{"v":"weave.cli/v1","ranked_candidates":[{"id":"a","score":0.4}]}'`, false},
		"extra":   {`printf '%s' '{"v":"weave.cli/v1","ranked_candidates":[{"id":"a","score":0.4},{"id":"b","score":0.2},{"id":"c","score":0.1}]}'`, false},
		"dup":     {`printf '%s' '{"v":"weave.cli/v1","ranked_candidates":[{"id":"a","score":0.4},{"id":"a","score":0.2}]}'`, false},
		"nan":     {`printf '%s' '{"v":"weave.cli/v1","ranked_candidates":[{"id":"a","score":"NaN"},{"id":"b","score":0.2}]}'`, false},
	}
	for name, testCase := range cases {
		path := filepath.Join(dir, "weave-"+name)
		writeScript(t, path, "cat >/dev/null\n"+testCase.body+"\n")
		scores, err := extensions.Runner{}.Score(context.Background(), extensions.Extension{Name: name, Path: path}, extensions.ScorerRequest{
			Query: "q",
			Mode:  "hybrid",
			Candidates: []extensions.ScorerCandidate{
				{ID: "a", Path: "a.md"},
				{ID: "b", Path: "b.md"},
			},
		})
		if testCase.valid {
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			if scores["b"] != 1 {
				t.Fatalf("%s: score must clamp to 1, got %v", name, scores["b"])
			}
			continue
		}
		if !errors.Is(err, extensions.ErrProtocol) {
			t.Fatalf("%s: expected protocol error, got %v", name, err)
		}
	}
}

func TestRunnerScorerReceivesContext(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "weave-context")
	writeScript(t, path, `cat >/dev/null
printf '%s' '{"v":"weave.cli/v1","ranked_candidates":[{"id":"a","score":0.9}]}'`)
	snippet := "bounded snippet"
	scores, err := extensions.Runner{}.Score(context.Background(), extensions.Extension{Name: "context", Path: path}, extensions.ScorerRequest{
		Query: "q",
		Mode:  "fuzzy",
		Candidates: []extensions.ScorerCandidate{{
			ID: "a", Path: "a.md", Title: "Alpha", Keywords: []string{"alpha"}, Snippet: &snippet,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if scores["a"] != 0.9 {
		t.Fatalf("unexpected score: %v", scores)
	}
}
