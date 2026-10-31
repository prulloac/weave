// Command schema-diff checks that contract schemas change additively.
//
// The committed baseline records every definition, property and entrypoint
// variant. A check fails when a definition, property or schema variant is
// removed, when an existing property changes shape or type, or when a new
// property is required. Additive optional properties and enum values pass.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
)

type baseline struct {
	Version int              `json:"version"`
	Defs    map[string]def   `json:"defs"`
	Entries map[string]entry `json:"entries"`
}

type def struct {
	Type                 any              `json:"type,omitempty"`
	Enum                 []string         `json:"enum,omitempty"`
	Required             []string         `json:"required,omitempty"`
	AdditionalProperties any              `json:"additional_properties,omitempty"`
	Properties           map[string]shape `json:"properties,omitempty"`
	Variants             []shape          `json:"variants,omitempty"`
}

type entry struct {
	Ref  string   `json:"ref,omitempty"`
	Refs []string `json:"refs,omitempty"`
}

type shape struct {
	Type    any      `json:"type,omitempty"`
	Ref     string   `json:"ref,omitempty"`
	Refs    []string `json:"refs,omitempty"`
	Enum    []string `json:"enum,omitempty"`
	Items   *shape   `json:"items,omitempty"`
	Format  string   `json:"format,omitempty"`
	Const   any      `json:"const,omitempty"`
	Minimum any      `json:"minimum,omitempty"`
	Maximum any      `json:"maximum,omitempty"`
	Pattern string   `json:"pattern,omitempty"`
}

func main() {
	dir := flag.String("dir", "contract/v1/schema", "schema directory")
	baselinePath := flag.String("baseline", "contract/v1/golden/schema-baseline.json", "baseline file")
	write := flag.Bool("write", false, "write the baseline from the current schemas")
	flag.Parse()

	current, err := snapshot(*dir)
	if err != nil {
		fatal(err)
	}
	if *write {
		if err := writeBaseline(*baselinePath, current); err != nil {
			fatal(err)
		}
		fmt.Printf("schema-diff: wrote %s\n", *baselinePath)
		return
	}

	recorded, err := readBaseline(*baselinePath)
	if err != nil {
		fatal(err)
	}
	problems := diff(recorded, current)
	if len(problems) > 0 {
		for _, problem := range problems {
			fmt.Fprintln(os.Stderr, "schema-diff:", problem)
		}
		os.Exit(1)
	}
	fmt.Printf("schema-diff: %d definitions and %d entrypoints are additive\n",
		len(current.Defs), len(current.Entries))
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "schema-diff:", err)
	os.Exit(1)
}

func readBaseline(path string) (*baseline, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var value baseline
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, err
	}
	if value.Version != 1 {
		return nil, fmt.Errorf("%s: unsupported baseline version %d", path, value.Version)
	}
	return &value, nil
}

func writeBaseline(path string, value *baseline) error {
	data, err := json.MarshalIndent(value, "", "\t")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func snapshot(dir string) (*baseline, error) {
	root, err := readSchema(filepath.Join(dir, "defs.schema.json"))
	if err != nil {
		return nil, err
	}
	rawDefs, ok := root["$defs"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s: missing $defs", filepath.Join(dir, "defs.schema.json"))
	}

	value := &baseline{Version: 1, Defs: map[string]def{}, Entries: map[string]entry{}}
	for name, raw := range rawDefs {
		value.Defs[name] = snapshotDef(raw)
	}

	files, err := filepath.Glob(filepath.Join(dir, "*.schema.json"))
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		name := filepath.Base(file)
		if name == "defs.schema.json" {
			continue
		}
		root, err := readSchema(file)
		if err != nil {
			return nil, err
		}
		item := entry{}
		if ref, ok := root["$ref"].(string); ok {
			item.Ref = ref
		}
		if variants, ok := root["oneOf"].([]any); ok {
			item.Refs = refsOf(variants)
		}
		value.Entries[name] = item
	}
	return value, nil
}

func readSchema(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return root, nil
}

func snapshotDef(raw any) def {
	object, _ := raw.(map[string]any)
	value := def{
		Type:                 object["type"],
		AdditionalProperties: object["additionalProperties"],
		Properties:           map[string]shape{},
	}
	value.Enum = stringsOf(object["enum"])
	value.Required = stringsOf(object["required"])
	sort.Strings(value.Required)
	if properties, ok := object["properties"].(map[string]any); ok {
		for name, property := range properties {
			value.Properties[name] = shapeOf(property)
		}
	}
	for _, variant := range mapsOf(object["oneOf"], object["anyOf"], object["allOf"]) {
		value.Variants = append(value.Variants, shapeOf(variant))
	}
	return value
}

func shapeOf(raw any) shape {
	object, ok := raw.(map[string]any)
	if !ok {
		return shape{}
	}
	value := shape{
		Type:    object["type"],
		Format:  stringOf(object["format"]),
		Const:   object["const"],
		Minimum: object["minimum"],
		Maximum: object["maximum"],
		Pattern: stringOf(object["pattern"]),
		Enum:    stringsOf(object["enum"]),
	}
	value.Ref = stringOf(object["$ref"])
	value.Refs = refsOf(mapsOf(object["oneOf"], object["anyOf"], object["allOf"]))
	if items, ok := object["items"]; ok {
		nested := shapeOf(items)
		value.Items = &nested
	}
	return value
}

func mapsOf(values ...any) []any {
	result := make([]any, 0)
	for _, value := range values {
		if items, ok := value.([]any); ok {
			result = append(result, items...)
		}
	}
	return result
}

func refsOf(values []any) []string {
	refs := make([]string, 0, len(values))
	for _, value := range values {
		object, ok := value.(map[string]any)
		if !ok {
			continue
		}
		if ref, ok := object["$ref"].(string); ok {
			refs = append(refs, ref)
		}
	}
	if len(refs) == 0 {
		return nil
	}
	sort.Strings(refs)
	return refs
}

func stringsOf(raw any) []string {
	values, ok := raw.([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if text, ok := value.(string); ok {
			result = append(result, text)
		}
	}
	if len(result) == 0 {
		return nil
	}
	sort.Strings(result)
	return result
}

func stringOf(raw any) string {
	text, _ := raw.(string)
	return text
}

func diff(recorded, current *baseline) []string {
	problems := make([]string, 0)
	for name, old := range recorded.Defs {
		next, ok := current.Defs[name]
		if !ok {
			problems = append(problems, fmt.Sprintf("definition %q removed", name))
			continue
		}
		problems = append(problems, diffDef(name, old, next)...)
	}
	for name, old := range recorded.Entries {
		next, ok := current.Entries[name]
		if !ok {
			problems = append(problems, fmt.Sprintf("schema %q removed", name))
			continue
		}
		if old.Ref != next.Ref {
			problems = append(problems, fmt.Sprintf("%s: entrypoint ref changed from %q to %q", name, old.Ref, next.Ref))
		}
		for _, ref := range old.Refs {
			if !contains(next.Refs, ref) {
				problems = append(problems, fmt.Sprintf("%s: variant %q removed", name, ref))
			}
		}
	}
	return problems
}

func diffDef(name string, old, next def) []string {
	path := "defs." + name
	problems := make([]string, 0)
	if !reflect.DeepEqual(old.Type, next.Type) {
		problems = append(problems, fmt.Sprintf("%s: type changed from %v to %v", path, old.Type, next.Type))
	}
	for _, value := range old.Enum {
		if !contains(next.Enum, value) {
			problems = append(problems, fmt.Sprintf("%s: enum value %q removed", path, value))
		}
	}
	if old.AdditionalProperties != false && next.AdditionalProperties == false {
		problems = append(problems, fmt.Sprintf("%s: additionalProperties became stricter", path))
	}
	for _, required := range next.Required {
		if !contains(old.Required, required) {
			problems = append(problems, fmt.Sprintf("%s: new required property %q", path, required))
		}
	}
	for property, oldShape := range old.Properties {
		nextShape, ok := next.Properties[property]
		if !ok {
			problems = append(problems, fmt.Sprintf("%s.%s: property removed", path, property))
			continue
		}
		problems = append(problems, diffShape(path+"."+property, oldShape, nextShape)...)
	}
	if old.Variants == nil && next.Variants != nil {
		problems = append(problems, fmt.Sprintf("%s: union introduced", path))
	}
	for _, oldVariant := range old.Variants {
		matched := false
		for _, nextVariant := range next.Variants {
			if len(diffShape(path+"[]", oldVariant, nextVariant)) == 0 {
				matched = true
				break
			}
		}
		if !matched {
			problems = append(problems, fmt.Sprintf("%s: variant removed or changed", path))
		}
	}
	return problems
}

func diffShape(path string, old, next shape) []string {
	problems := make([]string, 0)
	oldCopy, nextCopy := old, next
	oldCopy.Enum, nextCopy.Enum = nil, nil
	oldCopy.Refs, nextCopy.Refs = nil, nil
	oldCopy.Items, nextCopy.Items = nil, nil
	if !reflect.DeepEqual(oldCopy, nextCopy) {
		problems = append(problems, fmt.Sprintf("%s: shape changed", path))
	}
	if old.Enum == nil && next.Enum != nil {
		problems = append(problems, fmt.Sprintf("%s: enum introduced", path))
	}
	for _, value := range old.Enum {
		if !contains(next.Enum, value) {
			problems = append(problems, fmt.Sprintf("%s: enum value %q removed", path, value))
		}
	}
	if old.Refs == nil && next.Refs != nil {
		problems = append(problems, fmt.Sprintf("%s: union introduced", path))
	}
	for _, ref := range old.Refs {
		if !contains(next.Refs, ref) {
			problems = append(problems, fmt.Sprintf("%s: variant %q removed", path, ref))
		}
	}
	if old.Items != nil {
		if next.Items == nil {
			problems = append(problems, fmt.Sprintf("%s: array items removed", path))
		} else {
			problems = append(problems, diffShape(path+"[]", *old.Items, *next.Items)...)
		}
	} else if next.Items != nil {
		problems = append(problems, fmt.Sprintf("%s: array items added", path))
	}
	return problems
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
