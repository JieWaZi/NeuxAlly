package wiki

import (
	"bytes"
	"text/template"
)

// StagePromptVersion 固定整理阶段提示词来源，用于任务 configDigest。
const StagePromptVersion = "weknora-8776a2d-lite-direct-v1"

// StagePrompt 保存固定上游的系统消息与用户消息，供直接模型端口接收。
type StagePrompt struct {
	System string // 系统侧稳定规则。
	User   string // 用户侧冻结输入投影。
}

func renderTemplate(text string, data any) (string, error) {
	t, err := template.New("prompt").Parse(text)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// RenderLitePrompt 原样渲染上游模板与指令作用域，不附加宿主工具协议。
func RenderLitePrompt(prompt string, data map[string]string) (StagePrompt, error) {
	user, err := renderTemplate(prompt, data)
	if err != nil {
		return StagePrompt{}, err
	}
	if prompt == WikiPageModifyUserPrompt {
		return StagePrompt{System: AppendCustomPromptInstructions(WikiPageModifySystemPrompt, data["CustomInstructions"], data["InstructionScope"]), User: user}, nil
	}
	return StagePrompt{User: AppendCustomPromptInstructions(user, data["CustomInstructions"], data["InstructionScope"])}, nil
}
