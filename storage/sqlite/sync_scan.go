package sqlite

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
)

func scanBundleHashes(root string) (map[string]string, error) {
	hashes := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == ".git" || entry.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(entry.Name(), ".md") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		hashes[filepath.ToSlash(relative)] = hex.EncodeToString(sum[:])
		return nil
	})
	return hashes, err
}

func changedPaths(existing, scanned map[string]string) (added, updated, removed []string) {
	for path, hash := range scanned {
		old, ok := existing[path]
		if !ok {
			added = append(added, path)
			continue
		}
		if old != hash {
			updated = append(updated, path)
		}
	}
	for path := range existing {
		if _, ok := scanned[path]; !ok {
			removed = append(removed, path)
		}
	}
	return added, updated, removed
}
