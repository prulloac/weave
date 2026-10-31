// Package benchutil locates the generated benchmark corpus and query set.
package benchutil

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type Manifest struct {
	Seed     int64                     `json:"seed"`
	Profiles map[string]ProfileSummary `json:"profiles"`
}

type ProfileSummary struct {
	Documents int    `json:"documents"`
	BodyBytes int64  `json:"body_bytes"`
	Edges     int    `json:"edges"`
	Keywords  int    `json:"keywords"`
	SHA256    string `json:"sha256"`
}

type QuerySet struct {
	Version int     `json:"version"`
	Seed    int64   `json:"seed"`
	Queries []Query `json:"queries"`
}

type Query struct {
	Name   string `json:"name"`
	Query  string `json:"query"`
	Mode   string `json:"mode"`
	Expect string `json:"expect"`
}

type Registry struct {
	Version int             `json:"version"`
	Entries []RegistryEntry `json:"entries"`
}

type RegistryEntry struct {
	ID        string   `json:"id"`
	Runner    string   `json:"runner"`
	Selector  string   `json:"selector"`
	Tier      string   `json:"tier"`
	Aggregate string   `json:"aggregate"`
	Unit      string   `json:"unit"`
	Budget    *float64 `json:"budget"`
	Blocking  bool     `json:"blocking"`
}

// Root walks up from the working directory to the module root.
func Root() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "."
		}
		dir = parent
	}
}

// Corpus returns the path for one generated corpus profile.
func Corpus(name string) string {
	return filepath.Join(Root(), "benchmarks", "corpus", name)
}

// RequireCorpus skips the benchmark or test when the corpus has not been generated.
func RequireCorpus(tb testing.TB, name string) string {
	tb.Helper()
	path := Corpus(name)
	if _, err := os.Stat(path); err != nil {
		tb.Skipf("corpus %s missing; run go run ./benchmarks/corpus/gen -out benchmarks/corpus", name)
	}
	return path
}

func LoadManifest() (Manifest, error) {
	data, err := os.ReadFile(filepath.Join(Root(), "benchmarks", "corpus", "manifest.json"))
	if err != nil {
		return Manifest{}, err
	}
	var value Manifest
	if err := json.Unmarshal(data, &value); err != nil {
		return Manifest{}, err
	}
	return value, nil
}

func LoadQueries() (QuerySet, error) {
	data, err := os.ReadFile(filepath.Join(Root(), "benchmarks", "queries.json"))
	if err != nil {
		return QuerySet{}, err
	}
	var value QuerySet
	if err := json.Unmarshal(data, &value); err != nil {
		return QuerySet{}, err
	}
	return value, nil
}

func LoadRegistry() (Registry, error) {
	data, err := os.ReadFile(filepath.Join(Root(), "benchmarks", "registry.json"))
	if err != nil {
		return Registry{}, err
	}
	var value Registry
	if err := json.Unmarshal(data, &value); err != nil {
		return Registry{}, err
	}
	return value, nil
}

func ReadManifest(tb testing.TB) Manifest {
	tb.Helper()
	value, err := LoadManifest()
	if err != nil {
		tb.Fatalf("read manifest: %v", err)
	}
	return value
}

func ReadQueries(tb testing.TB) QuerySet {
	tb.Helper()
	value, err := LoadQueries()
	if err != nil {
		tb.Fatalf("read queries: %v", err)
	}
	return value
}

func ReadRegistry(tb testing.TB) Registry {
	tb.Helper()
	value, err := LoadRegistry()
	if err != nil {
		tb.Fatalf("read registry: %v", err)
	}
	return value
}
