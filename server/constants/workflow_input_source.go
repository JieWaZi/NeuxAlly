package constants

//go:generate toolx gen enum WorkflowInputSource

// WorkflowInputSource 指定自动任务参数的唯一来源。
type WorkflowInputSource string

const (
	// WorkflowInputSourceUnspecified 表示尚未配置参数来源。
	WorkflowInputSourceUnspecified WorkflowInputSource = ""
	// WorkflowInputSourceFixed 使用任务保存的固定 JSON 值。
	WorkflowInputSourceFixed WorkflowInputSource = "fixed"
	// WorkflowInputSourceAPI 使用每次调用显式提供的值。
	WorkflowInputSourceAPI WorkflowInputSource = "api"
	// WorkflowInputSourceDefault 使用实际工作流版本的默认或省略行为。
	WorkflowInputSourceDefault WorkflowInputSource = "workflow_default"
)
