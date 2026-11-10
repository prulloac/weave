// Package nav implements list, show, backlinks and path queries.
package nav

import (
	"path/filepath"
	"sort"
	"strings"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine/graph"
)

// ListFilters filter concepts by metadata.
type ListFilters struct {
	Types  []string
	Tags   []string
	Status string
	Limit  int
}

// FitFilters returns concepts matching metadata filters.
func List(bundle *contractv1.Bundle, filters ListFilters) []contractv1.ListResult {
	limit := filters.Limit
	if limit <= 0 {
		limit = 20
	}

	matches := make([]contractv1.ListResult, 0)
	for _, concept := range bundle.Concepts {
		if len(filters.Types) > 0 && !contains(filters.Types, concept.Type) {
			continue
		}
		if filters.Status != "" {
			if concept.Status == nil || string(*concept.Status) != filters.Status {
				continue
			}
		}
		if len(filters.Tags) > 0 {
			tags := make(map[string]bool, len(concept.Tags))
			for _, tag := range concept.Tags {
				tags[tag] = true
			}
			missing := false
			for _, tag := range filters.Tags {
				if !tags[tag] {
					missing = true
					break
				}
			}
			if missing {
				continue
			}
		}

		title := concept.ID
		if concept.Title != nil {
			title = *concept.Title
		} else if concept.ContentKind == "blob" {
			title = filepath.Base(concept.Path)
		}
		kind := concept.ContentKind
		if kind == "" {
			kind = "document"
		}
		mediaType := concept.MediaType
		if mediaType == "" {
			mediaType = "text/markdown"
		}
		matches = append(matches, contractv1.ListResult{
			ID:          concept.ID,
			Title:       title,
			Path:        concept.Path,
			Type:        concept.Type,
			Keywords:    concept.Keywords,
			Status:      concept.Status,
			Tags:        concept.Tags,
			ContentKind: kind,
			MediaType:   mediaType,
		})
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].ID < matches[j].ID
	})
	if len(matches) > limit {
		matches = matches[:limit]
	}
	return matches
}

// Show returns the detailed concept view.
func Show(bundle *contractv1.Bundle, nodeGraph *graph.Graph, target string) (*contractv1.ShowResult, bool) {
	id, ok := NormalizeTarget(bundle, target)
	if !ok {
		return nil, false
	}

	var concept *contractv1.OkfConcept
	for position := range bundle.Concepts {
		if bundle.Concepts[position].ID == id {
			concept = &bundle.Concepts[position]
			break
		}
	}
	if concept == nil {
		return nil, false
	}

	backlinks := make([]string, 0)
	seen := make(map[string]bool)
	for _, edge := range nodeGraph.Incoming[id] {
		if seen[edge.From] {
			continue
		}
		seen[edge.From] = true
		backlinks = append(backlinks, edge.From)
	}

	links := make([]contractv1.ShowLink, 0, len(concept.Links))
	for _, link := range concept.Links {
		links = append(links, contractv1.ShowLink{
			Raw:              link.Raw,
			Target:           link.Target,
			ResolvesInBundle: link.ResolvesInBundle,
		})
	}

	kind := concept.ContentKind
	if kind == "" {
		kind = "document"
	}
	mediaType := concept.MediaType
	if mediaType == "" {
		mediaType = "text/markdown"
	}
	bodyForm := concept.BodyForm
	if bodyForm == "" {
		bodyForm = "raw"
	}
	size := concept.Size
	if size == 0 && kind == "document" {
		size = int64(len(concept.Body))
	}
	return &contractv1.ShowResult{
		ID:              concept.ID,
		Path:            concept.Path,
		Type:            concept.Type,
		Keywords:        concept.Keywords,
		Title:           concept.Title,
		Description:     concept.Description,
		Tags:            concept.Tags,
		Status:          concept.Status,
		Body:            concept.Body,
		Links:           links,
		Backlinks:       backlinks,
		ContentKind:     kind,
		MediaType:       mediaType,
		ContentMetadata: concept.ContentMetadata,
		BodyForm:        bodyForm,
		BodyCodec:       concept.BodyCodec,
		Size:            size,
		StoredSize:      concept.StoredSize,
		ContentHash:     concept.ContentHash,
		BodyBytes:       concept.BodyBytes,
	}, true
}

// Backlinks returns incoming references for a concept.
func Backlinks(bundle *contractv1.Bundle, nodeGraph *graph.Graph, target string) (*contractv1.BacklinksResult, bool) {
	id, ok := NormalizeTarget(bundle, target)
	if !ok {
		return nil, false
	}
	if _, exists := nodeGraph.Nodes[id]; !exists {
		return nil, false
	}

	seen := make(map[string]bool)
	entries := make([]contractv1.Backlink, 0)
	for _, edge := range nodeGraph.Incoming[id] {
		if seen[edge.From] {
			continue
		}
		seen[edge.From] = true
		title := edge.From
		if node, exists := nodeGraph.Nodes[edge.From]; exists {
			title = node.Title
		}
		entries = append(entries, contractv1.Backlink{From: edge.From, Title: title})
	}

	return &contractv1.BacklinksResult{ID: id, Backlinks: entries}, true
}

// Path returns the shortest directed path between two concepts using default relations.
func Path(bundle *contractv1.Bundle, nodeGraph *graph.Graph, fromInput, toInput string) (*contractv1.PathResult, bool) {
	return PathWithOptions(bundle, nodeGraph, fromInput, toInput, graph.TraversalOptions{})
}

// PathWithOptions returns the shortest directed path allowed by options.
func PathWithOptions(
	bundle *contractv1.Bundle,
	nodeGraph *graph.Graph,
	fromInput, toInput string,
	options graph.TraversalOptions,
) (*contractv1.PathResult, bool) {
	from, ok := NormalizeTarget(bundle, fromInput)
	if !ok {
		return nil, false
	}
	to, ok := NormalizeTarget(bundle, toInput)
	if !ok {
		return nil, false
	}
	return &contractv1.PathResult{From: from, To: to, Path: graph.FindPathWithOptions(nodeGraph, from, to, options)}, true
}

// NormalizeTarget resolves an id, relative path or absolute path to a concept id.
func NormalizeTarget(bundle *contractv1.Bundle, input string) (string, bool) {
	cleaned := strings.TrimSpace(input)
	if cleaned == "" {
		return "", false
	}

	absolute, err := filepath.Abs(cleaned)
	if err == nil {
		root, rootErr := filepath.Abs(bundle.Root)
		if rootErr == nil && strings.HasPrefix(absolute, root+string(filepath.Separator)) {
			if relative, relErr := filepath.Rel(root, absolute); relErr == nil {
				cleaned = filepath.ToSlash(relative)
			}
		}
	}

	cleaned = strings.TrimSuffix(cleaned, ".md")
	cleaned = strings.TrimPrefix(cleaned, "./")
	cleaned = strings.TrimPrefix(cleaned, "/")

	for _, concept := range bundle.Concepts {
		if concept.ID == cleaned {
			return cleaned, true
		}
	}
	return "", false
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
