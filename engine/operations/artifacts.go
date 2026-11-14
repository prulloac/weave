package operations

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	sqlitestore "github.com/prulloac/weave/storage/sqlite"
)

func writeStoredArtifacts(ctx context.Context, target string, store *sqlitestore.Store, root string) error {
	records, err := store.ListArtifacts(ctx, root)
	if err != nil {
		return err
	}
	for _, record := range records {
		relative := filepath.ToSlash(filepath.Clean(record.Path))
		if relative == "." || relative == ".." || strings.HasPrefix(relative, "../") || filepath.IsAbs(relative) {
			return fmt.Errorf("unsafe artifact path %q", record.Path)
		}
		path := filepath.Join(target, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		content := record.Content
		if len(content) == 0 {
			content = []byte("null")
		}
		if strings.HasPrefix(record.MediaType, "application/json") && (len(content) == 0 || content[len(content)-1] != '\n') {
			content = append(content, '\n')
		}
		if err := os.WriteFile(path, content, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func hasStoredArtifact(ctx context.Context, store *sqlitestore.Store, root, path string) bool {
	records, err := store.ListArtifacts(ctx, root)
	if err != nil {
		return false
	}
	for _, record := range records {
		if record.Path == path {
			return true
		}
	}
	return false
}
