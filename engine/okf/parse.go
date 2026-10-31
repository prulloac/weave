// Package okf parses OKF bundles, validates concepts and extracts links.
package okf

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine/privacy"
)

const (
	maxSymlinkDepth    = 1
	defaultConcurrency = 32
)

var indexEntryPattern = regexp.MustCompile(`^\s*[-*+]\s+\[([^\]]*)\]\(\s*(?:<([^>]+)>|([^()\s]+))(?:\s+(?:"[^"]*"|'[^']*'))?\s*\)\s*(?:[-–—]\s*(.+))?\s*$`)

// Options controls bundle parsing.
type Options struct {
	Cache       bool
	CacheDir    string
	Concurrency int
	Permissive  bool
	FS          FileSystem
	OnCacheHit  func(path string)
}

type conceptPresence struct {
	Frontmatter bool
	Type        bool
	Keywords    bool
}

type fileOutcome struct {
	Concept    *contractv1.OkfConcept        `json:"concept,omitempty"`
	Validation *contractv1.ConceptValidation `json:"validation,omitempty"`
	Presence   *conceptPresence              `json:"-"`
	Index      []contractv1.IndexEntry       `json:"index,omitempty"`
	Version    any                           `json:"version,omitempty"`
	HasIndex   bool                          `json:"has_index,omitempty"`
	IndexError string                        `json:"index_error,omitempty"`
	Skip       bool                          `json:"-"`
}

type fileTask struct {
	rel     string
	base    string
	outcome fileOutcome
}

// ParseBundle parses an OKF bundle into the contract model.
func ParseBundle(ctx context.Context, root string, options Options) (*contractv1.Bundle, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}

	fsys := options.FS
	if fsys == nil {
		fsys = OSFileSystem{}
	}

	visited := make(map[string]bool)
	if real, err := fsys.RealPath(ctx, absRoot); err == nil {
		visited[real] = true
	}

	files, err := walkMarkdown(ctx, fsys, absRoot, visited, 0)
	if err != nil {
		return nil, err
	}
	sort.Strings(files)

	var cache *parseCache
	if options.Cache && !options.Permissive {
		cache, err = openParseCache(absRoot, options.CacheDir)
		if err != nil {
			cache = nil
		}
	}

	concurrency := options.Concurrency
	if concurrency <= 0 {
		concurrency = defaultConcurrency
	}

	tasks, err := mapLimit(ctx, files, concurrency, func(file string) fileTask {
		return processFile(ctx, fsys, file, absRoot, cache, options.OnCacheHit, options.Permissive)
	})
	if err != nil {
		return nil, err
	}

	var index []contractv1.IndexEntry
	hasIndex := false
	var version any
	warnings := make([]string, 0)
	concepts := make([]contractv1.OkfConcept, 0)
	validation := make(map[string]contractv1.ConceptValidation)
	presence := make(map[string]conceptPresence)

	for _, task := range tasks {
		if task.outcome.Skip {
			continue
		}
		if task.base == "index.md" {
			if task.outcome.IndexError != "" {
				warnings = append(warnings, task.outcome.IndexError)
				continue
			}
			index = task.outcome.Index
			hasIndex = task.outcome.HasIndex
			version = task.outcome.Version
			continue
		}
		if task.outcome.Validation != nil {
			validation[conceptID(task.rel)] = *task.outcome.Validation
			continue
		}
		if task.outcome.Concept != nil {
			concepts = append(concepts, *task.outcome.Concept)
			if task.outcome.Presence != nil {
				presence[task.outcome.Concept.ID] = *task.outcome.Presence
			}
		}
	}

	okfVersion := (*contractv1.OkfVersion)(nil)
	if version != nil {
		declared := fmt.Sprint(version)
		if declared == "0.2" {
			value := contractv1.OkfVersion02
			okfVersion = &value
		} else {
			warnings = append(warnings, "unsupported okf_version: "+declared)
		}
	}

	conceptIndex := make(map[string]int, len(concepts))
	for position := range concepts {
		conceptIndex[concepts[position].ID] = position
	}

	for position := range concepts {
		concept := &concepts[position]
		linkWarnings := make([]string, 0)
		directive, privacyErr := privacy.ParseFrontmatter(concept.Frontmatter)
		if privacyErr != nil {
			warnings = append(warnings, privacy.CodeDirectiveInvalid)
			linkWarnings = append(linkWarnings, privacyErr.Error())
		} else if directive != nil {
			concept.Privacy = directive
		}
		if concept.Privacy != nil {
			if _, inlineWarnings := privacy.ScanInline(concept.Body); len(inlineWarnings) > 0 {
				warnings = append(warnings, privacy.CodeDirectiveConflict)
				linkWarnings = append(linkWarnings, privacy.CodeDirectiveConflict+": inline directives ignored")
			}
		} else if _, inlineWarnings := privacy.ScanInline(concept.Body); len(inlineWarnings) > 0 {
			warnings = append(warnings, privacy.CodeDirectiveInvalid)
			linkWarnings = append(linkWarnings, inlineWarnings...)
		}
		for linkPosition := range concept.Links {
			link := &concept.Links[linkPosition]
			link.ResolvesInBundle = false
			if link.Resolved == nil {
				continue
			}
			targetID := strings.TrimSuffix(*link.Resolved, ".md")
			if _, exists := conceptIndex[targetID]; exists {
				link.ResolvesInBundle = true
			} else {
				linkWarnings = append(linkWarnings, "broken link: "+link.Target)
			}
		}
		validation[concept.ID] = validateConcept(*concept, linkWarnings)
		if original, ok := presence[concept.ID]; ok {
			validation[concept.ID] = validatePermissiveConcept(*concept, original, linkWarnings)
		}
	}

	sort.Slice(concepts, func(i, j int) bool {
		return concepts[i].ID < concepts[j].ID
	})

	validationEntries := make([]contractv1.ConceptValidation, 0, len(validation))
	valid := true
	for _, entry := range validation {
		validationEntries = append(validationEntries, entry)
		if !entry.HasFrontmatter || !entry.HasType || !entry.HasKeywords {
			valid = false
		}
	}
	sort.Slice(validationEntries, func(i, j int) bool {
		return validationEntries[i].ID < validationEntries[j].ID
	})

	if len(files) == 0 {
		warnings = append(warnings, "empty bundle: no markdown files found")
	}

	bundle := &contractv1.Bundle{
		Root:       absRoot,
		OkfVersion: okfVersion,
		Concepts:   concepts,
		Index:      nil,
		Validation: contractv1.Validation{
			Valid:      valid,
			Permissive: options.Permissive,
			Concepts:   validationEntries,
			Warnings:   warnings,
		},
	}
	if hasIndex {
		bundle.Index = index
	}

	if cache != nil {
		_ = cache.save()
	}

	return bundle, nil
}

func processFile(
	ctx context.Context,
	fsys FileSystem,
	file string,
	bundleRoot string,
	cache *parseCache,
	onCacheHit func(string),
	permissive bool,
) fileTask {
	rel, err := filepath.Rel(bundleRoot, file)
	if err != nil {
		return fileTask{outcome: fileOutcome{Skip: true}}
	}
	rel = filepath.ToSlash(rel)
	base := filepath.Base(rel)

	if base == "index.md" {
		if rel != "index.md" {
			return fileTask{rel: rel, base: base, outcome: fileOutcome{Skip: true}}
		}
	} else if base == "log.md" {
		return fileTask{rel: rel, base: base, outcome: fileOutcome{Skip: true}}
	}

	var cacheInfo *FileInfo
	if cache != nil {
		if info, statErr := fsys.Stat(ctx, file); statErr == nil {
			cacheInfo = &info
			if outcome, ok := cache.get(file, info); ok {
				if onCacheHit != nil {
					onCacheHit(file)
				}
				return fileTask{rel: rel, base: base, outcome: outcome}
			}
		}
	}

	var outcome fileOutcome
	if base == "index.md" {
		outcome = parseIndexFile(ctx, fsys, file)
	} else {
		outcome = parseConceptFile(ctx, fsys, file, bundleRoot, permissive)
	}

	if cache != nil && cacheInfo != nil && outcome.IndexError == "" && !outcome.Skip {
		cache.set(file, *cacheInfo, outcome)
	}

	return fileTask{rel: rel, base: base, outcome: outcome}
}

func mapLimit(
	ctx context.Context,
	items []string,
	limit int,
	fn func(string) fileTask,
) ([]fileTask, error) {
	results := make([]fileTask, len(items))
	jobs := make(chan int)

	var waitGroup sync.WaitGroup
	workers := limit
	if workers > len(items) {
		workers = len(items)
	}
	for worker := 0; worker < workers; worker++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			for index := range jobs {
				results[index] = fn(items[index])
			}
		}()
	}

	for index := range items {
		select {
		case jobs <- index:
		case <-ctx.Done():
			close(jobs)
			waitGroup.Wait()
			return nil, ctx.Err()
		}
	}
	close(jobs)
	waitGroup.Wait()

	return results, nil
}

func walkMarkdown(
	ctx context.Context,
	fsys FileSystem,
	root string,
	visited map[string]bool,
	symlinkDepth int,
) ([]string, error) {
	entries, err := fsys.ReadDir(ctx, root)
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})

	files := make([]string, 0)
	for _, entry := range entries {
		if entry.Name == ".git" || entry.Name == "node_modules" {
			continue
		}

		absolute := filepath.Join(root, entry.Name)
		isDir := entry.IsDir
		isFile := !entry.IsDir && !entry.IsLink

		if entry.IsLink {
			info, err := fsys.Stat(ctx, absolute)
			if err != nil {
				continue
			}
			isDir = info.IsDir
			isFile = info.IsFile
			if isDir && symlinkDepth >= maxSymlinkDepth {
				continue
			}
		}

		if isDir {
			real, err := fsys.RealPath(ctx, absolute)
			if err != nil {
				continue
			}
			if visited[real] {
				continue
			}
			visited[real] = true
			depth := symlinkDepth
			if entry.IsLink {
				depth++
			}
			nested, err := walkMarkdown(ctx, fsys, absolute, visited, depth)
			if err != nil {
				return nil, err
			}
			files = append(files, nested...)
			continue
		}

		if isFile && strings.HasSuffix(entry.Name, ".md") {
			files = append(files, absolute)
		}
	}

	return files, nil
}

func parseConceptFile(
	ctx context.Context,
	fsys FileSystem,
	file string,
	bundleRoot string,
	permissive bool,
) fileOutcome {
	raw, err := readText(ctx, fsys, file)
	if err != nil {
		return invalidOutcome(conceptIDForFile(bundleRoot, file), "parse error: "+err.Error())
	}

	frontmatter, body, ok, err := ExtractFrontmatter(raw)
	if err != nil {
		return invalidOutcome(conceptIDForFile(bundleRoot, file), "parse error: "+err.Error())
	}
	hadFrontmatter := ok
	if !ok {
		if !permissive {
			return invalidOutcome(conceptIDForFile(bundleRoot, file), "missing frontmatter")
		}
		frontmatter = map[string]any{}
		body = raw
	}

	rel, err := filepath.Rel(bundleRoot, file)
	if err != nil {
		return invalidOutcome(conceptIDForFile(bundleRoot, file), "parse error: "+err.Error())
	}
	rel = filepath.ToSlash(rel)
	id := conceptID(rel)
	dir := "."
	if slash := strings.LastIndex(rel, "/"); slash >= 0 {
		dir = rel[:slash]
	}

	links := ExtractLinks(body)
	for position := range links {
		ResolveLink(&links[position], dir)
	}

	originalType := conceptType(frontmatter["type"])
	originalKeywords := keywords(frontmatter["keywords"])
	hasType := strings.TrimSpace(originalType) != ""
	hasKeywords := len(originalKeywords) > 0

	concept := &contractv1.OkfConcept{
		ID:          id,
		Path:        rel,
		Frontmatter: frontmatter,
		Type:        originalType,
		Keywords:    originalKeywords,
		Title:       optionalString(frontmatter["title"]),
		Description: optionalString(frontmatter["description"]),
		Tags:        optionalStringSlice(frontmatter["tags"]),
		Status:      optionalStatus(frontmatter["status"]),
		Body:        body,
		Links:       links,
	}
	if !permissive {
		return fileOutcome{Concept: concept}
	}

	if concept.Type == "" {
		concept.Type = "Document"
	}
	if concept.Keywords == nil {
		concept.Keywords = []string{}
	}
	if concept.Title == nil {
		concept.Title = fallbackTitle(body, rel)
	}
	if concept.Description == nil {
		concept.Description = fallbackDescription(body)
	}

	return fileOutcome{
		Concept: concept,
		Presence: &conceptPresence{
			Frontmatter: hadFrontmatter,
			Type:        hasType,
			Keywords:    hasKeywords,
		},
	}
}

func validatePermissiveConcept(
	concept contractv1.OkfConcept,
	presence conceptPresence,
	linkWarnings []string,
) contractv1.ConceptValidation {
	warnings := append([]string{}, linkWarnings...)
	if !presence.Frontmatter {
		warnings = append(warnings, "missing frontmatter")
	} else {
		if !presence.Type {
			warnings = append(warnings, "missing type")
		}
		if !presence.Keywords {
			warnings = append(warnings, "missing keywords")
		}
	}
	return contractv1.ConceptValidation{
		ID:             concept.ID,
		HasFrontmatter: presence.Frontmatter,
		HasType:        presence.Type,
		HasKeywords:    presence.Keywords,
		Warnings:       warnings,
	}
}

var fallbackHeading = regexp.MustCompile(`(?m)^#\s+(.+?)\s*$`)

func fallbackTitle(body, rel string) *string {
	if match := fallbackHeading.FindStringSubmatch(body); match != nil {
		title := strings.TrimSpace(match[1])
		if title != "" {
			return &title
		}
	}
	base := filepath.Base(rel)
	title := strings.TrimSuffix(base, filepath.Ext(base))
	return &title
}

func fallbackDescription(body string) *string {
	lines := strings.Split(body, "\n")
	inFence := false
	paragraph := make([]string, 0)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if trimmed == "" {
			if len(paragraph) > 0 {
				break
			}
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			if len(paragraph) > 0 {
				break
			}
			continue
		}
		paragraph = append(paragraph, trimmed)
	}
	if len(paragraph) == 0 {
		return nil
	}
	text := strings.Join(paragraph, " ")
	runes := []rune(text)
	if len(runes) > 160 {
		text = string(runes[:160])
	}
	return &text
}

func parseIndexFile(ctx context.Context, fsys FileSystem, file string) fileOutcome {
	raw, err := readText(ctx, fsys, file)
	if err != nil {
		return fileOutcome{IndexError: "parse error: index.md (" + err.Error() + ")"}
	}

	frontmatter, body, ok, err := ExtractFrontmatter(raw)
	if err != nil {
		return fileOutcome{IndexError: "parse error: index.md (" + err.Error() + ")"}
	}
	if !ok {
		frontmatter = map[string]any{}
		body = raw
	}

	version := frontmatter["okf_version"]
	if version == nil {
		version = frontmatter["okfVersion"]
	}

	return fileOutcome{
		Index:    parseIndexBody(body),
		Version:  version,
		HasIndex: true,
	}
}

func parseIndexBody(body string) []contractv1.IndexEntry {
	entries := make([]contractv1.IndexEntry, 0)
	for _, line := range strings.Split(body, "\n") {
		match := indexEntryPattern.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		target := match[2]
		if target == "" {
			target = match[3]
		}
		target = strings.TrimPrefix(target, "/")
		if target == "" {
			continue
		}
		entry := contractv1.IndexEntry{Title: match[1], Target: target}
		if match[4] != "" {
			description := strings.TrimSpace(match[4])
			if description != "" {
				entry.Description = &description
			}
		}
		entries = append(entries, entry)
	}
	if len(entries) > 0 {
		return entries
	}

	for _, link := range ExtractLinks(body) {
		target := strings.TrimPrefix(link.Target, "/")
		entries = append(entries, contractv1.IndexEntry{
			Title:  LinkText(link.Raw),
			Target: target,
		})
	}
	return entries
}

func readText(ctx context.Context, fsys FileSystem, file string) (string, error) {
	data, err := fsys.ReadFile(ctx, file)
	if err != nil {
		return "", err
	}
	if !utf8.Valid(data) {
		return "", fmt.Errorf("invalid UTF-8")
	}
	return string(data), nil
}

func invalidOutcome(id string, warning string) fileOutcome {
	validation := contractv1.ConceptValidation{
		ID:             id,
		HasFrontmatter: false,
		HasType:        false,
		HasKeywords:    false,
		Warnings:       []string{warning},
	}
	return fileOutcome{Validation: &validation}
}

func conceptIDForFile(bundleRoot string, file string) string {
	rel, err := filepath.Rel(bundleRoot, file)
	if err != nil {
		return ""
	}
	return conceptID(filepath.ToSlash(rel))
}

func conceptID(rel string) string {
	if strings.HasSuffix(strings.ToLower(rel), ".md") {
		return rel[:len(rel)-3]
	}
	return rel
}

func validateConcept(concept contractv1.OkfConcept, warnings []string) contractv1.ConceptValidation {
	hasType := strings.TrimSpace(concept.Type) != ""
	hasKeywords := len(concept.Keywords) > 0
	result := contractv1.ConceptValidation{
		ID:             concept.ID,
		HasFrontmatter: true,
		HasType:        hasType,
		HasKeywords:    hasKeywords,
		Warnings:       warnings,
	}
	if !hasType {
		result.Warnings = append(result.Warnings, "missing type")
	}
	if !hasKeywords {
		result.Warnings = append(result.Warnings, "missing keywords")
	}
	if concept.Title == nil {
		result.Warnings = append(result.Warnings, "missing optional field: title")
	}
	if concept.Description == nil {
		result.Warnings = append(result.Warnings, "missing optional field: description")
	}
	if concept.Tags == nil {
		result.Warnings = append(result.Warnings, "missing optional field: tags")
	}
	if concept.Status == nil {
		result.Warnings = append(result.Warnings, "missing optional field: status")
	}
	return result
}
