package extensions

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

// Extension data limits. They are fixed for the first post-GA slice.
const (
	ExtensionDataVersion = "weave.extension-data/v1"
	MaxManifestSize      = 1 << 20
	MaxItems             = 256
	MaxFileSize          = 32 << 20
	MaxWorkspaceSize     = 256 << 20
	MaxMetadata          = 8 << 10
	MaxKeyLength         = 256
	MaxMediaType         = 128
)

// Extension data errors.
var (
	ErrDataInvalid  = errors.New("extension data invalid")
	ErrDataTooLarge = errors.New("extension data too large")
	ErrDataQuota    = errors.New("extension data quota exceeded")
)

var dataKeyPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+(?:/[A-Za-z0-9._-]+)*$`)

// DataItem is one manifest entry.
type DataItem struct {
	Key       string          `json:"key"`
	Path      string          `json:"path"`
	MediaType string          `json:"media_type,omitempty"`
	Metadata  json.RawMessage `json:"metadata,omitempty"`
	SHA256    string          `json:"sha256,omitempty"`
}

// DataManifest is the workspace manifest.
type DataManifest struct {
	V     string     `json:"v"`
	Items []DataItem `json:"items"`
}

// DataFile is one validated workspace file ready for import.
type DataFile struct {
	Key        string
	MediaType  string
	Metadata   json.RawMessage
	SHA256     string
	Size       int64
	SourcePath string
	Content    []byte
}

// ValidateWorkspace reads and validates manifest.json and its files.
// It returns (nil, nil) when no manifest is present.
func ValidateWorkspace(workspace string) ([]DataFile, error) {
	if strings.TrimSpace(workspace) == "" {
		return nil, fmt.Errorf("%w: workspace is required", ErrDataInvalid)
	}
	manifestPath := filepath.Join(workspace, "manifest.json")
	info, err := os.Lstat(manifestPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDataInvalid, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%w: manifest must be a regular file", ErrDataInvalid)
	}
	if info.Size() > MaxManifestSize {
		return nil, fmt.Errorf("%w: manifest exceeds %d bytes", ErrDataTooLarge, MaxManifestSize)
	}
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDataInvalid, err)
	}
	var manifest DataManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDataInvalid, err)
	}
	if manifest.V != ExtensionDataVersion {
		return nil, fmt.Errorf("%w: protocol version %q", ErrDataInvalid, manifest.V)
	}
	if len(manifest.Items) > MaxItems {
		return nil, fmt.Errorf("%w: more than %d items", ErrDataInvalid, MaxItems)
	}

	filesDir := filepath.Join(workspace, "files")
	realFilesDir, err := filepath.EvalSymlinks(filesDir)
	if err != nil {
		if len(manifest.Items) == 0 {
			return nil, nil
		}
		return nil, fmt.Errorf("%w: files directory is missing", ErrDataInvalid)
	}

	seen := make(map[string]bool, len(manifest.Items))
	files := make([]DataFile, 0, len(manifest.Items))
	var total int64
	for _, item := range manifest.Items {
		file, err := validateDataItem(filesDir, realFilesDir, item, seen)
		if err != nil {
			return nil, err
		}
		total += file.Size
		if total > MaxWorkspaceSize {
			return nil, fmt.Errorf("%w: workspace exceeds %d bytes", ErrDataQuota, MaxWorkspaceSize)
		}
		files = append(files, file)
	}
	return files, nil
}

func validateDataItem(filesDir, realFilesDir string, item DataItem, seen map[string]bool) (DataFile, error) {
	if len(item.Key) == 0 || len(item.Key) > MaxKeyLength {
		return DataFile{}, fmt.Errorf("%w: invalid key length", ErrDataInvalid)
	}
	if !dataKeyPattern.MatchString(item.Key) {
		return DataFile{}, fmt.Errorf("%w: invalid key %q", ErrDataInvalid, item.Key)
	}
	for _, segment := range strings.Split(item.Key, "/") {
		if segment == "." || segment == ".." {
			return DataFile{}, fmt.Errorf("%w: invalid key segment %q", ErrDataInvalid, segment)
		}
	}
	if seen[item.Key] {
		return DataFile{}, fmt.Errorf("%w: duplicate key %q", ErrDataInvalid, item.Key)
	}
	seen[item.Key] = true

	if len(item.MediaType) > MaxMediaType {
		return DataFile{}, fmt.Errorf("%w: media type exceeds %d bytes", ErrDataInvalid, MaxMediaType)
	}
	if len(item.Metadata) > 0 {
		if len(item.Metadata) > MaxMetadata {
			return DataFile{}, fmt.Errorf("%w: metadata exceeds %d bytes", ErrDataTooLarge, MaxMetadata)
		}
		if bytes.Equal(bytes.TrimSpace(item.Metadata), []byte("null")) {
			return DataFile{}, fmt.Errorf("%w: metadata must be an object", ErrDataInvalid)
		}
		var object map[string]any
		if err := json.Unmarshal(item.Metadata, &object); err != nil {
			return DataFile{}, fmt.Errorf("%w: metadata must be an object", ErrDataInvalid)
		}
	}

	cleanPath := path.Clean(item.Path)
	if cleanPath != item.Path || path.IsAbs(cleanPath) || !strings.HasPrefix(cleanPath, "files/") {
		return DataFile{}, fmt.Errorf("%w: invalid path %q", ErrDataInvalid, item.Path)
	}
	absolute := filepath.Join(filesDir, filepath.FromSlash(strings.TrimPrefix(cleanPath, "files/")))
	relative, err := filepath.Rel(realFilesDir, absolute)
	if err != nil || strings.HasPrefix(relative, "..") || filepath.IsAbs(relative) {
		return DataFile{}, fmt.Errorf("%w: path escapes the workspace", ErrDataInvalid)
	}
	info, err := os.Lstat(absolute)
	if err != nil {
		return DataFile{}, fmt.Errorf("%w: %v", ErrDataInvalid, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return DataFile{}, fmt.Errorf("%w: symlinks are not allowed", ErrDataInvalid)
	}
	if !info.Mode().IsRegular() {
		return DataFile{}, fmt.Errorf("%w: path is not a regular file", ErrDataInvalid)
	}
	realPath, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return DataFile{}, fmt.Errorf("%w: %v", ErrDataInvalid, err)
	}
	relative, err = filepath.Rel(realFilesDir, realPath)
	if err != nil || strings.HasPrefix(relative, "..") || filepath.IsAbs(relative) {
		return DataFile{}, fmt.Errorf("%w: path escapes the workspace", ErrDataInvalid)
	}
	if info.Size() > MaxFileSize {
		return DataFile{}, fmt.Errorf("%w: file exceeds %d bytes", ErrDataTooLarge, MaxFileSize)
	}

	hash, err := hashFile(realPath)
	if err != nil {
		return DataFile{}, fmt.Errorf("%w: %v", ErrDataInvalid, err)
	}
	if item.SHA256 != "" && !strings.EqualFold(item.SHA256, hash) {
		return DataFile{}, fmt.Errorf("%w: sha256 mismatch for %q", ErrDataInvalid, item.Key)
	}
	return DataFile{
		Key:        item.Key,
		MediaType:  item.MediaType,
		Metadata:   append(json.RawMessage(nil), item.Metadata...),
		SHA256:     hash,
		Size:       info.Size(),
		SourcePath: realPath,
	}, nil
}

func hashFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}
