package search

import (
	"encoding/json"
	"sort"
	"strings"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine/graph"
)

var Fields = []contractv1.SearchField{
	contractv1.FieldTitle,
	contractv1.FieldTags,
	contractv1.FieldDescription,
	contractv1.FieldBody,
	contractv1.FieldPath,
	contractv1.FieldType,
	contractv1.FieldFrontmatter,
}

// Document is the indexed projection of a concept.
type Document struct {
	ID      string
	Title   string
	Path    string
	Type    string
	BodyRaw string
}

// Index is the in-memory BM25 index.
type Index struct {
	Documents     []Document
	Postings      map[contractv1.SearchField]map[string]map[int]bool
	TF            []map[contractv1.SearchField]map[string]int
	Lengths       []map[contractv1.SearchField]int
	AverageLength map[contractv1.SearchField]float64
	DegreeBoost   map[string]int
}

// Build indexes a parsed bundle, optionally using graph degree as a boost.
func Build(bundle *contractv1.Bundle, nodeGraph *graph.Graph) *Index {
	index := &Index{
		Documents:     make([]Document, 0, len(bundle.Concepts)),
		Postings:      make(map[contractv1.SearchField]map[string]map[int]bool),
		TF:            make([]map[contractv1.SearchField]map[string]int, 0, len(bundle.Concepts)),
		Lengths:       make([]map[contractv1.SearchField]int, 0, len(bundle.Concepts)),
		AverageLength: make(map[contractv1.SearchField]float64),
		DegreeBoost:   make(map[string]int),
	}

	for _, field := range Fields {
		index.Postings[field] = make(map[string]map[int]bool)
	}

	totalLength := make(map[contractv1.SearchField]int)

	concepts := append([]contractv1.OkfConcept(nil), bundle.Concepts...)
	sortConcepts(concepts)

	for position, concept := range concepts {
		title := concept.ID
		if concept.Title != nil {
			title = *concept.Title
		}
		index.Documents = append(index.Documents, Document{
			ID:      concept.ID,
			Title:   title,
			Path:    concept.Path,
			Type:    concept.Type,
			BodyRaw: StripMarkdown(concept.Body),
		})

		fieldTF := make(map[contractv1.SearchField]map[string]int)
		fieldLengths := make(map[contractv1.SearchField]int)

		for _, field := range Fields {
			tokens := Tokenize(fieldText(concept, field))
			fieldLengths[field] = len(tokens)
			totalLength[field] += len(tokens)

			frequencies := make(map[string]int)
			for _, token := range tokens {
				frequencies[token]++
				posting := index.Postings[field][token]
				if posting == nil {
					posting = make(map[int]bool)
					index.Postings[field][token] = posting
				}
				posting[position] = true
			}
			fieldTF[field] = frequencies
		}

		index.TF = append(index.TF, fieldTF)
		index.Lengths = append(index.Lengths, fieldLengths)
	}

	for _, field := range Fields {
		if len(index.Documents) > 0 {
			index.AverageLength[field] = float64(totalLength[field]) / float64(len(index.Documents))
		}
	}

	if nodeGraph != nil {
		for id, node := range nodeGraph.Nodes {
			index.DegreeBoost[id] = node.InDegree + node.OutDegree
		}
	}

	return index
}

func fieldText(concept contractv1.OkfConcept, field contractv1.SearchField) string {
	switch field {
	case contractv1.FieldTitle:
		if concept.Title == nil {
			return ""
		}
		return *concept.Title
	case contractv1.FieldDescription:
		if concept.Description == nil {
			return ""
		}
		return *concept.Description
	case contractv1.FieldTags:
		return strings.Join(concept.Tags, " ")
	case contractv1.FieldType:
		return concept.Type
	case contractv1.FieldPath:
		return concept.ID + " " + concept.Path
	case contractv1.FieldFrontmatter:
		parts := make([]string, 0, len(concept.Frontmatter))
		for key, value := range concept.Frontmatter {
			encoded := ""
			if text, ok := value.(string); ok {
				encoded = text
			} else {
				data, err := json.Marshal(value)
				if err == nil {
					encoded = string(data)
				}
			}
			parts = append(parts, key+" "+encoded)
		}
		return strings.Join(parts, " ")
	case contractv1.FieldBody:
		return StripMarkdown(concept.Body)
	default:
		return ""
	}
}

func sortConcepts(concepts []contractv1.OkfConcept) {
	sort.Slice(concepts, func(i, j int) bool {
		return concepts[i].ID < concepts[j].ID
	})
}
