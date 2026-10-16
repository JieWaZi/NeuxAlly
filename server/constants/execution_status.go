package constants

//go:generate toolx gen enum ExecutionStatus

// ExecutionStatus 表示一次执行当前所处的生命周期状态。
type ExecutionStatus string

const (
	// ExecutionStatusUnspecified 表示数据库或框架产生的空状态。
	ExecutionStatusUnspecified ExecutionStatus = ""
	// ExecutionStatusPending 表示执行已经创建但尚未开始。
	ExecutionStatusPending ExecutionStatus = "pending"
	// ExecutionStatusRunning 表示执行正在产生事件。
	ExecutionStatusRunning ExecutionStatus = "running"
	// ExecutionStatusCompleted 表示执行已经成功结束。
	ExecutionStatusCompleted ExecutionStatus = "completed"
	// ExecutionStatusFailed 表示执行因内部错误结束。
	ExecutionStatusFailed ExecutionStatus = "failed"
	// ExecutionStatusCanceled 表示执行因上游取消结束。
	ExecutionStatusCanceled ExecutionStatus = "canceled"
)
