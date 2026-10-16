package constants

//go:generate toolx gen enum RuntimeAuthStatus

// RuntimeAuthStatus 表示 Runtime 无副作用鉴权探测的结论。
type RuntimeAuthStatus string

const (
	// RuntimeAuthStatusUnspecified 表示尚未形成鉴权探测结论。
	RuntimeAuthStatusUnspecified RuntimeAuthStatus = ""
	// RuntimeAuthStatusUnknown 表示 SDK 无法可靠判断鉴权状态。
	RuntimeAuthStatusUnknown RuntimeAuthStatus = "unknown"
	// RuntimeAuthStatusOK 表示鉴权探测没有发现缺失凭据。
	RuntimeAuthStatusOK RuntimeAuthStatus = "ok"
	// RuntimeAuthStatusMissing 表示 ACP Agent 明确报告尚未登录或缺少凭据。
	RuntimeAuthStatusMissing RuntimeAuthStatus = "missing"
)
