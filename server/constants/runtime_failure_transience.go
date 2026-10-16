package constants

//go:generate toolx gen enum RuntimeFailureTransience

// RuntimeFailureTransience 表示失败是否属于 Host 可以考虑重试的瞬时错误。
type RuntimeFailureTransience string

const (
	// RuntimeFailureTransienceUnspecified 表示尚未判定失败瞬时性。
	RuntimeFailureTransienceUnspecified RuntimeFailureTransience = ""
	// RuntimeFailureTransienceUnknown 表示 SDK 无法可靠判断失败是否瞬时。
	RuntimeFailureTransienceUnknown RuntimeFailureTransience = "unknown"
	// RuntimeFailureTransiencePermanent 表示相同输入不应直接自动重试。
	RuntimeFailureTransiencePermanent RuntimeFailureTransience = "permanent"
	// RuntimeFailureTransienceTransient 表示失败可能由短暂上游状态导致。
	RuntimeFailureTransienceTransient RuntimeFailureTransience = "transient"
)
