package constants

//go:generate toolx gen enum RolePresetInputMode

// RolePresetInputMode 表示角色预设档案通过结构化表单还是完整 Markdown 写入。
type RolePresetInputMode string

const (
	// RolePresetInputModeManual 表示使用结构化字段 Patch 三份档案。
	RolePresetInputModeManual RolePresetInputMode = "manual"
	// RolePresetInputModeMarkdown 表示直接提交三份完整 Markdown。
	RolePresetInputModeMarkdown RolePresetInputMode = "markdown"
)
