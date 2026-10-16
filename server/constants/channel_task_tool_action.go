package constants

//go:generate toolx gen enum ChannelTaskToolAction

// ChannelTaskToolAction 是 Collaboration MCP 单工具允许的封闭任务板动作。
type ChannelTaskToolAction string

const (
	// ChannelTaskToolActionList 表示列出任务板。
	ChannelTaskToolActionList ChannelTaskToolAction = "list"
	// ChannelTaskToolActionSearch 表示检索任务板。
	ChannelTaskToolActionSearch ChannelTaskToolAction = "search"
	// ChannelTaskToolActionGet 表示读取单项任务。
	ChannelTaskToolActionGet ChannelTaskToolAction = "get"
	// ChannelTaskToolActionCreate 表示创建任务。
	ChannelTaskToolActionCreate ChannelTaskToolAction = "create"
	// ChannelTaskToolActionUpdate 表示更新待执行任务。
	ChannelTaskToolActionUpdate ChannelTaskToolAction = "update"
	// ChannelTaskToolActionFollowUp 表示补充执行中任务。
	ChannelTaskToolActionFollowUp ChannelTaskToolAction = "follow_up"
	// ChannelTaskToolActionRetry 表示重试失败任务。
	ChannelTaskToolActionRetry ChannelTaskToolAction = "retry"
	// ChannelTaskToolActionCancel 表示取消未验收任务。
	ChannelTaskToolActionCancel ChannelTaskToolAction = "cancel"
	// ChannelTaskToolActionReport 表示执行成员向负责人提交真实工作报告。
	ChannelTaskToolActionReport ChannelTaskToolAction = "report"
)
