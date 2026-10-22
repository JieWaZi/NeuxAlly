// 来源：Tencent/WeKnora8776 config/prompt_templates/generate_summary.yaml，原完整模板嵌入不改正文。
package wiki

import (
	_ "embed"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed prompts/generate_summary.yaml
var documentProfileTemplates string

// RenderDocumentSummaryPrompt 保持上游普通摘要的系统模板和用户正文分离。
func RenderDocumentSummaryPrompt(language, content string) (StagePrompt, error) {
	var cfg struct {
		Templates []struct {
			Content string `yaml:"content"` /* 上游原系统提示。 */
		} `yaml:"templates"` /* 上游模板列表。 */
	}
	if err := yaml.Unmarshal([]byte(documentProfileTemplates), &cfg); err != nil {
		return StagePrompt{}, err
	}
	return StagePrompt{System: strings.ReplaceAll(cfg.Templates[0].Content, "{{language}}", language), User: content}, nil
}
