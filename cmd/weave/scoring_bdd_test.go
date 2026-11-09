package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const scorerProgram = `package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"
)

type candidate struct {
	ID string ` + "`json:\"id\"`" + `
}

type request struct {
	V          string      ` + "`json:\"v\"`" + `
	Type       string      ` + "`json:\"type\"`" + `
	Query      string      ` + "`json:\"query\"`" + `
	Mode       string      ` + "`json:\"mode\"`" + `
	CandidateK int         ` + "`json:\"candidate_k\"`" + `
	Candidates []candidate ` + "`json:\"candidates\"`" + `
}

func main() {
	raw, _ := io.ReadAll(os.Stdin)
	if path := os.Getenv("SCORER_CAPTURE"); path != "" {
		_ = os.WriteFile(path, raw, 0o644)
	}
	var req request
	if err := json.Unmarshal(raw, &req); err != nil {
		os.Exit(2)
	}
	mode := os.Getenv("SCORER_MODE")
	switch mode {
	case "malformed":
		fmt.Print("{not-json")
		return
	case "wrongversion":
		fmt.Print(` + "`" + `{"v":"weave.cli/v2","ranked_candidates":[]}` + "`" + `)
		return
	case "timeout":
		time.Sleep(750 * time.Millisecond)
	}
	ranked := make([]map[string]any, 0, len(req.Candidates))
	switch mode {
	case "missing":
		for index, candidate := range req.Candidates {
			if index == len(req.Candidates)-1 {
				continue
			}
			ranked = append(ranked, map[string]any{"id": candidate.ID, "score": 0.5})
		}
	case "extra":
		for _, candidate := range req.Candidates {
			ranked = append(ranked, map[string]any{"id": candidate.ID, "score": 0.5})
		}
		ranked = append(ranked, map[string]any{"id": "unknown", "score": 0.5})
	case "dup":
		for _, candidate := range req.Candidates {
			ranked = append(ranked, map[string]any{"id": candidate.ID, "score": 0.5}, map[string]any{"id": candidate.ID, "score": 0.5})
		}
	default:
		for index, candidate := range req.Candidates {
			score := 0.0
			if len(req.Candidates) > 1 {
				score = float64(index) / float64(len(req.Candidates)-1)
			}
			ranked = append(ranked, map[string]any{"id": candidate.ID, "score": score})
		}
	}
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
		"v":                 "weave.cli/v1",
		"ranked_candidates": ranked,
	})
}
`

func scorerBundle(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	bin := filepath.Join(root, ".weave", "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	program := filepath.Join(t.TempDir(), "main.go")
	if err := os.WriteFile(program, []byte(scorerProgram), 0o644); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(bin, "weave-score")
	build := exec.Command("go", "build", "-o", binary, program)
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build scorer fixture: %v\n%s", err, output)
	}
	for _, name := range []string{"alpha", "beta", "gamma"} {
		body := "---\ntype: Concept\nkeywords: [alpha]\ntitle: " + name + "\n---\n\nalpha " + name + " body.\n"
		if err := os.WriteFile(filepath.Join(root, name+".md"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	capture := filepath.Join(t.TempDir(), "request.json")
	return root, capture
}

func searchEnvelope(t *testing.T, stdout string) struct {
	OK     bool `json:"ok"`
	Result struct {
		TopK    int `json:"top_k"`
		Keyword []struct {
			ID          string   `json:"id"`
			Score       float64  `json:"score"`
			ScorerScore *float64 `json:"scorer_score"`
		} `json:"keyword"`
		Scorer *struct {
			Name       string  `json:"name"`
			Weight     float64 `json:"weight"`
			CandidateK int     `json:"candidate_k"`
		} `json:"scorer"`
	} `json:"result"`
} {
	t.Helper()
	var envelope struct {
		OK     bool `json:"ok"`
		Result struct {
			TopK    int `json:"top_k"`
			Keyword []struct {
				ID          string   `json:"id"`
				Score       float64  `json:"score"`
				ScorerScore *float64 `json:"scorer_score"`
			} `json:"keyword"`
			Scorer *struct {
				Name       string  `json:"name"`
				Weight     float64 `json:"weight"`
				CandidateK int     `json:"candidate_k"`
			} `json:"scorer"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("decode search envelope: %v\n%s", err, stdout)
	}
	return envelope
}

func TestScorerMissingExtension(t *testing.T) {
	root, _ := scorerBundle(t)
	code, stdout, stderr := Invoke(t, "search", root, "alpha", "--no-db", "--scorer", "missing", "--json")
	if code != 2 {
		t.Fatalf("expected exit 2, got %d stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, "EXTENSION_NOT_FOUND") {
		t.Fatalf("expected EXTENSION_NOT_FOUND, got %s", stdout)
	}
}

func TestScorerWeightZeroSkipsInvocation(t *testing.T) {
	root, capture := scorerBundle(t)
	t.Setenv("SCORER_CAPTURE", capture)
	code, core, stderr := Invoke(t, "search", root, "alpha", "--no-db", "--mode", "keyword", "--json")
	if code != 0 {
		t.Fatalf("core exit=%d stderr=%q", code, stderr)
	}
	code, scored, stderr := Invoke(t, "search", root, "alpha", "--no-db", "--mode", "keyword",
		"--scorer", "score", "--scorer-weight", "0", "--json")
	if code != 0 {
		t.Fatalf("scored exit=%d stderr=%q", code, stderr)
	}
	if core != scored {
		t.Fatalf("weight zero must not change output\ncore: %s\nscored: %s", core, scored)
	}
	if _, err := os.Stat(capture); !os.IsNotExist(err) {
		t.Fatalf("scorer must not be invoked at weight zero: %v", err)
	}
}

func TestScorerWeightOneReversesOrder(t *testing.T) {
	root, _ := scorerBundle(t)
	t.Setenv("SCORER_MODE", "reverse")
	code, stdout, stderr := Invoke(t, "search", root, "alpha", "--no-db", "--mode", "keyword",
		"--top-k", "3", "--candidate-k", "3", "--scorer", "score", "--scorer-weight", "1", "--json")
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
	envelope := searchEnvelope(t, stdout)
	if len(envelope.Result.Keyword) != 3 {
		t.Fatalf("expected 3 hits, got %+v", envelope.Result.Keyword)
	}
	got := []string{
		envelope.Result.Keyword[0].ID,
		envelope.Result.Keyword[1].ID,
		envelope.Result.Keyword[2].ID,
	}
	want := []string{"gamma", "beta", "alpha"}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
	if envelope.Result.Scorer == nil || envelope.Result.Scorer.Weight != 1 || envelope.Result.Scorer.CandidateK != 3 {
		t.Fatalf("unexpected scorer metadata: %+v", envelope.Result.Scorer)
	}
	for _, hit := range envelope.Result.Keyword {
		if hit.ScorerScore == nil {
			t.Fatalf("missing scorer_score on %s", hit.ID)
		}
	}
}

func TestScorerWeightHalfBlends(t *testing.T) {
	root, _ := scorerBundle(t)
	t.Setenv("SCORER_MODE", "reverse")
	code, stdout, stderr := Invoke(t, "search", root, "alpha", "--no-db", "--mode", "keyword",
		"--top-k", "2", "--candidate-k", "3", "--scorer", "score", "--scorer-weight", "0.5", "--json")
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
	envelope := searchEnvelope(t, stdout)
	if len(envelope.Result.Keyword) != 2 {
		t.Fatalf("expected top_k=2 hits, got %d", len(envelope.Result.Keyword))
	}
	// Core keyword score is 1.0; reverse scorer gives 0, 0.5, 1.0 by core rank.
	for _, hit := range envelope.Result.Keyword {
		if hit.ScorerScore == nil {
			t.Fatalf("missing scorer_score on %s", hit.ID)
		}
		want := 0.5 + 0.5*(*hit.ScorerScore)
		if hit.Score != want {
			t.Fatalf("%s score = %v, want %v", hit.ID, hit.Score, want)
		}
	}
}

func TestScorerCandidateValidation(t *testing.T) {
	root, _ := scorerBundle(t)
	cases := []struct {
		name string
		args []string
	}{
		{"candidate below top-k", []string{"--scorer", "score", "--top-k", "5", "--candidate-k", "2"}},
		{"candidate above cap", []string{"--scorer", "score", "--candidate-k", "101"}},
		{"weight above one", []string{"--scorer", "score", "--scorer-weight", "1.5"}},
		{"weight without scorer", []string{"--scorer-weight", "0.5"}},
		{"candidate without scorer", []string{"--candidate-k", "5"}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			args := append([]string{"search", root, "alpha", "--no-db", "--json"}, testCase.args...)
			if code, _, stderr := Invoke(t, args...); code != 2 {
				t.Fatalf("expected exit 2, got %d stderr=%q", code, stderr)
			}
		})
	}
}

func TestScorerFallback(t *testing.T) {
	for _, mode := range []string{"missing", "extra", "dup", "malformed", "wrongversion"} {
		t.Run(mode, func(t *testing.T) {
			root, _ := scorerBundle(t)
			t.Setenv("SCORER_MODE", mode)
			code, stdout, stderr := Invoke(t, "search", root, "alpha", "--no-db", "--mode", "keyword",
				"--top-k", "3", "--candidate-k", "3", "--scorer", "score", "--json")
			if code != 0 {
				t.Fatalf("exit=%d stderr=%q", code, stderr)
			}
			if !strings.Contains(stderr, "scorer score") {
				t.Fatalf("expected warning, got %q", stderr)
			}
			envelope := searchEnvelope(t, stdout)
			if envelope.Result.Scorer != nil {
				t.Fatalf("fallback must not add scorer metadata: %+v", envelope.Result.Scorer)
			}
			if len(envelope.Result.Keyword) != 3 {
				t.Fatalf("core result changed: %+v", envelope.Result.Keyword)
			}
		})
	}
}

func TestScorerRequestContext(t *testing.T) {
	root, capture := scorerBundle(t)
	t.Setenv("SCORER_CAPTURE", capture)
	code, _, stderr := Invoke(t, "search", root, "alpha", "--no-db", "--mode", "keyword",
		"--top-k", "2", "--candidate-k", "3", "--scorer", "score", "--json")
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
	raw, err := os.ReadFile(capture)
	if err != nil {
		t.Fatal(err)
	}
	var request struct {
		Type       string `json:"type"`
		Query      string `json:"query"`
		Mode       string `json:"mode"`
		CandidateK int    `json:"candidate_k"`
		Candidates []struct {
			ID       string   `json:"id"`
			Keywords []string `json:"keywords"`
			Snippet  *string  `json:"snippet"`
			Scores   struct {
				Keyword float64 `json:"keyword"`
				Core    float64 `json:"core"`
			} `json:"scores"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(raw, &request); err != nil {
		t.Fatal(err)
	}
	if request.Type != "scorer_request" || request.Query != "alpha" || request.Mode != "keyword" || request.CandidateK != 3 {
		t.Fatalf("unexpected request metadata: %+v", request)
	}
	if len(request.Candidates) != 3 {
		t.Fatalf("expected 3 candidates, got %d", len(request.Candidates))
	}
	for _, candidate := range request.Candidates {
		if candidate.Scores.Core != candidate.Scores.Keyword || candidate.Snippet == nil {
			t.Fatalf("unexpected candidate context: %+v", candidate)
		}
	}
}

func TestScorerTimeoutFallsBack(t *testing.T) {
	root, _ := scorerBundle(t)
	t.Setenv("SCORER_MODE", "timeout")
	code, stdout, stderr := Invoke(t, "search", root, "alpha", "--no-db", "--mode", "keyword",
		"--top-k", "3", "--candidate-k", "3", "--scorer", "score", "--timeout", "50ms", "--json")
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
	if !strings.Contains(stderr, "timed out") {
		t.Fatalf("expected timeout warning, got %q", stderr)
	}
	envelope := searchEnvelope(t, stdout)
	if envelope.Result.Scorer != nil || len(envelope.Result.Keyword) != 3 {
		t.Fatalf("expected unchanged core result: %s", stdout)
	}
}

func TestScorerExplain(t *testing.T) {
	root, _ := scorerBundle(t)
	t.Setenv("SCORER_MODE", "reverse")
	code, stdout, stderr := Invoke(t, "explain", root, "alpha", "--id", "alpha", "--no-db", "--mode", "keyword",
		"--top-k", "3", "--candidate-k", "3", "--scorer", "score", "--scorer-weight", "1", "--json")
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
	var envelope struct {
		OK     bool `json:"ok"`
		Result struct {
			Rank   int `json:"rank"`
			Scorer *struct {
				Name        string  `json:"name"`
				CoreScore   float64 `json:"core_score"`
				ScorerScore float64 `json:"scorer_score"`
				FinalScore  float64 `json:"final_score"`
			} `json:"explain_scorer"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatal(err)
	}
	if !envelope.OK || envelope.Result.Scorer == nil {
		t.Fatalf("missing explain_scorer: %s", stdout)
	}
	if envelope.Result.Scorer.Name != "score" || envelope.Result.Scorer.CoreScore == 0 {
		t.Fatalf("unexpected explain_scorer: %+v", envelope.Result.Scorer)
	}
	if envelope.Result.Rank != 3 {
		t.Fatalf("rank = %d, want 3 after reversal", envelope.Result.Rank)
	}
}

func TestScorerPersistentHybridAndTopK(t *testing.T) {
	root, _ := scorerBundle(t)
	db := filepath.Join(t.TempDir(), "index.db")
	if code, _, stderr := Invoke(t, "ingest", root, "--db", db, "--strict", "--json"); code != 0 {
		t.Fatalf("ingest exit=%d stderr=%q", code, stderr)
	}
	t.Setenv("SCORER_MODE", "reverse")
	code, stdout, stderr := Invoke(t, "search", root, "alpha", "--db", db, "--mode", "hybrid",
		"--top-k", "2", "--candidate-k", "3", "--scorer", "score", "--scorer-weight", "1", "--json")
	if code != 0 {
		t.Fatalf("search exit=%d stderr=%q", code, stderr)
	}
	var envelope struct {
		OK     bool `json:"ok"`
		Result struct {
			TopK   int `json:"top_k"`
			Hybrid []struct {
				ID          string   `json:"id"`
				ScorerScore *float64 `json:"scorer_score"`
			} `json:"hybrid"`
			Scorer *struct {
				CandidateK int `json:"candidate_k"`
			} `json:"scorer"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatal(err)
	}
	if !envelope.OK || len(envelope.Result.Hybrid) != 2 || envelope.Result.TopK != 2 {
		t.Fatalf("unexpected hybrid result: %s", stdout)
	}
	if envelope.Result.Scorer == nil || envelope.Result.Scorer.CandidateK != 3 {
		t.Fatalf("unexpected scorer metadata: %s", stdout)
	}
	for _, hit := range envelope.Result.Hybrid {
		if hit.ScorerScore == nil {
			t.Fatalf("missing scorer_score: %+v", hit)
		}
	}
}

func TestScorerAndRerankAreExclusive(t *testing.T) {
	root, _ := scorerBundle(t)
	code, _, stderr := Invoke(t, "search", root, "alpha", "--no-db", "--scorer", "score", "--rerank", "score", "--json")
	if code != 2 || !strings.Contains(stderr, "mutually exclusive") {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
}
