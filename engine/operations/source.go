package operations

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	contractv1 "github.com/prulloac/weave/contract/v1"
	sqlitestore "github.com/prulloac/weave/storage/sqlite"
)

// applySourceRepository adds generic synthetic source documents.
func applySourceRepository(bundle *contractv1.Bundle, source *contractv1.SourceContext) error {
	if bundle == nil || source == nil || source.Provider == "" || source.Provider == "path" {
		return nil
	}
	if len(source.Documents) == 0 {
		return fmt.Errorf("source provider returned no synthetic documents")
	}
	return applySyntheticSourceOutput(bundle, source)
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

func filePath(concept *contractv1.OkfConcept) string {
	if path := strings.TrimSpace(filepath.ToSlash(concept.Path)); path != "" {
		return strings.TrimPrefix(path, "./")
	}
	return strings.TrimPrefix(filepath.ToSlash(concept.ID), "./")
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
		"provider":   source.Provider,
		"uri":        source.URI,
		"ref_name":   source.Ref,
		"revision":   source.Commit,
		"commit_sha": source.Commit,
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
