package constants

//go:generate toolx gen enum InvocationRecoveryStrategy

// InvocationRecoveryStrategy 表示 Host 启动或恢复 Runtime Invocation 的策略。
type InvocationRecoveryStrategy string

const (
	// InvocationRecoveryInitial 表示 Execution 的首次 Runtime 调用。
	InvocationRecoveryInitial InvocationRecoveryStrategy = "initial"
	// InvocationRecoveryTranscriptReseed 表示 Session 丢失后使用权威历史重新播种。
	InvocationRecoveryTranscriptReseed InvocationRecoveryStrategy = "transcript_reseed"
	// InvocationRecoverySafeRetry 表示零副作用瞬时失败后的 Host 安全重试。
	InvocationRecoverySafeRetry InvocationRecoveryStrategy = "safe_retry"
)
