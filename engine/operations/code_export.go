package operations

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"

	contractv1 "github.com/prulloac/weave/contract/v1"
)

type codeExportSymbol struct {
	ID              string         `json:"id"`
	Type            string         `json:"type,omitempty"`
	Title           string         `json:"title,omitempty"`
	Description     string         `json:"description,omitempty"`
	Keywords        []string       `json:"keywords,omitempty"`
	Body            string         `json:"body,omitempty"`
	ContentMetadata map[string]any `json:"content_metadata,omitempty"`
}

type codeExportSymbols struct {
	V       string             `json:"v"`
	Symbols []codeExportSymbol `json:"symbols"`
}

type codeExportRelation struct {
	From string `json:"from"`
	To   string `json:"to"`
	Kind string `json:"kind"`
}

type codeExportRelations struct {
	V         string               `json:"v"`
	Relations []codeExportRelation `json:"relations"`
}

type repositoryExport struct {
	ID              string               `json:"id"`
	Title           string               `json:"title,omitempty"`
	Description     string               `json:"description,omitempty"`
	Keywords        []string             `json:"keywords,omitempty"`
	Body            string               `json:"body,omitempty"`
	ContentMetadata map[string]any       `json:"content_metadata,omitempty"`
	Contains        []codeExportRelation `json:"contains,omitempty"`
}

type repositoriesExport struct {
	V            string             `json:"v"`
	Repositories []repositoryExport `json:"repositories"`
}

// writeCodeExport writes synthetic symbol documents and code relations under
// _code/. It does nothing when the bundle has no code data, so Markdown-only
// exports are unchanged.
func writeCodeExport(target string, concepts []contractv1.OkfConcept) error {
	symbols := make([]codeExportSymbol, 0)
	relations := make([]codeExportRelation, 0)
	for _, concept := range concepts {
		if concept.ContentKind == "symbol" {
			symbols = append(symbols, codeExportSymbol{
				ID:              concept.ID,
				Type:            concept.Type,
				Title:           stringValue(concept.Title),
				Description:     stringValue(concept.Description),
				Keywords:        concept.Keywords,
				Body:            concept.Body,
				ContentMetadata: concept.ContentMetadata,
			})
		}
		for _, relation := range concept.Relations {
			if !codeExportRelationKind(string(relation.Kind)) {
				continue
			}
			relations = append(relations, codeExportRelation{
				From: concept.ID,
				To:   relation.To,
				Kind: string(relation.Kind),
			})
		}
	}
	if len(symbols) == 0 && len(relations) == 0 {
		return nil
	}
	sort.Slice(symbols, func(i, j int) bool { return symbols[i].ID < symbols[j].ID })
	relations = dedupeCodeRelations(relations)
	sort.Slice(relations, func(i, j int) bool {
		if relations[i].From != relations[j].From {
			return relations[i].From < relations[j].From
		}
		if relations[i].To != relations[j].To {
			return relations[i].To < relations[j].To
		}
		return relations[i].Kind < relations[j].Kind
	})

	dir := filepath.Join(target, "_code")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := writeCodeJSON(filepath.Join(dir, "symbols.json"), codeExportSymbols{
		V: "weave.code-symbols/v1", Symbols: symbols,
	}); err != nil {
		return err
	}
	return writeCodeJSON(filepath.Join(dir, "relations.json"), codeExportRelations{
		V: "weave.code-relations/v1", Relations: relations,
	})
}

func writeCodeJSON(path string, value any) error {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(encoded, '\n'), 0o644)
}

// writeRepositoryExport writes synthetic repository documents under _sources/.
func writeRepositoryExport(target string, concepts []contractv1.OkfConcept) error {
	repositories := make([]repositoryExport, 0)
	for _, concept := range concepts {
		if concept.Type != "Repository" {
			continue
		}
		record := repositoryExport{
			ID:              concept.ID,
			Title:           stringValue(concept.Title),
			Description:     stringValue(concept.Description),
			Keywords:        concept.Keywords,
			Body:            concept.Body,
			ContentMetadata: concept.ContentMetadata,
		}
		for _, relation := range concept.Relations {
			if relation.Kind != contractv1.RelationContains {
				continue
			}
			record.Contains = append(record.Contains, codeExportRelation{
				From: concept.ID,
				To:   relation.To,
				Kind: string(relation.Kind),
			})
		}
		repositories = append(repositories, record)
	}
	if len(repositories) == 0 {
		return nil
	}
	sort.Slice(repositories, func(i, j int) bool { return repositories[i].ID < repositories[j].ID })
	dir := filepath.Join(target, "_sources")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return writeCodeJSON(filepath.Join(dir, "repositories.json"), repositoriesExport{
		V: "weave.repositories/v1", Repositories: repositories,
	})
}

func codeExportRelationKind(kind string) bool {
	switch kind {
	case "imports", "declares", "references", "calls", "inherits", "implements":
		return true
	default:
		return false
	}
}

func dedupeCodeRelations(relations []codeExportRelation) []codeExportRelation {
	seen := make(map[string]bool, len(relations))
	result := make([]codeExportRelation, 0, len(relations))
	for _, relation := range relations {
		key := relation.From + "\x00" + relation.To + "\x00" + relation.Kind
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, relation)
	}
	return result
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
