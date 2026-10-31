package privacy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/prulloac/weave/engine/extensions"
)

// Provider performs cryptographic operations for privacy directives.
type Provider interface {
	Name() string
	Capabilities(ctx context.Context) (ProviderCapabilities, error)
	Encrypt(ctx context.Context, algorithm, keyID string, plaintext []byte) (string, int, error)
	Decrypt(ctx context.Context, algorithm, keyID string, keyVersion int, ciphertext string) (string, error)
	Tokenize(ctx context.Context, algorithm, keyID, value string) (string, int, error)
	Hash(ctx context.Context, algorithm, keyID, value string) (string, int, error)
	Close()
}

// ProviderCapabilities is the capability handshake payload.
type ProviderCapabilities struct {
	Provider   string   `json:"provider"`
	Operations []string `json:"operations"`
	Algorithms []string `json:"algorithms"`
	KeyIDs     []string `json:"key_ids"`
}

type cryptoConfig struct {
	DefaultProvider string                    `yaml:"default_provider"`
	Providers       map[string]providerConfig `yaml:"providers"`
}

type providerConfig struct {
	Binary string `yaml:"binary"`
}

// ResolveProvider resolves a provider name from crypto.yaml and returns a
// provider. A nil provider means the built-in local provider.
func ResolveProvider(root, requested string) (Provider, error) {
	cfg := cryptoConfig{}
	path := filepath.Join(root, ".weave", "crypto.yaml")
	if data, err := os.ReadFile(path); err == nil {
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("%w: invalid crypto.yaml", ErrKeyUnavailable)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("%w: cannot read crypto.yaml", ErrKeyUnavailable)
	}
	name := strings.TrimSpace(requested)
	if name == "" {
		name = strings.TrimSpace(cfg.DefaultProvider)
	}
	if name == "" || name == "local" {
		return nil, nil
	}

	binary := ""
	if entry, ok := cfg.Providers[name]; ok {
		binary = strings.TrimSpace(entry.Binary)
	}
	if binary == "" {
		resolved, err := extensions.Resolve(root, "crypto-"+name)
		if err != nil {
			return nil, fmt.Errorf("%w: provider %s not found", ErrKeyUnavailable, name)
		}
		return &subprocessProvider{name: name, path: resolved.Path}, nil
	}
	if !strings.ContainsRune(binary, os.PathSeparator) {
		located, err := exec.LookPath(binary)
		if err != nil {
			return nil, fmt.Errorf("%w: provider %s not found", ErrKeyUnavailable, name)
		}
		binary = located
	}
	return &subprocessProvider{name: name, path: binary}, nil
}

type subprocessProvider struct {
	name string
	path string
}

func (p *subprocessProvider) Name() string { return p.name }

func (p *subprocessProvider) Close() {}

func (p *subprocessProvider) Capabilities(ctx context.Context) (ProviderCapabilities, error) {
	var response struct {
		V          string   `json:"v"`
		Type       string   `json:"type"`
		Provider   string   `json:"provider"`
		Operations []string `json:"operations"`
		Algorithms []string `json:"algorithms"`
		KeyIDs     []string `json:"key_ids"`
	}
	if err := p.invoke(ctx, []string{"--capabilities"}, nil, &response); err != nil {
		return ProviderCapabilities{}, err
	}
	if response.V != "weave.cli/v1" || response.Type != "crypto_capabilities" {
		return ProviderCapabilities{}, fmt.Errorf("%w: %s provider: invalid capabilities", ErrKeyUnavailable, p.name)
	}
	return ProviderCapabilities{
		Provider:   response.Provider,
		Operations: response.Operations,
		Algorithms: response.Algorithms,
		KeyIDs:     response.KeyIDs,
	}, nil
}

func (p *subprocessProvider) Encrypt(
	ctx context.Context,
	algorithm, keyID string,
	plaintext []byte,
) (string, int, error) {
	response, err := p.request(ctx, "encrypt", algorithm, keyID, 0, map[string]any{
		"plaintext": string(plaintext),
	})
	if err != nil {
		return "", 0, err
	}
	if response.Ciphertext == nil || *response.Ciphertext == "" {
		return "", 0, fmt.Errorf("%w: %s provider: empty ciphertext", ErrKeyUnavailable, p.name)
	}
	return *response.Ciphertext, response.KeyVersion, nil
}

func (p *subprocessProvider) Decrypt(
	ctx context.Context,
	algorithm, keyID string,
	keyVersion int,
	ciphertext string,
) (string, error) {
	response, err := p.request(ctx, "decrypt", algorithm, keyID, keyVersion, map[string]any{
		"ciphertext": ciphertext,
	})
	if err != nil {
		return "", err
	}
	if response.Plaintext == nil {
		return "", fmt.Errorf("%w: %s provider: empty plaintext", ErrKeyUnavailable, p.name)
	}
	return *response.Plaintext, nil
}

func (p *subprocessProvider) Tokenize(ctx context.Context, algorithm, keyID, value string) (string, int, error) {
	response, err := p.request(ctx, "tokenize", algorithm, keyID, 0, map[string]any{"value": value})
	if err != nil {
		return "", 0, err
	}
	if response.Token == "" {
		return "", 0, fmt.Errorf("%w: %s provider: empty token", ErrKeyUnavailable, p.name)
	}
	return response.Token, response.KeyVersion, nil
}

func (p *subprocessProvider) Hash(ctx context.Context, algorithm, keyID, value string) (string, int, error) {
	response, err := p.request(ctx, "hash", algorithm, keyID, 0, map[string]any{"value": value})
	if err != nil {
		return "", 0, err
	}
	if response.Digest == "" {
		return "", 0, fmt.Errorf("%w: %s provider: empty digest", ErrKeyUnavailable, p.name)
	}
	return response.Digest, response.KeyVersion, nil
}

type cryptoResponse struct {
	Ciphertext *string `json:"ciphertext"`
	Plaintext  *string `json:"plaintext"`
	Token      string  `json:"token"`
	Digest     string  `json:"digest"`
	KeyVersion int     `json:"key_version"`
}

func (p *subprocessProvider) request(
	ctx context.Context,
	operation, algorithm, keyID string,
	keyVersion int,
	payload map[string]any,
) (cryptoResponse, error) {
	request := map[string]any{
		"v":         "weave.cli/v1",
		"type":      "crypto_request",
		"operation": operation,
		"provider":  p.name,
		"algorithm": algorithm,
		"key_id":    keyID,
	}
	if keyVersion > 0 {
		request["key_version"] = keyVersion
	}
	for key, value := range payload {
		request[key] = value
	}
	var response struct {
		V          string  `json:"v"`
		Type       string  `json:"type"`
		Provider   string  `json:"provider"`
		KeyID      string  `json:"key_id"`
		Operation  string  `json:"operation"`
		Ciphertext *string `json:"ciphertext"`
		Plaintext  *string `json:"plaintext"`
		Token      string  `json:"token"`
		Digest     string  `json:"digest"`
		KeyVersion int     `json:"key_version"`
	}
	if err := p.invoke(ctx, nil, request, &response); err != nil {
		return cryptoResponse{}, err
	}
	if response.V != "weave.cli/v1" ||
		response.Provider != p.name ||
		response.KeyID != keyID ||
		(response.Operation != "" && response.Operation != operation) {
		return cryptoResponse{}, fmt.Errorf("%w: %s provider: mismatched response", ErrKeyUnavailable, p.name)
	}
	return cryptoResponse{
		Ciphertext: response.Ciphertext,
		Plaintext:  response.Plaintext,
		Token:      response.Token,
		Digest:     response.Digest,
		KeyVersion: response.KeyVersion,
	}, nil
}

func (p *subprocessProvider) invoke(ctx context.Context, args []string, payload any, target any) error {
	timeout := 5 * time.Second
	runContext, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	command := exec.CommandContext(runContext, p.path, args...)
	command.WaitDelay = 100 * time.Millisecond
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("%w: %s provider: invalid request", ErrKeyUnavailable, p.name)
		}
		command.Stdin = bytes.NewReader(append(data, '\n'))
	}
	stdout := &limitedBuffer{limit: 1 << 20}
	command.Stdout = stdout
	command.Stderr = io.Discard
	if err := command.Run(); err != nil {
		if runContext.Err() == context.DeadlineExceeded {
			return fmt.Errorf("%w: %s provider: timeout", ErrKeyUnavailable, p.name)
		}
		return fmt.Errorf("%w: %s provider: process failed", ErrKeyUnavailable, p.name)
	}
	if stdout.exceeded {
		return fmt.Errorf("%w: %s provider: oversized response", ErrKeyUnavailable, p.name)
	}
	if err := json.Unmarshal(stdout.Bytes(), target); err != nil {
		return fmt.Errorf("%w: %s provider: invalid response", ErrKeyUnavailable, p.name)
	}
	return nil
}

type limitedBuffer struct {
	buf      bytes.Buffer
	limit    int
	exceeded bool
}

func (b *limitedBuffer) Write(data []byte) (int, error) {
	remaining := b.limit - b.buf.Len()
	if remaining <= 0 {
		b.exceeded = true
		return len(data), nil
	}
	if len(data) > remaining {
		b.buf.Write(data[:remaining])
		b.exceeded = true
		return len(data), nil
	}
	b.buf.Write(data)
	return len(data), nil
}

func (b *limitedBuffer) Bytes() []byte { return b.buf.Bytes() }
