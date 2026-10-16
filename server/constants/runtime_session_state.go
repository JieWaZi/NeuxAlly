package constants

//go:generate toolx gen enum RuntimeSessionState

// RuntimeSessionState 表示 Invocation 结束时的原生 Session 状态。
type RuntimeSessionState string

const (
	// RuntimeSessionStateUnspecified 表示尚未形成 Runtime Session 结论。
	RuntimeSessionStateUnspecified RuntimeSessionState = ""
	// RuntimeSessionStateNone 表示本次调用没有使用原生 Session。
	RuntimeSessionStateNone RuntimeSessionState = "none"
	// RuntimeSessionStateCreated 表示 Runtime 已经创建或确认新 Session。
	RuntimeSessionStateCreated RuntimeSessionState = "created"
	// RuntimeSessionStateResumed 表示 Runtime 已经成功恢复输入 Session。
	RuntimeSessionStateResumed RuntimeSessionState = "resumed"
	// RuntimeSessionStateMissing 表示输入 Session 已不存在或不可恢复。
	RuntimeSessionStateMissing RuntimeSessionState = "missing"
	// RuntimeSessionStateUnknown 表示 SDK 无法确认 Session 最终状态。
	RuntimeSessionStateUnknown RuntimeSessionState = "unknown"
)
