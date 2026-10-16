package constants

//go:generate toolx gen enum WorkflowRunStatus

// WorkflowRunStatus 表示 Go 编排的持久生命周期。
type WorkflowRunStatus string

const (
	// WorkflowRunStatusQueued 表示已经接纳且等待执行。
	WorkflowRunStatusQueued WorkflowRunStatus = "queued"
	// WorkflowRunStatusRunning 表示至少一个流程正在推进。
	WorkflowRunStatusRunning WorkflowRunStatus = "running"
	// WorkflowRunStatusWaiting 表示流程等待用户或外部结果。
	WorkflowRunStatusWaiting WorkflowRunStatus = "waiting"
	// WorkflowRunStatusRecovering 表示正在恢复未完成的执行检查点。
	WorkflowRunStatusRecovering WorkflowRunStatus = "recovering"
	// WorkflowRunStatusCancelling 表示正在停止外部执行。
	WorkflowRunStatusCancelling WorkflowRunStatus = "cancelling"
	// WorkflowRunStatusSucceeded 表示返回值已经校验并提交。
	WorkflowRunStatusSucceeded WorkflowRunStatus = "succeeded"
	// WorkflowRunStatusFailed 表示流程已经失败并收敛。
	WorkflowRunStatusFailed WorkflowRunStatus = "failed"
	// WorkflowRunStatusCancelled 表示取消已经完成收敛。
	WorkflowRunStatusCancelled WorkflowRunStatus = "cancelled"
)
