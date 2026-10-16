package constants

//go:generate toolx gen enum ChannelTaskStatus

// ChannelTaskStatus 是团队任务持久化的生命周期状态。
type ChannelTaskStatus string

const (
	// ChannelTaskStatusPending 表示任务已创建但尚未开始执行。
	ChannelTaskStatusPending ChannelTaskStatus = "pending"
	// ChannelTaskStatusInProgress 表示任务当前正在成员 Chat 中执行。
	ChannelTaskStatusInProgress ChannelTaskStatus = "in_progress"
	// ChannelTaskStatusInReview 表示执行结果正在等待用户验收。
	ChannelTaskStatusInReview ChannelTaskStatus = "in_review"
	// ChannelTaskStatusCompleted 表示任务已经完成并获得所需验收。
	ChannelTaskStatusCompleted ChannelTaskStatus = "completed"
	// ChannelTaskStatusFailed 表示任务执行失败且当前没有继续尝试。
	ChannelTaskStatusFailed ChannelTaskStatus = "failed"
	// ChannelTaskStatusCancelled 表示任务已由用户或团队停止操作取消。
	ChannelTaskStatusCancelled ChannelTaskStatus = "cancelled"
)

// Terminal 判断状态是否已经结束当前任务生命周期。
func (status ChannelTaskStatus) Terminal() bool {
	return status == ChannelTaskStatusCompleted ||
		status == ChannelTaskStatusFailed ||
		status == ChannelTaskStatusCancelled
}
