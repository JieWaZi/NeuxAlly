package constants

//go:generate toolx gen enum WorkBoardState

// WorkBoardState 是工作记录多选筛选的当前状态或最近终态。
type WorkBoardState string

const (
	// WorkBoardStateRunning 表示当前正在执行或排队。
	WorkBoardStateRunning WorkBoardState = "running"
	// WorkBoardStateWaiting 表示当前等待输入、审批或验收。
	WorkBoardStateWaiting WorkBoardState = "waiting"
	// WorkBoardStateIdle 表示当前空闲且尚无结束的执行。
	WorkBoardStateIdle WorkBoardState = "idle"
	// WorkBoardStateCompleted 表示当前空闲且最近执行成功。
	WorkBoardStateCompleted WorkBoardState = "completed"
	// WorkBoardStateFailed 表示当前空闲且最近执行失败。
	WorkBoardStateFailed WorkBoardState = "failed"
	// WorkBoardStateCanceled 表示当前空闲且最近执行取消。
	WorkBoardStateCanceled WorkBoardState = "canceled"
)
