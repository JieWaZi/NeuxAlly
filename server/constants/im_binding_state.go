package constants

//go:generate toolx gen enum IMBindingState

// IMBindingState 定义 IM 集成的稳定业务值。
type IMBindingState string

const (
	// IMBindingStateUnspecified 表示尚无授权状态。
	IMBindingStateUnspecified IMBindingState = ""
	// IMBindingStatePending 表示等待桌面批准接入。
	IMBindingStatePending IMBindingState = "pending"
	// IMBindingStateApproved 表示允许聊天全体成员使用执行目标。
	IMBindingStateApproved IMBindingState = "approved"
	// IMBindingStateRejected 表示已拒绝该聊天接入。
	IMBindingStateRejected IMBindingState = "rejected"
)
