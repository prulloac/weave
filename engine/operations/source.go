package operations

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	contractv1 "github.com/prulloac/weave/contract/v1"
	sqlitestore "github.com/prulloac/weave/storage/sqlite"
)

const repositoryDocumentID = "_repo"

// applySourceRepository adds synthetic source documents. Generic descriptors are
// applied as data; legacy provider fields keep their existing adapter.
func applySourceRepository(bundle *contractv1.Bundle, source *contractv1.SourceContext) error {
	if bundle == nil || source == nil {
		return nil
	}
	if len(source.Documents) > 0 || len(source.Relations) > 0 {
		return applySyntheticSourceOutput(bundle, source)
	}
	applyLegacySourceRepository(bundle, source)
	return nil
}

func applyLegacySourceRepository(bundle *contractv1.Bundle, source *contractv1.SourceContext) {
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
			"v":          1,
			"provider":   source.Provider,
			"uri":        source.URI,
			"ref":        source.Ref,
			"commit":     source.Commit,
			"host":       source.Host,
			"owner":      source.Owner,
			"name":       source.RepositoryName,
			"host_kind":  source.HostKind,
			"web_url":    source.WebURL,
			"commit_url": source.CommitURL,
			"readme":     source.Readme,
			"license":    source.License,
			"worktree":   source.Worktree,
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
	repositoryIndex := len(bundle.Concepts) - 1
	applySourceProjects(bundle, repositoryIndex, source)
	applySourceCommits(bundle, repositoryIndex, source)
	bundle.Validation.Warnings = append(bundle.Validation.Warnings, source.Warnings...)
}

func applySyntheticSourceOutput(bundle *contractv1.Bundle, source *contractv1.SourceContext) error {
	if len(source.Documents) > 0 && len(source.DocumentNamespaces) == 0 {
		return fmt.Errorf("synthetic documents require declared document namespaces")
	}
	for _, namespace := range source.DocumentNamespaces {
		if !strings.HasPrefix(strings.TrimSpace(namespace), "_") {
			return fmt.Errorf("invalid document namespace %q", namespace)
		}
	}
	consumed := make(map[string]bool)
	for _, document := range source.Documents {
		for _, path := range document.Consumes {
			clean := cleanSourcePath(path)
			if strings.TrimSpace(path) != "" && clean == "" {
				return fmt.Errorf("invalid consumed path %q", path)
			}
			if clean != "" {
				consumed[clean] = true
			}
		}
	}
	if len(consumed) > 0 {
		filtered := bundle.Concepts[:0]
		for _, concept := range bundle.Concepts {
			path := cleanSourcePath(filePath(&concept))
			if path != "" && consumed[path] {
				continue
			}
			filtered = append(filtered, concept)
		}
		bundle.Concepts = filtered
	}
	positions := make(map[string]int, len(bundle.Concepts)+len(source.Documents))
	pathIndex := make(map[string]string, len(bundle.Concepts)+len(source.Documents))
	for index := range bundle.Concepts {
		concept := &bundle.Concepts[index]
		positions[concept.ID] = index
		if path := cleanSourcePath(filePath(concept)); path != "" {
			pathIndex[path] = concept.ID
		}
	}
	syntheticIDs := make(map[string]bool, len(source.Documents))
	for _, document := range source.Documents {
		id := strings.TrimSpace(document.ID)
		if id == "" {
			return fmt.Errorf("synthetic document id is required")
		}
		if !safeSyntheticID(id) {
			return fmt.Errorf("invalid synthetic document id %q", id)
		}
		if !hasNamespace(id, source.DocumentNamespaces) {
			return fmt.Errorf("synthetic document %q is outside declared namespaces", id)
		}
		if _, exists := positions[id]; exists {
			return fmt.Errorf("synthetic document %q collides with an existing document", id)
		}
		path := strings.TrimSpace(document.Path)
		if path == "" {
			path = id
		} else if path = cleanSourcePath(path); path == "" {
			return fmt.Errorf("invalid synthetic document path %q", document.Path)
		}
		contentKind := strings.TrimSpace(document.ContentKind)
		if contentKind == "" {
			contentKind = "symbol"
		}
		title := strings.TrimSpace(document.Title)
		keywords := boundedKeywords(document.Keywords)
		concept := contractv1.OkfConcept{
			ID:              id,
			Path:            path,
			Type:            strings.TrimSpace(document.Type),
			Title:           stringPointer(title),
			Description:     stringPointer(document.Description),
			Keywords:        keywords,
			Body:            document.Body,
			BodyText:        true,
			ContentKind:     contentKind,
			ContentMetadata: document.Metadata,
			Links:           []contractv1.OkfLink{},
		}
		if concept.Type == "" {
			return fmt.Errorf("synthetic document %q type is required", id)
		}
		bundle.Concepts = append(bundle.Concepts, concept)
		positions[id] = len(bundle.Concepts) - 1
		if path := cleanSourcePath(path); path != "" {
			pathIndex[path] = id
		}
		syntheticIDs[id] = true
	}
	for _, document := range source.Documents {
		if document.Scope == nil {
			continue
		}
		position, ok := positions[document.ID]
		if !ok {
			continue
		}
		scope := cleanSourcePath(*document.Scope)
		if strings.TrimSpace(*document.Scope) != "" && scope == "" {
			return fmt.Errorf("invalid document scope %q", *document.Scope)
		}
		for index := range bundle.Concepts {
			file := &bundle.Concepts[index]
			if syntheticIDs[file.ID] {
				continue
			}
			path := cleanSourcePath(filePath(file))
			if path == "" {
				continue
			}
			if scope != "" && path != scope && !strings.HasPrefix(path, scope+"/") {
				continue
			}
			addConceptRelation(&bundle.Concepts[position], file.ID, contractv1.RelationContains)
		}
	}
	for _, relation := range source.Relations {
		from := strings.TrimSpace(relation.From)
		if from == "" {
			return fmt.Errorf("relation from is required")
		}
		fromPosition, ok := positions[from]
		if !ok {
			return fmt.Errorf("relation source %q does not exist", from)
		}
		target := strings.TrimSpace(relation.To)
		if target == "" {
			path := cleanSourcePath(relation.ToPath)
			if path == "" {
				return fmt.Errorf("relation target is required")
			}
			target, ok = pathIndex[path]
			if !ok {
				bundle.Validation.Warnings = append(bundle.Validation.Warnings,
					fmt.Sprintf("relation target %q was skipped because the file is not indexed", relation.ToPath))
				continue
			}
		}
		if _, ok := positions[target]; !ok {
			return fmt.Errorf("relation target %q does not exist", target)
		}
		kind := contractv1.RelationKind(strings.TrimSpace(string(relation.Kind)))
		if !allowedSyntheticRelation(kind, source.RelationKinds) {
			return fmt.Errorf("relation kind %q is not registered", kind)
		}
		raw := relation.Raw
		if !conceptHasRelation(&bundle.Concepts[fromPosition], target, kind) {
			bundle.Concepts[fromPosition].Relations = append(bundle.Concepts[fromPosition].Relations, contractv1.ConceptRelation{
				To: target, Kind: kind, Raw: raw,
			})
		}
	}
	bundle.Validation.Warnings = append(bundle.Validation.Warnings, source.Warnings...)
	return nil
}

func conceptHasRelation(concept *contractv1.OkfConcept, to string, kind contractv1.RelationKind) bool {
	for _, relation := range concept.Relations {
		if relation.To == to && relation.Kind == kind {
			return true
		}
	}
	return false
}

func hasNamespace(id string, namespaces []string) bool {
	for _, namespace := range namespaces {
		if strings.HasPrefix(id, strings.TrimSpace(namespace)) {
			return true
		}
	}
	return false
}

func safeSyntheticID(id string) bool {
	if strings.Contains(id, "\\") || strings.HasPrefix(id, "/") {
		return false
	}
	for _, segment := range strings.Split(id, "/") {
		if segment == ".." {
			return false
		}
	}
	return true
}

func allowedSyntheticRelation(kind contractv1.RelationKind, declared []string) bool {
	switch kind {
	case contractv1.RelationLink, contractv1.RelationRelated, contractv1.RelationContains:
		return true
	}
	for _, value := range declared {
		if kind == contractv1.RelationKind(strings.TrimSpace(value)) {
			return true
		}
	}
	return false
}

func cleanSourcePath(path string) string {
	value := strings.TrimSpace(filepath.ToSlash(path))
	value = strings.TrimPrefix(value, "./")
	value = strings.Trim(value, "/")
	if value == "" {
		return ""
	}
	clean := filepath.ToSlash(filepath.Clean(value))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || filepath.IsAbs(clean) {
		return ""
	}
	return clean
}

func applySourceCommits(bundle *contractv1.Bundle, repositoryIndex int, source *contractv1.SourceContext) {
	if bundle == nil || source == nil || len(source.Commits) == 0 {
		return
	}
	positions := make(map[string]int, len(source.Commits))
	for _, commit := range source.Commits {
		sha := strings.ToLower(strings.TrimSpace(commit.SHA))
		if sha == "" {
			continue
		}
		if _, exists := positions[sha]; exists {
			continue
		}
		id := "_commit/" + sha
		title := strings.TrimSpace(commit.Subject)
		if title == "" {
			title = shortSHA(sha)
		}
		parents := make([]string, 0, len(commit.Parents))
		for _, parent := range commit.Parents {
			if value := strings.ToLower(strings.TrimSpace(parent)); value != "" {
				parents = append(parents, value)
			}
		}
		keywords := boundedKeywords([]string{"commit", shortSHA(sha), commit.Author, title})
		concept := contractv1.OkfConcept{
			ID:          id,
			Path:        id,
			Type:        "Commit",
			Title:       stringPointer(title),
			Description: stringPointer(strings.TrimSpace(commit.Subject)),
			Keywords:    keywords,
			BodyText:    true,
			ContentKind: "symbol",
			ContentMetadata: map[string]any{
				"v":        1,
				"sha":      sha,
				"parents":  parents,
				"author":   commit.Author,
				"date":     commit.Date,
				"subject":  commit.Subject,
				"repo_uri": source.URI,
				"repo_ref": source.Ref,
			},
			Links: []contractv1.OkfLink{},
		}
		bundle.Concepts = append(bundle.Concepts, concept)
		positions[sha] = len(bundle.Concepts) - 1
		addConceptRelation(&bundle.Concepts[repositoryIndex], id, contractv1.RelationContains)
	}
	for _, commit := range source.Commits {
		sha := strings.ToLower(strings.TrimSpace(commit.SHA))
		position, ok := positions[sha]
		if !ok {
			continue
		}
		for _, parent := range commit.Parents {
			parent = strings.ToLower(strings.TrimSpace(parent))
			parentPosition, ok := positions[parent]
			if !ok {
				continue
			}
			addConceptRelation(&bundle.Concepts[position], bundle.Concepts[parentPosition].ID, contractv1.RelationParentOf)
		}
	}
}

func shortSHA(sha string) string {
	if len(sha) > 8 {
		return sha[:8]
	}
	return sha
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

func sourceRelationDefaults(source *contractv1.SourceContext) []string {
	if source == nil {
		return nil
	}
	return source.DefaultRelations
}

func artifactRecords(source *contractv1.SourceContext) ([]sqlitestore.ArtifactRecord, error) {
	if source == nil {
		return nil, nil
	}
	return artifactRecordsFrom(source.Artifacts)
}

func artifactRecordsFrom(values []contractv1.SyntheticArtifact) ([]sqlitestore.ArtifactRecord, error) {
	if len(values) == 0 {
		return nil, nil
	}
	result := make([]sqlitestore.ArtifactRecord, 0, len(values))
	for _, artifact := range values {
		content, err := json.Marshal(artifact.Content)
		if err != nil {
			return nil, err
		}
		result = append(result, sqlitestore.ArtifactRecord{
			Path: artifact.Path, MediaType: artifact.MediaType, Content: content,
		})
	}
	return result, nil
}

func mergeSourceMetadata(source *contractv1.SourceContext) map[string]any {
	values := map[string]any{
		"v":          1,
		"uri":        source.URI,
		"ref_name":   source.Ref,
		"revision":   source.Commit,
		"commit_sha": source.Commit,
		"host":       source.Host,
		"owner":      source.Owner,
		"name":       source.RepositoryName,
		"worktree":   source.Worktree,
	}
	for key, value := range source.Metadata {
		if strings.TrimSpace(key) != "" {
			values[key] = value
		}
	}
	return values
}

func recordSource(ctx context.Context, store *sqlitestore.Store, source *contractv1.SourceContext) error {
	if store == nil || source == nil || source.Provider == "" || source.Provider == "path" {
		return nil
	}
	metadata, err := json.Marshal(mergeSourceMetadata(source))
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
