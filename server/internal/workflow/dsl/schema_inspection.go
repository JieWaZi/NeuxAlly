package dsl

import (
	"sort"
	"strconv"
	"strings"

	"github.com/dop251/goja/ast"
	"github.com/google/jsonschema-go/jsonschema"
)

// schemaInspection 用同一验证库定位实例字段及选择唯一合法默认分支。
type schemaInspection struct {
	// root 是原始局部引用的唯一根文档。
	root map[string]any
	// wrapped 是引用重定向后的只读文档，避免子 schema 丢失原始引用范围。
	wrapped map[string]any
	// resolved 缓存有限数量的子 schema 验证器。
	resolved map[string]*jsonschema.Resolved
}

func newSchemaInspection(root map[string]any) *schemaInspection {
	if normalized, ok := cloneJSON(root).(map[string]any); ok {
		root = normalized
	}
	return &schemaInspection{root: root, wrapped: rewriteSchemaReferences(root), resolved: map[string]*jsonschema.Resolved{}}
}

func pointerKey(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(value, "~", "~0"), "/", "~1")
}

func (s *schemaInspection) schema(path string) map[string]any {
	var value any = s.root
	if path == "" {
		return s.root
	}
	for _, part := range strings.Split(strings.TrimPrefix(path, "/"), "/") {
		part = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
		switch current := value.(type) {
		case map[string]any:
			value = current[part]
		case []any:
			index, err := strconv.Atoi(part)
			if err != nil || index < 0 || index >= len(current) {
				return nil
			}
			value = current[index]
		default:
			return nil
		}
	}
	result, _ := value.(map[string]any)
	return result
}

func (s *schemaInspection) valid(path string, value any) bool {
	resolved := s.resolved[path]
	if resolved == nil {
		if len(s.resolved) >= 128 {
			return false
		}
		wrapper := map[string]any{"$schema": "https://json-schema.org/draft/2020-12/schema", "$ref": "#/$defs/workflowRoot" + path, "$defs": map[string]any{"workflowRoot": s.wrapped}}
		var err error
		resolved, err = ResolveSchema(wrapper)
		if err != nil || resolved == nil {
			return false
		}
		s.resolved[path] = resolved
	}
	return resolved.Validate(value) == nil
}

func (s *schemaInspection) failurePointer(path, instance string, value any, depth int) string {
	if depth > 64 {
		return instance
	}
	schema := s.schema(path)
	if schema == nil {
		return instance
	}
	if ref, ok := schema["$ref"].(string); ok && strings.HasPrefix(ref, "#/") && !s.valid(ref[1:], value) {
		return s.failurePointer(ref[1:], instance, value, depth+1)
	}
	if object, ok := value.(map[string]any); ok {
		if required, ok := schema["required"].([]any); ok {
			for _, entry := range required {
				if key, ok := entry.(string); ok {
					if _, present := object[key]; !present {
						return instance + "/" + pointerKey(key)
					}
				}
			}
		}
		properties, _ := schema["properties"].(map[string]any)
		keys := make([]string, 0, len(object))
		for key := range object {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			childPath := path + "/properties/" + pointerKey(key)
			if _, declared := properties[key]; declared {
				if !s.valid(childPath, object[key]) {
					return s.failurePointer(childPath, instance+"/"+pointerKey(key), object[key], depth+1)
				}
			} else if schema["additionalProperties"] == false {
				return instance + "/" + pointerKey(key)
			} else if _, ok := schema["additionalProperties"].(map[string]any); ok && !s.valid(path+"/additionalProperties", object[key]) {
				return s.failurePointer(path+"/additionalProperties", instance+"/"+pointerKey(key), object[key], depth+1)
			}
		}
	}
	if array, ok := value.([]any); ok {
		prefix, _ := schema["prefixItems"].([]any)
		for index, item := range array {
			childPath := path + "/items"
			if index < len(prefix) {
				childPath = path + "/prefixItems/" + strconv.Itoa(index)
			}
			if s.schema(childPath) != nil && !s.valid(childPath, item) {
				return s.failurePointer(childPath, instance+"/"+strconv.Itoa(index), item, depth+1)
			}
		}
	}
	for _, keyword := range []string{"allOf", "oneOf", "anyOf"} {
		branches, _ := schema[keyword].([]any)
		common, failures := "", 0
		for index := range branches {
			childPath := path + "/" + keyword + "/" + strconv.Itoa(index)
			if s.valid(childPath, value) {
				if keyword != "allOf" {
					failures = -1
					break
				}
				continue
			}
			pointer := s.failurePointer(childPath, instance, value, depth+1)
			if keyword == "allOf" {
				return pointer
			}
			if failures > 0 && common != pointer {
				common = instance
			} else if failures == 0 {
				common = pointer
			}
			failures++
		}
		if failures > 0 {
			return common
		}
	}
	return instance
}

func (s *schemaInspection) defaultValue(path string, depth int) (any, bool) {
	if depth > 64 {
		return nil, false
	}
	schema := s.schema(path)
	if value, present := schema["default"]; present {
		return cloneJSON(value), true
	}
	if ref, ok := schema["$ref"].(string); ok && strings.HasPrefix(ref, "#/") {
		return s.defaultValue(ref[1:], depth+1)
	}
	return nil, false
}

func (s *schemaInspection) defaults(path string, value any, depth int) any {
	if depth > 64 || value == nil {
		return value
	}
	schema := s.schema(path)
	if ref, ok := schema["$ref"].(string); ok && strings.HasPrefix(ref, "#/") {
		value = s.defaults(ref[1:], value, depth+1)
	}
	if object, ok := value.(map[string]any); ok {
		properties, _ := schema["properties"].(map[string]any)
		for key := range properties {
			child := path + "/properties/" + pointerKey(key)
			current, present := object[key]
			if !present {
				current, present = s.defaultValue(child, 0)
			}
			if present {
				object[key] = s.defaults(child, current, depth+1)
			}
		}
	}
	if array, ok := value.([]any); ok {
		prefix, _ := schema["prefixItems"].([]any)
		for index, item := range array {
			child := path + "/items"
			if index < len(prefix) {
				child = path + "/prefixItems/" + strconv.Itoa(index)
			}
			if s.schema(child) != nil {
				array[index] = s.defaults(child, item, depth+1)
			}
		}
	}
	for _, keyword := range []string{"allOf", "oneOf", "anyOf"} {
		branches, _ := schema[keyword].([]any)
		matches := []any{}
		for index := range branches {
			child := path + "/" + keyword + "/" + strconv.Itoa(index)
			if keyword == "allOf" {
				value = s.defaults(child, value, depth+1)
				continue
			}
			candidate := s.defaults(child, cloneJSON(value), depth+1)
			if s.valid(child, candidate) {
				matches = append(matches, candidate)
			}
		}
		if len(matches) == 1 {
			value = matches[0]
		}
	}
	return value
}

func rewriteSchemaReferences(schema map[string]any) map[string]any {
	result := make(map[string]any, len(schema))
	for key, value := range schema {
		switch key {
		case "$ref":
			ref, _ := value.(string)
			if strings.HasPrefix(ref, "#/") {
				value = "#/$defs/workflowRoot" + ref[1:]
			}
		case "properties", "$defs":
			fields, _ := value.(map[string]any)
			children := map[string]any{}
			for name, item := range fields {
				child, _ := item.(map[string]any)
				children[name] = rewriteSchemaReferences(child)
			}
			value = children
		case "items", "not", "additionalProperties":
			if child, ok := value.(map[string]any); ok {
				value = rewriteSchemaReferences(child)
			}
		case "allOf", "oneOf", "anyOf", "prefixItems":
			items, _ := value.([]any)
			children := make([]any, len(items))
			for index, item := range items {
				child, _ := item.(map[string]any)
				children[index] = rewriteSchemaReferences(child)
			}
			value = children
		}
		result[key] = value
	}
	return result
}

func pointerPart(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(value, "~", "~0"), "/", "~1")
}

func inputPropertyOrder(meta ast.Expression) map[string][]string {
	result := map[string][]string{}
	object, ok := meta.(*ast.ObjectLiteral)
	if !ok {
		return result
	}
	var walk func(ast.Expression, string)
	walk = func(expression ast.Expression, path string) {
		switch node := expression.(type) {
		case *ast.ObjectLiteral:
			for _, property := range node.Value {
				entry, ok := property.(*ast.PropertyKeyed)
				if !ok {
					continue
				}
				key, ok := propertyName(entry.Key)
				if !ok {
					continue
				}
				if key == "properties" {
					properties, ok := entry.Value.(*ast.ObjectLiteral)
					if ok {
						keys := []string{}
						for _, property := range properties.Value {
							entry, ok := property.(*ast.PropertyKeyed)
							if !ok {
								continue
							}
							key, ok := propertyName(entry.Key)
							if ok {
								keys = append(keys, key)
							}
						}
						result[path] = keys
					}
				}
				walk(entry.Value, path+"/"+pointerPart(key))
			}
		case *ast.ArrayLiteral:
			for index, item := range node.Value {
				walk(item, path+"/"+strconv.Itoa(index))
			}
		}
	}
	for _, property := range object.Value {
		entry, ok := property.(*ast.PropertyKeyed)
		if !ok {
			continue
		}
		key, _ := propertyName(entry.Key)
		if key == "inputSchema" {
			walk(entry.Value, "")
		}
	}
	return result
}
