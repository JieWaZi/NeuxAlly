package constants

//go:generate toolx gen enum RuntimeDetectionState

// RuntimeDetectionState 表示 Runtime 当前是否可以启动。
type RuntimeDetectionState string

const (
	// RuntimeDetectionStateUnspecified 表示尚未执行 Runtime 探测。
	RuntimeDetectionStateUnspecified RuntimeDetectionState = ""
	// RuntimeDetectionStateUnavailable 表示未找到或不能调用 Runtime 可执行文件。
	RuntimeDetectionStateUnavailable RuntimeDetectionState = "unavailable"
	// RuntimeDetectionStateIncompatible 表示可执行文件存在但缺少必要协议能力。
	RuntimeDetectionStateIncompatible RuntimeDetectionState = "incompatible"
	// RuntimeDetectionStateReady 表示 Runtime 必要能力满足且可以准备调用。
	RuntimeDetectionStateReady RuntimeDetectionState = "ready"
)
