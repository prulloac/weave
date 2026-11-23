package extensions

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"
)

// Manifest is the capability document returned by `weave-<name> --describe --json`.
type Manifest struct {
	V                  string                    `json:"v"`
	Name               string                    `json:"name"`
	Version            string                    `json:"version,omitempty"`
	Protocols          []string                  `json:"protocols,omitempty"`
	Requires           []Requirement             `json:"requires,omitempty"`
	Roles              []string                  `json:"roles,omitempty"`
	CacheKey           string                    `json:"cache_key,omitempty"`
	Capabilities       []string                  `json:"capabilities,omitempty"`
	AssetTypes         []string                  `json:"asset_types,omitempty"`
	SourceExtensions   []string                  `json:"source_extensions,omitempty"`
	AnalysisModes      []string                  `json:"analysis_modes,omitempty"`
	SourceProviders    []string                  `json:"source_providers,omitempty"`
	DocumentNamespaces []string                  `json:"document_namespaces,omitempty"`
	ArtifactNamespaces []string                  `json:"artifact_namespaces,omitempty"`
	RelationKinds      []RelationKindDeclaration `json:"relation_kinds,omitempty"`
}

// Requirement is one version requirement on `weave` or on another extension.
type Requirement struct {
	Name       string `json:"name"`
	MinVersion string `json:"min_version"`
}

// RelationKindDeclaration is one extension-contributed graph relation kind.
type RelationKindDeclaration struct {
	Name    string `json:"name"`
	Default bool   `json:"default,omitempty"`
}

// SupportsProjectAnalysis reports whether the manifest declares project analysis.
func (m Manifest) SupportsProjectAnalysis() bool {
	for _, mode := range m.AnalysisModes {
		if mode == "project" {
			return true
		}
	}
	return false
}

// SupportsSourceProvider reports whether the extension provides the named source provider.
func (m Manifest) SupportsSourceProvider(provider string) bool {
	for _, value := range m.SourceProviders {
		if value == provider {
			return true
		}
	}
	return false
}

// SupportsCapability reports whether the manifest advertises one capability.
func (m Manifest) SupportsCapability(capability string) bool {
	for _, value := range m.Capabilities {
		if value == capability {
			return true
		}
	}
	return false
}

// Describe reads one extension capability manifest with the default timeout.
func Describe(ctx context.Context, extension Extension) (Manifest, error) {
	return DescribeWithTimeout(ctx, extension, 5*time.Second)
}

// DescribeWithTimeout reads one extension capability manifest.
func DescribeWithTimeout(ctx context.Context, extension Extension, timeout time.Duration) (Manifest, error) {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	runContext, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	command := exec.CommandContext(runContext, extension.Path, "--describe", "--json")
	command.WaitDelay = 100 * time.Millisecond
	stdout := &cappedBuffer{limit: maxOutput}
	stderr := &cappedBuffer{limit: maxStderr}
	command.Stdout = stdout
	command.Stderr = stderr
	if err := command.Run(); err != nil {
		if runContext.Err() == context.DeadlineExceeded {
			return Manifest{}, fmt.Errorf("%w: %s describe timed out", ErrTimeout, extension.Name)
		}
		return Manifest{}, fmt.Errorf("%w: %s: %s", ErrFailed, extension.Name, stderrLine(stderr.String()))
	}
	if stdout.exceeded {
		return Manifest{}, fmt.Errorf("%w: %s produced oversized describe output", ErrProtocol, extension.Name)
	}
	var manifest Manifest
	if err := json.Unmarshal(stdout.Bytes(), &manifest); err != nil {
		return Manifest{}, fmt.Errorf("%w: %s returned invalid manifest JSON", ErrProtocol, extension.Name)
	}
	if manifest.V != "weave.extension/v1" || manifest.Name != extension.Name {
		return Manifest{}, fmt.Errorf("%w: %s returned an invalid manifest", ErrProtocol, extension.Name)
	}
	return manifest, nil
}
