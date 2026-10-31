package main

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type config struct {
	Provider       string        `yaml:"provider"`
	Model          string        `yaml:"model"`
	BaseURL        string        `yaml:"base_url"`
	APIKeyEnv      string        `yaml:"api_key_env"`
	AllowSensitive bool          `yaml:"allow_sensitive"`
	Timeout        time.Duration `yaml:"-"`
	TimeoutValue   string        `yaml:"timeout"`
}

func defaultConfig() config {
	return config{
		Provider:     "ollama",
		Model:        "qwen2.5:7b",
		APIKeyEnv:    "WEAVE_AI_API_KEY",
		Timeout:      30 * time.Second,
		TimeoutValue: "30s",
	}
}

func loadConfig() (config, error) {
	cfg := defaultConfig()
	path := os.Getenv("WEAVE_AI_CONFIG")
	if path == "" {
		path = ".weave/ai.yaml"
	}
	if data, err := os.ReadFile(path); err == nil {
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return config{}, fmt.Errorf("config: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return config{}, fmt.Errorf("config: %w", err)
	}

	if value := os.Getenv("WEAVE_AI_PROVIDER"); value != "" {
		cfg.Provider = value
	}
	if value := os.Getenv("WEAVE_AI_MODEL"); value != "" {
		cfg.Model = value
	}
	if value := os.Getenv("WEAVE_AI_BASE_URL"); value != "" {
		cfg.BaseURL = value
	}
	if value := os.Getenv("WEAVE_AI_TIMEOUT"); value != "" {
		cfg.TimeoutValue = value
	}
	if value := os.Getenv("WEAVE_AI_ALLOW_SENSITIVE"); value != "" {
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return config{}, fmt.Errorf("config: invalid WEAVE_AI_ALLOW_SENSITIVE")
		}
		cfg.AllowSensitive = parsed
	}
	if cfg.APIKeyEnv == "" {
		cfg.APIKeyEnv = "WEAVE_AI_API_KEY"
	}
	if cfg.Model == "" {
		cfg.Model = "qwen2.5:7b"
	}
	if cfg.TimeoutValue != "" {
		parsed, err := time.ParseDuration(cfg.TimeoutValue)
		if err != nil || parsed <= 0 {
			return config{}, fmt.Errorf("config: invalid timeout")
		}
		cfg.Timeout = parsed
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}

	switch cfg.Provider {
	case "ollama":
		if cfg.BaseURL == "" {
			cfg.BaseURL = "http://127.0.0.1:11434"
		}
	case "openai":
		if cfg.BaseURL == "" {
			cfg.BaseURL = "https://api.openai.com/v1"
		}
	case "gemini":
		if cfg.BaseURL == "" {
			cfg.BaseURL = "https://generativelanguage.googleapis.com"
		}
	default:
		return config{}, fmt.Errorf("config: unsupported provider %q", cfg.Provider)
	}
	parsed, err := url.Parse(cfg.BaseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return config{}, fmt.Errorf("config: invalid base_url")
	}
	return cfg, nil
}

func (c config) apiKey() string {
	return strings.TrimSpace(os.Getenv(c.APIKeyEnv))
}
