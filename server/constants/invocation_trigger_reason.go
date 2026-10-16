package constants

//go:generate toolx gen enum InvocationTriggerReason

// InvocationTriggerReason 表示触发 Invocation 重建或安全重试的原因。
type InvocationTriggerReason string

const (
	// InvocationTriggerUnspecified 表示首次调用没有恢复触发原因。
	InvocationTriggerUnspecified InvocationTriggerReason = ""
	// InvocationTriggerSessionMissing 表示重建由原生 Session 目标丢失触发。
	InvocationTriggerSessionMissing InvocationTriggerReason = "session_missing"
)
