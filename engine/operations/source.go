package operations

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	contractv1 "github.com/prulloac/weave/contract/v1"
	sqlitestore "github.com/prulloac/weave/storage/sqlite"
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
		if current.ID == repositoryDocumentID {
			continue
		}
		concept.Relations = append(concept.Relations, contractv1.ConceptRelation{
			To:   current.ID,
			Kind: contractv1.RelationContains,
		})
	}
	bundle.Concepts = append(bundle.Concepts, concept)
	applySourceProjects(bundle, len(bundle.Concepts)-1, source)
	bundle.Validation.Warnings = append(bundle.Validation.Warnings, source.Warnings...)
}

func applySourceProjects(bundle *contractv1.Bundle, repositoryIndex int, source *contractv1.SourceContext) {
	if bundle == nil || source == nil || len(source.Projects) == 0 {
		return
	}
	projects := make([]int, 0, len(source.Projects))
	projectByPath := make(map[string]int, len(source.Projects))
	projectIDs := make(map[string]bool, len(source.Projects))
	type packageAggregate struct {
		ID        string
		Name      string
		Ecosystem string
		Versions  map[string]bool
		Scopes    map[string]bool
	}
	packages := make(map[string]*packageAggregate)
	for _, project := range source.Projects {
		projectPath := cleanProjectPath(project.Path)
		if projectPath == "" && project.Path != "" {
			bundle.Validation.Warnings = append(bundle.Validation.Warnings, "skipped project with unsafe path "+project.Path)
			continue
		}
		projectID := projectDocumentID(projectPath)
		if projectIDs[projectID] {
			continue
		}
		projectIDs[projectID] = true
		name := strings.TrimSpace(project.Name)
		if name == "" {
			name = strings.TrimSpace(source.RepositoryName)
		}
		if name == "" {
			name = projectID
		}
		title := name
		description := strings.TrimSpace(project.Description)
		keywords := projectKeywords(project, name, projectPath)
		concept := contractv1.OkfConcept{
			ID:          projectID,
			Path:        projectID,
			Type:        "Project",
			Title:       stringPointer(title),
			Description: stringPointer(description),
			Keywords:    keywords,
			Body:        "",
			BodyText:    true,
			ContentKind: "symbol",
			ContentMetadata: map[string]any{
				"v":            1,
				"provider":     source.Provider,
				"uri":          source.URI,
				"ref":          source.Ref,
				"commit":       source.Commit,
				"path":         projectPath,
				"name":         name,
				"ecosystem":    project.Ecosystem,
				"manifest":     project.Manifest,
				"dependencies": len(project.Dependencies),
			},
			Links: []contractv1.OkfLink{},
		}
		for _, dependency := range project.Dependencies {
			dependencyName := strings.TrimSpace(dependency.Name)
			if dependencyName == "" {
				continue
			}
			key := packageDocumentID(project.Ecosystem, dependencyName)
			aggregate := packages[key]
			if aggregate == nil {
				aggregate = &packageAggregate{
					ID:        key,
					Name:      dependencyName,
					Ecosystem: project.Ecosystem,
					Versions:  map[string]bool{},
					Scopes:    map[string]bool{},
				}
				packages[key] = aggregate
			}
			if version := strings.TrimSpace(dependency.Version); version != "" {
				aggregate.Versions[version] = true
			}
			if scope := strings.TrimSpace(dependency.Scope); scope != "" {
				aggregate.Scopes[scope] = true
			}
			addConceptRelation(&concept, key, contractv1.RelationDependsOn)
		}
		bundle.Concepts = append(bundle.Concepts, concept)
		projects = append(projects, len(bundle.Concepts)-1)
		projectByPath[projectPath] = len(bundle.Concepts) - 1
	}

	for _, index := range projects {
		addConceptRelation(&bundle.Concepts[repositoryIndex], bundle.Concepts[index].ID, contractv1.RelationContains)
	}

	owners := projectOwners(bundle, projectByPath)
	for index := range bundle.Concepts {
		file := &bundle.Concepts[index]
		if isSyntheticProjectDocument(file.ID) {
			continue
		}
		if owner := owners.owner(filePath(file)); owner != nil {
			addConceptRelation(owner, file.ID, contractv1.RelationContains)
		}
	}

	packageIDs := make([]string, 0, len(packages))
	for id := range packages {
		packageIDs = append(packageIDs, id)
	}
	sort.Strings(packageIDs)
	for _, id := range packageIDs {
		aggregate := packages[id]
		versions := sortedKeys(aggregate.Versions)
		scopes := sortedKeys(aggregate.Scopes)
		keywords := []string{aggregate.Name, aggregate.Ecosystem, "external-package"}
		keywords = append(keywords, scopes...)
		title := aggregate.Name
		concept := contractv1.OkfConcept{
			ID:          id,
			Path:        id,
			Type:        "ExternalPackage",
			Title:       stringPointer(title),
			Description: stringPointer(aggregate.Ecosystem + " dependency"),
			Keywords:    boundedKeywords(keywords),
			BodyText:    true,
			ContentKind: "symbol",
			ContentMetadata: map[string]any{
				"v":         1,
				"name":      aggregate.Name,
				"ecosystem": aggregate.Ecosystem,
				"versions":  versions,
				"scopes":    scopes,
			},
			Links: []contractv1.OkfLink{},
		}
		bundle.Concepts = append(bundle.Concepts, concept)
	}
}

type projectOwnerIndex struct {
	projects []int
	bundle   *contractv1.Bundle
}

func projectOwners(bundle *contractv1.Bundle, projects map[string]int) projectOwnerIndex {
	values := make([]int, 0, len(projects))
	for _, index := range projects {
		values = append(values, index)
	}
	sort.Slice(values, func(i, j int) bool {
		return projectPathValue(&bundle.Concepts[values[i]]) > projectPathValue(&bundle.Concepts[values[j]])
	})
	return projectOwnerIndex{projects: values, bundle: bundle}
}

func (index projectOwnerIndex) owner(path string) *contractv1.OkfConcept {
	for _, position := range index.projects {
		project := &index.bundle.Concepts[position]
		projectPath := projectPathValue(project)
		if projectPath == "" {
			return project
		}
		if path == projectPath || strings.HasPrefix(path, projectPath+"/") {
			return project
		}
	}
	return nil
}

func projectPathValue(concept *contractv1.OkfConcept) string {
	if concept == nil || concept.ContentMetadata == nil {
		return ""
	}
	value, _ := concept.ContentMetadata["path"].(string)
	return value
}

func isSyntheticProjectDocument(id string) bool {
	return id == repositoryDocumentID || strings.HasPrefix(id, "_project") || strings.HasPrefix(id, "_package/")
}

func filePath(concept *contractv1.OkfConcept) string {
	if path := strings.TrimSpace(filepath.ToSlash(concept.Path)); path != "" {
		return strings.TrimPrefix(path, "./")
	}
	return strings.TrimPrefix(filepath.ToSlash(concept.ID), "./")
}

func cleanProjectPath(path string) string {
	value := strings.TrimSpace(filepath.ToSlash(path))
	value = strings.TrimPrefix(value, "./")
	value = strings.Trim(value, "/")
	if value == "" {
		return ""
	}
	clean := filepath.ToSlash(filepath.Clean(value))
	if clean == "." {
		return ""
	}
	if clean == ".." || strings.HasPrefix(clean, "../") || filepath.IsAbs(clean) {
		return ""
	}
	return clean
}

func projectDocumentID(path string) string {
	if path == "" {
		return "_project"
	}
	return "_project/" + path
}

func packageDocumentID(ecosystem, name string) string {
	ecosystem = strings.ToLower(strings.TrimSpace(ecosystem))
	if ecosystem == "" {
		ecosystem = "unknown"
	}
	return "_package/" + ecosystem + "/" + packageSlug(name)
}

func packageSlug(name string) string {
	var builder strings.Builder
	lastDash := false
	for _, char := range strings.ToLower(strings.TrimSpace(name)) {
		switch {
		case char >= 'a' && char <= 'z', char >= '0' && char <= '9':
			builder.WriteRune(char)
			lastDash = false
		default:
			if !lastDash && builder.Len() > 0 {
				builder.WriteByte('-')
				lastDash = true
			}
		}
	}
	slug := strings.Trim(builder.String(), "-")
	if slug == "" {
		return "unknown"
	}
	if len(slug) > 96 {
		slug = slug[:96]
	}
	return slug
}

func projectKeywords(project contractv1.SourceProject, name, path string) []string {
	keywords := []string{"project", project.Ecosystem, name}
	if path != "" {
		keywords = append(keywords, path)
	}
	if project.Manifest != "" {
		keywords = append(keywords, filepath.Base(project.Manifest))
	}
	for _, dependency := range project.Dependencies {
		keywords = append(keywords, dependency.Name)
	}
	return boundedKeywords(keywords)
}

func boundedKeywords(values []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[strings.ToLower(value)] {
			continue
		}
		seen[strings.ToLower(value)] = true
		result = append(result, value)
		if len(result) == 64 {
			break
		}
	}
	return result
}

func addConceptRelation(concept *contractv1.OkfConcept, to string, kind contractv1.RelationKind) {
	if concept == nil || strings.TrimSpace(to) == "" {
		return
	}
	for _, relation := range concept.Relations {
		if relation.To == to && relation.Kind == kind {
			return
		}
	}
	concept.Relations = append(concept.Relations, contractv1.ConceptRelation{To: to, Kind: kind})
}

func sortedKeys(values map[string]bool) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
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

func recordSource(ctx context.Context, store *sqlitestore.Store, source *contractv1.SourceContext) error {
	if store == nil || source == nil || source.Provider == "" || source.Provider == "path" {
		return nil
	}
	metadata, err := json.Marshal(map[string]any{
		"v":          1,
		"uri":        source.URI,
		"ref_name":   source.Ref,
		"commit_sha": source.Commit,
		"host":       source.Host,
		"owner":      source.Owner,
		"name":       source.RepositoryName,
		"worktree":   source.Worktree,
	})
	if err != nil {
		return err
	}
	alias := strings.TrimSpace(source.Alias)
	if alias == "" {
		alias = filepath.Base(filepath.Clean(source.SnapshotPath))
	}
	return store.UpsertSource(ctx, sqlitestore.SourceRecord{
		Alias:       alias,
		Path:        source.SnapshotPath,
		Status:      sqlitestore.SourceActive,
		Generation:  1,
		LastSynced:  time.Now().Unix(),
		ContentHash: source.Commit,
		Provider:    source.Provider,
		Metadata:    metadata,
	})
}
