package operations

import (
	"os"
	"path/filepath"
	"strings"

	contractv1 "github.com/prulloac/weave/contract/v1"
)

const repositoryDocumentID = "_repo"

// applySourceRepository adds the synthetic repository node and contains edges
// for a materialized source snapshot.
func applySourceRepository(bundle *contractv1.Bundle, source *contractv1.SourceContext) {
	if bundle == nil || source == nil || source.Provider == "" || source.Provider == "path" {
		return
	}
	concepts := bundle.Concepts[:0]
	readmePath := strings.TrimSpace(source.Readme)
	readmeID := strings.TrimSuffix(filepath.ToSlash(readmePath), filepath.Ext(readmePath))
	for _, concept := range bundle.Concepts {
		if concept.ID == repositoryDocumentID {
			continue
		}
		if readmePath != "" && (concept.Path == readmePath || concept.ID == readmeID) {
			continue
		}
		concepts = append(concepts, concept)
	}
	bundle.Concepts = concepts

	title := strings.TrimSpace(source.RepositoryTitle)
	if title == "" {
		title = strings.TrimSpace(source.RepositoryName)
	}
	if title == "" {
		title = filepath.Base(filepath.Clean(source.SnapshotPath))
	}
	keywords := repositoryKeywords(source, title)
	concept := contractv1.OkfConcept{
		ID:          repositoryDocumentID,
		Path:        repositoryDocumentID,
		Type:        "Repository",
		Title:       stringPointer(title),
		Description: stringPointer(source.Description),
		Keywords:    keywords,
		Body:        readRepositoryBody(source),
		BodyText:    true,
		ContentKind: "blob",
		ContentMetadata: map[string]any{
			"v":        1,
			"provider": source.Provider,
			"uri":      source.URI,
			"ref":      source.Ref,
			"commit":   source.Commit,
			"host":     source.Host,
			"owner":    source.Owner,
			"name":     source.RepositoryName,
			"readme":   source.Readme,
			"license":  source.License,
			"worktree": source.Worktree,
		},
	}
	for _, current := range bundle.Concepts {
		if current.ID == repositoryDocumentID || current.ContentKind != "blob" {
			continue
		}
		concept.Relations = append(concept.Relations, contractv1.ConceptRelation{
			To:   current.ID,
			Kind: contractv1.RelationContains,
		})
	}
	bundle.Concepts = append(bundle.Concepts, concept)
}

func readRepositoryBody(source *contractv1.SourceContext) string {
	readme := strings.TrimSpace(source.Readme)
	if readme == "" || filepath.IsAbs(readme) {
		return ""
	}
	clean := filepath.Clean(filepath.FromSlash(readme))
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return ""
	}
	path := filepath.Join(source.SnapshotPath, clean)
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func repositoryKeywords(source *contractv1.SourceContext, title string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, 6)
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" || seen[strings.ToLower(value)] || len(result) == 64 {
			return
		}
		seen[strings.ToLower(value)] = true
		result = append(result, value)
	}
	add("repository")
	add(source.Provider)
	add(source.RepositoryName)
	add(title)
	add(source.License)
	return result
}
