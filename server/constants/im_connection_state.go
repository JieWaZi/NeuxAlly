package constants

//go:generate toolx gen enum IMConnectionState

// IMConnectionState 定义 IM 集成的稳定业务值。
type IMConnectionState string

const (
	// IMConnectionStateUnspecified 表示尚无连接状态。
	IMConnectionStateUnspecified IMConnectionState = ""
	// IMConnectionStateDisconnected 表示连接已停用。
	IMConnectionStateDisconnected IMConnectionState = "disconnected"
	// IMConnectionStateConnecting 表示正在建立连接。
	IMConnectionStateConnecting IMConnectionState = "connecting"
	// IMConnectionStateConnected 表示已建立平台连接。
	IMConnectionStateConnected IMConnectionState = "connected"
	// IMConnectionStateLoginRequired 表示需要重新登录。
	IMConnectionStateLoginRequired IMConnectionState = "login_required"
	// IMConnectionStateFailed 表示平台连接失败。
	IMConnectionStateFailed IMConnectionState = "failed"
)
