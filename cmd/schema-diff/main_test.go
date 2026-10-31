package main

import (
	"path/filepath"
	"testing"
)

func TestBaselineMatchesSchemas(t *testing.T) {
	current, err := snapshot(filepath.Join("..", "..", "contract", "v1", "schema"))
	if err != nil {
		t.Fatal(err)
	}
	recorded, err := readBaseline(filepath.Join("..", "..", "contract", "v1", "golden", "schema-baseline.json"))
	if err != nil {
		t.Fatal(err)
	}
	problems := diff(recorded, current)
	if len(problems) > 0 {
		t.Fatalf("schema baseline is stale:\n%v", problems)
	}
}

func TestDiffAllowsOptionalAddition(t *testing.T) {
	recorded := baselineOf("Thing", def{
		Type: "object", Required: []string{"a"}, AdditionalProperties: false,
		Properties: map[string]shape{"a": {Type: "string"}},
	})
	current := baselineOf("Thing", def{
		Type: "object", Required: []string{"a"}, AdditionalProperties: false,
		Properties: map[string]shape{"a": {Type: "string"}, "b": {Type: "boolean"}},
	})
	if problems := diff(recorded, current); len(problems) != 0 {
		t.Fatalf("optional addition failed: %v", problems)
	}
}

func TestDiffRejectsRemovedProperty(t *testing.T) {
	recorded := baselineOf("Thing", def{
		Type: "object", AdditionalProperties: false,
		Properties: map[string]shape{"a": {Type: "string"}},
	})
	current := baselineOf("Thing", def{
		Type: "object", AdditionalProperties: false, Properties: map[string]shape{},
	})
	problems := diff(recorded, current)
	if len(problems) != 1 {
		t.Fatalf("expected one problem, got %v", problems)
	}
}

func TestDiffRejectsRequiredAddition(t *testing.T) {
	recorded := baselineOf("Thing", def{
		Type: "object", Required: []string{"a"}, AdditionalProperties: false,
		Properties: map[string]shape{"a": {Type: "string"}},
	})
	current := baselineOf("Thing", def{
		Type: "object", Required: []string{"a", "b"}, AdditionalProperties: false,
		Properties: map[string]shape{"a": {Type: "string"}, "b": {Type: "boolean"}},
	})
	problems := diff(recorded, current)
	if len(problems) != 1 {
		t.Fatalf("expected one problem, got %v", problems)
	}
}

func TestDiffRejectsTypeChange(t *testing.T) {
	recorded := baselineOf("Thing", def{
		Type: "object", AdditionalProperties: false,
		Properties: map[string]shape{"a": {Type: "string"}},
	})
	current := baselineOf("Thing", def{
		Type: "object", AdditionalProperties: false,
		Properties: map[string]shape{"a": {Type: "integer"}},
	})
	problems := diff(recorded, current)
	if len(problems) != 1 {
		t.Fatalf("expected one problem, got %v", problems)
	}
}

func TestDiffRejectsStricterAdditionalProperties(t *testing.T) {
	recorded := baselineOf("Thing", def{Type: "object"})
	current := baselineOf("Thing", def{Type: "object", AdditionalProperties: false})
	problems := diff(recorded, current)
	if len(problems) != 1 {
		t.Fatalf("expected one problem, got %v", problems)
	}

	recorded = baselineOf("Thing", def{Type: "object", AdditionalProperties: false})
	current = baselineOf("Thing", def{Type: "object"})
	if problems := diff(recorded, current); len(problems) != 0 {
		t.Fatalf("lenient change failed: %v", problems)
	}
}

func TestDiffAllowsEnumAddition(t *testing.T) {
	recorded := baselineOf("Thing", def{
		Type: "object", AdditionalProperties: false,
		Properties: map[string]shape{"mode": {Type: "string", Enum: []string{"a"}}},
	})
	current := baselineOf("Thing", def{
		Type: "object", AdditionalProperties: false,
		Properties: map[string]shape{"mode": {Type: "string", Enum: []string{"a", "b"}}},
	})
	if problems := diff(recorded, current); len(problems) != 0 {
		t.Fatalf("enum addition failed: %v", problems)
	}
}

func TestDiffRejectsEnumRemoval(t *testing.T) {
	recorded := baselineOf("Thing", def{
		Type: "object", AdditionalProperties: false,
		Properties: map[string]shape{"mode": {Type: "string", Enum: []string{"a", "b"}}},
	})
	current := baselineOf("Thing", def{
		Type: "object", AdditionalProperties: false,
		Properties: map[string]shape{"mode": {Type: "string", Enum: []string{"a"}}},
	})
	problems := diff(recorded, current)
	if len(problems) != 1 {
		t.Fatalf("expected one problem, got %v", problems)
	}
}

func TestDiffRejectsEnumIntroduction(t *testing.T) {
	recorded := baselineOf("Thing", def{
		Type: "object", AdditionalProperties: false,
		Properties: map[string]shape{"mode": {Type: "string"}},
	})
	current := baselineOf("Thing", def{
		Type: "object", AdditionalProperties: false,
		Properties: map[string]shape{"mode": {Type: "string", Enum: []string{"a"}}},
	})
	problems := diff(recorded, current)
	if len(problems) != 1 {
		t.Fatalf("expected one problem, got %v", problems)
	}
}

func TestDiffAllowsDefVariantAddition(t *testing.T) {
	recorded := baselineOf("Thing", def{
		Type: "object", Variants: []shape{{Type: "string"}},
	})
	current := baselineOf("Thing", def{
		Type: "object", Variants: []shape{{Type: "string"}, {Type: "integer"}},
	})
	if problems := diff(recorded, current); len(problems) != 0 {
		t.Fatalf("variant addition failed: %v", problems)
	}
}

func TestDiffRejectsDefVariantRemoval(t *testing.T) {
	recorded := baselineOf("Thing", def{
		Type: "object", Variants: []shape{{Type: "string"}, {Type: "integer"}},
	})
	current := baselineOf("Thing", def{
		Type: "object", Variants: []shape{{Type: "string"}},
	})
	problems := diff(recorded, current)
	if len(problems) != 1 {
		t.Fatalf("expected one problem, got %v", problems)
	}
}

func TestDiffRejectsItemsAddition(t *testing.T) {
	recorded := baselineOf("Thing", def{
		Type: "object", AdditionalProperties: false,
		Properties: map[string]shape{"values": {Type: "array"}},
	})
	current := baselineOf("Thing", def{
		Type: "object", AdditionalProperties: false,
		Properties: map[string]shape{"values": {Type: "array", Items: &shape{Type: "string"}}},
	})
	problems := diff(recorded, current)
	if len(problems) != 1 {
		t.Fatalf("expected one problem, got %v", problems)
	}
}

func TestDiffRejectsRemovedVariant(t *testing.T) {
	recorded := &baseline{
		Version: 1,
		Defs:    map[string]def{},
		Entries: map[string]entry{
			"thing.schema.json": {Refs: []string{"#/$defs/A", "#/$defs/B"}},
		},
	}
	current := &baseline{
		Version: 1,
		Defs:    map[string]def{},
		Entries: map[string]entry{
			"thing.schema.json": {Refs: []string{"#/$defs/A"}},
		},
	}
	problems := diff(recorded, current)
	if len(problems) != 1 {
		t.Fatalf("expected one problem, got %v", problems)
	}
}

func baselineOf(name string, value def) *baseline {
	return &baseline{
		Version: 1,
		Defs:    map[string]def{name: value},
		Entries: map[string]entry{},
	}
}
