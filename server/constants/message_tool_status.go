package constants

//go:generate toolx gen enum MessageToolStatus

// MessageToolStatus 表示消息投影中的工具执行状态。
type MessageToolStatus string

const (
	// MessageToolStatusRunning 表示工具调用尚未收到结果。
	MessageToolStatusRunning MessageToolStatus = "running"
	// MessageToolStatusCompleted 表示工具调用已经成功返回结果。
	MessageToolStatusCompleted MessageToolStatus = "completed"
	// MessageToolStatusFailed 表示工具调用返回失败结果。
	MessageToolStatusFailed MessageToolStatus = "failed"
)
