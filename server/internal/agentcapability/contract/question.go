package contract

// questionSchema 只接纳当前共享问答组件能实际填写的字符串与字符串多选表单。
func questionSchema() map[string]any {
	text := map[string]any{"type": "string"}
	stringsSchema := map[string]any{"type": "array", "items": map[string]any{"type": "string", "minLength": 1}, "uniqueItems": true}
	choiceStrings := map[string]any{"type": "array", "items": map[string]any{"type": "string", "minLength": 1}, "uniqueItems": true, "minItems": 1, "maxItems": 20}
	option := map[string]any{"type": "object", "properties": map[string]any{"const": map[string]any{"type": "string", "minLength": 1}, "title": text, "description": text}, "required": []string{"const"}, "additionalProperties": false}
	options := map[string]any{"type": "array", "items": option, "minItems": 1, "maxItems": 20}
	field := map[string]any{"type": "object", "properties": map[string]any{
		"type": map[string]any{"enum": []string{"string", "array"}}, "title": text, "description": text,
		"enum": choiceStrings, "oneOf": options, "minLength": map[string]any{"type": "integer", "minimum": 0}, "maxLength": map[string]any{"type": "integer", "minimum": 1},
		"items": map[string]any{"type": "object", "properties": map[string]any{"type": map[string]any{"const": "string"}, "enum": choiceStrings, "anyOf": options}, "additionalProperties": false},
	}, "required": []string{"type"}, "additionalProperties": false}
	return map[string]any{"type": "object", "properties": map[string]any{
		"type": map[string]any{"const": "object"}, "title": text, "description": text,
		"properties": map[string]any{"type": "object", "minProperties": 1, "maxProperties": 8, "propertyNames": map[string]any{"type": "string", "minLength": 1, "not": map[string]any{"pattern": "(_custom|__other)$"}}, "additionalProperties": field},
		"required":   stringsSchema, "additionalProperties": map[string]any{"const": false},
	}, "required": []string{"type", "properties"}, "additionalProperties": false}
}
