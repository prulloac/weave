// Package codecs resolves built-in and extension-provided body codecs.
package codecs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/prulloac/weave/engine/compression"
)

const (
	// DefaultTimeout bounds one codec invocation.
	DefaultTimeout = 5 * time.Second
	maxOutput      = 64 << 20
)

var namePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// ValidName reports whether name is a syntactically valid codec name.
func ValidName(name string) bool { return namePattern.MatchString(name) }

// Info describes one available codec.
type Info struct {
	Name          string `json:"name"`
	Source        string `json:"source"`
	Path          string `json:"path,omitempty"`
	CanCompress   bool   `json:"can_compress"`
	CanDecompress bool   `json:"can_decompress"`
}

// Error is a codec resolution or execution failure.
type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Message }

func newError(code, message string) *Error {
	return &Error{Code: code, Message: message}
}

// Registry resolves codecs for one bundle root.
type Registry struct {
	BundleRoot string
	Timeout    time.Duration
}

// New creates a registry. A zero timeout uses DefaultTimeout.
func New(bundleRoot string, timeout time.Duration) *Registry {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	return &Registry{BundleRoot: bundleRoot, Timeout: timeout}
}

type manifest struct {
	V          string   `json:"v"`
	Name       string   `json:"name"`
	Kind       string   `json:"kind"`
	Version    string   `json:"version"`
	Directions []string `json:"directions"`
	Frame      bool     `json:"frame"`
}

func (m manifest) can(direction string) bool {
	for _, value := range m.Directions {
		if value == direction {
			return true
		}
	}
	return false
}

// List returns built-in and extension codecs.
func (r *Registry) List(ctx context.Context) ([]Info, error) {
	infos := []Info{
		{Name: "none", Source: "builtin", CanCompress: false, CanDecompress: true},
		{Name: "zstd", Source: "builtin", CanCompress: true, CanDecompress: true},
	}
	seen := map[string]bool{"none": true, "zstd": true}
	for _, candidate := range r.candidates() {
		if seen[candidate.name] {
			continue
		}
		m, err := r.describe(ctx, candidate.path)
		if err != nil {
			continue
		}
		if m.Name != candidate.name || m.Kind != "codec" {
			continue
		}
		seen[candidate.name] = true
		infos = append(infos, Info{
			Name:          candidate.name,
			Source:        candidate.source,
			Path:          candidate.path,
			CanCompress:   m.can("compress"),
			CanDecompress: m.can("decompress"),
		})
	}
	sort.Slice(infos, func(i, j int) bool { return infos[i].Name < infos[j].Name })
	return infos, nil
}

type candidate struct {
	name   string
	path   string
	source string
}

func (r *Registry) candidates() []candidate {
	result := make([]candidate, 0)
	seen := map[string]bool{}
	bundleDir := filepath.Join(r.BundleRoot, ".weave", "bin")
	if entries, err := os.ReadDir(bundleDir); err == nil {
		for _, entry := range entries {
			name, ok := codecName(entry.Name())
			if !ok {
				continue
			}
			path := filepath.Join(bundleDir, entry.Name())
			info, err := entry.Info()
			if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
				continue
			}
			seen[name] = true
			result = append(result, candidate{name: name, path: path, source: "bundle"})
		}
	}
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if dir == "" {
			continue
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			name, ok := codecName(entry.Name())
			if !ok || seen[name] {
				continue
			}
			path := filepath.Join(dir, entry.Name())
			info, err := entry.Info()
			if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
				continue
			}
			seen[name] = true
			result = append(result, candidate{name: name, path: path, source: "path"})
		}
	}
	return result
}

func codecName(file string) (string, bool) {
	if !strings.HasPrefix(file, "weave-codec-") {
		return "", false
	}
	name := strings.TrimPrefix(file, "weave-codec-")
	if !namePattern.MatchString(name) {
		return "", false
	}
	return name, true
}

func (r *Registry) resolve(ctx context.Context, name string) (Info, error) {
	if !namePattern.MatchString(name) {
		return Info{}, newError("CODEC_NOT_FOUND", "codec not found: "+name)
	}
	switch name {
	case "none":
		return Info{Name: "none", Source: "builtin", CanDecompress: true}, nil
	case "zstd":
		return Info{Name: "zstd", Source: "builtin", CanCompress: true, CanDecompress: true}, nil
	}
	for _, candidate := range r.candidates() {
		if candidate.name != name {
			continue
		}
		m, err := r.describe(ctx, candidate.path)
		if err != nil {
			return Info{}, err
		}
		if m.Name != name || m.Kind != "codec" {
			return Info{}, newError("CODEC_PROTOCOL", "codec manifest does not match "+name)
		}
		return Info{
			Name:          name,
			Source:        candidate.source,
			Path:          candidate.path,
			CanCompress:   m.can("compress"),
			CanDecompress: m.can("decompress"),
		}, nil
	}
	return Info{}, newError("CODEC_NOT_FOUND", "codec not found: "+name)
}

func (r *Registry) describe(ctx context.Context, path string) (manifest, error) {
	output, err := r.execute(ctx, path, []string{"--describe", "--json"}, nil)
	if err != nil {
		return manifest{}, err
	}
	var m manifest
	if err := json.Unmarshal(output, &m); err != nil {
		return manifest{}, newError("CODEC_PROTOCOL", "invalid codec manifest")
	}
	if m.V != "weave.extension/v1" || m.Name == "" || m.Kind != "codec" {
		return manifest{}, newError("CODEC_PROTOCOL", "invalid codec manifest")
	}
	return m, nil
}

// Compress encodes data with the named codec.
func (r *Registry) Compress(ctx context.Context, name string, data []byte) ([]byte, error) {
	info, err := r.resolve(ctx, name)
	if err != nil {
		return nil, err
	}
	if name == "none" {
		return data, nil
	}
	if name == "zstd" {
		encoded, _, err := compression.EncodeBlob(compression.CodecZstd, data)
		return encoded, err
	}
	if !info.CanCompress || info.Path == "" {
		return nil, newError("CODEC_FAILED", "codec cannot compress: "+name)
	}
	return r.execute(ctx, info.Path, []string{"compress"}, data)
}

// Decompress decodes data with the named codec.
func (r *Registry) Decompress(ctx context.Context, name string, data []byte) ([]byte, error) {
	info, err := r.resolve(ctx, name)
	if err != nil {
		return nil, err
	}
	switch name {
	case "none":
		return data, nil
	case "zstd":
		return compression.Decode(compression.CodecZstd, data)
	}
	if !info.CanDecompress || info.Path == "" {
		return nil, newError("CODEC_FAILED", "codec cannot decompress: "+name)
	}
	return r.execute(ctx, info.Path, []string{"decompress"}, data)
}

func (r *Registry) execute(ctx context.Context, path string, args []string, input []byte) ([]byte, error) {
	timeout := r.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	runContext, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	command := exec.CommandContext(runContext, path, args...)
	if input != nil {
		command.Stdin = bytes.NewReader(input)
	}
	var stdout, stderr limitedBuffer
	stdout.limit = maxOutput
	stderr.limit = 4096
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	if errors.Is(runContext.Err(), context.DeadlineExceeded) {
		return nil, newError("CODEC_TIMEOUT", "codec timed out: "+filepath.Base(path))
	}
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return nil, newError("CODEC_FAILED", fmt.Sprintf("codec failed with exit %d", exit.ExitCode()))
		}
		return nil, newError("CODEC_FAILED", "codec failed: "+filepath.Base(path))
	}
	if stdout.overflow {
		return nil, newError("CODEC_FAILED", "codec output exceeded the limit")
	}
	return stdout.Bytes(), nil
}

type limitedBuffer struct {
	bytes.Buffer
	limit    int
	overflow bool
}

func (b *limitedBuffer) Write(data []byte) (int, error) {
	if b.Buffer.Len()+len(data) > b.limit {
		b.overflow = true
		return 0, io.ErrShortWrite
	}
	return b.Buffer.Write(data)
}
