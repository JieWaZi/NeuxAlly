package accountconfig

import (
	_ "embed"
	"encoding/json"
	"net/url"
	"slices"
	"strings"
)

// deepSeekModelCatalog 是 DeepSeek Codex Responses 模型目录。
//
//go:embed deepseek_models.json
var deepSeekModelCatalog []byte

func isOfficialDeepSeekEndpoint(baseURL string) bool {
	parsed, err := url.Parse(strings.TrimRight(strings.TrimSpace(baseURL), "/"))
	if err != nil || parsed.Scheme != "https" || parsed.Host != "api.deepseek.com" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return false
	}
	switch strings.TrimRight(parsed.Path, "/") {
	case "", "/v1", "/anthropic":
		return true
	default:
		return false
	}
}

func isDeepSeekCurrentModel(model string) bool {
	return model == "deepseek-flash" || model == "deepseek-v4-pro"
}

// codexCatalog 将账号已选模型映射为 Codex 原生目录；只有官方 DeepSeek 入口启用其专用能力。
func codexCatalog(spec Spec) ([]byte, error) {
	var officialEntries map[string][]map[string]any
	if isOfficialDeepSeekEndpoint(spec.BaseURL) {
		if err := json.Unmarshal(deepSeekModelCatalog, &officialEntries); err != nil {
			return nil, err
		}
	}
	selected := slices.Clone(spec.SelectedModels)
	if spec.Model != "" && !slices.Contains(selected, spec.Model) {
		selected = append(selected, spec.Model)
	}
	models := make([]map[string]any, 0, len(selected))
	for index, model := range selected {
		if model == "" {
			continue
		}
		name := spec.ModelDisplayNames[model]
		var official map[string]any
		for _, candidate := range officialEntries["models"] {
			if candidate["slug"] == model {
				official = candidate
				break
			}
		}
		if name == "" && official != nil {
			name, _ = official["display_name"].(string)
		}
		if name == "" {
			name = model
		}
		context := spec.ModelContextSizes[model]
		if context < 1 {
			context = 262144
		}
		levels := spec.ModelReasoningLevels[model]
		if len(levels) == 0 {
			levels = []string{"low", "medium", "high"}
		}
		defaultLevel := levels[0]
		if slices.Contains(levels, "high") {
			defaultLevel = "high"
		}
		reasoning := make([]map[string]string, 0, len(levels))
		for _, level := range levels {
			reasoning = append(reasoning, map[string]string{"effort": level, "description": level})
		}
		entry := map[string]any{
			"slug": model, "display_name": name, "description": name,
			"base_instructions": "", "visibility": "list", "priority": index + 1,
			"supported_in_api": true, "context_window": context, "max_context_window": context,
			"effective_context_window_percent": 95, "input_modalities": []string{"text", "image"},
			"supports_reasoning_summaries": false, "supports_parallel_tool_calls": false,
			"support_verbosity": false, "shell_type": "shell_command",
			"supports_image_detail_original": false, "supports_search_tool": false,
			"experimental_supported_tools": []string{},
			"default_reasoning_level":      defaultLevel, "supported_reasoning_levels": reasoning,
			"default_reasoning_summary": "none",
			"truncation_policy":         map[string]any{"mode": "bytes", "limit": 10000},
		}
		if official != nil {
			entry = official
			entry["display_name"] = name
			entry["context_window"] = context
			entry["max_context_window"] = context
			entry["priority"] = index + 1
			if declared := spec.ModelReasoningLevels[model]; len(declared) > 0 {
				entry["supported_reasoning_levels"] = reasoning
				preferred := declared[len(declared)-1]
				if slices.Contains(declared, "high") {
					preferred = "high"
				}
				entry["default_reasoning_level"] = preferred
			}
		}
		models = append(models, entry)
	}
	return json.Marshal(map[string]any{"models": models})
}

// kimiReasoningLevels 先用账号声明，再使用官方 DeepSeek 已确认的模型能力。
func kimiReasoningLevels(spec Spec) []string {
	configured := spec.ModelReasoningLevels[spec.Model]
	levels := make([]string, 0, len(configured))
	for _, level := range configured {
		if level == "low" || level == "high" || level == "max" {
			levels = append(levels, level)
		}
	}
	if len(levels) == 0 && isOfficialDeepSeekEndpoint(spec.BaseURL) && isDeepSeekCurrentModel(spec.Model) {
		return []string{"low", "high", "max"}
	}
	return levels
}
