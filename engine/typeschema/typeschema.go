// Package typeschema loads and applies per-type frontmatter schemas from .weave/types.
package typeschema

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"

	contractv1 "github.com/prulloac/weave/contract/v1"
)

// ErrInvalid marks a schema file that cannot be loaded or compiled.
var ErrInvalid = errors.New("invalid type schema")

// ErrUnsupported marks a schema that relies on a remote reference.
var ErrUnsupported = errors.New("unsupported type schema reference")

var typeNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)

// Violation is one schema failure for a document.
type Violation struct {
	Path    string
	Message string
}

// Set is the schema collection for one bundle.
type Set struct {
	schemas map[string]*jsonschema.Schema
}

// Empty reports whether the bundle declared no type schemas.
func (s *Set) Empty() bool { return s == nil || len(s.schemas) == 0 }

// Load reads .weave/types/*.schema.json under root. A missing directory yields an empty set.
func Load(root string) (*Set, error) {
	dir := filepath.Join(root, ".weave", "types")
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return &Set{schemas: map[string]*jsonschema.Schema{}}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	type schemaFile struct {
		name string
		typ  string
		path string
		doc  any
	}
	files := make([]schemaFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".schema.json") {
			continue
		}
		typ := strings.TrimSuffix(entry.Name(), ".schema.json")
		if !typeNamePattern.MatchString(typ) {
			return nil, fmt.Errorf("%w: invalid type file name %q", ErrInvalid, entry.Name())
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalid, err)
		}
		if remoteRef(data) {
			return nil, fmt.Errorf("%w: %s uses a remote $ref", ErrUnsupported, entry.Name())
		}
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("%w: %s: %v", ErrInvalid, entry.Name(), err)
		}
		files = append(files, schemaFile{name: entry.Name(), typ: typ, path: path, doc: doc})
	}
	compiler := jsonschema.NewCompiler()
	for _, file := range files {
		if err := compiler.AddResource(fileURI(file.path), file.doc); err != nil {
			return nil, fmt.Errorf("%w: %s: %v", ErrInvalid, file.name, err)
		}
	}
	schemas := make(map[string]*jsonschema.Schema, len(files))
	for _, file := range files {
		schema, err := compiler.Compile(fileURI(file.path))
		if err != nil {
			return nil, fmt.Errorf("%w: %s: %v", ErrInvalid, file.name, err)
		}
		schemas[file.typ] = schema
	}
	return &Set{schemas: schemas}, nil
}

// Validate checks one concept against its type schema. Concepts without a schema pass.
func (s *Set) Validate(concept contractv1.OkfConcept) []Violation {
	if s == nil {
		return nil
	}
	schema, ok := s.schemas[concept.Type]
	if !ok {
		return nil
	}
	frontmatter := concept.Frontmatter
	if frontmatter == nil {
		frontmatter = map[string]any{}
	}
	if err := schema.Validate(frontmatter); err != nil {
		return []Violation{{Path: concept.Path, Message: err.Error()}}
	}
	return nil
}

func fileURI(path string) string {
	absolute, err := filepath.Abs(path)
	if err != nil {
		absolute = path
	}
	return (&url.URL{Scheme: "file", Path: filepath.ToSlash(absolute)}).String()
}

func remoteRef(data []byte) bool {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return false
	}
	return walkRefs(value)
}

func walkRefs(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if key == "$ref" {
				ref, _ := child.(string)
				if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") {
					return true
				}
			}
			if walkRefs(child) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if walkRefs(child) {
				return true
			}
		}
	}
	return false
}
