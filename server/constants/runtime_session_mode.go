package constants

//go:generate toolx gen enum RuntimeSessionMode

// RuntimeSessionMode 表示调用方要求的原生 Session 动作。
type RuntimeSessionMode string

const (
	// RuntimeSessionModeNone 表示本次调用不使用原生 Session。
	RuntimeSessionModeNone RuntimeSessionMode = ""
	// RuntimeSessionModeNew 表示创建一个新的原生 Session。
	RuntimeSessionModeNew RuntimeSessionMode = "new"
	// RuntimeSessionModeResume 表示恢复调用方提供的原生 Session。
	RuntimeSessionModeResume RuntimeSessionMode = "resume"
)
