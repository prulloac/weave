package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const enrichmentInput = `{"v":"weave.cli/v1","type":"enrichment_request","doc_id":"a","title":"A","frontmatter":{},"body":"text"}`
const rerankInput = `{"v":"weave.cli/v1","type":"rerank_request","query":"q","candidates":[{"id":"a"},{"id":"b"}]}`

func invokePlugin(t *testing.T, input string, env map[string]string) (int, string, string) {
	t.Helper()
	t.Setenv("WEAVE_AI_CONFIG", "")
	t.Setenv("WEAVE_AI_PROVIDER", "")
	t.Setenv("WEAVE_AI_MODEL", "")
	t.Setenv("WEAVE_AI_BASE_URL", "")
	t.Setenv("WEAVE_AI_API_KEY", "")
	t.Setenv("WEAVE_AI_TIMEOUT", "")
	for key, value := range env {
		t.Setenv(key, value)
	}
	var stdout, stderr bytes.Buffer
	code := run(strings.NewReader(input), &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestOllamaEnrichment(t *testing.T) {
	content := `{"inferred_keywords":["sqlite","wal"],"custom_metadata":{"summary":"ok"}}`
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/chat" {
			t.Errorf("unexpected path %s", request.URL.Path)
		}
		var payload map[string]any
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Errorf("decode request: %v", err)
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"message": map[string]any{"content": content},
		})
	}))
	defer server.Close()

	code, stdout, stderr := invokePlugin(t, enrichmentInput, map[string]string{
		"WEAVE_AI_BASE_URL": server.URL,
	})
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
	var response struct {
		V                string   `json:"v"`
		InferredKeywords []string `json:"inferred_keywords"`
	}
	if err := json.Unmarshal([]byte(stdout), &response); err != nil {
		t.Fatalf("decode response: %v\n%s", err, stdout)
	}
	if response.V != "weave.cli/v1" || len(response.InferredKeywords) != 2 {
		t.Fatalf("unexpected response: %s", stdout)
	}
}

func TestOpenAIRerank(t *testing.T) {
	content := `{"ranked_candidates":[{"id":"a","rerank_score":0.9},{"id":"b","rerank_score":1.5}]}`
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/chat/completions" {
			t.Errorf("unexpected path %s", request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("missing bearer token: %q", request.Header.Get("Authorization"))
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"choices": []any{map[string]any{"message": map[string]any{"content": content}}},
		})
	}))
	defer server.Close()

	code, stdout, stderr := invokePlugin(t, rerankInput, map[string]string{
		"WEAVE_AI_PROVIDER": "openai",
		"WEAVE_AI_BASE_URL": server.URL,
		"WEAVE_AI_API_KEY":  "test-key",
	})
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
	var response struct {
		Ranked []struct {
			ID    string  `json:"id"`
			Score float64 `json:"rerank_score"`
		} `json:"ranked_candidates"`
	}
	if err := json.Unmarshal([]byte(stdout), &response); err != nil {
		t.Fatalf("decode: %v\n%s", err, stdout)
	}
	if len(response.Ranked) != 2 || response.Ranked[1].Score != 1 {
		t.Fatalf("expected clamped scores, got %s", stdout)
	}
}

func TestGeminiEnrichment(t *testing.T) {
	content := `{"inferred_keywords":["gemini"],"custom_metadata":{}}`
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if !strings.Contains(request.URL.Path, ":generateContent") {
			t.Errorf("unexpected path %s", request.URL.Path)
		}
		if request.Header.Get("x-goog-api-key") != "gem-key" {
			t.Errorf("missing gemini key")
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"candidates": []any{map[string]any{
				"content": map[string]any{"parts": []any{map[string]any{"text": content}}},
			}},
		})
	}))
	defer server.Close()

	code, stdout, stderr := invokePlugin(t, enrichmentInput, map[string]string{
		"WEAVE_AI_PROVIDER": "gemini",
		"WEAVE_AI_BASE_URL": server.URL,
		"WEAVE_AI_API_KEY":  "gem-key",
	})
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, "gemini") {
		t.Fatalf("unexpected response: %s", stdout)
	}
}

func TestConfigFile(t *testing.T) {
	content := `{"inferred_keywords":["config"],"custom_metadata":{}}`
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"message": map[string]any{"content": content},
		})
	}))
	defer server.Close()

	config := "provider: ollama\nmodel: test\nbase_url: " + server.URL + "\ntimeout: 5s\n"
	path := filepath.Join(t.TempDir(), "ai.yaml")
	if err := os.WriteFile(path, []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := invokePlugin(t, enrichmentInput, map[string]string{
		"WEAVE_AI_CONFIG": path,
	})
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, "config") {
		t.Fatalf("unexpected response: %s", stdout)
	}
}

func TestMalformedModelOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"message": map[string]any{"content": "not json"},
		})
	}))
	defer server.Close()

	code, stdout, _ := invokePlugin(t, enrichmentInput, map[string]string{
		"WEAVE_AI_BASE_URL": server.URL,
	})
	if code != 1 || stdout != "" {
		t.Fatalf("expected failure without output, code=%d stdout=%q", code, stdout)
	}
}

func TestUnknownRequestType(t *testing.T) {
	code, stdout, _ := invokePlugin(t, `{"v":"weave.cli/v1","type":"unsupported"}`, nil)
	if code != 1 || stdout != "" {
		t.Fatalf("expected failure, code=%d stdout=%q", code, stdout)
	}
}

func TestInvalidProvider(t *testing.T) {
	code, _, stderr := invokePlugin(t, enrichmentInput, map[string]string{
		"WEAVE_AI_PROVIDER": "bogus",
	})
	if code != 1 || !strings.Contains(stderr, "provider") {
		t.Fatalf("expected provider error, code=%d stderr=%q", code, stderr)
	}
}

func TestErrorsDoNotLeakSecrets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusUnauthorized)
		_, _ = writer.Write([]byte("provider body super-secret-body"))
	}))
	defer server.Close()

	code, stdout, stderr := invokePlugin(t, enrichmentInput, map[string]string{
		"WEAVE_AI_PROVIDER": "openai",
		"WEAVE_AI_BASE_URL": server.URL,
		"WEAVE_AI_API_KEY":  "key-secret-value",
	})
	if code != 1 || stdout != "" {
		t.Fatalf("expected failure, code=%d stdout=%q", code, stdout)
	}
	for _, secret := range []string{"key-secret-value", "super-secret-body", "text"} {
		if strings.Contains(stderr, secret) {
			t.Fatalf("stderr leaked %q: %q", secret, stderr)
		}
	}
}
