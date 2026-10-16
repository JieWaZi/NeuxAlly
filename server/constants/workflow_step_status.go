package constants

//go:generate toolx gen enum WorkflowStepStatus

// WorkflowStepStatus 是持久 fiber、节点、效果与人工请求共用的收敛状态。
type WorkflowStepStatus string

const (
	// WorkflowStepStatusEmpty 表示尚未设置状态，不能用于已接纳的对象。
	WorkflowStepStatusEmpty WorkflowStepStatus = ""
	// WorkflowStepStatusReady 表示可以推进。
	WorkflowStepStatusReady WorkflowStepStatus = "ready"
	// WorkflowStepStatusWaiting 表示等待效果、子任务或用户。
	WorkflowStepStatusWaiting WorkflowStepStatus = "waiting"
	// WorkflowStepStatusPrepared 表示效果已经写入 outbox，尚未收到派发收据。
	WorkflowStepStatusPrepared WorkflowStepStatus = "prepared"
	// WorkflowStepStatusRunning 表示已经形成真实派发收据。
	WorkflowStepStatusRunning WorkflowStepStatus = "running"
	// WorkflowStepStatusSucceeded 表示值已经校验并提交。
	WorkflowStepStatusSucceeded WorkflowStepStatus = "succeeded"
	// WorkflowStepStatusFailed 表示失败已经提交。
	WorkflowStepStatusFailed WorkflowStepStatus = "failed"
	// WorkflowStepStatusCancelled 表示取消已经收敛。
	WorkflowStepStatusCancelled WorkflowStepStatus = "cancelled"
	// WorkflowStepStatusAnswered 表示人工请求被真实用户回答。
	WorkflowStepStatusAnswered WorkflowStepStatus = "answered"
	// WorkflowStepStatusSkipped 表示用户明确跳过。
	WorkflowStepStatusSkipped WorkflowStepStatus = "skipped"
	// WorkflowStepStatusTimedOut 表示请求期限到达。
	WorkflowStepStatusTimedOut WorkflowStepStatus = "timed_out"
	// WorkflowStepStatusUnknown 保存外部效果结果不确定的内部证据，节点展示和运行结果按失败处理。
	WorkflowStepStatusUnknown WorkflowStepStatus = "unknown"
)
