package constants

//go:generate toolx gen enum IMInboxState

// IMInboxState 定义 IM 集成的稳定业务值。
type IMInboxState string

const (
	// IMInboxStateUnspecified 表示尚无处理状态。
	IMInboxStateUnspecified IMInboxState = ""
	// IMInboxStatePending 表示消息已持久收取等待路由。
	IMInboxStatePending IMInboxState = "pending"
	// IMInboxStateRouted 表示执行目标及会话已冻结。
	IMInboxStateRouted IMInboxState = "routed"
	// IMInboxStateAccepted 表示内部工作已权威接纳。
	IMInboxStateAccepted IMInboxState = "accepted"
	// IMInboxStateProcessed 表示命令已处理或结果已投递登记。
	IMInboxStateProcessed IMInboxState = "processed"
	// IMInboxStateSkipped 表示因聊天授权撤销而跳过。
	IMInboxStateSkipped IMInboxState = "skipped"
	// IMInboxStateFailed 表示处理失败需要查看。
	IMInboxStateFailed IMInboxState = "failed"
)
