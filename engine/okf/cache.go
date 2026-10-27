package okf

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

const cacheVersion = 1

type cacheEntry struct {
	MTime   int64       `json:"mtime_ns"`
	Size    int64       `json:"size"`
	Outcome fileOutcome `json:"outcome"`
}

type cacheManifest struct {
	Version int                   `json:"version"`
	Files   map[string]cacheEntry `json:"files"`
}

type parseCache struct {
	mu       sync.Mutex
	path     string
	manifest cacheManifest
	seen     map[string]bool
	dirty    bool
}

func openParseCache(root, directory string) (*parseCache, error) {
	if directory == "" {
		directory = os.Getenv("WEAVE_CACHE_DIR")
	}
	if directory == "" {
		directory = filepath.Join(os.TempDir(), "weave-okf-cache")
	}

	hash := sha256.Sum256([]byte(root))
	name := hex.EncodeToString(hash[:])[:24] + ".json"
	path := filepath.Join(directory, name)

	cache := &parseCache{
		path: path,
		manifest: cacheManifest{
			Version: cacheVersion,
			Files:   make(map[string]cacheEntry),
		},
		seen: make(map[string]bool),
	}

	if err := os.MkdirAll(directory, 0o755); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err == nil {
		var manifest cacheManifest
		if json.Unmarshal(data, &manifest) == nil && manifest.Version == cacheVersion && manifest.Files != nil {
			cache.manifest = manifest
		}
	}
	return cache, nil
}

func (cache *parseCache) get(path string, info FileInfo) (fileOutcome, bool) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	entry, ok := cache.manifest.Files[path]
	if !ok {
		return fileOutcome{}, false
	}
	if entry.MTime != info.ModTime.UnixNano() || entry.Size != info.Size {
		return fileOutcome{}, false
	}
	cache.seen[path] = true
	return entry.Outcome, true
}

func (cache *parseCache) set(path string, info FileInfo, outcome fileOutcome) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	cache.seen[path] = true
	cache.manifest.Files[path] = cacheEntry{
		MTime:   info.ModTime.UnixNano(),
		Size:    info.Size,
		Outcome: outcome,
	}
	cache.dirty = true
}

func (cache *parseCache) save() error {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	if !cache.dirty {
		return nil
	}

	files := make(map[string]cacheEntry, len(cache.seen))
	for path := range cache.seen {
		if entry, ok := cache.manifest.Files[path]; ok {
			files[path] = entry
		}
	}
	cache.manifest.Files = files

	data, err := json.Marshal(cache.manifest)
	if err != nil {
		return err
	}
	temp := cache.path + ".tmp"
	if err := os.WriteFile(temp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(temp, cache.path)
}
