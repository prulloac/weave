package okf

import (
	"context"
	"os"
	"path/filepath"
	"time"
)

// DirEntry is the minimal directory entry surface the walker needs.
type DirEntry struct {
	Name   string
	IsDir  bool
	IsLink bool
}

// FileInfo is the minimal stat surface used by the parser and cache.
type FileInfo struct {
	Size    int64
	ModTime time.Time
	IsDir   bool
	IsFile  bool
}

// FileSystem abstracts bundle IO for tests and future hosts.
type FileSystem interface {
	ReadDir(ctx context.Context, dir string) ([]DirEntry, error)
	ReadFile(ctx context.Context, path string) ([]byte, error)
	Stat(ctx context.Context, path string) (FileInfo, error)
	RealPath(ctx context.Context, path string) (string, error)
}

// OSFileSystem reads bundles from the host filesystem.
type OSFileSystem struct{}

func (OSFileSystem) ReadDir(_ context.Context, dir string) ([]DirEntry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	result := make([]DirEntry, 0, len(entries))
	for _, entry := range entries {
		result = append(result, DirEntry{
			Name:   entry.Name(),
			IsDir:  entry.IsDir(),
			IsLink: entry.Type()&os.ModeSymlink != 0,
		})
	}
	return result, nil
}

func (OSFileSystem) ReadFile(_ context.Context, path string) ([]byte, error) {
	return os.ReadFile(path)
}

func (OSFileSystem) Stat(_ context.Context, path string) (FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return FileInfo{}, err
	}
	return FileInfo{
		Size:    info.Size(),
		ModTime: info.ModTime(),
		IsDir:   info.IsDir(),
		IsFile:  info.Mode().IsRegular(),
	}, nil
}

func (OSFileSystem) RealPath(_ context.Context, path string) (string, error) {
	return filepath.EvalSymlinks(path)
}
