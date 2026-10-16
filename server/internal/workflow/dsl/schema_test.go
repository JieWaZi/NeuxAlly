package dsl

import (
	"errors"
	"testing"

	"github.com/shrewx/ginx/pkg/statuserror"
)

func TestSchemaDefaultsRespectMissingNullReferencesAndUniqueBranch(t *testing.T) {
	schema := map[string]any{
		"type": "object", "$defs": map[string]any{"count": map[string]any{"type": "integer", "minimum": 1, "default": 2}},
		"properties": map[string]any{
			"count":    map[string]any{"$ref": "#/$defs/count"},
			"nullable": map[string]any{"type": []any{"string", "null"}, "default": "fallback"},
			"optional": map[string]any{"type": "object", "properties": map[string]any{"child": map[string]any{"default": "must not create parent"}}},
			"selection": map[string]any{"oneOf": []any{
				map[string]any{"type": "object", "required": []any{"kind", "value"}, "properties": map[string]any{"kind": map[string]any{"const": "a"}, "value": map[string]any{"type": "string", "default": "alpha"}}},
				map[string]any{"type": "object", "required": []any{"kind", "value"}, "properties": map[string]any{"kind": map[string]any{"const": "b"}, "value": map[string]any{"type": "string", "default": "beta"}}},
			}},
		},
	}
	input := map[string]any{"nullable": nil, "selection": map[string]any{"kind": "b"}}
	result := ApplyDefaults(schema, input).(map[string]any)
	if result["count"] != float64(2) || result["nullable"] != nil || result["selection"].(map[string]any)["value"] != "beta" {
		t.Fatalf("defaults failed: %+v", result)
	}
	if _, added := result["optional"]; added {
		t.Fatal("a missing parent object was invented")
	}
	if _, changed := input["count"]; changed {
		t.Fatal("input was mutated")
	}
	if err := ValidateJSON(schema, result); err != nil {
		t.Fatalf("defaulted arguments invalid: %v", err)
	}
	ambiguous := map[string]any{"oneOf": []any{map[string]any{"properties": map[string]any{"a": map[string]any{"default": 1}}}, map[string]any{"properties": map[string]any{"b": map[string]any{"default": 2}}}}}
	if value := ApplyDefaults(ambiguous, map[string]any{}).(map[string]any); len(value) != 0 {
		t.Fatalf("ambiguous branch selected implicitly: %+v", value)
	}
}

func TestSchemaValidationLocatesActualInstancePointer(t *testing.T) {
	schema := map[string]any{"type": "object", "$defs": map[string]any{"item": map[string]any{"type": "object", "required": []string{"a/b~c"}, "properties": map[string]any{"a/b~c": map[string]any{"type": "integer", "minimum": 1}}}}, "properties": map[string]any{"items": map[string]any{"type": "array", "items": map[string]any{"$ref": "#/$defs/item"}}}}
	for _, input := range []any{map[string]any{"items": []any{map[string]any{"a/b~c": 2}, map[string]any{"a/b~c": 0}}}, map[string]any{"items": []any{map[string]any{"a/b~c": 2}, map[string]any{}}}} {
		err := ValidateJSON(schema, input)
		var detail *statuserror.StatusErr
		if !errors.As(err, &detail) || detail.Fields["jsonPointer"] != "/items/1/a~1b~0c" || detail.Fields["reason"] == "" {
			t.Fatalf("instance pointer lost: %v %+v", err, detail)
		}
	}
}
