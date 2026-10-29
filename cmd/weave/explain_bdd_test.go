package main

import (
	"encoding/json"
	"math"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
)

// These are behavior-level tests for the explain command. They compare the
// explanation against the production search output instead of duplicating the
// scoring formula in the test.

type bddExplainResult struct {
	Query   string            `json:"query"`
	Mode    string            `json:"mode"`
	TopK    int               `json:"top_k"`
	ID      string            `json:"id"`
	Rank    int               `json:"rank"`
	Found   bool              `json:"found"`
	Keyword bddExplainKeyword `json:"keyword"`
	Fuzzy   bddExplainFuzzy   `json:"fuzzy"`
	Hybrid  bddExplainHybrid  `json:"hybrid"`
}

type bddExplainKeyword struct {
	Score           float64           `json:"score"`
	MatchedKeywords []string          `json:"matched_keywords"`
	Tokens          []bddKeywordToken `json:"tokens"`
}

type bddKeywordToken struct {
	QueryToken     string  `json:"query_token"`
	MatchedKeyword string  `json:"matched_keyword"`
	Match          string  `json:"match"`
	Score          float64 `json:"score"`
}

type bddExplainFuzzy struct {
	Score         float64         `json:"score"`
	MatchedFields []string        `json:"matched_fields"`
	Fields        []bddFuzzyField `json:"fields"`
	Tokens        []bddFuzzyToken `json:"tokens"`
	Snippet       *string         `json:"snippet,omitempty"`
}

type bddFuzzyField struct {
	Field        string  `json:"field"`
	Score        float64 `json:"score"`
	Weight       float64 `json:"weight"`
	Contribution float64 `json:"contribution"`
}

type bddFuzzyToken struct {
	QueryToken   string  `json:"query_token"`
	MatchedToken string  `json:"matched_token"`
	Field        string  `json:"field"`
	Similarity   float64 `json:"similarity"`
}

type bddExplainHybrid struct {
	KeywordWeight float64 `json:"keyword_weight"`
	FuzzyWeight   float64 `json:"fuzzy_weight"`
	Total         float64 `json:"total"`
}

func explainFixture() string {
	return filepath.Join("testdata", "explain", "bundle")
}

func runExplainJSON(t *testing.T, args ...string) (int, testEnvelope, bddExplainResult) {
	t.Helper()

	fullArgs := append([]string{"explain"}, args...)
	code, stdout, stderr := invoke(t, fullArgs...)
	if stderr != "" {
		t.Fatalf("expected empty stderr in JSON mode, got %q", stderr)
	}

	envelope := decodeEnvelope(t, stdout)
	if !envelope.OK {
		t.Fatalf("expected completed explain envelope, got failure: %+v", envelope.Error)
	}

	var result bddExplainResult
	if err := json.Unmarshal(envelope.Result, &result); err != nil {
		t.Fatalf("decode explain result: %v\n%s", err, envelope.Result)
	}
	return code, envelope, result
}

func runSearchJSON(t *testing.T, path, query, mode string, topK int) contractv1.SearchResponse {
	t.Helper()

	args := []string{"search", path, query, "--mode", mode, "--json"}
	if topK > 0 {
		args = append(args, "--top-k", strconv.Itoa(topK))
	}
	code, stdout, stderr := invoke(t, args...)
	if code != 0 || stderr != "" {
		t.Fatalf("search exit=%d stderr=%q", code, stderr)
	}
	envelope := decodeEnvelope(t, stdout)
	if !envelope.OK {
		t.Fatalf("search failed: %+v", envelope.Error)
	}
	var response contractv1.SearchResponse
	if err := json.Unmarshal(envelope.Result, &response); err != nil {
		t.Fatalf("decode search result: %v", err)
	}
	return response
}

func keywordHit(response contractv1.SearchResponse, id string) (contractv1.KeywordHit, bool) {
	for _, hit := range response.Keyword {
		if hit.ID == id {
			return hit, true
		}
	}
	return contractv1.KeywordHit{}, false
}

func fuzzyHit(response contractv1.SearchResponse, id string) (contractv1.FuzzyHit, bool) {
	for _, hit := range response.Fuzzy {
		if hit.ID == id {
			return hit, true
		}
	}
	return contractv1.FuzzyHit{}, false
}

func hybridHit(response contractv1.SearchResponse, id string) (contractv1.HybridHit, bool) {
	for _, hit := range response.Hybrid {
		if hit.ID == id {
			return hit, true
		}
	}
	return contractv1.HybridHit{}, false
}

func hybridRank(response contractv1.SearchResponse, id string) int {
	for index, hit := range response.Hybrid {
		if hit.ID == id {
			return index + 1
		}
	}
	return 0
}

func almostEqual(left, right float64) bool {
	return math.Abs(left-right) < 1e-9
}

func TestExplainBehavior(t *testing.T) {
	t.Run("hybrid explanation matches search scores and rank", func(t *testing.T) {
		path := explainFixture()
		search := runSearchJSON(t, path, "oauth", "hybrid", 10)
		code, _, result := runExplainJSON(t,
			path, "oauth", "--id", "concepts/oauth", "--json")

		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
		if !result.Found || result.ID != "concepts/oauth" {
			t.Fatalf("unexpected target: %+v", result)
		}
		if result.Mode != "hybrid" || result.TopK != 10 {
			t.Fatalf("unexpected mode/top-k: %s/%d", result.Mode, result.TopK)
		}
		if want := hybridRank(search, "concepts/oauth"); result.Rank != want {
			t.Fatalf("rank=%d, want %d", result.Rank, want)
		}

		keyword, ok := keywordHit(search, "concepts/oauth")
		if !ok {
			t.Fatal("search keyword hit missing")
		}
		fuzzy, ok := fuzzyHit(search, "concepts/oauth")
		if !ok {
			t.Fatal("search fuzzy hit missing")
		}
		hybrid, ok := hybridHit(search, "concepts/oauth")
		if !ok {
			t.Fatal("search hybrid hit missing")
		}
		if !almostEqual(result.Keyword.Score, keyword.Score) {
			t.Fatalf("keyword score=%f, want %f", result.Keyword.Score, keyword.Score)
		}
		if !almostEqual(result.Fuzzy.Score, fuzzy.Score) {
			t.Fatalf("fuzzy score=%f, want %f", result.Fuzzy.Score, fuzzy.Score)
		}
		if !almostEqual(result.Hybrid.Total, hybrid.TotalScore) {
			t.Fatalf("hybrid total=%f, want %f", result.Hybrid.Total, hybrid.TotalScore)
		}
		if result.Hybrid.KeywordWeight != 0.5 || result.Hybrid.FuzzyWeight != 0.5 {
			t.Fatalf("unexpected hybrid weights: %+v", result.Hybrid)
		}

		exact, ok := findKeywordToken(result.Keyword.Tokens, "oauth")
		if !ok || exact.Match != "exact" || !almostEqual(exact.Score, 1) {
			t.Fatalf("unexpected oauth token evidence: %+v", result.Keyword.Tokens)
		}
	})

	t.Run("defaults to the rank-1 hybrid result", func(t *testing.T) {
		path := explainFixture()
		search := runSearchJSON(t, path, "oauth", "hybrid", 10)
		if len(search.Hybrid) == 0 {
			t.Fatal("expected hybrid results")
		}
		code, _, result := runExplainJSON(t, path, "oauth", "--json")

		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
		if result.ID != search.Hybrid[0].ID || result.Rank != 1 {
			t.Fatalf("got id=%s rank=%d, want id=%s rank=1", result.ID, result.Rank, search.Hybrid[0].ID)
		}
		if !result.Found {
			t.Fatal("expected found=true")
		}
	})

	t.Run("keyword mode explains exact and prefix matches", func(t *testing.T) {
		path := explainFixture()

		code, _, exact := runExplainJSON(t,
			path, "oauth", "--id", "concepts/oauth", "--mode", "keyword", "--json")
		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
		if exact.Mode != "keyword" || !almostEqual(exact.Keyword.Score, 1) {
			t.Fatalf("unexpected exact keyword explanation: %+v", exact)
		}
		token, ok := findKeywordToken(exact.Keyword.Tokens, "oauth")
		if !ok || token.Match != "exact" || token.MatchedKeyword != "oauth" {
			t.Fatalf("unexpected exact evidence: %+v", exact.Keyword.Tokens)
		}

		code, _, prefix := runExplainJSON(t,
			path, "sec", "--id", "concepts/oauth", "--mode", "keyword", "--json")
		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
		if !almostEqual(prefix.Keyword.Score, 0.75) {
			t.Fatalf("prefix score=%f, want 0.75", prefix.Keyword.Score)
		}
		token, ok = findKeywordToken(prefix.Keyword.Tokens, "sec")
		if !ok || token.Match != "prefix" || token.MatchedKeyword != "security" {
			t.Fatalf("unexpected prefix evidence: %+v", prefix.Keyword.Tokens)
		}
	})

	t.Run("keyword mode reports a no-match query token", func(t *testing.T) {
		path := explainFixture()
		code, _, result := runExplainJSON(t,
			path, "oauth missing", "--id", "concepts/oauth", "--mode", "keyword", "--json")
		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
		if !result.Found || result.Rank != 0 {
			t.Fatalf("expected found=true rank=0, got found=%t rank=%d", result.Found, result.Rank)
		}
		if !almostEqual(result.Keyword.Score, 0.5) {
			t.Fatalf("keyword score=%f, want 0.5", result.Keyword.Score)
		}
		token, ok := findKeywordToken(result.Keyword.Tokens, "missing")
		if !ok || token.Match != "none" {
			t.Fatalf("unexpected missing evidence: %+v", result.Keyword.Tokens)
		}
	})

	t.Run("fuzzy mode explains a typo", func(t *testing.T) {
		path := explainFixture()
		search := runSearchJSON(t, path, "tokn", "fuzzy", 10)
		hit, ok := fuzzyHit(search, "concepts/token")
		if !ok {
			t.Fatal("expected token fuzzy hit")
		}
		code, _, result := runExplainJSON(t,
			path, "tokn", "--id", "concepts/token", "--mode", "fuzzy", "--json")

		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
		if result.Mode != "fuzzy" || !almostEqual(result.Fuzzy.Score, hit.Score) {
			t.Fatalf("fuzzy score=%f, want %f", result.Fuzzy.Score, hit.Score)
		}
		found := false
		for _, token := range result.Fuzzy.Tokens {
			if strings.HasPrefix(token.MatchedToken, "token") && token.Similarity > 0 {
				found = true
			}
		}
		if !found {
			t.Fatalf("expected a token typo match, got %+v", result.Fuzzy.Tokens)
		}
	})

	t.Run("known id outside top-K reports rank zero", func(t *testing.T) {
		path := explainFixture()
		search := runSearchJSON(t, path, "token", "hybrid", 10)
		if hybridRank(search, "concepts/token") <= 1 {
			t.Fatalf("fixture must rank token below the first hybrid result: %v", search.Hybrid)
		}
		keyword, _ := keywordHit(search, "concepts/token")
		fuzzy, _ := fuzzyHit(search, "concepts/token")
		hybrid, _ := hybridHit(search, "concepts/token")

		code, _, result := runExplainJSON(t,
			path, "token", "--id", "concepts/token", "--top-k", "1", "--json")
		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
		if !result.Found || result.Rank != 0 {
			t.Fatalf("expected found=true rank=0, got found=%t rank=%d", result.Found, result.Rank)
		}
		if !almostEqual(result.Keyword.Score, keyword.Score) ||
			!almostEqual(result.Fuzzy.Score, fuzzy.Score) ||
			!almostEqual(result.Hybrid.Total, hybrid.TotalScore) {
			t.Fatalf("outside-top-K scores do not match search: %+v", result)
		}
	})

	t.Run("unknown id returns CONCEPT_NOT_FOUND", func(t *testing.T) {
		code, stdout, _ := invoke(t,
			"explain", explainFixture(), "oauth", "--id", "concepts/missing", "--json")
		if code != 1 {
			t.Fatalf("expected exit 1, got %d", code)
		}
		envelope := decodeEnvelope(t, stdout)
		if envelope.OK {
			t.Fatal("expected a failure envelope")
		}
		if envelope.Error == nil || envelope.Error.Code != "CONCEPT_NOT_FOUND" {
			t.Fatalf("expected CONCEPT_NOT_FOUND, got %+v", envelope.Error)
		}
	})

	t.Run("empty query exits 1", func(t *testing.T) {
		code, _, _ := invoke(t, "explain", explainFixture(), "", "--json")
		if code != 1 {
			t.Fatalf("expected exit 1, got %d", code)
		}
	})

	t.Run("no matches exits 1", func(t *testing.T) {
		code, _, _ := invoke(t, "explain", explainFixture(), "zzzz", "--json")
		if code != 1 {
			t.Fatalf("expected exit 1, got %d", code)
		}
	})

	t.Run("explanations are deterministic", func(t *testing.T) {
		path := explainFixture()
		_, _, first := runExplainJSON(t,
			path, "oauth tokn", "--id", "concepts/oauth", "--json")
		_, _, second := runExplainJSON(t,
			path, "oauth tokn", "--id", "concepts/oauth", "--json")

		if !reflect.DeepEqual(first, second) {
			t.Fatalf("explanations are not deterministic:\nfirst:  %+v\nsecond: %+v", first, second)
		}
	})

	t.Run("human output shows components and rank", func(t *testing.T) {
		code, stdout, stderr := invoke(t, "explain", explainFixture(), "oauth")
		if stderr != "" {
			t.Fatalf("expected empty stderr, got %q", stderr)
		}
		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
		for _, want := range []string{
			"query: oauth",
			"mode: hybrid",
			"result: concepts/oauth",
			"rank 1",
			"keyword",
			"fuzzy",
			"hybrid",
		} {
			if !strings.Contains(stdout, want) {
				t.Fatalf("expected human output to contain %q:\n%s", want, stdout)
			}
		}
	})
}

func findKeywordToken(tokens []bddKeywordToken, queryToken string) (bddKeywordToken, bool) {
	for _, token := range tokens {
		if token.QueryToken == queryToken {
			return token, true
		}
	}
	return bddKeywordToken{}, false
}
