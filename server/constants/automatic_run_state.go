package constants

//go:generate toolx gen enum AutomaticRunState

// AutomaticRunState 是接纳记录与首次 Execution 权威状态的展示结果。
type AutomaticRunState string

const (
	// AutomaticRunStateUnspecified 表示没有展示状态。
	AutomaticRunStateUnspecified AutomaticRunState = ""
	// AutomaticRunStatePending 表示等待持久接纳或运行时启动。
	AutomaticRunStatePending AutomaticRunState = "pending"
	// AutomaticRunStateRunning 表示首次执行正在运行。
	AutomaticRunStateRunning AutomaticRunState = "running"
	// AutomaticRunStateWaitingApproval 表示首次执行正在等待操作审批。
	AutomaticRunStateWaitingApproval AutomaticRunState = "waiting_approval"
	// AutomaticRunStateWaitingInput 表示首次执行正在等待用户补充输入。
	AutomaticRunStateWaitingInput AutomaticRunState = "waiting_input"
	// AutomaticRunStateCompleted 表示首次执行已经完成。
	AutomaticRunStateCompleted AutomaticRunState = "completed"
	// AutomaticRunStateFailed 表示接纳或首次执行失败。
	AutomaticRunStateFailed AutomaticRunState = "failed"
	// AutomaticRunStateCanceled 表示首次执行被取消。
	AutomaticRunStateCanceled AutomaticRunState = "canceled"
	// AutomaticRunStateInterrupted 表示未形成 Chat 的接纳在重启时中断。
	AutomaticRunStateInterrupted AutomaticRunState = "interrupted"
	// AutomaticRunStateSkipped 表示计划机会因任务忙碌跳过。
	AutomaticRunStateSkipped AutomaticRunState = "skipped"
)
