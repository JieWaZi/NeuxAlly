package constants

//go:generate toolx gen enum AccountAuthMode

// AccountAuthMode 表示受管 AI 账号的认证方式。
type AccountAuthMode string

const (
	// AccountAuthModeUnspecified 表示尚未选择认证方式。
	AccountAuthModeUnspecified AccountAuthMode = ""
	// AccountAuthModeAPIKey 使用服务商签发的 API Key。
	AccountAuthModeAPIKey AccountAuthMode = "api-key"
	// AccountAuthModeSubscription 使用官方设备授权取得的订阅凭据。
	AccountAuthModeSubscription AccountAuthMode = "subscription"
)
