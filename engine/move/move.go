// Package move plans and applies a document move with link and related-id rewriting.
package move

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	contractv1 "github.com/prulloac/weave/contract/v1"
)

var (
	// ErrInvalidRequest marks an invalid source or destination path.
	ErrInvalidRequest = errors.New("invalid move request")
	// ErrDestinationExists marks an occupied destination.
	ErrDestinationExists = errors.New("destination exists")
	// ErrConceptNotFound marks a missing source document.
	ErrConceptNotFound = errors.New("source document not found")
	// ErrPrivacyDirective marks protected content that cannot be inspected.
	ErrPrivacyDirective = errors.New("privacy directive blocks inspection")
)

// Write is one file to write as part of a move.
type Write struct {
	Path string
	Data []byte
}

// Plan is a computed, not yet applied move.
type Plan struct {
	Root      string
	From      string
	To        string
	Writes    []Write
	Remove    string
	Rewritten []contractv1.MoveRewrite
}

// Build plans a move from one bundle-relative Markdown path to another.
func Build(root, from, to string) (*Plan, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
	}
	info, err := os.Stat(absRoot)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("%w: bundle root is not a directory: %s", ErrInvalidRequest, root)
	}
	from, err = cleanPath(from)
	if err != nil {
		return nil, err
	}
	to, err = cleanPath(to)
	if err != nil {
		return nil, err
	}
	fromAbs := filepath.Join(absRoot, filepath.FromSlash(from))
	toAbs := filepath.Join(absRoot, filepath.FromSlash(to))

	sourceInfo, err := os.Lstat(fromAbs)
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%w: %s", ErrConceptNotFound, from)
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
	}
	if !sourceInfo.Mode().IsRegular() || sourceInfo.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("%w: source must be a regular Markdown file: %s", ErrInvalidRequest, from)
	}

	sameFile := false
	destinationInfo, err := os.Lstat(toAbs)
	switch {
	case err == nil:
		sameFile = os.SameFile(sourceInfo, destinationInfo)
		if !sameFile {
			return nil, fmt.Errorf("%w: %s", ErrDestinationExists, to)
		}
	case !errors.Is(err, os.ErrNotExist):
		return nil, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
	}
	if strings.EqualFold(from, to) && !sameFile {
		// A case-only rename on a case-sensitive filesystem is still a valid move.
		sameFile = false
	}

	files, err := markdownFiles(absRoot)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
	}
	ids := make(map[string]bool, len(files))
	for _, file := range files {
		ids[strings.TrimSuffix(file, ".md")] = true
	}

	plan := &Plan{Root: absRoot, From: from, To: to, Rewritten: []contractv1.MoveRewrite{}}
	if from == to {
		return plan, nil
	}
	plan.Remove = from

	for _, file := range files {
		fileAbs := filepath.Join(absRoot, filepath.FromSlash(file))
		data, err := os.ReadFile(fileAbs)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
		}
		moved := file == from
		outputPath := file
		outputDir := path.Dir(file)
		resolveDir := path.Dir(file)
		if moved {
			outputPath = to
			outputDir = path.Dir(to)
		}
		context := rewriteContext{
			oldPath:    from,
			oldID:      strings.TrimSuffix(from, ".md"),
			newPath:    to,
			newID:      strings.TrimSuffix(to, ".md"),
			fileID:     strings.TrimSuffix(file, ".md"),
			resolveDir: resolveDir,
			outputDir:  outputDir,
			moved:      moved,
			ids:        ids,
		}
		if protectedContent(string(data)) {
			return nil, fmt.Errorf("%w: %s", ErrPrivacyDirective, file)
		}
		rewritten, references, err := rewriteDocument(string(data), context)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
		}
		if moved || references > 0 {
			plan.Writes = append(plan.Writes, Write{Path: outputPath, Data: []byte(rewritten)})
			plan.Rewritten = append(plan.Rewritten, contractv1.MoveRewrite{
				Path:       outputPath,
				References: references,
			})
		}
	}
	sort.Slice(plan.Rewritten, func(i, j int) bool {
		return plan.Rewritten[i].Path < plan.Rewritten[j].Path
	})
	sort.Slice(plan.Writes, func(i, j int) bool {
		return plan.Writes[i].Path < plan.Writes[j].Path
	})
	if sameFile {
		plan.Remove = ""
	}
	return plan, nil
}

// Apply writes the plan and runs after. Any failure restores the original files.
func (p *Plan) Apply(after func() error) error {
	if len(p.Writes) == 0 && p.Remove == "" {
		if after != nil {
			return after()
		}
		return nil
	}
	type backup struct {
		path string
		data []byte
		mode fs.FileMode
	}
	backups := make(map[string]backup)
	for _, write := range p.Writes {
		abs := filepath.Join(p.Root, filepath.FromSlash(write.Path))
		data, err := os.ReadFile(abs)
		if err == nil {
			info, statErr := os.Stat(abs)
			if statErr != nil {
				return statErr
			}
			backups[abs] = backup{path: abs, data: data, mode: info.Mode()}
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if p.Remove != "" {
		abs := filepath.Join(p.Root, filepath.FromSlash(p.Remove))
		data, err := os.ReadFile(abs)
		if err == nil {
			info, statErr := os.Stat(abs)
			if statErr != nil {
				return statErr
			}
			backups[abs] = backup{path: abs, data: data, mode: info.Mode()}
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	restore := func() {
		for _, write := range p.Writes {
			abs := filepath.Join(p.Root, filepath.FromSlash(write.Path))
			if original, ok := backups[abs]; ok {
				_ = os.MkdirAll(filepath.Dir(abs), 0o755)
				_ = os.WriteFile(abs, original.data, original.mode.Perm())
				continue
			}
			_ = os.Remove(abs)
		}
		if p.Remove != "" {
			oldAbs := filepath.Join(p.Root, filepath.FromSlash(p.Remove))
			if original, ok := backups[oldAbs]; ok {
				_ = os.WriteFile(oldAbs, original.data, original.mode.Perm())
			}
		}
	}

	for _, write := range p.Writes {
		abs := filepath.Join(p.Root, filepath.FromSlash(write.Path))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			restore()
			return err
		}
		if err := os.WriteFile(abs, write.Data, 0o644); err != nil {
			restore()
			return err
		}
	}
	if p.Remove != "" {
		oldAbs := filepath.Join(p.Root, filepath.FromSlash(p.Remove))
		newAbs := filepath.Join(p.Root, filepath.FromSlash(p.To))
		if oldAbs != newAbs {
			if err := os.Remove(oldAbs); err != nil {
				restore()
				return err
			}
		}
	}
	if after != nil {
		if err := after(); err != nil {
			restore()
			return err
		}
	}
	return nil
}

func cleanPath(value string) (string, error) {
	value = filepath.ToSlash(strings.TrimSpace(value))
	if value == "" || filepath.IsAbs(value) || strings.HasPrefix(value, "/") {
		return "", fmt.Errorf("%w: path must be bundle-relative: %q", ErrInvalidRequest, value)
	}
	cleaned := path.Clean(value)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("%w: path escapes the bundle: %q", ErrInvalidRequest, value)
	}
	if !strings.HasSuffix(strings.ToLower(cleaned), ".md") {
		return "", fmt.Errorf("%w: path must end in .md: %q", ErrInvalidRequest, value)
	}
	return cleaned, nil
}

func relSlash(fromDir, destination string) (string, error) {
	relative, err := filepath.Rel(filepath.FromSlash(fromDir), filepath.FromSlash(destination))
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(relative), nil
}

func markdownFiles(root string) ([]string, error) {
	files := make([]string, 0)
	err := filepath.WalkDir(root, func(file string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if file != root && (entry.Name() == ".weave" || entry.Name() == ".git") {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(entry.Name()), ".md") {
			return nil
		}
		rel, err := filepath.Rel(root, file)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

func protectedContent(content string) bool {
	_, body := splitFrontmatter(content)
	if !strings.HasPrefix(strings.TrimSpace(body), "enc:v1:") {
		return false
	}
	frontmatter, _ := splitFrontmatter(content)
	for _, line := range strings.Split(frontmatter, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "policy: encrypt" || trimmed == "policy: \"encrypt\"" || trimmed == "policy: 'encrypt'" {
			return true
		}
	}
	return false
}

// splitFrontmatter returns the frontmatter block (without delimiters) and the body.
func splitFrontmatter(content string) (string, string) {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	if !strings.HasPrefix(normalized, "---\n") {
		return "", content
	}
	end := strings.Index(normalized[4:], "\n---")
	if end < 0 {
		return "", content
	}
	end += 4
	frontmatter := normalized[4:end]
	bodyStart := end + len("\n---")
	if bodyStart < len(normalized) && normalized[bodyStart] == '\n' {
		bodyStart++
	}
	return frontmatter, normalized[bodyStart:]
}
