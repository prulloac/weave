package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type provider interface {
	chat(ctx context.Context, system, user string) (string, error)
}

func newProvider(cfg config, client *http.Client) (provider, error) {
	switch cfg.Provider {
	case "ollama":
		return &ollamaProvider{baseURL: strings.TrimSuffix(cfg.BaseURL, "/"), model: cfg.Model, client: client}, nil
	case "openai":
		key := cfg.apiKey()
		if key == "" {
			return nil, fmt.Errorf("%s provider: missing api key", cfg.Provider)
		}
		return &openAIProvider{baseURL: strings.TrimSuffix(cfg.BaseURL, "/"), model: cfg.Model, key: key, client: client}, nil
	case "gemini":
		key := cfg.apiKey()
		if key == "" {
			return nil, fmt.Errorf("%s provider: missing api key", cfg.Provider)
		}
		return &geminiProvider{baseURL: strings.TrimSuffix(cfg.BaseURL, "/"), model: cfg.Model, key: key, client: client}, nil
	default:
		return nil, fmt.Errorf("%s provider: unsupported", cfg.Provider)
	}
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaProvider struct {
	baseURL string
	model   string
	client  *http.Client
}

func (p *ollamaProvider) chat(ctx context.Context, system, user string) (string, error) {
	payload := map[string]any{
		"model":  p.model,
		"stream": false,
		"format": "json",
		"options": map[string]any{
			"temperature": 0,
		},
		"messages": []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
	}
	var response struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}
	if err := postJSON(ctx, p.client, p.baseURL+"/api/chat", nil, payload, &response); err != nil {
		return "", fmt.Errorf("ollama provider: %w", err)
	}
	return response.Message.Content, nil
}

type openAIProvider struct {
	baseURL string
	model   string
	key     string
	client  *http.Client
}

func (p *openAIProvider) chat(ctx context.Context, system, user string) (string, error) {
	payload := map[string]any{
		"model":       p.model,
		"temperature": 0,
		"response_format": map[string]any{
			"type": "json_object",
		},
		"messages": []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
	}
	headers := map[string]string{"Authorization": "Bearer " + p.key}
	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := postJSON(ctx, p.client, p.baseURL+"/chat/completions", headers, payload, &response); err != nil {
		return "", fmt.Errorf("openai provider: %w", err)
	}
	if len(response.Choices) == 0 {
		return "", fmt.Errorf("openai provider: empty response")
	}
	return response.Choices[0].Message.Content, nil
}

type geminiProvider struct {
	baseURL string
	model   string
	key     string
	client  *http.Client
}

func (p *geminiProvider) chat(ctx context.Context, system, user string) (string, error) {
	endpoint := p.baseURL + "/v1beta/models/" + url.PathEscape(p.model) + ":generateContent"
	payload := map[string]any{
		"systemInstruction": map[string]any{
			"parts": []any{map[string]any{"text": system}},
		},
		"contents": []any{map[string]any{
			"role":  "user",
			"parts": []any{map[string]any{"text": user}},
		}},
		"generationConfig": map[string]any{
			"temperature":      0,
			"responseMimeType": "application/json",
		},
	}
	headers := map[string]string{"x-goog-api-key": p.key}
	var response struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := postJSON(ctx, p.client, endpoint, headers, payload, &response); err != nil {
		return "", fmt.Errorf("gemini provider: %w", err)
	}
	if len(response.Candidates) == 0 || len(response.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("gemini provider: empty response")
	}
	return response.Candidates[0].Content.Parts[0].Text, nil
}

func postJSON(
	ctx context.Context,
	client *http.Client,
	endpoint string,
	headers map[string]string,
	payload any,
	target any,
) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("request failed")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("status %d", response.StatusCode)
	}
	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		return fmt.Errorf("invalid response")
	}
	return nil
}
