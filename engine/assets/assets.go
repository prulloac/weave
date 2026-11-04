// Package assets discovers opaque bundle files and converts them to blob
// concepts without interpreting their media.
package assets

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	contractv1 "github.com/prulloac/weave/contract/v1"
)

const (
	maxBlobBytes   = 32 << 20
	maxBundleBytes = 1 << 30
)

// Filter selects media types. Entries may be exact MIME types or suffix
// wildcards such as image/*.
type Filter struct {
	Types []string
}

// Empty reports whether no media types are selected.
func (f Filter) Empty() bool { return len(f.Types) == 0 }

// Matches reports whether mediaType is selected by the filter.
func (f Filter) Matches(mediaType string) bool {
	for _, value := range f.Types {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		if value == mediaType {
			return true
		}
		if strings.HasSuffix(value, "/*") && strings.HasPrefix(mediaType, strings.TrimSuffix(value, "*")) {
			return true
		}
	}
	return false
}

// Result is the generic scanner output.
type Result struct {
	Concepts []contractv1.OkfConcept
	Warnings []string
}

// Scan walks root and returns blob concepts matching filter.
func Scan(ctx context.Context, root string, filter Filter) (Result, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return Result{}, err
	}
	absolute = filepath.Clean(absolute)

	result := Result{Concepts: make([]contractv1.OkfConcept, 0)}
	total := 0
	err = filepath.WalkDir(absolute, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if path != absolute && ignored(entry.Name(), entry.IsDir()) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(absolute, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if strings.EqualFold(filepath.Ext(rel), ".md") {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			real, err := filepath.EvalSymlinks(path)
			if err != nil || !within(absolute, real) {
				result.Warnings = append(result.Warnings, "ASSET_SYMLINK_ESCAPE: "+rel)
				return nil
			}
			info, err = os.Stat(path)
			if err != nil {
				return err
			}
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		if info.Size() > maxBlobBytes {
			result.Warnings = append(result.Warnings, "ASSET_TOO_LARGE: "+rel)
			return nil
		}
		mediaType, mismatch := detect(rel, path)
		if !filter.Matches(mediaType) {
			return nil
		}
		if mismatch {
			result.Warnings = append(result.Warnings, "ASSET_MEDIA_MISMATCH: "+rel)
		}
		if total+int(info.Size()) > maxBundleBytes {
			return errors.New("asset bundle size limit exceeded")
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		total += len(data)
		text, isText := textBody(mediaType, data)
		concept := contractv1.OkfConcept{
			ID:              rel,
			Path:            rel,
			Keywords:        []string{},
			ContentKind:     "blob",
			MediaType:       mediaType,
			ContentMetadata: map[string]any{"v": 1},
			BodyBytes:       data,
		}
		concept.BodyText = isText
		if isText {
			concept.Body = text
		}
		result.Concepts = append(result.Concepts, concept)
		return nil
	})
	if err != nil {
		return Result{}, err
	}
	return result, nil
}

func ignored(name string, dir bool) bool {
	if name == ".git" || name == "node_modules" || name == ".weave" {
		return true
	}
	if dir && strings.HasPrefix(name, ".") {
		return true
	}
	if !dir {
		if strings.HasPrefix(name, ".") {
			return true
		}
		ext := strings.ToLower(filepath.Ext(name))
		switch ext {
		case ".tmp", ".swp", ".swo":
			return true
		}
		if strings.HasSuffix(name, "~") {
			return true
		}
	}
	return false
}

func within(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func detect(rel, path string) (string, bool) {
	extType := extensionType(strings.ToLower(filepath.Ext(rel)))
	data, err := os.ReadFile(path)
	if err != nil {
		return extType, false
	}
	detected := http.DetectContentType(data)
	if index := strings.IndexByte(detected, ';'); index >= 0 {
		detected = strings.TrimSpace(detected[:index])
	}
	detected = magicType(data, detected)
	if extType == "" {
		return detected, false
	}
	if detected == "" || detected == "application/octet-stream" {
		return extType, false
	}
	if detected != extType {
		return detected, true
	}
	return extType, false
}

func magicType(data []byte, detected string) string {
	if len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		return "image/webp"
	}
	if detected == "text/plain" && len(data) > 0 && !utf8.Valid(data) {
		return "application/octet-stream"
	}
	return detected
}

func extensionType(ext string) string {
	switch ext {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".pdf":
		return "application/pdf"
	case ".txt", ".log", ".md":
		return "text/plain"
	case ".csv":
		return "text/csv"
	case ".json":
		return "application/json"
	case ".xml":
		return "application/xml"
	case ".yaml", ".yml":
		return "application/x-yaml"
	case ".toml":
		return "application/toml"
	case ".html", ".htm":
		return "text/html"
	case ".css":
		return "text/css"
	case ".js":
		return "application/javascript"
	case ".ts":
		return "text/x-typescript"
	case ".go":
		return "text/x-go"
	case ".py":
		return "text/x-python"
	case ".rs":
		return "text/x-rust"
	case ".sh":
		return "text/x-shellscript"
	case ".sql":
		return "text/x-sql"
	case ".ini":
		return "text/plain"
	default:
		return ""
	}
}

func textBody(mediaType string, data []byte) (string, bool) {
	if len(data) == 0 {
		return "", true
	}
	if !utf8.Valid(data) || strings.IndexByte(string(data), 0) >= 0 {
		return "", false
	}
	if textLike(mediaType) {
		return string(data), true
	}
	printable := 0
	total := 0
	for _, value := range string(data) {
		total++
		if value == '\n' || value == '\r' || value == '\t' || value >= 0x20 {
			printable++
		}
	}
	if total > 0 && float64(printable)/float64(total) >= 0.9 {
		return string(data), true
	}
	return "", false
}

func textLike(mediaType string) bool {
	if strings.HasPrefix(mediaType, "text/") {
		return true
	}
	switch mediaType {
	case "application/json", "application/xml", "application/javascript",
		"application/x-yaml", "application/toml":
		return true
	default:
		return false
	}
}
